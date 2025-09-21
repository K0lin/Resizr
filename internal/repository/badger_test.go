package repository

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewBadgerRepository(t *testing.T) {
	// Create temporary directory for test
	tempDir, err := os.MkdirTemp("", "badger_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	cfg := &CacheConfig{
		Type:      CacheTypeBadger,
		Directory: tempDir,
		TTL:       5 * time.Minute,
	}

	repo, err := NewBadgerRepository(cfg)
	assert.NoError(t, err)
	assert.NotNil(t, repo)

	// Clean up
	err = repo.Close()
	assert.NoError(t, err)
}

func TestBadgerRepository_SetCachedURL(t *testing.T) {
	// Create temporary directory for test
	tempDir, err := os.MkdirTemp("", "badger_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	cfg := &CacheConfig{
		Type:      CacheTypeBadger,
		Directory: tempDir,
		TTL:       5 * time.Minute,
	}
	repo, err := NewBadgerRepository(cfg)
	require.NoError(t, err)
	defer repo.Close()

	ctx := context.Background()
	imageID := "test-image-123"
	resolution := "800x600"
	url := "https://example.com/image.jpg"
	ttl := 5 * time.Minute

	err = repo.SetCachedURL(ctx, imageID, resolution, url, ttl)
	assert.NoError(t, err)
}

func TestBadgerRepository_GetCachedURL(t *testing.T) {
	// Create temporary directory for test
	tempDir, err := os.MkdirTemp("", "badger_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	cfg := &CacheConfig{
		Type:      CacheTypeBadger,
		Directory: tempDir,
		TTL:       5 * time.Minute,
	}
	repo, err := NewBadgerRepository(cfg)
	require.NoError(t, err)
	defer repo.Close()

	ctx := context.Background()
	imageID := "test-image-123"
	resolution := "800x600"
	url := "https://example.com/image.jpg"
	ttl := 5 * time.Minute

	// Set URL first
	err = repo.SetCachedURL(ctx, imageID, resolution, url, ttl)
	require.NoError(t, err)

	// Get URL
	retrievedURL, err := repo.GetCachedURL(ctx, imageID, resolution)
	assert.NoError(t, err)
	assert.Equal(t, url, retrievedURL)
}

func TestBadgerRepository_GetCachedURL_NotFound(t *testing.T) {
	// Create temporary directory for test
	tempDir, err := os.MkdirTemp("", "badger_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	cfg := &CacheConfig{
		Type:      CacheTypeBadger,
		Directory: tempDir,
		TTL:       5 * time.Minute,
	}
	repo, err := NewBadgerRepository(cfg)
	require.NoError(t, err)
	defer repo.Close()

	ctx := context.Background()
	imageID := "nonexistent-image"
	resolution := "800x600"

	// Get non-existent URL
	retrievedURL, err := repo.GetCachedURL(ctx, imageID, resolution)
	assert.Error(t, err)
	assert.Empty(t, retrievedURL)
}

func TestBadgerRepository_DeleteCachedURL(t *testing.T) {
	// Create temporary directory for test
	tempDir, err := os.MkdirTemp("", "badger_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	cfg := &CacheConfig{
		Type:      CacheTypeBadger,
		Directory: tempDir,
		TTL:       5 * time.Minute,
	}
	repo, err := NewBadgerRepository(cfg)
	require.NoError(t, err)
	defer repo.Close()

	ctx := context.Background()
	imageID := "test-image-123"
	resolution := "800x600"
	url := "https://example.com/image.jpg"
	ttl := 5 * time.Minute

	// Set URL first
	err = repo.SetCachedURL(ctx, imageID, resolution, url, ttl)
	require.NoError(t, err)

	// Delete URL
	err = repo.DeleteCachedURL(ctx, imageID, resolution)
	assert.NoError(t, err)

	// Verify it's deleted
	retrievedURL, err := repo.GetCachedURL(ctx, imageID, resolution)
	assert.Error(t, err)
	assert.Empty(t, retrievedURL)
}

func TestBadgerRepository_Set(t *testing.T) {
	// Create temporary directory for test
	tempDir, err := os.MkdirTemp("", "badger_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	cfg := &CacheConfig{
		Type:      CacheTypeBadger,
		Directory: tempDir,
		TTL:       5 * time.Minute,
	}
	repo, err := NewBadgerRepository(cfg)
	require.NoError(t, err)
	defer repo.Close()

	ctx := context.Background()
	key := "test-key"
	value := "test-value"
	ttl := 5 * time.Minute

	err = repo.Set(ctx, key, value, ttl)
	assert.NoError(t, err)
}

func TestBadgerRepository_Get(t *testing.T) {
	// Create temporary directory for test
	tempDir, err := os.MkdirTemp("", "badger_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	cfg := &CacheConfig{
		Type:      CacheTypeBadger,
		Directory: tempDir,
		TTL:       5 * time.Minute,
	}
	repo, err := NewBadgerRepository(cfg)
	require.NoError(t, err)
	defer repo.Close()

	ctx := context.Background()
	key := "test-key"
	value := "test-value"
	ttl := 5 * time.Minute

	// Set value first
	err = repo.Set(ctx, key, value, ttl)
	require.NoError(t, err)

	// Get value
	retrievedValue, err := repo.Get(ctx, key)
	assert.NoError(t, err)
	assert.Equal(t, value, retrievedValue)
}

func TestBadgerRepository_Delete(t *testing.T) {
	// Create temporary directory for test
	tempDir, err := os.MkdirTemp("", "badger_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	cfg := &CacheConfig{
		Type:      CacheTypeBadger,
		Directory: tempDir,
		TTL:       5 * time.Minute,
	}
	repo, err := NewBadgerRepository(cfg)
	require.NoError(t, err)
	defer repo.Close()

	ctx := context.Background()
	key := "test-key"
	value := "test-value"
	ttl := 5 * time.Minute

	// Set value first
	err = repo.Set(ctx, key, value, ttl)
	require.NoError(t, err)

	// Delete value
	err = repo.Delete(ctx, key)
	assert.NoError(t, err)

	// Verify it's deleted
	retrievedValue, err := repo.Get(ctx, key)
	assert.Error(t, err)
	assert.Empty(t, retrievedValue)
}

func TestBadgerRepository_Health(t *testing.T) {
	// Create temporary directory for test
	tempDir, err := os.MkdirTemp("", "badger_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	cfg := &CacheConfig{
		Type:      CacheTypeBadger,
		Directory: tempDir,
		TTL:       5 * time.Minute,
	}
	repo, err := NewBadgerRepository(cfg)
	require.NoError(t, err)
	defer repo.Close()

	ctx := context.Background()

	err = repo.Health(ctx)
	assert.NoError(t, err)
}

func TestBadgerRepository_GetStats(t *testing.T) {
	// Create temporary directory for test
	tempDir, err := os.MkdirTemp("", "badger_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	cfg := &CacheConfig{
		Type:      CacheTypeBadger,
		Directory: tempDir,
		TTL:       5 * time.Minute,
	}
	repo, err := NewBadgerRepository(cfg)
	require.NoError(t, err)
	defer repo.Close()

	ctx := context.Background()

	stats, err := repo.GetStats(ctx)
	assert.NoError(t, err)
	assert.NotNil(t, stats)
	assert.GreaterOrEqual(t, stats.KeyCount, int64(0))
}
