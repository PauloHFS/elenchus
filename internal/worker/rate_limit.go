package worker

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	"github.com/PauloHFS/elenchus/internal/db"
	"github.com/PauloHFS/elenchus/internal/logging"
	"github.com/PauloHFS/elenchus/internal/metrics"
)

// processNextWithRateLimit processes next job with rate limiting
func (p *Processor) processNextWithRateLimit(ctx context.Context) {
	job, err := p.queries.PickNextJob(ctx)
	if err != nil {
		return // Fila vazia
	}

	ctx, event := logging.NewEventContext(ctx)
	event.Add(
		slog.Int64("job_id", int64(job.ID)),
		slog.String("job_type", string(job.Type)),
	)

	// Idempotency Check
	processed, err := p.queries.IsJobProcessed(ctx, job.ID)
	if err == nil && processed == 1 {
		p.logger.InfoContext(ctx, "job already processed, skipping", event.Attrs()...)
		_ = p.queries.CompleteJob(ctx, job.ID)
		return
	}

	// Get appropriate semaphore for job type
	semaphore := p.getSemaphoreForJob(job.Type)

	// Try to acquire semaphore (non-blocking)
	select {
	case semaphore <- struct{}{}:
		// Acquired, process job in goroutine
		p.wg.Add(1)
		go func() {
			defer p.wg.Done()
			defer func() { <-semaphore }() // Release semaphore
			p.processJobWithMetrics(ctx, job, event)
		}()
	default:
		// Semaphore full, skip this job (will be processed next cycle)
		p.logger.DebugContext(ctx, "rate limit reached, skipping job", 
			append(event.Attrs(), 
				slog.String("reason", "concurrent limit reached"),
			)...)
	}
}

// getSemaphoreForJob returns the appropriate semaphore for a job type
func (p *Processor) getSemaphoreForJob(jobType string) chan struct{} {
	switch jobType {
	case "run_evaluation", "process_ai":
		return p.geminiSemaphore
	case "send_email", "send_password_reset_email", "send_verification_email":
		return p.emailSemaphore
	default:
		return p.genericSemaphore
	}
}

// processJobWithMetrics processes a single job with metrics and dead letter queue
func (p *Processor) processJobWithMetrics(ctx context.Context, job db.Job, event *logging.Event) {
	start := time.Now()
	
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
		errProcessing = fmt.Errorf("unknown job type: %s", job.Type)
	}

	// Record metrics
	duration := time.Since(start).Seconds()
	status := getJobStatus(errProcessing)
	metrics.JobDuration.WithLabelValues(string(job.Type), status).Observe(duration)
	metrics.JobsProcessed.WithLabelValues(string(job.Type), status).Inc()

	if errProcessing != nil {
		// Record retry metric
		attemptCount := int64(0)
		if job.AttemptCount.Valid {
			attemptCount = job.AttemptCount.Int64
		}
		
		if attemptCount > 0 {
			metrics.JobRetries.WithLabelValues(string(job.Type)).Inc()
		}
		
		// Check if we should move to dead letter queue
		shouldMoveToDLQ := p.shouldMoveToDeadLetterQueue(ctx, job)
		
		if shouldMoveToDLQ {
			p.moveToDeadLetterQueue(ctx, job, errProcessing)
			p.logger.ErrorContext(ctx, "job moved to dead letter queue after max retries",
				append(event.Attrs(), 
					slog.String("error", errProcessing.Error()),
					slog.Int64("attempts", attemptCount),
				)...)
		} else {
			// Record failure in DB for retry
			if err := p.queries.FailJob(ctx, db.FailJobParams{
				LastError: sql.NullString{String: errProcessing.Error(), Valid: true},
				ID:        job.ID,
			}); err != nil {
				p.logger.ErrorContext(ctx, "failed to record job failure in db", "error", err)
			}
			
			p.logger.ErrorContext(ctx, "job processing failed, will retry",
				append(event.Attrs(), 
					slog.String("error", errProcessing.Error()),
					slog.Int64("attempts", attemptCount),
				)...)
		}
		return
	}

	// Success: Record processing and complete job in transaction
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

	event.Add(slog.Float64("duration_ms", float64(duration)*1000))
	p.logger.InfoContext(ctx, "job completed successfully", event.Attrs()...)
	// Note: SSE events are sent via broker.SendEvaluationProgress/Complete
}

// shouldMoveToDeadLetterQueue determines if a job should be moved to DLQ
func (p *Processor) shouldMoveToDeadLetterQueue(ctx context.Context, job db.Job) bool {
	const maxAttempts = 5
	
	attemptCount := int64(0)
	if job.AttemptCount.Valid {
		attemptCount = job.AttemptCount.Int64
	}
	
	// Move to DLQ if:
	// 1. Max attempts reached
	// 2. Job is old (more than 24 hours)
	
	if attemptCount >= maxAttempts {
		return true
	}
	
	if !job.CreatedAt.Valid || time.Since(job.CreatedAt.Time) > 24*time.Hour {
		return true
	}
	
	return false
}

// moveToDeadLetterQueue moves a job to dead letter queue
func (p *Processor) moveToDeadLetterQueue(ctx context.Context, job db.Job, lastErr error) {
	// Record metric
	metrics.JobsDeadLetter.WithLabelValues(string(job.Type)).Inc()
	
	p.logger.ErrorContext(ctx, "dead letter queue: job moved",
		slog.Int64("original_job_id", job.ID),
		slog.String("original_job_type", string(job.Type)),
		slog.String("error", lastErr.Error()),
	)
	
	// Mark original job as failed permanently
	_ = p.queries.FailJob(ctx, db.FailJobParams{
		LastError: sql.NullString{
			String: fmt.Sprintf("MOVED_TO_DLQ: %v", lastErr),
			Valid:  true,
		},
		ID: job.ID,
	})
}

// getJobStatus returns "success" or "failed" based on error
func getJobStatus(err error) string {
	if err == nil {
		return "success"
	}
	return "failed"
}
