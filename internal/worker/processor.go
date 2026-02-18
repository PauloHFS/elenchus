package worker

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/PauloHFS/elenchus/internal/config"
	"github.com/PauloHFS/elenchus/internal/db"
	"github.com/PauloHFS/elenchus/internal/logging"
	"github.com/PauloHFS/elenchus/internal/mailer"
	"github.com/PauloHFS/elenchus/internal/metrics"
	"github.com/PauloHFS/elenchus/internal/service"
	"github.com/PauloHFS/elenchus/internal/sse"
)

// Rate limit configuration
const (
	MaxConcurrentGeminiJobs = 5  // Gemini free tier: 15 RPM, usamos 5 para segurança
	MaxConcurrentEmailJobs  = 10 // SMTP geralmente aguenta mais
	MaxConcurrentGenericJobs = 20
)

type Processor struct {
	db            *sql.DB
	queries       *db.Queries
	logger        *slog.Logger
	mailer        *mailer.Mailer
	broker        *sse.Broker
	wg            sync.WaitGroup
	
	// Semaphores for rate limiting
	geminiSemaphore   chan struct{}
	emailSemaphore    chan struct{}
	genericSemaphore  chan struct{}
}

func New(cfg *config.Config, dbConn *sql.DB, q *db.Queries, l *slog.Logger, broker *sse.Broker) *Processor {
	p := &Processor{
		db:      dbConn,
		queries: q,
		logger:  l,
		mailer:  mailer.New(cfg),
		broker:  broker,
		
		// Initialize semaphores
		geminiSemaphore:   make(chan struct{}, MaxConcurrentGeminiJobs),
		emailSemaphore:    make(chan struct{}, MaxConcurrentEmailJobs),
		genericSemaphore:  make(chan struct{}, MaxConcurrentGenericJobs),
	}
	
	return p
}

func (p *Processor) Start(ctx context.Context) {
	p.logger.Info("worker started")
	
	// Processa jobs normais
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	
	// Processa retries de avaliações a cada 30 segundos
	retryTicker := time.NewTicker(30 * time.Second)
	defer retryTicker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			p.logger.Info("worker signal received: waiting for active jobs to finish")
			return
		case <-ticker.C:
			p.processNextWithRateLimit(ctx)
		case <-retryTicker.C:
			p.processEvaluationRetries(ctx)
		}
	}
}

// Wait blocks until all active jobs are finished
func (p *Processor) Wait() {
	p.wg.Wait()
}

// processEvaluationRetries processa avaliações que estavam em retry por rate limit
func (p *Processor) processEvaluationRetries(ctx context.Context) {
	p.logger.Info("checking for evaluations to retry")

	// Cria service para buscar avaliações prontas para retry
	evalService, err := service.NewEvaluationService(p.queries, p.broker)
	if err != nil {
		p.logger.Error("failed to create evaluation service for retry check", "error", err)
		return
	}

	// Busca avaliações prontas para retry
	evaluations, err := evalService.GetEvaluationsToRetry(ctx)
	if err != nil {
		p.logger.Error("failed to get evaluations to retry", "error", err)
		return
	}

	if len(evaluations) == 0 {
		return
	}

	p.logger.Info("found evaluations to retry", "count", len(evaluations))

	// Re-enfileira cada avaliação para processamento
	for _, eval := range evaluations {
		// Cria novo job para retry
		jobPayload, _ := json.Marshal(map[string]interface{}{
			"evaluation_id": eval.ID,
			"tenant_id":     eval.TenantID,
			"user_id":       eval.UserID,
			"prompt":        eval.PromptBase,
			"is_retry":      true,
		})

		_, err := p.queries.CreateJob(ctx, db.CreateJobParams{
			TenantID: sql.NullString{String: eval.TenantID, Valid: true},
			Type:     "run_evaluation",
			Payload:  jobPayload,
			RunAt:    sql.NullTime{Time: time.Now(), Valid: true},
		})
		if err != nil {
			p.logger.Error("failed to create retry job", "evaluation_id", eval.ID, "error", err)
			continue
		}

		p.logger.Info("re-queued evaluation for retry", "evaluation_id", eval.ID)
	}
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
	case "run_evaluation":
		errProcessing = p.handleRunEvaluation(ctx, job.Payload)
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
	// Note: SSE events for evaluations are sent via broker.SendEvaluationProgress/Complete
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

func (p *Processor) handleRunEvaluation(ctx context.Context, payload json.RawMessage) error {
	var data struct {
		EvaluationID string `json:"evaluation_id"`
		TenantID     string `json:"tenant_id"`
		UserID       int64  `json:"user_id"`
		Prompt       string `json:"prompt"`
		IsRetry      bool   `json:"is_retry"`
	}

	if err := json.Unmarshal(payload, &data); err != nil {
		return fmt.Errorf("failed to unmarshal evaluation payload: %w", err)
	}

	p.logger.InfoContext(ctx, "starting evaluation protocol",
		slog.String("evaluation_id", data.EvaluationID),
		slog.Int64("user_id", data.UserID),
		slog.Bool("is_retry", data.IsRetry))

	// Criar serviço de avaliação e executar protocolo
	evalService, err := service.NewEvaluationService(p.queries, p.broker)
	if err != nil {
		return fmt.Errorf("failed to create evaluation service: %w", err)
	}

	// Executar o protocolo de estresse
	if err := evalService.RunEvaluationProtocol(ctx, data.EvaluationID, data.Prompt); err != nil {
		// Verifica se é erro de rate limit - não marca como falha, apenas retorna para retry
		if errors.Is(err, service.ErrRateLimitExceeded) {
			p.logger.InfoContext(ctx, "evaluation hit rate limit, will retry later",
				slog.String("evaluation_id", data.EvaluationID),
				slog.String("error", err.Error()))
			// Não retorna erro aqui - o job será completado e o retry é agendado via checkpoint
			return nil
		}

		// Verifica se é erro de too many retries
		if errors.Is(err, service.ErrTooManyRetries) {
			// Atualizar status para falha após muitas tentativas
			if updateErr := p.queries.UpdateEvaluationStatus(ctx, db.UpdateEvaluationStatusParams{
				Status: "failed",
				ID:     data.EvaluationID,
			}); updateErr != nil {
				p.logger.ErrorContext(ctx, "failed to update evaluation status to failed after max retries", slog.Any("error", updateErr))
			}
			return fmt.Errorf("evaluation failed after max retries: %w", err)
		}

		// Atualizar status para falha
		if updateErr := p.queries.UpdateEvaluationStatus(ctx, db.UpdateEvaluationStatusParams{
			Status: "failed",
			ID:     data.EvaluationID,
		}); updateErr != nil {
			p.logger.ErrorContext(ctx, "failed to update evaluation status to failed", slog.Any("error", updateErr))
		}
		return fmt.Errorf("evaluation protocol failed: %w", err)
	}

	p.logger.InfoContext(ctx, "evaluation protocol completed successfully",
		slog.String("evaluation_id", data.EvaluationID))

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
