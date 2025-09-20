package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"resizr/internal/models"
	"resizr/pkg/logger"

	"github.com/dgraph-io/badger/v4"
	"go.uber.org/zap"
)

// BadgerImageRepository implements ImageRepository and CacheRepository using BadgerDB
// This provides a complete replacement for Redis, storing both metadata and cache data
// in local BadgerDB files with no external dependencies.
type BadgerImageRepository struct {
	*BadgerRepository // Embed for Cache functionality
}

// Ensure BadgerImageRepository implements all interfaces
var _ ImageRepository = (*BadgerImageRepository)(nil)
var _ CacheRepository = (*BadgerImageRepository)(nil)
var _ DeduplicationRepository = (*BadgerImageRepository)(nil)

// NewBadgerImageRepository creates a new BadgerDB-based ImageRepository
func NewBadgerImageRepository(cfg *CacheConfig) (*BadgerImageRepository, error) {
	badgerRepo, err := NewBadgerRepository(cfg)
	if err != nil {
		return nil, err
	}

	return &BadgerImageRepository{
		BadgerRepository: badgerRepo,
	}, nil
}

// ImageRepository methods implementation

// Store saves image metadata to BadgerDB
func (b *BadgerImageRepository) Store(ctx context.Context, img *models.ImageMetadata) error {
	logger.DebugWithContext(ctx, "Storing image metadata",
		zap.String("image_id", img.ID))

	// Validate metadata
	if err := img.Validate(); err != nil {
		return fmt.Errorf("invalid metadata: %w", err)
	}

	key := b.getMetadataKey(img.ID)

	// Serialize metadata to JSON
	data, err := json.Marshal(img)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	// Store metadata (no TTL for metadata)
	err = b.db.Update(func(txn *badger.Txn) error {
		return txn.Set([]byte(key), data)
	})

	if err != nil {
		logger.ErrorWithContext(ctx, "Failed to store image metadata",
			zap.String("image_id", img.ID),
			zap.Error(err))
		return fmt.Errorf("failed to store metadata: %w", err)
	}

	logger.DebugWithContext(ctx, "Image metadata stored successfully",
		zap.String("image_id", img.ID))

	return nil
}

// Get retrieves image metadata by ID
func (b *BadgerImageRepository) Get(ctx context.Context, id string) (*models.ImageMetadata, error) {
	logger.DebugWithContext(ctx, "Getting image metadata",
		zap.String("image_id", id))

	key := b.getMetadataKey(id)

	var metadata models.ImageMetadata
	err := b.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(key))
		if err != nil {
			return err
		}

		return item.Value(func(val []byte) error {
			return json.Unmarshal(val, &metadata)
		})
	})

	if err != nil {
		if err == badger.ErrKeyNotFound {
			logger.DebugWithContext(ctx, "Image not found",
				zap.String("image_id", id))
			return nil, models.NotFoundError{
				Resource: "image",
				ID:       id,
			}
		}
		logger.ErrorWithContext(ctx, "Failed to get image metadata",
			zap.String("image_id", id),
			zap.Error(err))
		return nil, fmt.Errorf("failed to get metadata: %w", err)
	}

	logger.DebugWithContext(ctx, "Image metadata retrieved successfully",
		zap.String("image_id", id))

	return &metadata, nil
}

// Update updates existing image metadata
func (b *BadgerImageRepository) Update(ctx context.Context, img *models.ImageMetadata) error {
	logger.DebugWithContext(ctx, "Updating image metadata",
		zap.String("image_id", img.ID))

	// Check if image exists
	exists, err := b.Exists(ctx, img.ID)
	if err != nil {
		return err
	}

	if !exists {
		return models.NotFoundError{
			Resource: "image",
			ID:       img.ID,
		}
	}

	// Update timestamp
	img.UpdatedAt = time.Now()

	// Store updated metadata
	return b.Store(ctx, img)
}

// Delete removes image metadata from BadgerDB
func (b *BadgerImageRepository) Delete(ctx context.Context, id string) error {
	logger.DebugWithContext(ctx, "Deleting image metadata",
		zap.String("image_id", id))

	key := b.getMetadataKey(id)

	// Check if exists first
	exists, err := b.Exists(ctx, id)
	if err != nil {
		return err
	}

	if !exists {
		return models.NotFoundError{
			Resource: "image",
			ID:       id,
		}
	}

	// Clean up cached URLs first (before deleting metadata)
	// This prevents orphaned cache entries if metadata deletion succeeds but cache cleanup fails
	if err := b.DeleteAllCachedURLs(ctx, id); err != nil {
		logger.WarnWithContext(ctx, "Failed to cleanup cached URLs, proceeding with metadata deletion",
			zap.String("image_id", id),
			zap.Error(err))
		// Continue with metadata deletion even if cache cleanup fails
	}

	// Delete metadata
	err = b.db.Update(func(txn *badger.Txn) error {
		return txn.Delete([]byte(key))
	})

	if err != nil {
		logger.ErrorWithContext(ctx, "Failed to delete image metadata",
			zap.String("image_id", id),
			zap.Error(err))
		return fmt.Errorf("failed to delete metadata: %w", err)
	}

	logger.InfoWithContext(ctx, "Image metadata deleted successfully",
		zap.String("image_id", id))

	return nil
}

// Exists checks if image metadata exists
func (b *BadgerImageRepository) Exists(_ctx context.Context, id string) (bool, error) {
	key := b.getMetadataKey(id)

	err := b.db.View(func(txn *badger.Txn) error {
		_, err := txn.Get([]byte(key))
		return err
	})

	if err != nil {
		if err == badger.ErrKeyNotFound {
			return false, nil
		}
		return false, fmt.Errorf("failed to check existence: %w", err)
	}

	return true, nil
}

// List retrieves multiple image metadata with pagination
func (b *BadgerImageRepository) List(ctx context.Context, offset, limit int) ([]*models.ImageMetadata, error) {
	logger.DebugWithContext(ctx, "Listing images",
		zap.Int("offset", offset),
		zap.Int("limit", limit))

	var images []*models.ImageMetadata
	prefix := "image:metadata:"

	err := b.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		iter := txn.NewIterator(opts)
		defer iter.Close()

		// Collect all metadata keys
		var keys []string
		for iter.Seek([]byte(prefix)); iter.ValidForPrefix([]byte(prefix)); iter.Next() {
			key := string(iter.Item().Key())
			keys = append(keys, key)
		}

		// Apply pagination
		total := len(keys)
		if offset >= total {
			return nil // No results
		}

		end := offset + limit
		if end > total {
			end = total
		}

		pagedKeys := keys[offset:end]

		// Get metadata for each key
		for _, key := range pagedKeys {
			id := b.extractIDFromMetadataKey(key)
			if id == "" {
				continue
			}

			item, err := txn.Get([]byte(key))
			if err != nil {
				logger.WarnWithContext(ctx, "Failed to get metadata for key",
					zap.String("key", key),
					zap.String("image_id", id),
					zap.Error(err))
				continue
			}

			err = item.Value(func(val []byte) error {
				var metadata models.ImageMetadata
				if err := json.Unmarshal(val, &metadata); err != nil {
					return err
				}
				images = append(images, &metadata)
				return nil
			})

			if err != nil {
				logger.WarnWithContext(ctx, "Failed to unmarshal metadata",
					zap.String("image_id", id),
					zap.Error(err))
				continue
			}
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to list images: %w", err)
	}

	logger.DebugWithContext(ctx, "Images listed successfully",
		zap.Int("total_found", len(images)),
		zap.Int("offset", offset),
		zap.Int("limit", limit))

	return images, nil
}

// UpdateResolutions updates the resolutions list for an image
func (b *BadgerImageRepository) UpdateResolutions(ctx context.Context, id string, resolutions []string) error {
	logger.DebugWithContext(ctx, "Updating image resolutions",
		zap.String("image_id", id),
		zap.Strings("resolutions", resolutions))

	// Get existing metadata
	metadata, err := b.Get(ctx, id)
	if err != nil {
		return err
	}

	// Update resolutions and timestamp
	metadata.Resolutions = resolutions
	metadata.UpdatedAt = time.Now()

	// Store updated metadata
	return b.Store(ctx, metadata)
}

// GetStats retrieves repository statistics
func (b *BadgerImageRepository) GetStats(ctx context.Context) (*RepositoryStats, error) {
	lsm, vlog := b.db.Size()

	// Count total images
	totalImages, err := b.countImages(ctx)
	if err != nil {
		logger.WarnWithContext(ctx, "Failed to count images", zap.Error(err))
		totalImages = -1 // Unknown
	}

	// Count cache keys
	cacheKeys, err := b.countCacheKeys(ctx)
	if err != nil {
		logger.WarnWithContext(ctx, "Failed to count cache keys", zap.Error(err))
		cacheKeys = -1 // Unknown
	}

	stats := &RepositoryStats{
		TotalImages: totalImages,
		CacheHits:   atomic.LoadInt64(&b.cacheHits),
		CacheMisses: atomic.LoadInt64(&b.cacheMisses),
		StorageUsed: lsm + vlog,
		Connections: ConnectionStats{
			Active:  1, // BadgerDB is embedded
			MaxOpen: 1,
		},
		KeyCounts: map[string]int64{
			"metadata": totalImages,
			"cache":    cacheKeys,
		},
	}

	return stats, nil
}

// CacheRepository methods - delegate to embedded BadgerRepository

// SetCachedURL stores a pre-signed URL in cache with TTL
func (b *BadgerImageRepository) SetCachedURL(ctx context.Context, imageID, resolution, url string, ttl time.Duration) error {
	return b.BadgerRepository.SetCachedURL(ctx, imageID, resolution, url, ttl)
}

// GetCachedURL retrieves a cached pre-signed URL
func (b *BadgerImageRepository) GetCachedURL(ctx context.Context, imageID, resolution string) (string, error) {
	return b.BadgerRepository.GetCachedURL(ctx, imageID, resolution)
}

// DeleteCachedURL removes a cached URL
func (b *BadgerImageRepository) DeleteCachedURL(ctx context.Context, imageID, resolution string) error {
	return b.BadgerRepository.DeleteCachedURL(ctx, imageID, resolution)
}

// DeleteAllCachedURLs removes all cached URLs for an image
func (b *BadgerImageRepository) DeleteAllCachedURLs(ctx context.Context, imageID string) error {
	return b.BadgerRepository.DeleteAllCachedURLs(ctx, imageID)
}

// SetCache stores any value in cache with TTL
func (b *BadgerImageRepository) SetCache(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	return b.BadgerRepository.Set(ctx, key, value, ttl)
}

// GetCache retrieves any value from cache
func (b *BadgerImageRepository) GetCache(ctx context.Context, key string) (string, error) {
	return b.BadgerRepository.Get(ctx, key)
}

// DeleteCache removes any value from cache
func (b *BadgerImageRepository) DeleteCache(ctx context.Context, key string) error {
	return b.BadgerRepository.Delete(ctx, key)
}

// Helper methods for metadata operations

// getMetadataKey generates BadgerDB key for image metadata
func (b *BadgerImageRepository) getMetadataKey(id string) string {
	return fmt.Sprintf("image:metadata:%s", id)
}

// extractIDFromMetadataKey extracts image ID from metadata key
func (b *BadgerImageRepository) extractIDFromMetadataKey(key string) string {
	parts := strings.Split(key, ":")
	if len(parts) >= 3 && parts[0] == "image" && parts[1] == "metadata" {
		return parts[2]
	}
	return ""
}

// countImages counts total number of images
func (b *BadgerImageRepository) countImages(_ctx context.Context) (int64, error) {
	var count int64
	prefix := "image:metadata:"

	err := b.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = false
		iter := txn.NewIterator(opts)
		defer iter.Close()

		for iter.Seek([]byte(prefix)); iter.ValidForPrefix([]byte(prefix)); iter.Next() {
			count++
		}
		return nil
	})

	return count, err
}

// countCacheKeys counts total number of cache keys
func (b *BadgerImageRepository) countCacheKeys(_ctx context.Context) (int64, error) {
	var count int64
	prefix := "image:cache:"

	err := b.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = false
		iter := txn.NewIterator(opts)
		defer iter.Close()

		for iter.Seek([]byte(prefix)); iter.ValidForPrefix([]byte(prefix)); iter.Next() {
			count++
		}
		return nil
	})

	return count, err
}

// DeduplicationRepository implementation

// StoreDeduplicationInfo stores deduplication information for a hash
func (b *BadgerImageRepository) StoreDeduplicationInfo(ctx context.Context, info *models.DeduplicationInfo) error {
	logger.DebugWithContext(ctx, "Storing deduplication info",
		zap.String("hash", info.Hash.String()),
		zap.String("master_image_id", info.MasterImageID))

	key := b.getDeduplicationKey(info.Hash)

	// Serialize to JSON
	data, err := json.Marshal(info)
	if err != nil {
		return fmt.Errorf("failed to marshal deduplication info: %w", err)
	}

	// Store in BadgerDB
	err = b.db.Update(func(txn *badger.Txn) error {
		return txn.Set([]byte(key), data)
	})

	if err != nil {
		logger.ErrorWithContext(ctx, "Failed to store deduplication info",
			zap.String("hash", info.Hash.String()),
			zap.Error(err))
		return fmt.Errorf("failed to store deduplication info: %w", err)
	}

	return nil
}

// GetDeduplicationInfo retrieves deduplication info by hash
func (b *BadgerImageRepository) GetDeduplicationInfo(ctx context.Context, hash models.ImageHash) (*models.DeduplicationInfo, error) {
	logger.DebugWithContext(ctx, "Getting deduplication info",
		zap.String("hash", hash.String()))

	key := b.getDeduplicationKey(hash)

	var info models.DeduplicationInfo
	err := b.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(key))
		if err != nil {
			return err
		}

		return item.Value(func(val []byte) error {
			return json.Unmarshal(val, &info)
		})
	})

	if err != nil {
		if err == badger.ErrKeyNotFound {
			return nil, models.NotFoundError{
				Resource: "deduplication_info",
				ID:       hash.String(),
			}
		}
		return nil, fmt.Errorf("failed to get deduplication info: %w", err)
	}

	return &info, nil
}

// UpdateDeduplicationInfo updates existing deduplication info
func (b *BadgerImageRepository) UpdateDeduplicationInfo(ctx context.Context, info *models.DeduplicationInfo) error {
	return b.StoreDeduplicationInfo(ctx, info)
}

// DeleteDeduplicationInfo removes deduplication info
func (b *BadgerImageRepository) DeleteDeduplicationInfo(ctx context.Context, hash models.ImageHash) error {
	logger.DebugWithContext(ctx, "Deleting deduplication info",
		zap.String("hash", hash.String()))

	key := b.getDeduplicationKey(hash)

	err := b.db.Update(func(txn *badger.Txn) error {
		return txn.Delete([]byte(key))
	})

	if err != nil {
		logger.ErrorWithContext(ctx, "Failed to delete deduplication info",
			zap.String("hash", hash.String()),
			zap.Error(err))
		return fmt.Errorf("failed to delete deduplication info: %w", err)
	}

	return nil
}

// FindImageByHash looks for existing images with the same hash
func (b *BadgerImageRepository) FindImageByHash(ctx context.Context, hash models.ImageHash) (*models.DeduplicationInfo, error) {
	return b.GetDeduplicationInfo(ctx, hash)
}

// AddHashReference adds a new image reference to existing hash
func (b *BadgerImageRepository) AddHashReference(ctx context.Context, hash models.ImageHash, imageID string) error {
	logger.DebugWithContext(ctx, "Adding hash reference",
		zap.String("hash", hash.String()),
		zap.String("image_id", imageID))

	info, err := b.GetDeduplicationInfo(ctx, hash)
	if err != nil {
		return err
	}

	info.AddReference(imageID)
	return b.UpdateDeduplicationInfo(ctx, info)
}

// RemoveHashReference removes an image reference from hash
func (b *BadgerImageRepository) RemoveHashReference(ctx context.Context, hash models.ImageHash, imageID string) error {
	logger.DebugWithContext(ctx, "Removing hash reference",
		zap.String("hash", hash.String()),
		zap.String("image_id", imageID))

	info, err := b.GetDeduplicationInfo(ctx, hash)
	if err != nil {
		return err
	}

	info.RemoveReference(imageID)

	// If no more references, delete the deduplication info
	if info.IsOrphaned() {
		return b.DeleteDeduplicationInfo(ctx, hash)
	}

	return b.UpdateDeduplicationInfo(ctx, info)
}

// GetOrphanedHashes returns hashes with no image references
func (b *BadgerImageRepository) GetOrphanedHashes(ctx context.Context) ([]models.ImageHash, error) {
	var orphanedHashes []models.ImageHash
	prefix := "dedup:"

	err := b.db.View(func(txn *badger.Txn) error {
		iter := txn.NewIterator(badger.DefaultIteratorOptions)
		defer iter.Close()

		for iter.Seek([]byte(prefix)); iter.ValidForPrefix([]byte(prefix)); iter.Next() {
			item := iter.Item()

			err := item.Value(func(val []byte) error {
				var info models.DeduplicationInfo
				if err := json.Unmarshal(val, &info); err != nil {
					return err
				}

				if info.IsOrphaned() {
					orphanedHashes = append(orphanedHashes, info.Hash)
				}
				return nil
			})

			if err != nil {
				logger.WarnWithContext(ctx, "Failed to unmarshal deduplication info during orphan check",
					zap.String("key", string(item.Key())),
					zap.Error(err))
				continue
			}
		}
		return nil
	})

	return orphanedHashes, err
}

// Helper methods for deduplication

// getDeduplicationKey returns the key for storing deduplication info
func (b *BadgerImageRepository) getDeduplicationKey(hash models.ImageHash) string {
	return fmt.Sprintf("dedup:%s", hash.GetHashKey())
}
