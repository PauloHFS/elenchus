package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// HTTP Metrics
	HttpRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "Total number of HTTP requests",
	}, []string{"path", "method", "status"})

	// Job Metrics
	JobDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "job_processing_seconds",
		Help:    "Time taken to process jobs",
		Buckets: []float64{0.1, 0.5, 1, 5, 10, 30, 60, 120, 300},
	}, []string{"type", "status"})

	JobsProcessed = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "jobs_processed_total",
		Help: "Total number of jobs processed",
	}, []string{"type", "status"})

	JobsActive = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "jobs_active",
		Help: "Number of active jobs currently being processed",
	}, []string{"type"})

	JobRetries = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "job_retries_total",
		Help: "Total number of job retries",
	}, []string{"type"})

	JobsDeadLetter = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "jobs_dead_letter_total",
		Help: "Total number of jobs moved to dead letter queue",
	}, []string{"type"})

	// Gemini API Metrics
	GeminiAPILatency = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "gemini_api_latency_seconds",
		Help:    "Gemini API call latency",
		Buckets: []float64{0.5, 1, 2, 5, 10, 30, 60, 120},
	}, []string{"operation", "status"})

	GeminiAPIRequests = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "gemini_api_requests_total",
		Help: "Total number of Gemini API requests",
	}, []string{"operation", "status"})

	GeminiRateLimitRemaining = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "gemini_rate_limit_remaining",
		Help: "Remaining Gemini API requests in current window",
	})

	// SSE Metrics
	SSEConnections = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "sse_connections_active",
		Help: "Number of active SSE connections",
	})

	SSEEventsSent = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "sse_events_sent_total",
		Help: "Total number of SSE events sent",
	}, []string{"type"})
)
