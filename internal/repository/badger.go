package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"resizr/internal/models"
	"resizr/pkg/logger"

	"github.com/dgraph-io/badger/v4"
	"go.uber.org/zap"
)

// BadgerRepository implements both ImageRepository and CacheRepository interfaces using BadgerDB
type BadgerRepository struct {
	db        *badger.DB
	config    *CacheConfig
	directory string

	// Statistics (atomic counters)
	cacheHits   int64
	cacheMisses int64
}

// Ensure BadgerRepository implements the Cache interface
var _ Cache = (*BadgerRepository)(nil)

// NewBadgerRepository creates a new BadgerDB repository (both metadata and cache)
func NewBadgerRepository(cfg *CacheConfig) (*BadgerRepository, error) {
	logger.Info("Initializing BadgerDB cache repository",
		zap.String("directory", cfg.Directory),
		zap.Duration("ttl", cfg.TTL))

	// Create directory if it doesn't exist
	if err := os.MkdirAll(cfg.Directory, 0755); err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}

	// Configure BadgerDB options
	opts := badger.DefaultOptions(cfg.Directory)
	opts.Logger = &badgerLogger{} // Custom logger to suppress BadgerDB logs

	// Open BadgerDB
	db, err := badger.Open(opts)
	if err != nil {
		return nil, fmt.Errorf("failed to open BadgerDB: %w", err)
	}

	repo := &BadgerRepository{
		db:        db,
		config:    cfg,
		directory: cfg.Directory,
	}

	logger.Info("BadgerDB cache repository initialized successfully")
	return repo, nil
}

// SetCachedURL stores a pre-signed URL in cache with TTL
func (b *BadgerRepository) SetCachedURL(ctx context.Context, imageID, resolution, url string, ttl time.Duration) error {
	key := b.getCacheKey(imageID, resolution)

	err := b.db.Update(func(txn *badger.Txn) error {
		entry := badger.NewEntry([]byte(key), []byte(url)).WithTTL(ttl)
		return txn.SetEntry(entry)
	})

	if err != nil {
		logger.ErrorWithContext(ctx, "Failed to cache URL",
			zap.String("image_id", imageID),
			zap.String("resolution", resolution),
			zap.Duration("ttl", ttl),
			zap.Error(err))
		return fmt.Errorf("failed to cache URL: %w", err)
	}

	logger.DebugWithContext(ctx, "URL cached successfully",
		zap.String("image_id", imageID),
		zap.String("resolution", resolution),
		zap.Duration("ttl", ttl))

	return nil
}

// GetCachedURL retrieves a cached pre-signed URL
func (b *BadgerRepository) GetCachedURL(ctx context.Context, imageID, resolution string) (string, error) {
	key := b.getCacheKey(imageID, resolution)

	var url string
	err := b.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(key))
		if err != nil {
			return err
		}

		return item.Value(func(val []byte) error {
			url = string(val)
			return nil
		})
	})

	if err != nil {
		if err == badger.ErrKeyNotFound {
			// Cache miss
			atomic.AddInt64(&b.cacheMisses, 1)
			logger.DebugWithContext(ctx, "Cache miss for URL",
				zap.String("image_id", imageID),
				zap.String("resolution", resolution))
			return "", models.NotFoundError{
				Resource: "cached_url",
				ID:       fmt.Sprintf("%s/%s", imageID, resolution),
			}
		}
		return "", fmt.Errorf("failed to get cached URL: %w", err)
	}

	// Cache hit
	atomic.AddInt64(&b.cacheHits, 1)
	logger.DebugWithContext(ctx, "Cache hit for URL",
		zap.String("image_id", imageID),
		zap.String("resolution", resolution))

	return url, nil
}

// DeleteCachedURL removes a cached URL
func (b *BadgerRepository) DeleteCachedURL(ctx context.Context, imageID, resolution string) error {
	key := b.getCacheKey(imageID, resolution)

	return b.db.Update(func(txn *badger.Txn) error {
		return txn.Delete([]byte(key))
	})
}

// DeleteAllCachedURLs removes all cached URLs for an image
func (b *BadgerRepository) DeleteAllCachedURLs(ctx context.Context, imageID string) error {
	prefix := b.getCacheKey(imageID, "")

	return b.db.Update(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = false
		iter := txn.NewIterator(opts)
		defer iter.Close()

		var keysToDelete [][]byte
		for iter.Seek([]byte(prefix)); iter.ValidForPrefix([]byte(prefix)); iter.Next() {
			key := iter.Item().KeyCopy(nil)
			keysToDelete = append(keysToDelete, key)
		}

		for _, key := range keysToDelete {
			if err := txn.Delete(key); err != nil {
				return err
			}
		}

		return nil
	})
}

// Set stores any value in cache with TTL
func (b *BadgerRepository) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	// Serialize value to JSON
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal value: %w", err)
	}

	return b.db.Update(func(txn *badger.Txn) error {
		entry := badger.NewEntry([]byte(key), data).WithTTL(ttl)
		return txn.SetEntry(entry)
	})
}

// Get retrieves any value from cache
func (b *BadgerRepository) Get(ctx context.Context, key string) (string, error) {
	var value string

	err := b.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(key))
		if err != nil {
			return err
		}

		return item.Value(func(val []byte) error {
			// Try to unmarshal as JSON first, fallback to string
			var jsonValue string
			if err := json.Unmarshal(val, &jsonValue); err == nil {
				value = jsonValue
			} else {
				value = string(val)
			}
			return nil
		})
	})

	if err != nil {
		if err == badger.ErrKeyNotFound {
			return "", models.NotFoundError{
				Resource: "cache_key",
				ID:       key,
			}
		}
		return "", fmt.Errorf("failed to get cached value: %w", err)
	}

	return value, nil
}

// Delete removes any value from cache
func (b *BadgerRepository) Delete(ctx context.Context, key string) error {
	return b.db.Update(func(txn *badger.Txn) error {
		return txn.Delete([]byte(key))
	})
}

// Health checks cache health
func (b *BadgerRepository) Health(ctx context.Context) error {
	// Test a simple operation
	testKey := "health:check:" + fmt.Sprintf("%d", time.Now().Unix())

	if err := b.Set(ctx, testKey, "ok", time.Second); err != nil {
		return fmt.Errorf("BadgerDB write test failed: %w", err)
	}

	if err := b.Delete(ctx, testKey); err != nil {
		logger.WarnWithContext(ctx, "Failed to cleanup health check key", zap.Error(err))
		// Not a critical error
	}

	return nil
}

// Close closes the cache connection
func (b *BadgerRepository) Close() error {
	logger.Info("Closing BadgerDB cache repository")
	return b.db.Close()
}

// GetStats retrieves cache statistics
func (b *BadgerRepository) GetStats(ctx context.Context) (*CacheStats, error) {
	lsm, vlog := b.db.Size()

	// Count total keys
	keyCount, err := b.countKeys()
	if err != nil {
		logger.WarnWithContext(ctx, "Failed to count keys", zap.Error(err))
		keyCount = -1 // Unknown
	}

	stats := &CacheStats{
		Type:        CacheTypeBadger,
		CacheHits:   atomic.LoadInt64(&b.cacheHits),
		CacheMisses: atomic.LoadInt64(&b.cacheMisses),
		KeyCount:    keyCount,
		StorageUsed: lsm + vlog,
		Details: map[string]string{
			"directory":  b.directory,
			"lsm_size":   fmt.Sprintf("%d bytes", lsm),
			"vlog_size":  fmt.Sprintf("%d bytes", vlog),
			"total_size": fmt.Sprintf("%d bytes", lsm+vlog),
		},
	}

	return stats, nil
}

// Helper methods

// getCacheKey generates BadgerDB key for cached URLs
func (b *BadgerRepository) getCacheKey(imageID, resolution string) string {
	if resolution == "" {
		return fmt.Sprintf("image:cache:%s:", imageID)
	}
	return fmt.Sprintf("image:cache:%s:%s", imageID, resolution)
}

// countKeys counts total number of keys in the database
func (b *BadgerRepository) countKeys() (int64, error) {
	var count int64

	err := b.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = false
		iter := txn.NewIterator(opts)
		defer iter.Close()

		for iter.Rewind(); iter.Valid(); iter.Next() {
			count++
		}
		return nil
	})

	return count, err
}

// badgerLogger implements badger.Logger to suppress BadgerDB logs
type badgerLogger struct{}

func (l *badgerLogger) Errorf(format string, args ...interface{}) {
	// Only log actual errors
	if strings.Contains(format, "ERROR") || strings.Contains(format, "error") {
		logger.Error("BadgerDB error", zap.String("message", fmt.Sprintf(format, args...)))
	}
}

func (l *badgerLogger) Warningf(format string, args ...interface{}) {
	// Suppress warnings - BadgerDB is quite verbose
}

func (l *badgerLogger) Infof(format string, args ...interface{}) {
	// Suppress info logs
}

func (l *badgerLogger) Debugf(format string, args ...interface{}) {
	// Suppress debug logs
}
