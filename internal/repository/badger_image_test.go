package repository

import (
	"context"
	"testing"
	"time"

	"resizr/internal/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBadgerImageRepository_StoreAndGet(t *testing.T) {
	tmpDir := t.TempDir()

	cacheConfig := &CacheConfig{
		Type:      CacheTypeBadger,
		Directory: tmpDir,
	}

	repo, err := NewBadgerImageRepository(cacheConfig)
	require.NoError(t, err)
	defer repo.Close()

	ctx := context.Background()
	img := &models.ImageMetadata{
		ID:       "f47ac10b-58cc-4372-a567-0e02b2c3d479",
		Filename: "test.jpg",
		MimeType: "image/jpeg",
		Size:     1024,
		Width:    800,
		Height:   600,
	}

	err = repo.Store(ctx, img)
	assert.NoError(t, err)

	retrieved, err := repo.Get(ctx, "f47ac10b-58cc-4372-a567-0e02b2c3d479")
	assert.NoError(t, err)
	assert.NotNil(t, retrieved)
	assert.Equal(t, img.ID, retrieved.ID)
	assert.Equal(t, img.Filename, retrieved.Filename)
}

func TestBadgerImageRepository_Update(t *testing.T) {
	tmpDir := t.TempDir()

	cacheConfig := &CacheConfig{
		Type:      CacheTypeBadger,
		Directory: tmpDir,
	}

	repo, err := NewBadgerImageRepository(cacheConfig)
	require.NoError(t, err)
	defer repo.Close()

	ctx := context.Background()
	img := &models.ImageMetadata{
		ID:       "f47ac10b-58cc-4372-a567-0e02b2c3d479",
		Filename: "test.jpg",
		MimeType: "image/jpeg",
		Size:     1024,
		Width:    800,
		Height:   600,
	}

	err = repo.Store(ctx, img)
	assert.NoError(t, err)

	img.Filename = "new.jpg"
	err = repo.Update(ctx, img)
	assert.NoError(t, err)

	retrieved, err := repo.Get(ctx, "f47ac10b-58cc-4372-a567-0e02b2c3d479")
	assert.NoError(t, err)
	assert.NotNil(t, retrieved)
	assert.Equal(t, "new.jpg", retrieved.Filename)
}

func TestBadgerImageRepository_Delete(t *testing.T) {
	tmpDir := t.TempDir()

	cacheConfig := &CacheConfig{
		Type:      CacheTypeBadger,
		Directory: tmpDir,
	}

	repo, err := NewBadgerImageRepository(cacheConfig)
	require.NoError(t, err)
	defer repo.Close()

	ctx := context.Background()
	img := &models.ImageMetadata{
		ID:       "f47ac10b-58cc-4372-a567-0e02b2c3d479",
		Filename: "test.jpg",
		MimeType: "image/jpeg",
		Size:     1024,
		Width:    800,
		Height:   600,
	}

	err = repo.Store(ctx, img)
	assert.NoError(t, err)

	err = repo.Delete(ctx, "f47ac10b-58cc-4372-a567-0e02b2c3d479")
	assert.NoError(t, err)

	_, err = repo.Get(ctx, "f47ac10b-58cc-4372-a567-0e02b2c3d479")
	assert.Error(t, err)
}

func TestBadgerImageRepository_Exists(t *testing.T) {
	tmpDir := t.TempDir()

	cacheConfig := &CacheConfig{
		Type:      CacheTypeBadger,
		Directory: tmpDir,
	}

	repo, err := NewBadgerImageRepository(cacheConfig)
	require.NoError(t, err)
	defer repo.Close()

	ctx := context.Background()
	img := &models.ImageMetadata{
		ID:       "f47ac10b-58cc-4372-a567-0e02b2c3d479",
		Filename: "test.jpg",
		MimeType: "image/jpeg",
		Size:     1024,
		Width:    800,
		Height:   600,
	}

	err = repo.Store(ctx, img)
	assert.NoError(t, err)

	exists, err := repo.Exists(ctx, "f47ac10b-58cc-4372-a567-0e02b2c3d479")
	assert.NoError(t, err)
	assert.True(t, exists)

	exists, err = repo.Exists(ctx, "non-existent")
	assert.NoError(t, err)
	assert.False(t, exists)
}

func TestBadgerImageRepository_List(t *testing.T) {
	tmpDir := t.TempDir()

	cacheConfig := &CacheConfig{
		Type:      CacheTypeBadger,
		Directory: tmpDir,
	}

	repo, err := NewBadgerImageRepository(cacheConfig)
	require.NoError(t, err)
	defer repo.Close()

	ctx := context.Background()
	img1 := &models.ImageMetadata{
		ID:       "f47ac10b-58cc-4372-a567-0e02b2c3d479",
		Filename: "test1.jpg",
		MimeType: "image/jpeg",
		Size:     1024,
		Width:    800,
		Height:   600,
	}
	img2 := &models.ImageMetadata{
		ID:       "a47ac10b-58cc-4372-a567-0e02b2c3d479",
		Filename: "test2.jpg",
		MimeType: "image/jpeg",
		Size:     1024,
		Width:    800,
		Height:   600,
	}

	err = repo.Store(ctx, img1)
	assert.NoError(t, err)
	err = repo.Store(ctx, img2)
	assert.NoError(t, err)

	images, err := repo.List(ctx, 0, 10)
	assert.NoError(t, err)
	assert.Len(t, images, 2)
}

func TestBadgerImageRepository_UpdateResolutions(t *testing.T) {
	tmpDir := t.TempDir()

	cacheConfig := &CacheConfig{
		Type:      CacheTypeBadger,
		Directory: tmpDir,
	}

	repo, err := NewBadgerImageRepository(cacheConfig)
	require.NoError(t, err)
	defer repo.Close()

	ctx := context.Background()
	img := &models.ImageMetadata{
		ID:       "f47ac10b-58cc-4372-a567-0e02b2c3d479",
		Filename: "test.jpg",
		MimeType: "image/jpeg",
		Size:     1024,
		Width:    800,
		Height:   600,
	}

	err = repo.Store(ctx, img)
	assert.NoError(t, err)

	resolutions := []string{"800x600", "1024x768"}
	err = repo.UpdateResolutions(ctx, "f47ac10b-58cc-4372-a567-0e02b2c3d479", resolutions)
	assert.NoError(t, err)

	retrieved, err := repo.Get(ctx, "f47ac10b-58cc-4372-a567-0e02b2c3d479")
	assert.NoError(t, err)
	assert.NotNil(t, retrieved)
	assert.Equal(t, resolutions, retrieved.Resolutions)
}

func TestBadgerImageRepository_GetStats(t *testing.T) {
	tmpDir := t.TempDir()

	cacheConfig := &CacheConfig{
		Type:      CacheTypeBadger,
		Directory: tmpDir,
	}

	repo, err := NewBadgerImageRepository(cacheConfig)
	require.NoError(t, err)
	defer repo.Close()

	ctx := context.Background()
	img := &models.ImageMetadata{
		ID:       "f47ac10b-58cc-4372-a567-0e02b2c3d479",
		Filename: "test.jpg",
		MimeType: "image/jpeg",
		Size:     1024,
		Width:    800,
		Height:   600,
	}

	err = repo.Store(ctx, img)
	assert.NoError(t, err)

	stats, err := repo.GetStats(ctx)
	assert.NoError(t, err)
	assert.NotNil(t, stats)
	assert.Equal(t, int64(1), stats.TotalImages)
}

func TestBadgerImageRepository_StoreAndGetDeduplicationInfo(t *testing.T) {
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
		Value:     "test-hash",
		Size:      1024,
	}
	info := &models.DeduplicationInfo{
		Hash:          hash,
		MasterImageID: "master-image-1",
	}

	err = repo.StoreDeduplicationInfo(ctx, info)
	assert.NoError(t, err)

	retrieved, err := repo.GetDeduplicationInfo(ctx, hash)
	assert.NoError(t, err)
	assert.NotNil(t, retrieved)
	assert.Equal(t, info.MasterImageID, retrieved.MasterImageID)
}

func TestBadgerImageRepository_DeleteDeduplicationInfo(t *testing.T) {
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
		Value:     "test-hash",
		Size:      1024,
	}
	info := &models.DeduplicationInfo{
		Hash:          hash,
		MasterImageID: "master-image-1",
	}

	err = repo.StoreDeduplicationInfo(ctx, info)
	assert.NoError(t, err)

	err = repo.DeleteDeduplicationInfo(ctx, hash)
	assert.NoError(t, err)

	_, err = repo.GetDeduplicationInfo(ctx, hash)
	assert.Error(t, err)
}

func TestBadgerImageRepository_AddAndRemoveHashReference(t *testing.T) {
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
		Value:     "test-hash",
		Size:      1024,
	}
	info := &models.DeduplicationInfo{
		Hash:          hash,
		MasterImageID: "master-image-1",
	}

	err = repo.StoreDeduplicationInfo(ctx, info)
	assert.NoError(t, err)

	err = repo.AddHashReference(ctx, hash, "image-1")
	assert.NoError(t, err)

	retrieved, err := repo.GetDeduplicationInfo(ctx, hash)
	assert.NoError(t, err)
	assert.Contains(t, retrieved.ReferencingIDs, "image-1")

	err = repo.RemoveHashReference(ctx, hash, "image-1")
	assert.NoError(t, err)

	_, err = repo.GetDeduplicationInfo(ctx, hash)
	assert.Error(t, err) // Should be deleted because it's orphaned
}

func TestBadgerImageRepository_GetOrphanedHashes(t *testing.T) {
	tmpDir := t.TempDir()

	cacheConfig := &CacheConfig{
		Type:      CacheTypeBadger,
		Directory: tmpDir,
	}

	repo, err := NewBadgerImageRepository(cacheConfig)
	require.NoError(t, err)
	defer repo.Close()

	ctx := context.Background()
	hash1 := models.ImageHash{Algorithm: "SHA256", Value: "hash1"}
	hash2 := models.ImageHash{Algorithm: "SHA256", Value: "hash2"}

	info1 := &models.DeduplicationInfo{Hash: hash1, MasterImageID: "image-1", ReferencingIDs: []string{"image-1"}, ReferenceCount: 1}
	info2 := &models.DeduplicationInfo{Hash: hash2, MasterImageID: "image-2"}

	err = repo.StoreDeduplicationInfo(ctx, info1)
	assert.NoError(t, err)
	err = repo.StoreDeduplicationInfo(ctx, info2)
	assert.NoError(t, err)

	orphaned, err := repo.GetOrphanedHashes(ctx)
	assert.NoError(t, err)
	assert.Len(t, orphaned, 1)
	assert.Equal(t, hash2.Value, orphaned[0].Value)
}

func TestBadgerImageRepository_GetImageCountByFormat(t *testing.T) {
	tmpDir := t.TempDir()

	cacheConfig := &CacheConfig{
		Type:      CacheTypeBadger,
		Directory: tmpDir,
	}

	repo, err := NewBadgerImageRepository(cacheConfig)
	require.NoError(t, err)
	defer repo.Close()

	ctx := context.Background()
	img1 := &models.ImageMetadata{ID: "f47ac10b-58cc-4372-a567-0e02b2c3d479", MimeType: "image/jpeg", Filename: "test1.jpg", Size: 1, Width: 1, Height: 1}
	img2 := &models.ImageMetadata{ID: "a47ac10b-58cc-4372-a567-0e02b2c3d479", MimeType: "image/png", Filename: "test2.png", Size: 1, Width: 1, Height: 1}
	img3 := &models.ImageMetadata{ID: "b47ac10b-58cc-4372-a567-0e02b2c3d479", MimeType: "image/jpeg", Filename: "test3.jpg", Size: 1, Width: 1, Height: 1}

	err = repo.Store(ctx, img1)
	assert.NoError(t, err)
	err = repo.Store(ctx, img2)
	assert.NoError(t, err)
	err = repo.Store(ctx, img3)
	assert.NoError(t, err)

	counts, err := repo.GetImageCountByFormat(ctx)
	assert.NoError(t, err)
	assert.Equal(t, int64(2), counts["jpeg"])
	assert.Equal(t, int64(1), counts["png"])
}

func TestBadgerImageRepository_GetImageStatistics(t *testing.T) {
	tmpDir := t.TempDir()

	cacheConfig := &CacheConfig{
		Type:      CacheTypeBadger,
		Directory: tmpDir,
	}

	repo, err := NewBadgerImageRepository(cacheConfig)
	require.NoError(t, err)
	defer repo.Close()

	ctx := context.Background()
	img1 := &models.ImageMetadata{ID: "f47ac10b-58cc-4372-a567-0e02b2c3d479", MimeType: "image/jpeg", Filename: "test1.jpg", Size: 1, Width: 1, Height: 1, Resolutions: []string{"original", "thumbnail"}, CreatedAt: time.Now()}
	img2 := &models.ImageMetadata{ID: "a47ac10b-58cc-4372-a567-0e02b2c3d479", MimeType: "image/png", Filename: "test2.png", Size: 1, Width: 1, Height: 1, Resolutions: []string{"original"}, CreatedAt: time.Now()}

	err = repo.Store(ctx, img1)
	assert.NoError(t, err)
	err = repo.Store(ctx, img2)
	assert.NoError(t, err)

	stats, err := repo.GetImageStatistics(ctx)
	assert.NoError(t, err)
	assert.NotNil(t, stats)
	assert.Equal(t, int64(2), stats.TotalImages)
	assert.Equal(t, int64(1), stats.ImagesByFormat["jpeg"])
	assert.Equal(t, int64(1), stats.ImagesByFormat["png"])
}

func TestBadgerImageRepository_GetStorageStatistics(t *testing.T) {
	tmpDir := t.TempDir()

	cacheConfig := &CacheConfig{
		Type:      CacheTypeBadger,
		Directory: tmpDir,
	}

	repo, err := NewBadgerImageRepository(cacheConfig)
	require.NoError(t, err)
	defer repo.Close()

	ctx := context.Background()
	img1 := &models.ImageMetadata{ID: "f47ac10b-58cc-4372-a567-0e02b2c3d479", MimeType: "image/jpeg", Filename: "test1.jpg", Size: 100, Width: 1, Height: 1, Resolutions: []string{"original", "thumbnail"}}

	err = repo.Store(ctx, img1)
	assert.NoError(t, err)

	stats, err := repo.GetStorageStatistics(ctx)
	assert.NoError(t, err)
	assert.NotNil(t, stats)
	assert.Equal(t, int64(170), stats.TotalStorageUsed)
}
