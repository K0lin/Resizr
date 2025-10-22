package worker

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// WorkerMetrics holds Prometheus metrics for deletion workers
type WorkerMetrics struct {
	// Counters
	DeletionCompleted prometheus.CounterVec
	DeletionErrors    prometheus.CounterVec
	DeletionRetries   prometheus.CounterVec

	// Histograms
	DeletionDuration prometheus.HistogramVec
	QueueWaitTime    prometheus.Histogram

	// Gauges
	ActiveWorkers prometheus.Gauge
	QueueDepth    prometheus.Gauge
}

// NewWorkerMetrics creates a new worker metrics instance
func NewWorkerMetrics() *WorkerMetrics {
	return &WorkerMetrics{
		DeletionCompleted: *promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "deletion_completed_total",
				Help: "Total number of successful deletions",
			},
			[]string{"resolution"},
		),

		DeletionErrors: *promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "deletion_errors_total",
				Help: "Total number of deletion errors",
			},
			[]string{"resolution", "reason"},
		),

		DeletionRetries: *promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "deletion_retries_total",
				Help: "Total number of deletion retries",
			},
			[]string{"resolution"},
		),

		DeletionDuration: *promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "deletion_duration_seconds",
				Help:    "Time taken to delete a file from storage",
				Buckets: prometheus.ExponentialBuckets(0.1, 2, 10), // 0.1s to ~100s
			},
			[]string{"resolution"},
		),

		QueueWaitTime: promauto.NewHistogram(
			prometheus.HistogramOpts{
				Name:    "deletion_queue_wait_time_seconds",
				Help:    "Time a message spent in queue before processing",
				Buckets: prometheus.ExponentialBuckets(1, 2, 12), // 1s to ~4096s
			},
		),

		ActiveWorkers: promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: "deletion_workers_active",
				Help: "Number of currently active deletion workers",
			},
		),

		QueueDepth: promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: "deletion_queue_depth",
				Help: "Current depth of the deletion queue",
			},
		),
	}
}
