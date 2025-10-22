package repository

import (
	"context"
	"testing"

	"resizr/internal/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Redis tests removed - will be added later when Redis is available

func TestBadgerRepository_AddResolutionRef(t *testing.T) {
	tmpDir := t.TempDir()

	cacheConfig := &CacheConfig{
		Type:      CacheTypeBadger,
		Directory: tmpDir,
	}

	repo, err := NewBadgerImageRepository(cacheConfig)
	require.NoError(t, err)
	defer repo.Close()

	ctx := context.Background()
	hash := models.ImageHash{
		Algorithm: "SHA256",
		Value:     "badger-test-hash",
		Size:      1024,
	}

	// Add first reference
	err = repo.AddResolutionRef(ctx, hash, "thumbnail", "image-1")
	assert.NoError(t, err)

	// Add second reference
	err = repo.AddResolutionRef(ctx, hash, "thumbnail", "image-2")
	assert.NoError(t, err)

	// Decrement and verify
	result, err := repo.DecrementResolutionRefs(ctx, hash, "image-1", []string{"thumbnail"})
	require.NoError(t, err)
	assert.Equal(t, int64(1), result["thumbnail"].Count)
	assert.Contains(t, result["thumbnail"].ReferencingIDs, "image-2")
	assert.False(t, result["thumbnail"].ShouldDelete)
}

func TestBadgerRepository_DecrementResolutionRefs_Concurrent(t *testing.T) {
	tmpDir := t.TempDir()

	cacheConfig := &CacheConfig{
		Type:      CacheTypeBadger,
		Directory: tmpDir,
	}

	repo, err := NewBadgerImageRepository(cacheConfig)
	require.NoError(t, err)
	defer repo.Close()

	ctx := context.Background()
	hash := models.ImageHash{
		Algorithm: "SHA256",
		Value:     "concurrent-test-hash",
		Size:      1024,
	}

	// Add multiple references
	for i := 0; i < 10; i++ {
		err = repo.AddResolutionRef(ctx, hash, "thumbnail", "image-"+string(rune('0'+i)))
		require.NoError(t, err)
	}

	// Decrement concurrently (Badger should handle conflicts)
	done := make(chan bool, 5)
	for i := 0; i < 5; i++ {
		go func(idx int) {
			_, err := repo.DecrementResolutionRefs(ctx, hash, "image-"+string(rune('0'+idx)), []string{"thumbnail"})
			assert.NoError(t, err)
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 5; i++ {
		<-done
	}

	// Verify final count
	result, err := repo.DecrementResolutionRefs(ctx, hash, "image-5", []string{"thumbnail"})
	require.NoError(t, err)
	assert.Equal(t, int64(4), result["thumbnail"].Count) // 10 - 6 = 4
}

func TestBadgerRepository_AddResolutionRef_Idempotent(t *testing.T) {
	tmpDir := t.TempDir()

	cacheConfig := &CacheConfig{
		Type:      CacheTypeBadger,
		Directory: tmpDir,
	}

	repo, err := NewBadgerImageRepository(cacheConfig)
	require.NoError(t, err)
	defer repo.Close()

	ctx := context.Background()
	hash := models.ImageHash{
		Algorithm: "SHA256",
		Value:     "idempotent-test-hash",
		Size:      1024,
	}

	// Add same reference twice
	err = repo.AddResolutionRef(ctx, hash, "thumbnail", "image-1")
	assert.NoError(t, err)

	err = repo.AddResolutionRef(ctx, hash, "thumbnail", "image-1")
	assert.NoError(t, err)

	// Should only have one reference
	result, err := repo.DecrementResolutionRefs(ctx, hash, "image-1", []string{"thumbnail"})
	require.NoError(t, err)
	assert.Equal(t, int64(0), result["thumbnail"].Count)
	assert.True(t, result["thumbnail"].ShouldDelete)
}
