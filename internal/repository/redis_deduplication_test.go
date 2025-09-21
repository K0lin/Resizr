package repository

import (
	"context"
	"os"
	"testing"
	"time"

	"resizr/internal/config"
	"resizr/internal/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// skipIfRedisUnavailable skips the test if Redis is not available
func skipIfRedisUnavailable(t *testing.T) {
	// Skip if running in CI environment
	if os.Getenv("CI") == "true" || os.Getenv("GITHUB_ACTIONS") == "true" {
		t.Skip("Skipping Redis tests in CI environment")
	}

	// Try a quick connection test
	testConfig := &config.RedisConfig{
		URL:      "redis://localhost:6379/1",
		Password: "",
		DB:       1,
		PoolSize: 1,
		Timeout:  1000, // Short timeout for availability check
	}

	repo, err := NewRedisRepository(testConfig)
	if err != nil {
		t.Skipf("Skipping Redis tests: Redis unavailable (%v)", err)
	}
	_ = repo.Close()
}

// NewTestRedisRepository creates a Redis repository for testing
func NewTestRedisRepository(t *testing.T) ImageRepository {
	skipIfRedisUnavailable(t)

	// Use a test Redis configuration
	testConfig := &config.RedisConfig{
		URL:      "redis://localhost:6379/1", // Use DB 1 for tests
		Password: "",
		DB:       1,
		PoolSize: 5,
		Timeout:  5000,
	}

	repo, err := NewRedisRepository(testConfig)
	require.NoError(t, err, "Failed to create test Redis repository")

	// Clean up after test
	t.Cleanup(func() {
		_ = repo.Close()
	})

	return repo
}

// TestRedisRepository_DeduplicationFields tests the serialization/deserialization of deduplication fields
func TestRedisRepository_DeduplicationFields(t *testing.T) {
	t.Run("store_and_retrieve_deduplication_metadata", func(t *testing.T) {
		repo := NewTestRedisRepository(t)
		ctx := context.Background()

		// Create metadata with deduplication fields
		metadata := &models.ImageMetadata{
			ID:          "test-image-id",
			OriginalKey: "images/test-image-id/original.jpg",
			Filename:    "test.jpg",
			MimeType:    "image/jpeg",
			Size:        1024,
			Width:       800,
			Height:      600,
			Resolutions: []string{"original", "800x600", "100x100"},
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
			Hash: models.ImageHash{
				Value:     "test-hash-value",
				Algorithm: "SHA256",
				Size:      1024,
			},
			IsDeduped:     true,
			SharedImageID: "master-image-id",
		}

		// Store metadata
		err := repo.Store(ctx, metadata)
		require.NoError(t, err)

		// Retrieve metadata
		retrieved, err := repo.Get(ctx, metadata.ID)
		require.NoError(t, err)
		require.NotNil(t, retrieved)

		// Verify deduplication fields are preserved
		assert.Equal(t, metadata.ID, retrieved.ID)
		assert.Equal(t, metadata.Hash.Value, retrieved.Hash.Value)
		assert.Equal(t, metadata.Hash.Algorithm, retrieved.Hash.Algorithm)
		assert.Equal(t, metadata.Hash.Size, retrieved.Hash.Size)
		assert.Equal(t, metadata.IsDeduped, retrieved.IsDeduped)
		assert.Equal(t, metadata.SharedImageID, retrieved.SharedImageID)
	})

	t.Run("store_and_retrieve_non_deduplication_metadata", func(t *testing.T) {
		repo := NewTestRedisRepository(t)
		ctx := context.Background()

		// Create metadata without deduplication fields
		metadata := &models.ImageMetadata{
			ID:          "test-image-id-2",
			OriginalKey: "images/test-image-id-2/original.jpg",
			Filename:    "test2.jpg",
			MimeType:    "image/png",
			Size:        2048,
			Width:       1024,
			Height:      768,
			Resolutions: []string{"original", "800x600"},
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
			Hash:        models.ImageHash{}, // Empty hash
			IsDeduped:   false,
		}

		// Store metadata
		err := repo.Store(ctx, metadata)
		require.NoError(t, err)

		// Retrieve metadata
		retrieved, err := repo.Get(ctx, metadata.ID)
		require.NoError(t, err)
		require.NotNil(t, retrieved)

		// Verify deduplication fields are correctly handled
		assert.Equal(t, metadata.ID, retrieved.ID)
		assert.Equal(t, "", retrieved.Hash.Value)      // Should be empty
		assert.Equal(t, "", retrieved.Hash.Algorithm)  // Should be empty
		assert.Equal(t, int64(0), retrieved.Hash.Size) // Should be zero
		assert.Equal(t, false, retrieved.IsDeduped)    // Should be false
		assert.Equal(t, "", retrieved.SharedImageID)   // Should be empty
	})

	t.Run("backward_compatibility_empty_fields", func(t *testing.T) {
		repo := NewTestRedisRepository(t)
		ctx := context.Background()

		// Simulate old data without deduplication fields
		imageID := "legacy-image-id"

		// Manually set Redis data without deduplication fields (simulating old data)
		redisRepo := repo.(*RedisRepository)
		conn := redisRepo.client

		legacyData := map[string]interface{}{
			"id":           imageID,
			"original_key": "images/" + imageID + "/original.jpg",
			"filename":     "legacy.jpg",
			"mime_type":    "image/jpeg",
			"size":         "1024",
			"width":        "800",
			"height":       "600",
			"resolutions":  "original,800x600",
			"created_at":   time.Now().Format(time.RFC3339),
			"updated_at":   time.Now().Format(time.RFC3339),
			// Note: No hash, is_deduped, or shared_image_id fields
		}

		_, err := conn.HMSet(ctx, "image:metadata:"+imageID, legacyData).Result()
		require.NoError(t, err)

		// Retrieve metadata
		retrieved, err := repo.Get(ctx, imageID)
		require.NoError(t, err)
		require.NotNil(t, retrieved)

		// Verify backward compatibility - missing fields should have safe defaults
		assert.Equal(t, imageID, retrieved.ID)
		assert.Equal(t, "", retrieved.Hash.Value)
		assert.Equal(t, "", retrieved.Hash.Algorithm)
		assert.Equal(t, int64(0), retrieved.Hash.Size)
		assert.Equal(t, false, retrieved.IsDeduped)
		assert.Equal(t, "", retrieved.SharedImageID)
	})

	t.Run("update_metadata_preserves_deduplication_fields", func(t *testing.T) {
		repo := NewTestRedisRepository(t)
		ctx := context.Background()

		// Create initial metadata
		metadata := &models.ImageMetadata{
			ID:          "test-image-id",
			OriginalKey: "images/test-image-id/original.jpg",
			Filename:    "test.jpg",
			MimeType:    "image/jpeg",
			Size:        1024,
			Width:       800,
			Height:      600,
			Resolutions: []string{"original"},
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
			Hash: models.ImageHash{
				Value:     "test-hash-value",
				Algorithm: "SHA256",
				Size:      1024,
			},
			IsDeduped:     true,
			SharedImageID: "master-image-id",
		}

		// Store initial metadata
		err := repo.Store(ctx, metadata)
		require.NoError(t, err)

		// Update metadata (add resolution)
		metadata.Resolutions = []string{"original", "800x600"}
		metadata.UpdatedAt = time.Now()

		err = repo.Update(ctx, metadata)
		require.NoError(t, err)

		// Retrieve updated metadata
		retrieved, err := repo.Get(ctx, metadata.ID)
		require.NoError(t, err)
		require.NotNil(t, retrieved)

		// Verify deduplication fields are still preserved after update
		assert.Equal(t, metadata.ID, retrieved.ID)
		assert.Equal(t, metadata.Hash.Value, retrieved.Hash.Value)
		assert.Equal(t, metadata.Hash.Algorithm, retrieved.Hash.Algorithm)
		assert.Equal(t, metadata.Hash.Size, retrieved.Hash.Size)
		assert.Equal(t, metadata.IsDeduped, retrieved.IsDeduped)
		assert.Equal(t, metadata.SharedImageID, retrieved.SharedImageID)
		assert.Equal(t, []string{"original", "800x600"}, retrieved.Resolutions)
	})
}

// TestRedisRepository_HashSerialization tests hash field serialization/deserialization
func TestRedisRepository_HashSerialization(t *testing.T) {
	t.Run("serialize_deserialize_image_hash", func(t *testing.T) {
		repo := NewTestRedisRepository(t)
		ctx := context.Background()

		testCases := []models.ImageHash{
			{
				Value:     "sha256-hash-value",
				Algorithm: "SHA256",
				Size:      1024,
			},
			{
				Value:     "md5-hash-value",
				Algorithm: "MD5",
				Size:      2048,
			},
			{
				Value:     "",
				Algorithm: "",
				Size:      0,
			},
		}

		for i, hash := range testCases {
			imageID := "test-image-" + string(rune(i+'0'))

			metadata := &models.ImageMetadata{
				ID:          imageID,
				OriginalKey: "images/" + imageID + "/original.jpg",
				Filename:    "test.jpg",
				MimeType:    "image/jpeg",
				Size:        1024,
				Width:       800,
				Height:      600,
				Resolutions: []string{"original"},
				CreatedAt:   time.Now(),
				UpdatedAt:   time.Now(),
				Hash:        hash,
				IsDeduped:   hash.Value != "", // Set deduped based on whether hash exists
			}

			// Store metadata
			err := repo.Store(ctx, metadata)
			require.NoError(t, err)

			// Retrieve metadata
			retrieved, err := repo.Get(ctx, imageID)
			require.NoError(t, err)
			require.NotNil(t, retrieved)

			// Verify hash fields
			assert.Equal(t, hash.Value, retrieved.Hash.Value, "Hash value should match")
			assert.Equal(t, hash.Algorithm, retrieved.Hash.Algorithm, "Hash algorithm should match")
			assert.Equal(t, hash.Size, retrieved.Hash.Size, "Hash size should match")
		}
	})

	t.Run("default_algorithm_for_missing_algorithm", func(t *testing.T) {
		repo := NewTestRedisRepository(t)
		ctx := context.Background()

		// Create metadata with hash but missing algorithm
		metadata := &models.ImageMetadata{
			ID:          "test-image-id",
			OriginalKey: "images/test-image-id/original.jpg",
			Filename:    "test.jpg",
			MimeType:    "image/jpeg",
			Size:        1024,
			Width:       800,
			Height:      600,
			Resolutions: []string{"original"},
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
			Hash: models.ImageHash{
				Value: "test-hash-value",
				// Algorithm intentionally missing
				Size: 1024,
			},
			IsDeduped: true,
		}

		// Store metadata
		err := repo.Store(ctx, metadata)
		require.NoError(t, err)

		// Retrieve metadata
		retrieved, err := repo.Get(ctx, metadata.ID)
		require.NoError(t, err)
		require.NotNil(t, retrieved)

		// Verify default algorithm is set
		assert.Equal(t, "SHA256", retrieved.Hash.Algorithm, "Should default to SHA256 when algorithm is missing")
		assert.Equal(t, metadata.Hash.Value, retrieved.Hash.Value)
		assert.Equal(t, metadata.Hash.Size, retrieved.Hash.Size)
	})
}

// TestRedisRepository_DeduplicationFlag tests the IsDeduped flag handling
func TestRedisRepository_DeduplicationFlag(t *testing.T) {
	t.Run("deduplication_flag_true", func(t *testing.T) {
		repo := NewTestRedisRepository(t)
		ctx := context.Background()

		metadata := &models.ImageMetadata{
			ID:          "test-image-id",
			OriginalKey: "images/test-image-id/original.jpg",
			Filename:    "test.jpg",
			MimeType:    "image/jpeg",
			Size:        1024,
			Width:       800,
			Height:      600,
			Resolutions: []string{"original"},
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
			Hash: models.ImageHash{
				Value:     "test-hash-value",
				Algorithm: "SHA256",
				Size:      1024,
			},
			IsDeduped:     true,
			SharedImageID: "master-image-id",
		}

		// Store metadata
		err := repo.Store(ctx, metadata)
		require.NoError(t, err)

		// Retrieve metadata
		retrieved, err := repo.Get(ctx, metadata.ID)
		require.NoError(t, err)
		require.NotNil(t, retrieved)

		// Verify deduplication flag
		assert.True(t, retrieved.IsDeduped)
		assert.Equal(t, "master-image-id", retrieved.SharedImageID)
	})

	t.Run("deduplication_flag_false", func(t *testing.T) {
		repo := NewTestRedisRepository(t)
		ctx := context.Background()

		metadata := &models.ImageMetadata{
			ID:          "test-image-id",
			OriginalKey: "images/test-image-id/original.jpg",
			Filename:    "test.jpg",
			MimeType:    "image/jpeg",
			Size:        1024,
			Width:       800,
			Height:      600,
			Resolutions: []string{"original"},
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
			Hash:        models.ImageHash{}, // Empty hash
			IsDeduped:   false,
		}

		// Store metadata
		err := repo.Store(ctx, metadata)
		require.NoError(t, err)

		// Retrieve metadata
		retrieved, err := repo.Get(ctx, metadata.ID)
		require.NoError(t, err)
		require.NotNil(t, retrieved)

		// Verify deduplication flag
		assert.False(t, retrieved.IsDeduped)
		assert.Empty(t, retrieved.SharedImageID)
	})
}
