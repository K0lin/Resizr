package worker

import (
	"context"
	"fmt"
	"strings"
	"time"

	"resizr/internal/queue"
	"resizr/internal/storage"
	"resizr/pkg/logger"

	"go.uber.org/zap"
)

// DeletionWorker processes deletion messages from the queue
type DeletionWorker struct {
	id      string
	queue   queue.DeletionQueue
	storage storage.ImageStorage
	intent  IntentManager
	config  *WorkerConfig
	metrics *WorkerMetrics
}

// WorkerConfig holds configuration for deletion workers
type WorkerConfig struct {
	// Worker identification
	WorkerID string

	// Retry configuration
	MaxRetries   int
	RetryBackoff time.Duration

	// Timeout configuration
	DeletionTimeout time.Duration

	// Health check interval
	HealthCheckInterval time.Duration
}

// NewDeletionWorker creates a new deletion worker
func NewDeletionWorker(
	id string,
	queue queue.DeletionQueue,
	storage storage.ImageStorage,
	intent IntentManager,
	config *WorkerConfig,
	metrics *WorkerMetrics,
) *DeletionWorker {
	return &DeletionWorker{
		id:      id,
		queue:   queue,
		storage: storage,
		intent:  intent,
		config:  config,
		metrics: metrics,
	}
}

// Start begins processing messages from the queue
func (w *DeletionWorker) Start(ctx context.Context) error {
	logger.Info("Starting deletion worker",
		zap.String("worker_id", w.id))

	w.metrics.ActiveWorkers.Inc()
	defer w.metrics.ActiveWorkers.Dec()

	// Start consuming messages from queue
	msgChan, err := w.queue.Consume(ctx, w.id)
	if err != nil {
		logger.Error("Failed to start consumer",
			zap.String("worker_id", w.id),
			zap.Error(err))
		return fmt.Errorf("failed to start consumer: %w", err)
	}

	// Process messages until context is cancelled
	for {
		select {
		case <-ctx.Done():
			logger.Info("Stopping deletion worker (context cancelled)",
				zap.String("worker_id", w.id))
			return nil

		case msg, ok := <-msgChan:
			if !ok {
				logger.Info("Stopping deletion worker (channel closed)",
					zap.String("worker_id", w.id))
				return nil
			}

			w.processMessage(ctx, msg)
		}
	}
}

// processMessage handles a single deletion message
func (w *DeletionWorker) processMessage(ctx context.Context, msg *queue.DeletionMessage) {
	startTime := time.Now()

	// Calculate queue wait time
	queueWaitTime := time.Since(msg.EnqueuedAt)
	w.metrics.QueueWaitTime.Observe(queueWaitTime.Seconds())

	logger.Info("Processing deletion message",
		zap.String("worker_id", w.id),
		zap.String("intent_id", msg.IntentID),
		zap.String("image_id", msg.ImageID),
		zap.String("resolution", msg.Resolution),
		zap.String("storage_key", msg.StorageKey),
		zap.Int("retry_count", msg.RetryCount),
		zap.Duration("queue_wait_time", queueWaitTime))

	// Check intent status to prevent duplicate processing
	intent, err := w.intent.GetIntent(ctx, msg.IntentID)
	if err != nil {
		logger.Error("Failed to get intent status",
			zap.String("intent_id", msg.IntentID),
			zap.Error(err))
		// Nack and requeue for retry after 5 seconds
		if nackErr := w.queue.Nack(ctx, msg.MessageID, 5*time.Second); nackErr != nil {
			logger.Error("Failed to nack message",
				zap.String("message_id", msg.MessageID),
				zap.Error(nackErr))
		}
		return
	}

	// Skip if already processing or completed
	if intent.Status == "processing" || intent.Status == "completed" {
		logger.Warn("Intent already being processed or completed, skipping",
			zap.String("intent_id", msg.IntentID),
			zap.String("status", intent.Status),
			zap.String("worker_id", intent.WorkerID))
		// Acknowledge to remove from queue
		if err := w.queue.Ack(ctx, msg.MessageID); err != nil {
			logger.Error("Failed to acknowledge duplicate message",
				zap.String("message_id", msg.MessageID),
				zap.Error(err))
		}
		// Count as completed (duplicate)
		w.metrics.DeletionCompleted.WithLabelValues(msg.Resolution).Inc()
		return
	}

	// Update intent status to processing
	if err := w.intent.UpdateStatus(ctx, msg.IntentID, "processing", w.id, ""); err != nil {
		logger.Warn("Failed to update intent status to processing",
			zap.String("intent_id", msg.IntentID),
			zap.Error(err))
		// Continue anyway
	}

	// Create timeout context for deletion operation
	deleteCtx, cancel := context.WithTimeout(ctx, w.config.DeletionTimeout)
	defer cancel()

	// Execute S3 deletion
	deleteErr := w.storage.Delete(deleteCtx, msg.StorageKey)

	if deleteErr != nil {
		w.handleDeletionError(ctx, msg, deleteErr)
		return
	}

	// Success! Acknowledge message
	if err := w.queue.Ack(ctx, msg.MessageID); err != nil {
		logger.Error("Failed to acknowledge message after successful deletion",
			zap.String("intent_id", msg.IntentID),
			zap.String("message_id", msg.MessageID),
			zap.Error(err))
		// Don't return - message will be reprocessed but deletion is idempotent
	}

	// Try to clean up empty parent directory
	// Extract the image folder from the storage key (e.g., "images/abc123/file.jpg" -> "images/abc123")
	if folderPath := extractImageFolder(msg.StorageKey); folderPath != "" {
		// Check if folder is empty and delete if so
		if err := w.storage.DeleteFolder(ctx, folderPath); err != nil {
			// Log as debug - folder might not be empty yet (other files still exist)
			logger.Debug("Could not delete image folder (may not be empty yet)",
				zap.String("folder", folderPath),
				zap.String("storage_key", msg.StorageKey),
				zap.Error(err))
		} else {
			logger.Debug("Deleted empty image folder",
				zap.String("folder", folderPath),
				zap.String("storage_key", msg.StorageKey))
		}
	}

	// Update intent status to completed
	if err := w.intent.UpdateStatus(ctx, msg.IntentID, "completed", w.id, ""); err != nil {
		logger.Warn("Failed to update intent status to completed",
			zap.String("intent_id", msg.IntentID),
			zap.Error(err))
		// Continue anyway - deletion was successful
	}

	// Record metrics
	duration := time.Since(startTime)
	w.metrics.DeletionDuration.WithLabelValues(msg.Resolution).Observe(duration.Seconds())
	w.metrics.DeletionCompleted.WithLabelValues(msg.Resolution).Inc()

	logger.Info("Deletion completed successfully",
		zap.String("worker_id", w.id),
		zap.String("intent_id", msg.IntentID),
		zap.String("image_id", msg.ImageID),
		zap.String("resolution", msg.Resolution),
		zap.String("storage_key", msg.StorageKey),
		zap.Duration("duration", duration),
		zap.Duration("queue_wait_time", queueWaitTime))
}

// handleDeletionError handles deletion failures with retry logic
func (w *DeletionWorker) handleDeletionError(ctx context.Context, msg *queue.DeletionMessage, err error) {
	logger.Error("Deletion failed",
		zap.String("worker_id", w.id),
		zap.String("intent_id", msg.IntentID),
		zap.String("storage_key", msg.StorageKey),
		zap.Int("retry_count", msg.RetryCount),
		zap.Error(err))

	// Check if we should retry
	if msg.RetryCount < w.config.MaxRetries {
		// Calculate retry delay with exponential backoff
		retryDelay := w.config.RetryBackoff * time.Duration(1<<uint(msg.RetryCount))
		if retryDelay > 5*time.Minute {
			retryDelay = 5 * time.Minute // Cap at 5 minutes
		}

		logger.Warn("Deletion will be retried",
			zap.String("intent_id", msg.IntentID),
			zap.Int("retry_count", msg.RetryCount),
			zap.Int("max_retries", w.config.MaxRetries),
			zap.Duration("retry_delay", retryDelay))

		// Nack message for retry
		if err := w.queue.Nack(ctx, msg.MessageID, retryDelay); err != nil {
			logger.Error("Failed to nack message for retry",
				zap.String("intent_id", msg.IntentID),
				zap.Error(err))
		}

		// Update intent with retry info
		if err := w.intent.UpdateStatus(ctx, msg.IntentID, "retrying", w.id, err.Error()); err != nil {
			logger.Warn("Failed to update intent status to retrying",
				zap.String("intent_id", msg.IntentID),
				zap.Error(err))
		}

		// Record retry metric
		w.metrics.DeletionRetries.WithLabelValues(msg.Resolution).Inc()

		return
	}

	// Max retries exceeded - move to DLQ
	logger.Error("Deletion failed after max retries, moving to DLQ",
		zap.String("worker_id", w.id),
		zap.String("intent_id", msg.IntentID),
		zap.String("storage_key", msg.StorageKey),
		zap.Int("retry_count", msg.RetryCount),
		zap.Int("max_retries", w.config.MaxRetries))

	msg.LastError = err.Error()
	if err := w.queue.MoveToDLQ(ctx, msg, "max_retries_exceeded"); err != nil {
		logger.Error("Failed to move message to DLQ",
			zap.String("intent_id", msg.IntentID),
			zap.Error(err))
	}

	// Update intent status to failed
	if err := w.intent.UpdateStatus(ctx, msg.IntentID, "failed", w.id, err.Error()); err != nil {
		logger.Warn("Failed to update intent status to failed",
			zap.String("intent_id", msg.IntentID),
			zap.Error(err))
	}

	// Record error metric
	w.metrics.DeletionErrors.WithLabelValues(msg.Resolution, "max_retries_exceeded").Inc()
}

// GetID returns the worker ID
func (w *DeletionWorker) GetID() string {
	return w.id
}

// extractImageFolder extracts the image folder path from a storage key
// e.g., "images/abc-123/800x600.jpg" -> "images/abc-123"
// e.g., "images/abc-123/original.jpg" -> "images/abc-123"
func extractImageFolder(storageKey string) string {
	// Split the path by separator
	parts := strings.Split(storageKey, "/")

	// We expect format: images/<image-id>/<filename>
	if len(parts) >= 3 && parts[0] == "images" {
		// Return images/<image-id>
		return parts[0] + "/" + parts[1]
	}

	return ""
}

// Health checks worker health
func (w *DeletionWorker) Health(ctx context.Context) error {
	// Check queue health
	if err := w.queue.Health(ctx); err != nil {
		return fmt.Errorf("queue unhealthy: %w", err)
	}

	// Check storage health (if storage implements health check)
	// This is optional - not all storage backends may support health checks

	return nil
}
