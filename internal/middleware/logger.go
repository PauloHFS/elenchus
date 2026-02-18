package middleware

import (
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/PauloHFS/elenchus/internal/logging"
	"github.com/PauloHFS/elenchus/internal/metrics"
	"github.com/google/uuid"
)

type responseWriter struct {
	http.ResponseWriter
	status      int
	size        int
	wroteHeader bool
}

func (rw *responseWriter) Flush() {
	if f, ok := rw.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (rw *responseWriter) WriteHeader(code int) {
	if rw.wroteHeader {
		return
	}
	rw.status = code
	rw.wroteHeader = true
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	if !rw.wroteHeader {
		rw.WriteHeader(http.StatusOK)
	}
	n, err := rw.ResponseWriter.Write(b)
	rw.size += n
	return n, err
}

func isDevelopment() bool {
	env := os.Getenv("ENV")
	if env == "" {
		env = os.Getenv("APP_ENV")
	}
	return env == "development" || env == "dev"
}

func shouldLogRequest(status int) bool {
	if isDevelopment() {
		return true
	}

	switch status {
	case http.StatusTooManyRequests, http.StatusForbidden:
		return false
	default:
		return true
	}
}

func Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		requestID := r.Header.Get("X-Request-ID")
		if requestID == "" {
			requestID = uuid.New().String()
		}
		w.Header().Set("X-Request-ID", requestID)

		ctx, event := logging.NewEventContext(r.Context())

		event.Add(
			slog.String("request_id", requestID),
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.String("query", r.URL.RawQuery),
			slog.String("remote_addr", r.RemoteAddr),
			slog.String("user_agent", r.UserAgent()),
			slog.String("referer", r.Header.Get("Referer")),
			slog.String("content_type", r.Header.Get("Content-Type")),
		)

		rw := &responseWriter{ResponseWriter: w, status: http.StatusOK}

		next.ServeHTTP(rw, r.WithContext(ctx))

		duration := time.Since(start)

		event.Add(
			slog.Int("status", rw.status),
			slog.Int("size", rw.size),
			duration_ms(duration),
		)

		metrics.HttpRequestsTotal.WithLabelValues(r.URL.Path, r.Method, strconv.Itoa(rw.status)).Inc()

		if !shouldLogRequest(rw.status) {
			return
		}

		level := slog.LevelInfo
		switch {
		case rw.status >= 500:
			level = slog.LevelError
		case rw.status >= 400:
			level = slog.LevelWarn
		}

		logging.Get().Log(ctx, level, "request completed", event.Attrs()...)
	})
}

func duration_ms(d time.Duration) slog.Attr {
	return slog.Float64("duration_ms", float64(d.Nanoseconds())/1e6)
}
