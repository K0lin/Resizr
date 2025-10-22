package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"resizr/internal/queue"
	"resizr/pkg/logger"

	"github.com/dgraph-io/badger/v4"
	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"
)

// IntentManager manages deletion intent logs for two-phase commit pattern
type IntentManager interface {
	// CreateIntent creates a new deletion intent
	CreateIntent(ctx context.Context, intent *queue.DeletionIntent) error

	// GetIntent retrieves a deletion intent
	GetIntent(ctx context.Context, intentID string) (*queue.DeletionIntent, error)

	// UpdateStatus updates the status of a deletion intent
	UpdateStatus(ctx context.Context, intentID, status, workerID, errorMsg string) error

	// DeleteIntent removes a completed/failed intent
	DeleteIntent(ctx context.Context, intentID string) error

	// ListPendingIntents returns intents that are pending or processing
	ListPendingIntents(ctx context.Context, olderThan time.Duration) ([]*queue.DeletionIntent, error)

	// Health checks intent manager health
	Health(ctx context.Context) error
}

// RedisIntentManager implements IntentManager using Redis
type RedisIntentManager struct {
	client redis.Cmdable
}

// NewRedisIntentManager creates a Redis-based intent manager
func NewRedisIntentManager(client redis.Cmdable) *RedisIntentManager {
	return &RedisIntentManager{
		client: client,
	}
}

// CreateIntent creates a new deletion intent
func (m *RedisIntentManager) CreateIntent(ctx context.Context, intent *queue.DeletionIntent) error {
	key := fmt.Sprintf("deletion_intent:%s", intent.IntentID)

	// Serialize intent
	data, err := json.Marshal(intent)
	if err != nil {
		return fmt.Errorf("failed to marshal intent: %w", err)
	}

	// Store with 24-hour TTL
	err = m.client.Set(ctx, key, data, 24*time.Hour).Err()
	if err != nil {
		return fmt.Errorf("failed to create intent: %w", err)
	}

	logger.Debug("Deletion intent created",
		zap.String("intent_id", intent.IntentID),
		zap.String("image_id", intent.ImageID),
		zap.String("resolution", intent.Resolution))

	return nil
}

// GetIntent retrieves a deletion intent
func (m *RedisIntentManager) GetIntent(ctx context.Context, intentID string) (*queue.DeletionIntent, error) {
	key := fmt.Sprintf("deletion_intent:%s", intentID)

	data, err := m.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return nil, fmt.Errorf("intent not found: %s", intentID)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get intent: %w", err)
	}

	var intent queue.DeletionIntent
	if err := json.Unmarshal([]byte(data), &intent); err != nil {
		return nil, fmt.Errorf("failed to unmarshal intent: %w", err)
	}

	return &intent, nil
}

// UpdateStatus updates the status of a deletion intent
func (m *RedisIntentManager) UpdateStatus(ctx context.Context, intentID, status, workerID, errorMsg string) error {
	key := fmt.Sprintf("deletion_intent:%s", intentID)

	// Get current intent
	intent, err := m.GetIntent(ctx, intentID)
	if err != nil {
		return err
	}

	// Update fields
	intent.Status = status
	intent.WorkerID = workerID
	intent.Error = errorMsg

	now := time.Now()
	switch status {
	case "processing":
		intent.StartedAt = &now
	case "completed", "failed":
		intent.CompletedAt = &now
	case "retrying":
		intent.RetryCount++
	}

	// Serialize and store
	data, err := json.Marshal(intent)
	if err != nil {
		return fmt.Errorf("failed to marshal intent: %w", err)
	}

	// Extend TTL for retrying intents
	ttl := 24 * time.Hour
	if status == "retrying" {
		ttl = 48 * time.Hour
	}

	err = m.client.Set(ctx, key, data, ttl).Err()
	if err != nil {
		return fmt.Errorf("failed to update intent: %w", err)
	}

	logger.Debug("Deletion intent status updated",
		zap.String("intent_id", intentID),
		zap.String("status", status),
		zap.String("worker_id", workerID))

	return nil
}

// DeleteIntent removes a completed/failed intent
func (m *RedisIntentManager) DeleteIntent(ctx context.Context, intentID string) error {
	key := fmt.Sprintf("deletion_intent:%s", intentID)
	return m.client.Del(ctx, key).Err()
}

// ListPendingIntents returns intents that are pending or processing
func (m *RedisIntentManager) ListPendingIntents(ctx context.Context, olderThan time.Duration) ([]*queue.DeletionIntent, error) {
	pattern := "deletion_intent:*"
	keys, err := m.findKeys(ctx, pattern)
	if err != nil {
		return nil, err
	}

	var pendingIntents []*queue.DeletionIntent
	cutoff := time.Now().Add(-olderThan)

	for _, key := range keys {
		data, err := m.client.Get(ctx, key).Result()
		if err != nil {
			continue
		}

		var intent queue.DeletionIntent
		if err := json.Unmarshal([]byte(data), &intent); err != nil {
			continue
		}

		// Filter by status and age
		if (intent.Status == "pending" || intent.Status == "processing") &&
			intent.CreatedAt.Before(cutoff) {
			pendingIntents = append(pendingIntents, &intent)
		}
	}

	return pendingIntents, nil
}

// findKeys finds all keys matching a pattern
func (m *RedisIntentManager) findKeys(ctx context.Context, pattern string) ([]string, error) {
	var keys []string
	var cursor uint64

	for {
		var scanKeys []string
		var err error

		scanKeys, cursor, err = m.client.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			return nil, err
		}

		keys = append(keys, scanKeys...)

		if cursor == 0 {
			break
		}
	}

	return keys, nil
}

// Health checks intent manager health
func (m *RedisIntentManager) Health(ctx context.Context) error {
	return m.client.Ping(ctx).Err()
}

// BadgerIntentManager implements IntentManager using BadgerDB
type BadgerIntentManager struct {
	db *badger.DB
}

// NewBadgerIntentManager creates a Badger-based intent manager
func NewBadgerIntentManager(db *badger.DB) *BadgerIntentManager {
	return &BadgerIntentManager{
		db: db,
	}
}

// CreateIntent creates a new deletion intent
func (m *BadgerIntentManager) CreateIntent(ctx context.Context, intent *queue.DeletionIntent) error {
	key := fmt.Sprintf("deletion_intent:%s", intent.IntentID)

	// Serialize intent
	data, err := json.Marshal(intent)
	if err != nil {
		return fmt.Errorf("failed to marshal intent: %w", err)
	}

	// Store with 24-hour TTL
	err = m.db.Update(func(txn *badger.Txn) error {
		entry := badger.NewEntry([]byte(key), data).WithTTL(24 * time.Hour)
		return txn.SetEntry(entry)
	})

	if err != nil {
		return fmt.Errorf("failed to create intent: %w", err)
	}

	logger.Debug("Deletion intent created (Badger)",
		zap.String("intent_id", intent.IntentID),
		zap.String("image_id", intent.ImageID),
		zap.String("resolution", intent.Resolution))

	return nil
}

// GetIntent retrieves a deletion intent
func (m *BadgerIntentManager) GetIntent(ctx context.Context, intentID string) (*queue.DeletionIntent, error) {
	key := fmt.Sprintf("deletion_intent:%s", intentID)

	var intent queue.DeletionIntent
	err := m.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(key))
		if err != nil {
			return err
		}

		return item.Value(func(val []byte) error {
			return json.Unmarshal(val, &intent)
		})
	})

	if err == badger.ErrKeyNotFound {
		return nil, fmt.Errorf("intent not found: %s", intentID)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get intent: %w", err)
	}

	return &intent, nil
}

// UpdateStatus updates the status of a deletion intent
func (m *BadgerIntentManager) UpdateStatus(ctx context.Context, intentID, status, workerID, errorMsg string) error {
	key := fmt.Sprintf("deletion_intent:%s", intentID)

	return m.db.Update(func(txn *badger.Txn) error {
		// Get current intent
		item, err := txn.Get([]byte(key))
		if err != nil {
			return err
		}

		var intent queue.DeletionIntent
		err = item.Value(func(val []byte) error {
			return json.Unmarshal(val, &intent)
		})
		if err != nil {
			return err
		}

		// Update fields
		intent.Status = status
		intent.WorkerID = workerID
		intent.Error = errorMsg

		now := time.Now()
		switch status {
		case "processing":
			intent.StartedAt = &now
		case "completed", "failed":
			intent.CompletedAt = &now
		case "retrying":
			intent.RetryCount++
		}

		// Serialize and store
		data, err := json.Marshal(&intent)
		if err != nil {
			return err
		}

		// Extend TTL for retrying intents
		ttl := 24 * time.Hour
		if status == "retrying" {
			ttl = 48 * time.Hour
		}

		entry := badger.NewEntry([]byte(key), data).WithTTL(ttl)
		return txn.SetEntry(entry)
	})
}

// DeleteIntent removes a completed/failed intent
func (m *BadgerIntentManager) DeleteIntent(ctx context.Context, intentID string) error {
	key := fmt.Sprintf("deletion_intent:%s", intentID)

	return m.db.Update(func(txn *badger.Txn) error {
		return txn.Delete([]byte(key))
	})
}

// ListPendingIntents returns intents that are pending or processing
func (m *BadgerIntentManager) ListPendingIntents(ctx context.Context, olderThan time.Duration) ([]*queue.DeletionIntent, error) {
	var pendingIntents []*queue.DeletionIntent
	cutoff := time.Now().Add(-olderThan)

	err := m.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		iter := txn.NewIterator(opts)
		defer iter.Close()

		prefix := []byte("deletion_intent:")
		for iter.Seek(prefix); iter.ValidForPrefix(prefix); iter.Next() {
			item := iter.Item()

			var intent queue.DeletionIntent
			err := item.Value(func(val []byte) error {
				return json.Unmarshal(val, &intent)
			})
			if err != nil {
				continue
			}

			// Filter by status and age
			if (intent.Status == "pending" || intent.Status == "processing") &&
				intent.CreatedAt.Before(cutoff) {
				pendingIntents = append(pendingIntents, &intent)
			}
		}

		return nil
	})

	return pendingIntents, err
}

// Health checks intent manager health
func (m *BadgerIntentManager) Health(ctx context.Context) error {
	return m.db.View(func(txn *badger.Txn) error {
		return nil
	})
}

// NewIntentManager creates an appropriate intent manager based on the backend
func NewIntentManager(cacheType string, redisClient redis.Cmdable, badgerDB *badger.DB) (IntentManager, error) {
	switch cacheType {
	case "redis":
		if redisClient == nil {
			return nil, fmt.Errorf("redis client required for redis intent manager")
		}
		return NewRedisIntentManager(redisClient), nil

	case "badger":
		if badgerDB == nil {
			return nil, fmt.Errorf("badger database required for badger intent manager")
		}
		return NewBadgerIntentManager(badgerDB), nil

	default:
		return nil, fmt.Errorf("unsupported cache type: %s", cacheType)
	}
}
