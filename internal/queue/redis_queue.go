package queue

import (
	"context"
	"fmt"
	"strings"
	"time"

	"resizr/pkg/logger"

	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"
)

// RedisQueue implements DeletionQueue using Redis Streams
type RedisQueue struct {
	client        redis.Cmdable
	streamName    string // "deletion_queue"
	dlqStreamName string // "deletion_queue:dlq"
	consumerGroup string // "deletion_workers"

	// Metrics
	metrics *QueueMetrics
}

// NewRedisQueue creates a new Redis-based deletion queue
func NewRedisQueue(client redis.Cmdable, cfg *QueueConfig) (*RedisQueue, error) {
	q := &RedisQueue{
		client:        client,
		streamName:    cfg.StreamName,
		dlqStreamName: cfg.StreamName + ":dlq",
		consumerGroup: cfg.ConsumerGroup,
		metrics:       NewQueueMetrics("redis"),
	}

	// Create consumer group if not exists
	if err := q.ensureConsumerGroup(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to create consumer group: %w", err)
	}

	logger.Info("Redis deletion queue initialized",
		zap.String("stream", q.streamName),
		zap.String("consumer_group", q.consumerGroup))

	return q, nil
}

// ensureConsumerGroup creates the consumer group if it doesn't exist
func (q *RedisQueue) ensureConsumerGroup(ctx context.Context) error {
	// Try to create the consumer group
	// XGROUP CREATE stream group 0 MKSTREAM
	err := q.client.XGroupCreateMkStream(ctx, q.streamName, q.consumerGroup, "0").Err()

	if err != nil && !strings.Contains(err.Error(), "BUSYGROUP") {
		return err
	}

	// Also create DLQ stream
	err = q.client.XGroupCreateMkStream(ctx, q.dlqStreamName, q.consumerGroup+"_dlq", "0").Err()
	if err != nil && !strings.Contains(err.Error(), "BUSYGROUP") {
		return err
	}

	return nil
}

// Enqueue adds a deletion message to the queue
func (q *RedisQueue) Enqueue(ctx context.Context, msg *DeletionMessage) (string, error) {
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

	// Calculate priority score (high priority gets negative timestamp to sort first)
	priorityScore := time.Now().UnixNano()
	if isPriorityHigh(msg.Priority) {
		priorityScore -= 1_000_000_000_000 // Subtract 1000 seconds worth of nanos
	}

	// Add to stream
	streamID, err := q.client.XAdd(ctx, &redis.XAddArgs{
		Stream: q.streamName,
		Values: map[string]interface{}{
			"payload":        string(data),
			"intent_id":      msg.IntentID,
			"priority":       msg.Priority,
			"priority_score": priorityScore,
			"image_id":       msg.ImageID,
			"resolution":     msg.Resolution,
		},
	}).Result()

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
		zap.String("stream_id", streamID),
		zap.String("intent_id", msg.IntentID),
		zap.String("image_id", msg.ImageID),
		zap.String("resolution", msg.Resolution))

	return streamID, nil
}

// Consume reads messages from the queue (blocking)
func (q *RedisQueue) Consume(ctx context.Context, consumerID string) (<-chan *DeletionMessage, error) {
	msgChan := make(chan *DeletionMessage, 10) // Buffered channel

	go func() {
		defer close(msgChan)

		logger.Info("Starting message consumer",
			zap.String("consumer_id", consumerID),
			zap.String("stream", q.streamName))

		for {
			select {
			case <-ctx.Done():
				logger.Info("Stopping message consumer",
					zap.String("consumer_id", consumerID))
				return

			default:
				// Read from stream using consumer group
				// XREADGROUP GROUP group consumer BLOCK 5000 COUNT 10 STREAMS stream >
				entries, err := q.client.XReadGroup(ctx, &redis.XReadGroupArgs{
					Group:    q.consumerGroup,
					Consumer: consumerID,
					Streams:  []string{q.streamName, ">"},
					Count:    10,
					Block:    5 * time.Second,
				}).Result()

				if err != nil {
					if err != redis.Nil {
						q.metrics.DequeueErrors.Inc()
						logger.Error("Failed to read from stream",
							zap.String("consumer_id", consumerID),
							zap.Error(err))
						time.Sleep(time.Second) // Backoff on error
					}
					continue
				}

				// Process messages
				for _, stream := range entries {
					for _, xmsg := range stream.Messages {
						msg, err := q.parseStreamMessage(xmsg)
						if err != nil {
							logger.Error("Failed to parse message",
								zap.String("message_id", xmsg.ID),
								zap.Error(err))
							// Acknowledge to remove corrupt message
							q.client.XAck(ctx, q.streamName, q.consumerGroup, xmsg.ID)
							continue
						}

						msg.MessageID = xmsg.ID
						msg.StartedAt = time.Now()
						msg.ConsumerID = consumerID

						q.metrics.IncrProcessing()

						select {
						case msgChan <- msg:
							// Message sent to worker
						case <-ctx.Done():
							return
						}
					}
				}
			}
		}
	}()

	return msgChan, nil
}

// parseStreamMessage extracts DeletionMessage from Redis stream entry
func (q *RedisQueue) parseStreamMessage(xmsg redis.XMessage) (*DeletionMessage, error) {
	payloadStr, ok := xmsg.Values["payload"].(string)
	if !ok {
		return nil, fmt.Errorf("missing payload in message")
	}

	msg, err := deserializeMessage([]byte(payloadStr))
	if err != nil {
		return nil, fmt.Errorf("failed to deserialize message: %w", err)
	}

	return msg, nil
}

// Ack acknowledges successful processing
func (q *RedisQueue) Ack(ctx context.Context, messageID string) error {
	// Acknowledge the message
	_, err := q.client.XAck(ctx, q.streamName, q.consumerGroup, messageID).Result()
	if err != nil {
		logger.Error("Failed to ack message",
			zap.String("message_id", messageID),
			zap.Error(err))
		return fmt.Errorf("failed to ack message: %w", err)
	}

	// Delete message from stream to prevent buildup
	q.client.XDel(ctx, q.streamName, messageID)

	q.metrics.DecrProcessing()
	q.metrics.DecrQueueSize()
	q.metrics.Completed.Inc()

	logger.Debug("Message acknowledged",
		zap.String("message_id", messageID))

	return nil
}

// Nack rejects a message for retry
func (q *RedisQueue) Nack(ctx context.Context, messageID string, retryAfter time.Duration) error {
	// Get the message from pending list
	pending, err := q.client.XPendingExt(ctx, &redis.XPendingExtArgs{
		Stream: q.streamName,
		Group:  q.consumerGroup,
		Start:  messageID,
		End:    messageID,
		Count:  1,
	}).Result()

	if err != nil || len(pending) == 0 {
		logger.Warn("Message not found in pending list",
			zap.String("message_id", messageID),
			zap.Error(err))
		return fmt.Errorf("message not found: %w", err)
	}

	// For Redis Streams, we don't actually re-queue
	// The message stays in pending list and will be retried
	// We just log the retry intent
	q.metrics.DecrProcessing()
	q.metrics.Retries.Inc()

	logger.Debug("Message marked for retry",
		zap.String("message_id", messageID),
		zap.Duration("retry_after", retryAfter))

	return nil
}

// MoveToDLQ moves a message to dead-letter queue
func (q *RedisQueue) MoveToDLQ(ctx context.Context, msg *DeletionMessage, reason string) error {
	// Serialize message with error info
	msg.LastError = reason
	data, err := serializeMessage(msg)
	if err != nil {
		return fmt.Errorf("failed to serialize message: %w", err)
	}

	// Add to DLQ stream
	_, err = q.client.XAdd(ctx, &redis.XAddArgs{
		Stream: q.dlqStreamName,
		Values: map[string]interface{}{
			"payload":          string(data),
			"intent_id":        msg.IntentID,
			"original_message": msg.MessageID,
			"failure_reason":   reason,
			"retry_count":      msg.RetryCount,
			"moved_at":         time.Now().Unix(),
			"last_error":       msg.LastError,
		},
	}).Result()

	if err != nil {
		logger.Error("Failed to move message to DLQ",
			zap.String("intent_id", msg.IntentID),
			zap.Error(err))
		return fmt.Errorf("failed to move to DLQ: %w", err)
	}

	// Acknowledge original message to remove from pending
	if msg.MessageID != "" {
		q.client.XAck(ctx, q.streamName, q.consumerGroup, msg.MessageID)
		q.client.XDel(ctx, q.streamName, msg.MessageID)
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
func (q *RedisQueue) GetPending(ctx context.Context, limit int) ([]*DeletionMessage, error) {
	// Get pending messages info
	pending, err := q.client.XPendingExt(ctx, &redis.XPendingExtArgs{
		Stream: q.streamName,
		Group:  q.consumerGroup,
		Start:  "-",
		End:    "+",
		Count:  int64(limit),
	}).Result()

	if err != nil {
		return nil, fmt.Errorf("failed to get pending messages: %w", err)
	}

	var messages []*DeletionMessage

	for _, p := range pending {
		// Read the actual message
		results, err := q.client.XRange(ctx, q.streamName, p.ID, p.ID).Result()
		if err != nil || len(results) == 0 {
			continue
		}

		msg, err := q.parseStreamMessage(results[0])
		if err != nil {
			continue
		}

		msg.MessageID = p.ID
		msg.ConsumerID = p.Consumer
		messages = append(messages, msg)
	}

	return messages, nil
}

// GetStats returns queue statistics
func (q *RedisQueue) GetStats(ctx context.Context) (*QueueStats, error) {
	// Get stream info
	info, err := q.client.XInfoStream(ctx, q.streamName).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get stream info: %w", err)
	}

	// Get pending count
	pendingInfo, err := q.client.XPending(ctx, q.streamName, q.consumerGroup).Result()
	var processing int64
	if err == nil {
		processing = pendingInfo.Count
	}

	// Get DLQ size
	dlqInfo, err := q.client.XInfoStream(ctx, q.dlqStreamName).Result()
	var dlqSize int64
	if err == nil {
		dlqSize = dlqInfo.Length
	}

	// Get oldest message timestamp
	var oldestMessageAt time.Time
	if info.Length > 0 {
		// Read first message
		results, err := q.client.XRange(ctx, q.streamName, "-", "+").Result()
		if err == nil && len(results) > 0 {
			// Parse timestamp from stream ID (format: timestamp-sequence)
			parts := strings.Split(results[0].ID, "-")
			if len(parts) > 0 {
				var ts int64
				if _, err := fmt.Sscanf(parts[0], "%d", &ts); err == nil {
					oldestMessageAt = time.Unix(ts/1000, (ts%1000)*1000000)
				}
			}
		}
	}

	var oldestAge int64
	if !oldestMessageAt.IsZero() {
		oldestAge = int64(time.Since(oldestMessageAt).Seconds())
	}

	// Get consumer count
	groups, err := q.client.XInfoGroups(ctx, q.streamName).Result()
	var activeConsumers int
	if err == nil && len(groups) > 0 {
		activeConsumers = int(groups[0].Consumers)
	}

	stats := &QueueStats{
		Type:             "redis",
		QueueSize:        info.Length,
		Processing:       processing,
		DLQSize:          dlqSize,
		OldestMessageAt:  oldestMessageAt,
		OldestMessageAge: oldestAge,
		ActiveConsumers:  activeConsumers,
		Details: map[string]string{
			"stream":         q.streamName,
			"consumer_group": q.consumerGroup,
		},
	}

	return stats, nil
}

// Health checks queue health
func (q *RedisQueue) Health(ctx context.Context) error {
	// Test stream access
	_, err := q.client.XInfoStream(ctx, q.streamName).Result()
	if err != nil {
		return fmt.Errorf("failed to access stream: %w", err)
	}

	return nil
}

// Close closes the queue
func (q *RedisQueue) Close() error {
	logger.Info("Closing Redis deletion queue")
	// Update metrics one last time
	q.metrics.UpdateGauges()
	return nil
}

// Ensure RedisQueue implements DeletionQueue
var _ DeletionQueue = (*RedisQueue)(nil)
