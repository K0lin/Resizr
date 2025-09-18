package repository

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"
	"math"

	"resizr/internal/config"
	"resizr/internal/models"
	"resizr/pkg/logger"

	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"
)

// RedisRepository implements ImageRepository and CacheRepository interfaces
type RedisRepository struct {
	client redis.Cmdable
	config *config.RedisConfig

	// Statistics (in-memory counters)
	cacheHits   int64
	cacheMisses int64
}

// NewRedisRepository creates a new Redis repository
func NewRedisRepository(cfg *config.RedisConfig) (ImageRepository, error) {
	logger.Info("Initializing Redis repository",
		zap.String("url", cfg.URL),
		zap.Int("db", cfg.DB),
		zap.Int("pool_size", cfg.PoolSize))

	// Parse Redis URL and create client
	opt, err := redis.ParseURL(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("invalid Redis URL: %w", err)
	}

	// Override with config values
	opt.Password = cfg.Password
	opt.DB = cfg.DB
	opt.PoolSize = cfg.PoolSize
	opt.DialTimeout = cfg.Timeout
	opt.ReadTimeout = cfg.Timeout
	opt.WriteTimeout = cfg.Timeout

	client := redis.NewClient(opt)

	repo := &RedisRepository{
		client: client,
		config: cfg,
	}

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	logger.Info("Redis repository initialized successfully")
	return repo, nil
}

// Store saves image metadata to Redis
func (r *RedisRepository) Store(ctx context.Context, img *models.ImageMetadata) error {
	logger.DebugWithContext(ctx, "Storing image metadata",
		zap.String("image_id", img.ID))

	// Validate metadata
	if err := img.Validate(); err != nil {
		return fmt.Errorf("invalid metadata: %w", err)
	}

	key := r.getMetadataKey(img.ID)

	// Convert metadata to Redis hash fields
	fields := r.metadataToFields(img)

	// Store using HMSET for atomic operation
	if err := r.client.HMSet(ctx, key, fields).Err(); err != nil {
		logger.ErrorWithContext(ctx, "Failed to store image metadata",
			zap.String("image_id", img.ID),
			zap.String("key", key),
			zap.Error(err))
		return fmt.Errorf("failed to store metadata: %w", err)
	}

	logger.DebugWithContext(ctx, "Image metadata stored successfully",
		zap.String("image_id", img.ID),
		zap.String("key", key))

	return nil
}

// Get retrieves image metadata by ID
func (r *RedisRepository) Get(ctx context.Context, id string) (*models.ImageMetadata, error) {
	logger.DebugWithContext(ctx, "Getting image metadata",
		zap.String("image_id", id))

	key := r.getMetadataKey(id)

	// Get all fields from hash
	fields, err := r.client.HGetAll(ctx, key).Result()
	if err != nil {
		logger.ErrorWithContext(ctx, "Failed to get image metadata",
			zap.String("image_id", id),
			zap.String("key", key),
			zap.Error(err))
		return nil, fmt.Errorf("failed to get metadata: %w", err)
	}

	// Check if image exists
	if len(fields) == 0 {
		logger.DebugWithContext(ctx, "Image not found",
			zap.String("image_id", id))
		return nil, models.NotFoundError{
			Resource: "image",
			ID:       id,
		}
	}

	// Convert fields to metadata
	metadata, err := r.fieldsToMetadata(fields)
	if err != nil {
		logger.ErrorWithContext(ctx, "Failed to parse image metadata",
			zap.String("image_id", id),
			zap.Error(err))
		return nil, fmt.Errorf("failed to parse metadata: %w", err)
	}

	logger.DebugWithContext(ctx, "Image metadata retrieved successfully",
		zap.String("image_id", id))

	return metadata, nil
}

// Update updates existing image metadata
func (r *RedisRepository) Update(ctx context.Context, img *models.ImageMetadata) error {
	logger.DebugWithContext(ctx, "Updating image metadata",
		zap.String("image_id", img.ID))

	// Check if image exists
	exists, err := r.Exists(ctx, img.ID)
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
	return r.Store(ctx, img)
}

// Delete removes image metadata from Redis
func (r *RedisRepository) Delete(ctx context.Context, id string) error {
	logger.DebugWithContext(ctx, "Deleting image metadata",
		zap.String("image_id", id))

	key := r.getMetadataKey(id)

	// Delete metadata
	deleted, err := r.client.Del(ctx, key).Result()
	if err != nil {
		logger.ErrorWithContext(ctx, "Failed to delete image metadata",
			zap.String("image_id", id),
			zap.Error(err))
		return fmt.Errorf("failed to delete metadata: %w", err)
	}

	if deleted == 0 {
		return models.NotFoundError{
			Resource: "image",
			ID:       id,
		}
	}

	// Clean up cached URLs for this image
	r.DeleteAllCachedURLs(ctx, id)

	logger.InfoWithContext(ctx, "Image metadata deleted successfully",
		zap.String("image_id", id))

	return nil
}

// Exists checks if image metadata exists
func (r *RedisRepository) Exists(ctx context.Context, id string) (bool, error) {
	key := r.getMetadataKey(id)

	exists, err := r.client.Exists(ctx, key).Result()
	if err != nil {
		return false, fmt.Errorf("failed to check existence: %w", err)
	}

	return exists > 0, nil
}

// List retrieves multiple image metadata with pagination
func (r *RedisRepository) List(ctx context.Context, offset, limit int) ([]*models.ImageMetadata, error) {
	logger.DebugWithContext(ctx, "Listing images",
		zap.Int("offset", offset),
		zap.Int("limit", limit))

	pattern := r.getMetadataKey("*")

	// Scan for all metadata keys
	var cursor uint64
	var keys []string

	for {
		var scanKeys []string
		var err error

		scanKeys, cursor, err = r.client.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			return nil, fmt.Errorf("failed to scan keys: %w", err)
		}

		keys = append(keys, scanKeys...)

		if cursor == 0 {
			break
		}
	}

	// Apply pagination
	total := len(keys)
	if offset >= total {
		return []*models.ImageMetadata{}, nil
	}

	end := offset + limit
	if end > total {
		end = total
	}

	pagedKeys := keys[offset:end]

	// Get metadata for each key
	var images []*models.ImageMetadata

	for _, key := range pagedKeys {
		// Extract ID from key
		id := r.extractIDFromKey(key)
		if id == "" {
			continue
		}

		metadata, err := r.Get(ctx, id)
		if err != nil {
			logger.WarnWithContext(ctx, "Failed to get metadata for key",
				zap.String("key", key),
				zap.String("image_id", id),
				zap.Error(err))
			continue
		}

		images = append(images, metadata)
	}

	logger.DebugWithContext(ctx, "Images listed successfully",
		zap.Int("total_found", len(images)),
		zap.Int("offset", offset),
		zap.Int("limit", limit))

	return images, nil
}

// UpdateResolutions updates the resolutions list for an image
func (r *RedisRepository) UpdateResolutions(ctx context.Context, id string, resolutions []string) error {
	logger.DebugWithContext(ctx, "Updating image resolutions",
		zap.String("image_id", id),
		zap.Strings("resolutions", resolutions))

	key := r.getMetadataKey(id)

	// Check if image exists
	exists, err := r.Exists(ctx, id)
	if err != nil {
		return err
	}

	if !exists {
		return models.NotFoundError{
			Resource: "image",
			ID:       id,
		}
	}

	// Update resolutions field and updated_at timestamp
	updates := map[string]interface{}{
		"resolutions": strings.Join(resolutions, ","),
		"updated_at":  time.Now().Format(time.RFC3339),
	}

	if err := r.client.HMSet(ctx, key, updates).Err(); err != nil {
		return fmt.Errorf("failed to update resolutions: %w", err)
	}

	return nil
}

// Cache Repository Implementation

// SetCachedURL stores a pre-signed URL in cache
func (r *RedisRepository) SetCachedURL(ctx context.Context, imageID, resolution, url string, ttl time.Duration) error {
	key := r.getCacheKey(imageID, resolution)

	if err := r.client.Set(ctx, key, url, ttl).Err(); err != nil {
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
func (r *RedisRepository) GetCachedURL(ctx context.Context, imageID, resolution string) (string, error) {
	key := r.getCacheKey(imageID, resolution)

	url, err := r.client.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			// Cache miss
			r.cacheMisses++
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
	r.cacheHits++
	logger.DebugWithContext(ctx, "Cache hit for URL",
		zap.String("image_id", imageID),
		zap.String("resolution", resolution))

	return url, nil
}

// DeleteCachedURL removes a cached URL
func (r *RedisRepository) DeleteCachedURL(ctx context.Context, imageID, resolution string) error {
	key := r.getCacheKey(imageID, resolution)
	return r.client.Del(ctx, key).Err()
}

// DeleteAllCachedURLs removes all cached URLs for an image
func (r *RedisRepository) DeleteAllCachedURLs(ctx context.Context, imageID string) error {
	pattern := r.getCacheKey(imageID, "*")

	// Find all cache keys for this image
	keys, err := r.findKeysByPattern(ctx, pattern)
	if err != nil {
		return err
	}

	if len(keys) == 0 {
		return nil
	}

	// Delete all found keys
	return r.client.Del(ctx, keys...).Err()
}

// Generic cache operations

// SetCache stores any value in cache with TTL
func (r *RedisRepository) SetCache(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	return r.client.Set(ctx, key, value, ttl).Err()
}

// GetCache retrieves any value from cache
func (r *RedisRepository) GetCache(ctx context.Context, key string) (string, error) {
	return r.client.Get(ctx, key).Result()
}

// DeleteCache removes any value from cache
func (r *RedisRepository) DeleteCache(ctx context.Context, key string) error {
	return r.client.Del(ctx, key).Err()
}

// GetStats retrieves repository statistics
/// safeInt64ToInt converts int64 to int safely, returning 0 if out of bounds.
func safeInt64ToInt(v int64) int {
	// On 32-bit platforms, int is 32 bits; on 64-bit, it's 64.
	// Set bounds based on this.
	const minInt = int64(int(^uint(0)>>1)) * -1 - 1 // matches int's min value
	const maxInt = int64(int(^uint(0)>>1))
	if v < minInt || v > maxInt {
		return 0 // or set to -1 to signal "invalid"
	}
	return int(v)
}

func (r *RedisRepository) GetStats(ctx context.Context) (*RepositoryStats, error) {
	// Get Redis info
	info, err := r.client.Info(ctx, "memory", "clients", "stats").Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get Redis info: %w", err)
	}

	// Count total images
	totalImages, err := r.countImages(ctx)
	if err != nil {
		logger.WarnWithContext(ctx, "Failed to count images", zap.Error(err))
		totalImages = -1 // Unknown
	}

	stats := &RepositoryStats{
		TotalImages: totalImages,
		CacheHits:   r.cacheHits,
		CacheMisses: r.cacheMisses,
		StorageUsed: r.parseInfoValue(info, "used_memory"),
		Connections: ConnectionStats{
			Active:  safeInt64ToInt(r.parseInfoValue(info, "connected_clients")),
			MaxOpen: r.config.PoolSize,
		},
		KeyCounts: map[string]int64{
			"metadata": totalImages,
			"cache":    r.countCacheKeys(ctx),
		},
	}

	return stats, nil
}

// Health checks repository health
func (r *RedisRepository) Health(ctx context.Context) error {
	// Simple ping test
	if err := r.client.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("Redis ping failed: %w", err)
	}

	// Test a simple operation
	testKey := "health:check:" + fmt.Sprintf("%d", time.Now().Unix())
	if err := r.client.Set(ctx, testKey, "ok", time.Second).Err(); err != nil {
		return fmt.Errorf("Redis write test failed: %w", err)
	}

	if err := r.client.Del(ctx, testKey).Err(); err != nil {
		logger.WarnWithContext(ctx, "Failed to cleanup health check key", zap.Error(err))
		// Not a critical error
	}

	return nil
}

// Close closes the repository connection
func (r *RedisRepository) Close() error {
	if client, ok := r.client.(*redis.Client); ok {
		return client.Close()
	}
	return nil
}

// Helper methods

// getMetadataKey generates Redis key for image metadata
func (r *RedisRepository) getMetadataKey(id string) string {
	return fmt.Sprintf("image:metadata:%s", id)
}

// getCacheKey generates Redis key for cached URLs
func (r *RedisRepository) getCacheKey(imageID, resolution string) string {
	return fmt.Sprintf("image:cache:%s:%s", imageID, resolution)
}

// extractIDFromKey extracts image ID from Redis key
func (r *RedisRepository) extractIDFromKey(key string) string {
	parts := strings.Split(key, ":")
	if len(parts) >= 3 && parts[0] == "image" && parts[1] == "metadata" {
		return parts[2]
	}
	return ""
}

// metadataToFields converts ImageMetadata to Redis hash fields
func (r *RedisRepository) metadataToFields(img *models.ImageMetadata) map[string]interface{} {
	return map[string]interface{}{
		"id":           img.ID,
		"original_key": img.OriginalKey,
		"filename":     img.Filename,
		"mime_type":    img.MimeType,
		"size":         img.Size,
		"width":        img.Width,
		"height":       img.Height,
		"resolutions":  strings.Join(img.Resolutions, ","),
		"created_at":   img.CreatedAt.Format(time.RFC3339),
		"updated_at":   img.UpdatedAt.Format(time.RFC3339),
	}
}

// fieldsToMetadata converts Redis hash fields to ImageMetadata
func (r *RedisRepository) fieldsToMetadata(fields map[string]string) (*models.ImageMetadata, error) {
	img := &models.ImageMetadata{}

	// Required fields
	img.ID = fields["id"]
	img.OriginalKey = fields["original_key"]
	img.Filename = fields["filename"]
	img.MimeType = fields["mime_type"]

	// Parse numeric fields
	if size, err := strconv.ParseInt(fields["size"], 10, 64); err == nil {
		img.Size = size
	}

	if width, err := strconv.Atoi(fields["width"]); err == nil {
		img.Width = width
	}

	if height, err := strconv.Atoi(fields["height"]); err == nil {
		img.Height = height
	}

	// Parse resolutions
	if resolutionsStr := fields["resolutions"]; resolutionsStr != "" {
		img.Resolutions = strings.Split(resolutionsStr, ",")
	}

	// Parse timestamps
	if createdAtStr := fields["created_at"]; createdAtStr != "" {
		if createdAt, err := time.Parse(time.RFC3339, createdAtStr); err == nil {
			img.CreatedAt = createdAt
		}
	}

	if updatedAtStr := fields["updated_at"]; updatedAtStr != "" {
		if updatedAt, err := time.Parse(time.RFC3339, updatedAtStr); err == nil {
			img.UpdatedAt = updatedAt
		}
	}

	return img, nil
}

// findKeysByPattern finds all keys matching a pattern
func (r *RedisRepository) findKeysByPattern(ctx context.Context, pattern string) ([]string, error) {
	var cursor uint64
	var keys []string

	for {
		var scanKeys []string
		var err error

		scanKeys, cursor, err = r.client.Scan(ctx, cursor, pattern, 100).Result()
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

// countImages counts total number of images
func (r *RedisRepository) countImages(ctx context.Context) (int64, error) {
	keys, err := r.findKeysByPattern(ctx, r.getMetadataKey("*"))
	if err != nil {
		return 0, err
	}
	return int64(len(keys)), nil
}

// countCacheKeys counts total number of cache keys
func (r *RedisRepository) countCacheKeys(ctx context.Context) int64 {
	keys, err := r.findKeysByPattern(context.Background(), "image:cache:*")
	if err != nil {
		return 0
	}
	return int64(len(keys))
}

// parseInfoValue parses numeric value from Redis INFO output
func (r *RedisRepository) parseInfoValue(info, key string) int64 {
	lines := strings.Split(info, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, key+":") {
			valueStr := strings.TrimPrefix(line, key+":")
			valueStr = strings.TrimSpace(valueStr)
			if value, err := strconv.ParseInt(valueStr, 10, 64); err == nil {
				return value
			}
		}
	}
	return 0
}
