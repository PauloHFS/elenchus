package logging

import (
	"context"
	"log/slog"
	"os"
	"sync"
)

var logger *slog.Logger

type contextKey string

const (
	loggerKey contextKey = "logger"
	eventKey  contextKey = "event"
)

// Event accumulates attributes for a single "wide" log entry.
type Event struct {
	mu    sync.Mutex
	attrs []slog.Attr
}

func (e *Event) Add(attrs ...slog.Attr) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.attrs = append(e.attrs, attrs...)
}

func (e *Event) Attrs() []any {
	e.mu.Lock()
	defer e.mu.Unlock()
	args := make([]any, len(e.attrs))
	for i, attr := range e.attrs {
		args[i] = attr
	}
	return args
}

func Init() {
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})

	version := os.Getenv("VERSION")
	if version == "" {
		version = "dev"
	}

	logger = slog.New(handler).With(
		slog.String("version", version),
		slog.String("service", "goth-api"),
	)

	slog.SetDefault(logger)
}

func Get() *slog.Logger {
	if logger == nil {
		Init()
	}
	return logger
}

func NewEventContext(ctx context.Context) (context.Context, *Event) {
	e := &Event{}
	return context.WithValue(ctx, eventKey, e), e
}

func EventFromContext(ctx context.Context) *Event {
	if e, ok := ctx.Value(eventKey).(*Event); ok {
		return e
	}
	return nil
}

// AddToEvent adds attributes to the event in the context, if it exists.
func AddToEvent(ctx context.Context, attrs ...slog.Attr) {
	if e := EventFromContext(ctx); e != nil {
		e.Add(attrs...)
	}
}
