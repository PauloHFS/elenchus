package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	HttpRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "Total number of HTTP requests",
	}, []string{"path", "method", "status"})

	JobDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name: "job_processing_seconds",
		Help: "Time taken to process jobs",
	}, []string{"type", "status"})
)
