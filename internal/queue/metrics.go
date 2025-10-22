package queue

import (
	"sync/atomic"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// QueueMetrics holds Prometheus metrics for queue operations
type QueueMetrics struct {
	// Counters
	Enqueued      prometheus.Counter
	Completed     prometheus.Counter
	Retries       prometheus.Counter
	EnqueueErrors prometheus.Counter
	DequeueErrors prometheus.Counter
	DLQMoved      prometheus.Counter

	// Gauges
	QueueSize  *int64 // Atomic counter
	Processing *int64 // Atomic counter

	// Prometheus gauges
	QueueSizeGauge  prometheus.Gauge
	ProcessingGauge prometheus.Gauge
}

// NewQueueMetrics creates a new metrics instance
// instanceID is used to make metrics unique in tests (can be empty in production)
func NewQueueMetrics(queueType string, instanceID ...string) *QueueMetrics {
	var queueSize, processing int64

	// Build const labels
	labels := prometheus.Labels{"queue_type": queueType}
	if len(instanceID) > 0 && instanceID[0] != "" {
		labels["instance"] = instanceID[0]
	}

	m := &QueueMetrics{
		QueueSize:  &queueSize,
		Processing: &processing,

		Enqueued: promauto.NewCounter(prometheus.CounterOpts{
			Name:        "deletion_queue_enqueued_total",
			Help:        "Total number of messages enqueued",
			ConstLabels: labels,
		}),

		Completed: promauto.NewCounter(prometheus.CounterOpts{
			Name:        "deletion_queue_completed_total",
			Help:        "Total number of messages completed successfully",
			ConstLabels: labels,
		}),

		Retries: promauto.NewCounter(prometheus.CounterOpts{
			Name:        "deletion_queue_retries_total",
			Help:        "Total number of message retries",
			ConstLabels: labels,
		}),

		EnqueueErrors: promauto.NewCounter(prometheus.CounterOpts{
			Name:        "deletion_queue_enqueue_errors_total",
			Help:        "Total number of enqueue errors",
			ConstLabels: labels,
		}),

		DequeueErrors: promauto.NewCounter(prometheus.CounterOpts{
			Name:        "deletion_queue_dequeue_errors_total",
			Help:        "Total number of dequeue errors",
			ConstLabels: labels,
		}),

		DLQMoved: promauto.NewCounter(prometheus.CounterOpts{
			Name:        "deletion_queue_dlq_moved_total",
			Help:        "Total number of messages moved to DLQ",
			ConstLabels: labels,
		}),

		QueueSizeGauge: promauto.NewGauge(prometheus.GaugeOpts{
			Name:        "deletion_queue_size",
			Help:        "Current number of messages in queue",
			ConstLabels: labels,
		}),

		ProcessingGauge: promauto.NewGauge(prometheus.GaugeOpts{
			Name:        "deletion_queue_processing",
			Help:        "Current number of messages being processed",
			ConstLabels: labels,
		}),
	}

	return m
}

// UpdateGauges updates Prometheus gauges from atomic counters
func (m *QueueMetrics) UpdateGauges() {
	m.QueueSizeGauge.Set(float64(atomic.LoadInt64(m.QueueSize)))
	m.ProcessingGauge.Set(float64(atomic.LoadInt64(m.Processing)))
}

// IncrQueueSize atomically increments queue size
func (m *QueueMetrics) IncrQueueSize() {
	atomic.AddInt64(m.QueueSize, 1)
	m.QueueSizeGauge.Inc()
}

// DecrQueueSize atomically decrements queue size
func (m *QueueMetrics) DecrQueueSize() {
	atomic.AddInt64(m.QueueSize, -1)
	m.QueueSizeGauge.Dec()
}

// IncrProcessing atomically increments processing count
func (m *QueueMetrics) IncrProcessing() {
	atomic.AddInt64(m.Processing, 1)
	m.ProcessingGauge.Inc()
}

// DecrProcessing atomically decrements processing count
func (m *QueueMetrics) DecrProcessing() {
	atomic.AddInt64(m.Processing, -1)
	m.ProcessingGauge.Dec()
}
