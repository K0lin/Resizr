package queue

import (
	"context"
	"time"
)

// DeletionQueue defines the interface for deletion message queuing
// Supports both Redis Streams and Badger implementations
type DeletionQueue interface {
	// Enqueue adds a deletion message to the queue
	// Returns the message ID for tracking
	Enqueue(ctx context.Context, msg *DeletionMessage) (string, error)

	// Consume reads messages from the queue (blocking)
	// Returns a channel that receives messages for processing
	Consume(ctx context.Context, consumerID string) (<-chan *DeletionMessage, error)

	// Ack acknowledges successful processing of a message
	// Removes the message from the queue
	Ack(ctx context.Context, messageID string) error

	// Nack rejects a message for retry after a delay
	// Message will be reprocessed after retryAfter duration
	Nack(ctx context.Context, messageID string, retryAfter time.Duration) error

	// MoveToDLQ moves a message to dead-letter queue
	// Used when a message exceeds max retries
	MoveToDLQ(ctx context.Context, msg *DeletionMessage, reason string) error

	// GetPending returns pending messages (for monitoring)
	GetPending(ctx context.Context, limit int) ([]*DeletionMessage, error)

	// GetStats returns queue statistics
	GetStats(ctx context.Context) (*QueueStats, error)

	// Health checks queue health
	Health(ctx context.Context) error

	// Close gracefully shuts down the queue
	Close() error
}

// DeletionMessage represents a deletion task in the queue
type DeletionMessage struct {
	// Message identifiers
	MessageID string `json:"message_id"` // Queue-specific message ID
	IntentID  string `json:"intent_id"`  // Deletion intent ID for tracking

	// Image information
	ImageID    string `json:"image_id"`
	Resolution string `json:"resolution"`
	StorageKey string `json:"storage_key"`
	Hash       string `json:"hash"`

	// Timing information
	EnqueuedAt time.Time `json:"enqueued_at"`
	StartedAt  time.Time `json:"started_at,omitempty"`

	// Retry tracking
	RetryCount int    `json:"retry_count"`
	LastError  string `json:"last_error,omitempty"`

	// Priority (normal | high)
	Priority string `json:"priority"`

	// Consumer ID (for tracking which worker is processing)
	ConsumerID string `json:"consumer_id,omitempty"`
}

// QueueStats represents queue statistics
type QueueStats struct {
	// Queue backend type (redis | badger)
	Type string `json:"type"`

	// Message counts
	QueueSize  int64 `json:"queue_size"` // Pending messages
	Processing int64 `json:"processing"` // Currently being processed
	DLQSize    int64 `json:"dlq_size"`   // Dead-letter queue size

	// Timing information
	OldestMessageAt  time.Time `json:"oldest_message_at,omitempty"`
	OldestMessageAge int64     `json:"oldest_message_age_seconds"`

	// Worker information
	ActiveConsumers int `json:"active_consumers"`

	// Additional details (backend-specific)
	Details map[string]string `json:"details,omitempty"`
}

// QueueConfig holds configuration for queue initialization
type QueueConfig struct {
	// Stream name (Redis) or prefix (Badger)
	StreamName string

	// Consumer group name (Redis only)
	ConsumerGroup string

	// Queue prefix (Badger only)
	QueuePrefix string

	// Maximum message TTL
	MessageTTL time.Duration

	// Maximum retries before DLQ
	MaxRetries int

	// Retry backoff duration
	RetryBackoff time.Duration
}

// DeletionIntent represents a deletion intent log entry
// Used for two-phase commit pattern
type DeletionIntent struct {
	// Intent identifier
	IntentID string `json:"intent_id"`

	// Image information
	ImageID    string `json:"image_id"`
	Resolution string `json:"resolution"`
	StorageKey string `json:"storage_key"`
	Hash       string `json:"hash"`

	// Status tracking
	Status string `json:"status"` // pending | processing | completed | failed

	// Timing information
	CreatedAt   time.Time  `json:"created_at"`
	StartedAt   *time.Time `json:"started_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`

	// Error information
	Error      string `json:"error,omitempty"`
	RetryCount int    `json:"retry_count"`

	// Worker information
	WorkerID string `json:"worker_id,omitempty"`

	// Queue message ID (for tracking)
	MessageID string `json:"message_id,omitempty"`
}
