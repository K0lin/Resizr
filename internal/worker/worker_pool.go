package worker

import (
	"context"
	"fmt"
	"sync"
	"time"

	"resizr/internal/queue"
	"resizr/internal/storage"
	"resizr/pkg/logger"

	"go.uber.org/zap"
)

// WorkerPool manages a pool of deletion workers
type WorkerPool struct {
	workers       []*DeletionWorker
	queue         queue.DeletionQueue
	storage       storage.ImageStorage
	intent        IntentManager
	config        *WorkerPoolConfig
	metrics       *WorkerMetrics
	cancelFunc    context.CancelFunc
	wg            sync.WaitGroup
	shutdownMutex sync.Mutex
	isShutdown    bool
}

// WorkerPoolConfig holds configuration for the worker pool
type WorkerPoolConfig struct {
	// Number of concurrent workers
	WorkerCount int

	// Worker configuration
	WorkerConfig WorkerConfig

	// Shutdown timeout
	ShutdownTimeout time.Duration

	// Health check interval
	HealthCheckInterval time.Duration
}

// NewWorkerPool creates a new worker pool
func NewWorkerPool(
	queue queue.DeletionQueue,
	storage storage.ImageStorage,
	intent IntentManager,
	config *WorkerPoolConfig,
) *WorkerPool {
	metrics := NewWorkerMetrics()

	pool := &WorkerPool{
		workers: make([]*DeletionWorker, 0, config.WorkerCount),
		queue:   queue,
		storage: storage,
		intent:  intent,
		config:  config,
		metrics: metrics,
	}

	// Create workers
	for i := 0; i < config.WorkerCount; i++ {
		workerID := fmt.Sprintf("worker-%d", i+1)
		workerConfig := config.WorkerConfig
		workerConfig.WorkerID = workerID

		worker := NewDeletionWorker(
			workerID,
			queue,
			storage,
			intent,
			&workerConfig,
			metrics,
		)

		pool.workers = append(pool.workers, worker)
	}

	return pool
}

// Start starts all workers in the pool
func (p *WorkerPool) Start(ctx context.Context) error {
	p.shutdownMutex.Lock()
	if p.isShutdown {
		p.shutdownMutex.Unlock()
		return fmt.Errorf("worker pool has been shutdown")
	}
	p.shutdownMutex.Unlock()

	// Create cancellable context
	workerCtx, cancel := context.WithCancel(ctx)
	p.cancelFunc = cancel

	logger.Info("Starting worker pool",
		zap.Int("worker_count", len(p.workers)))

	// Start each worker in its own goroutine
	for _, worker := range p.workers {
		p.wg.Add(1)
		w := worker // Capture loop variable

		go func() {
			defer p.wg.Done()

			if err := w.Start(workerCtx); err != nil {
				logger.Error("Worker stopped with error",
					zap.String("worker_id", w.GetID()),
					zap.Error(err))
			}
		}()
	}

	// Start health check goroutine
	if p.config.HealthCheckInterval > 0 {
		p.wg.Add(1)
		go func() {
			defer p.wg.Done()
			p.runHealthChecks(workerCtx)
		}()
	}

	// Start metrics updater
	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		p.updateMetrics(workerCtx)
	}()

	logger.Info("Worker pool started successfully",
		zap.Int("active_workers", len(p.workers)))

	return nil
}

// Shutdown gracefully shuts down all workers
func (p *WorkerPool) Shutdown(timeout time.Duration) error {
	p.shutdownMutex.Lock()
	if p.isShutdown {
		p.shutdownMutex.Unlock()
		return nil
	}
	p.isShutdown = true
	p.shutdownMutex.Unlock()

	logger.Info("Shutting down worker pool",
		zap.Int("worker_count", len(p.workers)),
		zap.Duration("timeout", timeout))

	// Cancel context to signal workers to stop
	if p.cancelFunc != nil {
		p.cancelFunc()
	}

	// Wait for workers to finish with timeout
	done := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		logger.Info("Worker pool shut down gracefully")
		return nil

	case <-time.After(timeout):
		logger.Warn("Worker pool shutdown timeout exceeded, forcing shutdown",
			zap.Duration("timeout", timeout))
		return fmt.Errorf("shutdown timeout exceeded")
	}
}

// runHealthChecks periodically checks worker health
func (p *WorkerPool) runHealthChecks(ctx context.Context) {
	ticker := time.NewTicker(p.config.HealthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return

		case <-ticker.C:
			p.performHealthCheck(ctx)
		}
	}
}

// performHealthCheck checks health of all workers
func (p *WorkerPool) performHealthCheck(ctx context.Context) {
	healthyCount := 0
	unhealthyCount := 0

	for _, worker := range p.workers {
		if err := worker.Health(ctx); err != nil {
			unhealthyCount++
			logger.Warn("Worker health check failed",
				zap.String("worker_id", worker.GetID()),
				zap.Error(err))
		} else {
			healthyCount++
		}
	}

	logger.Debug("Worker pool health check completed",
		zap.Int("healthy", healthyCount),
		zap.Int("unhealthy", unhealthyCount),
		zap.Int("total", len(p.workers)))

	// Alert if more than 50% of workers are unhealthy
	if unhealthyCount > len(p.workers)/2 {
		logger.Error("More than 50% of workers are unhealthy",
			zap.Int("unhealthy", unhealthyCount),
			zap.Int("total", len(p.workers)))
	}
}

// updateMetrics periodically updates queue metrics
func (p *WorkerPool) updateMetrics(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return

		case <-ticker.C:
			// Get queue stats
			stats, err := p.queue.GetStats(ctx)
			if err != nil {
				logger.Warn("Failed to get queue stats for metrics",
					zap.Error(err))
				continue
			}

			// Update metrics
			p.metrics.QueueDepth.Set(float64(stats.QueueSize))
		}
	}
}

// GetStats returns worker pool statistics
func (p *WorkerPool) GetStats(ctx context.Context) (*WorkerPoolStats, error) {
	// Get queue stats
	queueStats, err := p.queue.GetStats(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get queue stats: %w", err)
	}

	// Check worker health
	healthyWorkers := 0
	for _, worker := range p.workers {
		if err := worker.Health(ctx); err == nil {
			healthyWorkers++
		}
	}

	return &WorkerPoolStats{
		TotalWorkers:    len(p.workers),
		HealthyWorkers:  healthyWorkers,
		QueueSize:       queueStats.QueueSize,
		ProcessingCount: queueStats.Processing,
		DLQSize:         queueStats.DLQSize,
		OldestMessage:   queueStats.OldestMessageAt,
	}, nil
}

// WorkerPoolStats holds statistics about the worker pool
type WorkerPoolStats struct {
	TotalWorkers    int       `json:"total_workers"`
	HealthyWorkers  int       `json:"healthy_workers"`
	QueueSize       int64     `json:"queue_size"`
	ProcessingCount int64     `json:"processing_count"`
	DLQSize         int64     `json:"dlq_size"`
	OldestMessage   time.Time `json:"oldest_message,omitempty"`
}

// Health checks the health of the worker pool
func (p *WorkerPool) Health(ctx context.Context) error {
	// Check if shutdown
	p.shutdownMutex.Lock()
	isShutdown := p.isShutdown
	p.shutdownMutex.Unlock()

	if isShutdown {
		return fmt.Errorf("worker pool is shut down")
	}

	// Check queue health
	if err := p.queue.Health(ctx); err != nil {
		return fmt.Errorf("queue unhealthy: %w", err)
	}

	// Check intent manager health
	if err := p.intent.Health(ctx); err != nil {
		return fmt.Errorf("intent manager unhealthy: %w", err)
	}

	// Check worker health
	healthyWorkers := 0
	for _, worker := range p.workers {
		if err := worker.Health(ctx); err == nil {
			healthyWorkers++
		}
	}

	// Require at least one healthy worker
	if healthyWorkers == 0 {
		return fmt.Errorf("no healthy workers available")
	}

	return nil
}
