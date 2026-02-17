package worker

import (
	"context"
	"log/slog"
)

// RescueZombies resgata jobs que ficaram presos no status 'processing'
// devido a um crash ou restart inesperado do servidor.
func (p *Processor) RescueZombies(ctx context.Context) error {
	p.logger.Info("zombie hunter: searching for stuck jobs")
	err := p.queries.RescueZombies(ctx)
	if err != nil {
		p.logger.Error("zombie hunter: failed to rescue jobs", slog.String("error", err.Error()))
		return err
	}
	return nil
}
