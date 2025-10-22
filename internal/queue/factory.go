package queue

import (
	"fmt"
	"time"

	"resizr/internal/config"
	"resizr/pkg/logger"

	"github.com/dgraph-io/badger/v4"
	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"
)

// NewDeletionQueue creates a new deletion queue based on configuration
// Supports both Redis Streams and Badger implementations
func NewDeletionQueue(cfg *config.Config, redisClient redis.Cmdable, badgerDB *badger.DB) (DeletionQueue, error) {
	logger.Info("Initializing deletion queue",
		zap.String("type", cfg.Cache.Type))

	queueCfg := &QueueConfig{
		StreamName:    "deletion_queue",
		ConsumerGroup: "deletion_workers",
		QueuePrefix:   "queue:deletion:",
		MessageTTL:    24 * time.Hour,
		MaxRetries:    3,
		RetryBackoff:  30 * time.Second,
	}

	switch cfg.Cache.Type {
	case "redis":
		if redisClient == nil {
			return nil, fmt.Errorf("redis client is required for redis queue")
		}
		return NewRedisQueue(redisClient, queueCfg)

	case "badger":
		if badgerDB == nil {
			return nil, fmt.Errorf("badger database is required for badger queue")
		}
		return NewBadgerQueue(badgerDB, queueCfg)

	default:
		return nil, fmt.Errorf("unsupported cache type: %s (must be 'redis' or 'badger')", cfg.Cache.Type)
	}
}
