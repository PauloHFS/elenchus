package worker

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/PauloHFS/goth/internal/config"
	"github.com/PauloHFS/goth/internal/db"
	"github.com/PauloHFS/goth/internal/logging"
	"github.com/PauloHFS/goth/internal/mailer"
	"github.com/PauloHFS/goth/internal/metrics"
	"github.com/PauloHFS/goth/internal/web"
)

type Processor struct {
	db      *sql.DB
	queries *db.Queries
	logger  *slog.Logger
	mailer  *mailer.Mailer
	wg      sync.WaitGroup
}

func New(cfg *config.Config, dbConn *sql.DB, q *db.Queries, l *slog.Logger) *Processor {
	return &Processor{
		db:      dbConn,
		queries: q,
		logger:  l,
		mailer:  mailer.New(cfg),
	}
}

func (p *Processor) Start(ctx context.Context) {
	p.logger.Info("worker started")
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			p.logger.Info("worker signal received: waiting for active jobs to finish")
			return
		case <-ticker.C:
			p.processNext(ctx)
		}
	}
}

// Wait blocks until all active jobs are finished
func (p *Processor) Wait() {
	p.wg.Wait()
}

func (p *Processor) processNext(ctx context.Context) {
	p.wg.Add(1)
	defer p.wg.Done()

	start := time.Now()
	job, err := p.queries.PickNextJob(ctx)
	if err != nil {
		return // Fila vazia
	}

	ctx, event := logging.NewEventContext(ctx)
	event.Add(
		slog.Int64("job_id", int64(job.ID)),
		slog.String("job_type", string(job.Type)),
	)

	// Idempotency Check: Verifica se o job já foi processado com sucesso anteriormente
	processed, err := p.queries.IsJobProcessed(ctx, job.ID)
	if err == nil && processed == 1 {
		p.logger.InfoContext(ctx, "job already processed, skipping", event.Attrs()...)
		_ = p.queries.CompleteJob(ctx, job.ID) // Garante que o status está sincronizado
		return
	}

	var errProcessing error
	switch job.Type {
	case "send_email":
		errProcessing = p.handleSendEmail(ctx, job.Payload)
	case "send_password_reset_email":
		errProcessing = p.handleSendPasswordResetEmail(ctx, job.Payload)
	case "send_verification_email":
		errProcessing = p.handleSendVerificationEmail(ctx, job.Payload)
	case "process_ai":
		errProcessing = p.handleProcessAI(ctx, job.Payload)
	case "process_webhook":
		errProcessing = p.handleProcessWebhook(ctx, job.Payload)
	default:
		p.logger.WarnContext(ctx, "unknown job type", "type", job.Type)
	}

	if errProcessing != nil {
		if err := p.queries.FailJob(ctx, db.FailJobParams{
			LastError: sql.NullString{String: errProcessing.Error(), Valid: true},
			ID:        job.ID,
		}); err != nil {
			p.logger.ErrorContext(ctx, "failed to record job failure in db", "error", err)
		}
		metrics.JobDuration.WithLabelValues(string(job.Type), "failed").Observe(time.Since(start).Seconds())
		p.logger.ErrorContext(ctx, "job processing failed",
			append(event.Attrs(), slog.String("error", errProcessing.Error()))...)
		return
	}

	// Sucesso: Registrar que foi processado e completar o job em uma transação
	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		p.logger.ErrorContext(ctx, "failed to start transaction", "error", err)
		return
	}
	defer func() { _ = tx.Rollback() }()

	qtx := p.queries.WithTx(tx)

	if err := qtx.RecordJobProcessed(ctx, job.ID); err != nil {
		p.logger.ErrorContext(ctx, "failed to record job processed", "error", err)
		return
	}

	if err := qtx.CompleteJob(ctx, job.ID); err != nil {
		p.logger.ErrorContext(ctx, "failed to complete job", "error", err)
		return
	}

	if err := tx.Commit(); err != nil {
		p.logger.ErrorContext(ctx, "failed to commit transaction", "error", err)
		return
	}

	duration := time.Since(start)
	metrics.JobDuration.WithLabelValues(string(job.Type), "success").Observe(duration.Seconds())
	event.Add(slog.Float64("duration_ms", float64(duration.Nanoseconds())/1e6))

	p.logger.InfoContext(ctx, "job completed", event.Attrs()...)

	// Notificação em tempo real via SSE
	if job.TenantID.Valid {
		// Tenta converter tenant_id para userID se for um número
		var userID int64
		if _, err := fmt.Sscanf(job.TenantID.String, "%d", &userID); err != nil {
			p.logger.DebugContext(ctx, "tenant_id is not a numeric userID, skipping broadcast", "tenant_id", job.TenantID.String)
		} else if userID > 0 {
			web.BroadcastToUser(userID, "job_completed", string(job.Type))
			return
		}
	}

	web.Broadcast("job_completed", string(job.Type))
}

func (p *Processor) handleSendEmail(ctx context.Context, payload json.RawMessage) error {
	var data struct {
		To      string `json:"to"`
		Subject string `json:"subject"`
		Body    string `json:"body"`
	}

	if err := json.Unmarshal(payload, &data); err != nil {
		return err
	}

	return p.mailer.Send(data.To, data.Subject, data.Body)
}

func (p *Processor) handleSendVerificationEmail(ctx context.Context, payload json.RawMessage) error {
	var data struct {
		Email string `json:"email"`
		Token string `json:"token"`
	}

	if err := json.Unmarshal(payload, &data); err != nil {
		return err
	}

	subject := "Verifique seu E-mail"
	body := "Olá,\n\nBem-vindo! Clique no link abaixo para verificar seu e-mail:\n\n" +
		"http://localhost:8080/verify-email?token=" + data.Token

	return p.mailer.Send(data.Email, subject, body)
}

func (p *Processor) handleSendPasswordResetEmail(ctx context.Context, payload json.RawMessage) error {
	var data struct {
		Email string `json:"email"`
		Token string `json:"token"`
	}

	if err := json.Unmarshal(payload, &data); err != nil {
		return err
	}

	subject := "Recuperação de Senha"
	body := "Olá,\n\nClique no link abaixo para redefinir sua senha:\n\n" +
		"http://localhost:8080/reset-password?token=" + data.Token + "\n\n" +
		"Este link expira em 1 hora."

	return p.mailer.Send(data.Email, subject, body)
}

func (p *Processor) handleProcessAI(ctx context.Context, payload json.RawMessage) error {
	var data struct {
		Prompt string `json:"prompt"`
	}

	if err := json.Unmarshal(payload, &data); err != nil {
		return err
	}

	p.logger.InfoContext(ctx, "AI processing started", slog.String("prompt", data.Prompt))
	// Simular integração com OpenAI/Anthropic
	time.Sleep(2 * time.Second)

	return nil
}

func (p *Processor) handleProcessWebhook(ctx context.Context, payload json.RawMessage) error {
	var data struct {
		WebhookID int64 `json:"webhook_id"`
	}

	if err := json.Unmarshal(payload, &data); err != nil {
		return err
	}

	p.logger.InfoContext(ctx, "processing webhook event", slog.Int64("webhook_id", data.WebhookID))

	// Aqui você buscaria o payload bruto no banco se necessário
	return nil
}
