package repository

import (
	"context"
	"time"

	"resizr/internal/models"
)

// ImageRepository defines the interface for image metadata operations
type ImageRepository interface {
	// Store saves image metadata to the database
	Store(ctx context.Context, img *models.ImageMetadata) error

	// Get retrieves image metadata by ID
	Get(ctx context.Context, id string) (*models.ImageMetadata, error)

	// Update updates existing image metadata
	Update(ctx context.Context, img *models.ImageMetadata) error

	// Delete removes image metadata from database
	Delete(ctx context.Context, id string) error

	// Exists checks if image metadata exists
	Exists(ctx context.Context, id string) (bool, error)

	// List retrieves multiple image metadata with pagination
	List(ctx context.Context, offset, limit int) ([]*models.ImageMetadata, error)

	// UpdateResolutions updates the resolutions list for an image
	UpdateResolutions(ctx context.Context, id string, resolutions []string) error

	// GetStats retrieves storage statistics
	GetStats(ctx context.Context) (*RepositoryStats, error)

	// Health checks repository health
	Health(ctx context.Context) error

	// Close closes the repository connection
	Close() error
}

// DeduplicationRepository defines the interface for image deduplication operations
type DeduplicationRepository interface {
	// StoreDeduplicationInfo stores deduplication information for a hash
	StoreDeduplicationInfo(ctx context.Context, info *models.DeduplicationInfo) error

	// GetDeduplicationInfo retrieves deduplication info by hash
	GetDeduplicationInfo(ctx context.Context, hash models.ImageHash) (*models.DeduplicationInfo, error)

	// UpdateDeduplicationInfo updates existing deduplication info
	UpdateDeduplicationInfo(ctx context.Context, info *models.DeduplicationInfo) error

	// DeleteDeduplicationInfo removes deduplication info
	DeleteDeduplicationInfo(ctx context.Context, hash models.ImageHash) error

	// FindImageByHash looks for existing images with the same hash
	FindImageByHash(ctx context.Context, hash models.ImageHash) (*models.DeduplicationInfo, error)

	// AddHashReference adds a new image reference to existing hash
	AddHashReference(ctx context.Context, hash models.ImageHash, imageID string) error

	// RemoveHashReference removes an image reference from hash
	RemoveHashReference(ctx context.Context, hash models.ImageHash, imageID string) error

	// GetOrphanedHashes returns hashes with no image references
	GetOrphanedHashes(ctx context.Context) ([]models.ImageHash, error)
}

// CompositeRepository combines all repository interfaces for full functionality
type CompositeRepository interface {
	ImageRepository
	CacheRepository
	DeduplicationRepository
}

// CacheRepository defines the interface for caching operations
type CacheRepository interface {
	// SetCachedURL stores a pre-signed URL in cache with TTL
	SetCachedURL(ctx context.Context, imageID, resolution, url string, ttl time.Duration) error

	// GetCachedURL retrieves a cached pre-signed URL
	GetCachedURL(ctx context.Context, imageID, resolution string) (string, error)

	// DeleteCachedURL removes a cached URL
	DeleteCachedURL(ctx context.Context, imageID, resolution string) error

	// DeleteAllCachedURLs removes all cached URLs for an image
	DeleteAllCachedURLs(ctx context.Context, imageID string) error

	// SetCache stores any value in cache with TTL
	SetCache(ctx context.Context, key string, value interface{}, ttl time.Duration) error

	// GetCache retrieves any value from cache
	GetCache(ctx context.Context, key string) (string, error)

	// DeleteCache removes any value from cache
	DeleteCache(ctx context.Context, key string) error
}

// RepositoryStats represents repository statistics
type RepositoryStats struct {
	TotalImages int64            `json:"total_images"`
	CacheHits   int64            `json:"cache_hits"`
	CacheMisses int64            `json:"cache_misses"`
	StorageUsed int64            `json:"storage_used_bytes"`
	LastBackup  time.Time        `json:"last_backup,omitempty"`
	Connections ConnectionStats  `json:"connections"`
	KeyCounts   map[string]int64 `json:"key_counts"`
}

// ConnectionStats represents connection pool statistics
type ConnectionStats struct {
	Active  int `json:"active"`
	Idle    int `json:"idle"`
	Total   int `json:"total"`
	MaxOpen int `json:"max_open"`
	MaxIdle int `json:"max_idle"`
}

// BatchOperation represents a batch operation
type BatchOperation struct {
	Type  string      `json:"type"` // "store", "delete", "update"
	Key   string      `json:"key"`
	Value interface{} `json:"value,omitempty"`
}

// Repository that supports batch operations
type BatchRepository interface {
	// ExecuteBatch executes multiple operations in a transaction
	ExecuteBatch(ctx context.Context, operations []BatchOperation) error
}
