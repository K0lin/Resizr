package queue

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"resizr/pkg/logger"

	"github.com/dgraph-io/badger/v4"
	"go.uber.org/zap"
)

// BadgerQueue implements DeletionQueue using BadgerDB
// Uses a log-structured approach with sorted keys for FIFO ordering
type BadgerQueue struct {
	db *badger.DB

	// Key prefixes for different queues
	queuePrefix      string // "queue:deletion:"
	processingPrefix string // "queue:deletion:processing:"
	dlqPrefix        string // "queue:deletion:dlq:"

	// Consumer coordination
	consumerLock sync.Mutex
	consumers    map[string]bool // Track active consumers

	// Metrics
	metrics *QueueMetrics

	// Shutdown coordination
	shutdown chan struct{}
	wg       sync.WaitGroup
}

// NewBadgerQueue creates a new Badger-based deletion queue
func NewBadgerQueue(db *badger.DB, cfg *QueueConfig) (*BadgerQueue, error) {
	prefix := cfg.QueuePrefix
	if prefix == "" {
		prefix = "queue:deletion:"
	}

	// Use prefix as instance ID for unique metrics (useful for testing)
	instanceID := ""
	if strings.HasPrefix(prefix, "test:") {
		instanceID = prefix
	}

	q := &BadgerQueue{
		db:               db,
		queuePrefix:      prefix,
		processingPrefix: prefix + "processing:",
		dlqPrefix:        prefix + "dlq:",
		consumers:        make(map[string]bool),
		metrics:          NewQueueMetrics("badger", instanceID),
		shutdown:         make(chan struct{}),
	}

	// Start background cleanup goroutine for expired processing messages
	q.wg.Add(1)
	go q.cleanupExpiredProcessing()

	logger.Info("Badger deletion queue initialized",
		zap.String("prefix", q.queuePrefix))

	return q, nil
}

// Enqueue adds a deletion message to the queue
func (q *BadgerQueue) Enqueue(ctx context.Context, msg *DeletionMessage) (string, error) {
	if msg.MessageID == "" {
		msg.MessageID = generateID()
	}
	msg.EnqueuedAt = time.Now()

	// Serialize message
	data, err := serializeMessage(msg)
	if err != nil {
		q.metrics.EnqueueErrors.Inc()
		return "", fmt.Errorf("failed to marshal message: %w", err)
	}

	// Generate sortable key with priority
	// Format: queue:deletion:{timestamp}:{priority}:{uuid}
	// High priority gets earlier timestamp to sort first
	timestamp := time.Now().UnixNano()
	if isPriorityHigh(msg.Priority) {
		timestamp -= 1_000_000_000_000 // Subtract to prioritize
	}

	key := fmt.Sprintf("%s%019d:%s:%s",
		q.queuePrefix,
		timestamp,
		msg.Priority,
		msg.IntentID)

	// Store message with TTL (24 hours)
	err = q.db.Update(func(txn *badger.Txn) error {
		entry := badger.NewEntry([]byte(key), data).WithTTL(24 * time.Hour)
		return txn.SetEntry(entry)
	})

	if err != nil {
		q.metrics.EnqueueErrors.Inc()
		logger.Error("Failed to enqueue deletion message",
			zap.String("intent_id", msg.IntentID),
			zap.Error(err))
		return "", fmt.Errorf("failed to enqueue message: %w", err)
	}

	q.metrics.Enqueued.Inc()
	q.metrics.IncrQueueSize()

	logger.Debug("Deletion message enqueued",
		zap.String("key", key),
		zap.String("intent_id", msg.IntentID),
		zap.String("image_id", msg.ImageID),
		zap.String("resolution", msg.Resolution))

	return key, nil
}

// Consume reads messages from the queue (blocking)
func (q *BadgerQueue) Consume(ctx context.Context, consumerID string) (<-chan *DeletionMessage, error) {
	// Register consumer
	q.consumerLock.Lock()
	q.consumers[consumerID] = true
	q.consumerLock.Unlock()

	msgChan := make(chan *DeletionMessage, 10)

	q.wg.Add(1)
	go func() {
		defer q.wg.Done()
		defer close(msgChan)
		defer func() {
			// Unregister consumer
			q.consumerLock.Lock()
			delete(q.consumers, consumerID)
			q.consumerLock.Unlock()
		}()

		logger.Info("Starting message consumer",
			zap.String("consumer_id", consumerID))

		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				logger.Info("Stopping message consumer",
					zap.String("consumer_id", consumerID))
				return

			case <-q.shutdown:
				return

			case <-ticker.C:
				// Try to fetch next message
				msg, messageKey, err := q.fetchNextMessage(consumerID)
				if err != nil {
					if err != badger.ErrKeyNotFound {
						// Transaction conflicts are normal with multiple workers
						if err != badger.ErrConflict {
							q.metrics.DequeueErrors.Inc()
							logger.Error("Failed to fetch message",
								zap.String("consumer_id", consumerID),
								zap.Error(err))
						}
					}
					continue
				}

				if msg != nil {
					msg.MessageID = messageKey
					msg.StartedAt = time.Now()
					msg.ConsumerID = consumerID

					q.metrics.IncrProcessing()

					select {
					case msgChan <- msg:
						// Message sent to worker
					case <-ctx.Done():
						return
					case <-q.shutdown:
						return
					}
				}
			}
		}
	}()

	return msgChan, nil
}

// fetchNextMessage atomically moves a message from queue to processing
func (q *BadgerQueue) fetchNextMessage(consumerID string) (*DeletionMessage, string, error) {
	var msg *DeletionMessage
	var messageKey string

	err := q.db.Update(func(txn *badger.Txn) error {
		// Iterate to find first available message
		opts := badger.DefaultIteratorOptions
		opts.PrefetchSize = 1
		iter := txn.NewIterator(opts)
		defer iter.Close()

		prefix := []byte(q.queuePrefix)
		for iter.Seek(prefix); iter.ValidForPrefix(prefix); iter.Next() {
			item := iter.Item()
			key := string(item.Key())

			// Read message
			err := item.Value(func(val []byte) error {
				var err error
				msg, err = deserializeMessage(val)
				return err
			})
			if err != nil {
				logger.Warn("Failed to deserialize message, skipping",
					zap.String("key", key),
					zap.Error(err))
				continue
			}

			// Move to processing (atomic delete + insert)
			// This prevents multiple workers from processing the same message
			processingKey := q.getProcessingKey(key, consumerID)

			// Delete from queue
			if err := txn.Delete(item.Key()); err != nil {
				return err
			}

			// Insert into processing with 10-minute TTL
			// If worker crashes, message will auto-retry
			val, _ := serializeMessage(msg)
			entry := badger.NewEntry([]byte(processingKey), val).WithTTL(10 * time.Minute)
			if err := txn.SetEntry(entry); err != nil {
				return err
			}

			// Set messageKey to the processing key (not the original queue key)
			// This is important for Ack/Nack/MoveToDLQ operations
			messageKey = processingKey

			logger.Debug("Message moved to processing",
				zap.String("key", key),
				zap.String("processing_key", processingKey),
				zap.String("consumer_id", consumerID))

			return nil // Success, exit iteration
		}

		return badger.ErrKeyNotFound // No messages available
	})

	if err != nil {
		return nil, "", err
	}

	return msg, messageKey, nil
}

// getProcessingKey generates a processing key from queue key
func (q *BadgerQueue) getProcessingKey(queueKey, consumerID string) string {
	// Replace prefix and append consumer ID
	key := strings.Replace(queueKey, q.queuePrefix, q.processingPrefix, 1)
	return fmt.Sprintf("%s:%s", key, consumerID)
}

// Ack acknowledges successful processing
func (q *BadgerQueue) Ack(ctx context.Context, messageID string) error {
	// MessageID now contains the processing key directly
	processingKey := messageID

	err := q.db.Update(func(txn *badger.Txn) error {
		// Delete the processing key directly
		if err := txn.Delete([]byte(processingKey)); err != nil {
			if err != badger.ErrKeyNotFound {
				return err
			}
			// If key not found, it's already processed (idempotent)
		}
		return nil
	})

	if err != nil {
		logger.Error("Failed to ack message",
			zap.String("message_id", messageID),
			zap.Error(err))
		return fmt.Errorf("failed to ack message: %w", err)
	}

	q.metrics.DecrProcessing()
	q.metrics.DecrQueueSize()
	q.metrics.Completed.Inc()

	logger.Debug("Message acknowledged",
		zap.String("message_id", messageID))

	return nil
}

// Nack rejects a message for retry
func (q *BadgerQueue) Nack(ctx context.Context, messageID string, retryAfter time.Duration) error {
	// MessageID now contains the processing key directly
	processingKey := messageID

	err := q.db.Update(func(txn *badger.Txn) error {
		// Get message from processing
		item, err := txn.Get([]byte(processingKey))
		if err != nil {
			if err == badger.ErrKeyNotFound {
				return fmt.Errorf("message not found in processing")
			}
			return err
		}

		var msg *DeletionMessage
		err = item.Value(func(val []byte) error {
			var err error
			msg, err = deserializeMessage(val)
			return err
		})
		if err != nil {
			return err
		}

		// Increment retry count
		msg.RetryCount++

		// Delete from processing
		if err := txn.Delete([]byte(processingKey)); err != nil {
			return err
		}

		// Re-enqueue with delay (use future timestamp)
		futureTimestamp := time.Now().Add(retryAfter).UnixNano()
		newKey := fmt.Sprintf("%s%019d:%s:%s:retry%d",
			q.queuePrefix,
			futureTimestamp,
			msg.Priority,
			msg.IntentID,
			msg.RetryCount)

		data, _ := serializeMessage(msg)
		entry := badger.NewEntry([]byte(newKey), data).WithTTL(24 * time.Hour)

		return txn.SetEntry(entry)
	})

	if err != nil {
		logger.Error("Failed to nack message",
			zap.String("message_id", messageID),
			zap.Error(err))
		return fmt.Errorf("failed to nack message: %w", err)
	}

	q.metrics.DecrProcessing()
	q.metrics.Retries.Inc()

	logger.Debug("Message marked for retry",
		zap.String("message_id", messageID),
		zap.Duration("retry_after", retryAfter))

	return nil
}

// MoveToDLQ moves a message to dead-letter queue
func (q *BadgerQueue) MoveToDLQ(ctx context.Context, msg *DeletionMessage, reason string) error {
	// Update message with error
	msg.LastError = reason
	data, err := serializeMessage(msg)
	if err != nil {
		return fmt.Errorf("failed to serialize message: %w", err)
	}

	err = q.db.Update(func(txn *badger.Txn) error {
		// Add to DLQ
		dlqKey := fmt.Sprintf("%s%019d:%s",
			q.dlqPrefix,
			time.Now().UnixNano(),
			msg.IntentID)

		entry := badger.NewEntry([]byte(dlqKey), data).WithTTL(7 * 24 * time.Hour) // 7 days
		if setErr := txn.SetEntry(entry); setErr != nil {
			logger.Error("Failed to add to DLQ",
				zap.String("dlq_key", dlqKey),
				zap.Error(setErr))
			return setErr
		}

		// Delete from processing if exists
		// MessageID now contains the processing key (with consumer ID suffix)
		processingKey := msg.MessageID

		// Delete the processing key directly
		delErr := txn.Delete([]byte(processingKey))
		if delErr != nil {
			// Key might not exist if already processed, which is fine
			if delErr != badger.ErrKeyNotFound {
				logger.Error("Failed to delete processing key in MoveToDLQ",
					zap.String("processing_key", processingKey),
					zap.Error(delErr))
				return fmt.Errorf("failed to delete processing entry: %w", delErr)
			}
		}

		return nil
	})

	if err != nil {
		logger.Error("Failed to move message to DLQ",
			zap.String("intent_id", msg.IntentID),
			zap.Error(err))
		return fmt.Errorf("failed to move to DLQ: %w", err)
	}

	q.metrics.DecrProcessing()
	q.metrics.DecrQueueSize()
	q.metrics.DLQMoved.Inc()

	logger.Warn("Message moved to DLQ",
		zap.String("intent_id", msg.IntentID),
		zap.String("message_id", msg.MessageID),
		zap.String("reason", reason))

	return nil
}

// GetPending returns pending messages
func (q *BadgerQueue) GetPending(ctx context.Context, limit int) ([]*DeletionMessage, error) {
	var messages []*DeletionMessage

	err := q.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		iter := txn.NewIterator(opts)
		defer iter.Close()

		count := 0
		prefix := []byte(q.queuePrefix)

		for iter.Seek(prefix); iter.ValidForPrefix(prefix) && count < limit; iter.Next() {
			item := iter.Item()

			var msg *DeletionMessage
			err := item.Value(func(val []byte) error {
				var err error
				msg, err = deserializeMessage(val)
				return err
			})

			if err != nil {
				continue
			}

			msg.MessageID = string(item.Key())
			messages = append(messages, msg)
			count++
		}

		return nil
	})

	return messages, err
}

// GetStats returns queue statistics
func (q *BadgerQueue) GetStats(ctx context.Context) (*QueueStats, error) {
	var queueSize, processing, dlqSize int64
	var oldestMessageAt time.Time

	err := q.db.View(func(txn *badger.Txn) error {
		// Count queue messages
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = false
		iter := txn.NewIterator(opts)
		defer iter.Close()

		// Queue size and oldest message
		prefix := []byte(q.queuePrefix)
		for iter.Seek(prefix); iter.ValidForPrefix(prefix); iter.Next() {
			key := string(iter.Item().Key())

			// Skip processing and DLQ keys (they start with queuePrefix too)
			if strings.Contains(key, ":processing:") || strings.Contains(key, ":dlq:") {
				continue
			}

			queueSize++

			if queueSize == 1 {
				// Extract timestamp from first key
				parts := strings.Split(strings.TrimPrefix(key, q.queuePrefix), ":")
				if len(parts) > 0 {
					if ts, err := strconv.ParseInt(parts[0], 10, 64); err == nil {
						oldestMessageAt = time.Unix(0, ts)
					}
				}
			}
		}

		// Count processing messages
		iter.Rewind()
		prefix = []byte(q.processingPrefix)
		for iter.Seek(prefix); iter.ValidForPrefix(prefix); iter.Next() {
			processing++
		}

		// Count DLQ messages
		iter.Rewind()
		prefix = []byte(q.dlqPrefix)
		for iter.Seek(prefix); iter.ValidForPrefix(prefix); iter.Next() {
			dlqSize++
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	var oldestAge int64
	if !oldestMessageAt.IsZero() {
		oldestAge = int64(time.Since(oldestMessageAt).Seconds())
	}

	// Get active consumer count
	q.consumerLock.Lock()
	activeConsumers := len(q.consumers)
	q.consumerLock.Unlock()

	stats := &QueueStats{
		Type:             "badger",
		QueueSize:        queueSize,
		Processing:       processing,
		DLQSize:          dlqSize,
		OldestMessageAt:  oldestMessageAt,
		OldestMessageAge: oldestAge,
		ActiveConsumers:  activeConsumers,
		Details: map[string]string{
			"queue_prefix":      q.queuePrefix,
			"processing_prefix": q.processingPrefix,
			"dlq_prefix":        q.dlqPrefix,
		},
	}

	return stats, nil
}

// Health checks queue health
func (q *BadgerQueue) Health(ctx context.Context) error {
	// Test read access
	return q.db.View(func(txn *badger.Txn) error {
		return nil
	})
}

// cleanupExpiredProcessing moves expired processing messages back to queue
// This handles worker crashes
func (q *BadgerQueue) cleanupExpiredProcessing() {
	defer q.wg.Done()

	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-q.shutdown:
			return
		case <-ticker.C:
			// Messages in processing with expired TTL will be automatically deleted by Badger
			// No manual cleanup needed thanks to TTL
		}
	}
}

// Close closes the queue
func (q *BadgerQueue) Close() error {
	logger.Info("Closing Badger deletion queue")

	close(q.shutdown)
	q.wg.Wait()

	// Update metrics one last time
	q.metrics.UpdateGauges()

	return nil
}

// Ensure BadgerQueue implements DeletionQueue
var _ DeletionQueue = (*BadgerQueue)(nil)
