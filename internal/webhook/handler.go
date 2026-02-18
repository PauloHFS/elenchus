package webhook

import (
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/PauloHFS/elenchus/internal/db"
	"github.com/PauloHFS/elenchus/internal/validator"
)

type webhookInput struct {
	ExternalID string `json:"id" validate:"required"`
}

type Handler struct {
	queries *db.Queries
}

func NewHandler(q *db.Queries) *Handler {
	return &Handler{queries: q}
}

// ServeHTTP handles incoming webhooks
// @Summary Receber Webhook
// @Description Recebe um payload de webhook, persiste no banco e enfileira um job.
// @Tags webhooks
// @Accept json
// @Produce json
// @Param source path string true "Fonte do webhook (ex: stripe)"
// @Param payload body webhookInput true "Payload do webhook"
// @Success 200 {string} string "OK"
// @Failure 400 {string} string "Bad Request"
// @Router /webhooks/{source} [post]
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	source := r.PathValue("source")
	if source == "" {
		http.Error(w, "source required", http.StatusBadRequest)
		return
	}

	payload, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read body", http.StatusInternalServerError)
		return
	}

	var input webhookInput
	if err := json.Unmarshal(payload, &input); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	if err := validator.Validate(input); err != nil {
		http.Error(w, "validation failed", http.StatusUnprocessableEntity)
		return
	}

	headers, _ := json.Marshal(r.Header)

	// 1. Persistir o webhook bruto para auditoria
	webhook, err := h.queries.CreateWebhook(r.Context(), db.CreateWebhookParams{
		Source:     source,
		ExternalID: sql.NullString{String: input.ExternalID, Valid: true},
		Payload:    payload,
		Headers:    headers,
	})
	if err != nil {
		http.Error(w, "failed to store webhook", http.StatusInternalServerError)
		return
	}

	// 2. Enfileirar Job para processamento assíncrono
	jobPayload, _ := json.Marshal(map[string]int64{"webhook_id": webhook.ID})
	_, err = h.queries.CreateJob(r.Context(), db.CreateJobParams{
		TenantID: sql.NullString{String: "default", Valid: true},
		Type:     "process_webhook",
		Payload:  jobPayload,
		RunAt:    sql.NullTime{Time: time.Now(), Valid: true},
	})

	if err != nil {
		http.Error(w, "failed to enqueue job", http.StatusInternalServerError)
		return
	}

	// 3. Responder rápido (200 OK)
	w.WriteHeader(http.StatusOK)
}
