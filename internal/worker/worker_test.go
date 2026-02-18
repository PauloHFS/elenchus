package worker

import (
	"io"
	"log/slog"
	"testing"

	"github.com/PauloHFS/elenchus/internal/config"
)

func TestProcessor_New(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	cfg := &config.Config{SMTPHost: "localhost", SMTPPort: "1025"}

	t.Run("ProcessorInitialization", func(t *testing.T) {
		p := New(cfg, nil, nil, logger)
		if p == nil {
			t.Fatal("expected processor, got nil")
		}
		if p.logger != logger {
			t.Error("logger not correctly assigned")
		}
	})
}
