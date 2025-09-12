package repository

import (
	"context"
	"fmt"
	"time"

	"resizr/internal/config"
	"resizr/pkg/logger"

	"go.uber.org/zap"
)

// CacheType represents the type of cache implementation
type CacheType string

const (
	CacheTypeRedis  CacheType = "redis"
	CacheTypeBadger CacheType = "badger"
)

// CacheConfig represents cache configuration
type CacheConfig struct {
	Type      CacheType     `json:"type"`
	Directory string        `json:"directory,omitempty"` // For BadgerDB
	TTL       time.Duration `json:"ttl"`
}

// Cache defines a unified interface for different cache implementations
type Cache interface {
	// URL caching methods
	SetCachedURL(ctx context.Context, imageID, resolution, url string, ttl time.Duration) error
	GetCachedURL(ctx context.Context, imageID, resolution string) (string, error)
	DeleteCachedURL(ctx context.Context, imageID, resolution string) error
	DeleteAllCachedURLs(ctx context.Context, imageID string) error

	// Generic caching methods
	Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error
	Get(ctx context.Context, key string) (string, error)
	Delete(ctx context.Context, key string) error

	// Management methods
	Health(ctx context.Context) error
	Close() error

	// Statistics
	GetStats(ctx context.Context) (*CacheStats, error)
}

// CacheStats represents cache statistics
type CacheStats struct {
	Type        CacheType         `json:"type"`
	CacheHits   int64             `json:"cache_hits"`
	CacheMisses int64             `json:"cache_misses"`
	KeyCount    int64             `json:"key_count"`
	StorageUsed int64             `json:"storage_used_bytes"`
	Details     map[string]string `json:"details"`
}

// NewImageRepository creates a new image repository
// This creates either a Redis-only or BadgerDB-only repository based on CACHE_TYPE
func NewImageRepository(cfg *config.Config) (ImageRepository, error) {
	logger.Info("Initializing image repository",
		zap.String("type", cfg.Cache.Type))

	switch cfg.Cache.Type {
	case "redis":
		// Use Redis for both metadata and caching
		logger.Info("Using Redis for both metadata and caching")
		return NewRedisRepository(&cfg.Redis)

	case "badger":
		// Use BadgerDB for both metadata and caching (no Redis at all)
		logger.Info("Using BadgerDB for both metadata and caching")

		// Initialize BadgerDB with configuration
		cacheConfig := &CacheConfig{
			Type:      CacheTypeBadger,
			Directory: cfg.Cache.Directory,
			TTL:       cfg.Cache.TTL,
		}

		badgerRepo, err := NewBadgerImageRepository(cacheConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize BadgerDB repository: %w", err)
		}

		return badgerRepo, nil

	default:
		return nil, fmt.Errorf("unsupported cache type: %s", cfg.Cache.Type)
	}
}
