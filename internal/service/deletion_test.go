package service

import (
	"context"
	"io"
	"testing"
	"time"

	"resizr/internal/config"
	"resizr/internal/models"
	"resizr/internal/repository"
	"resizr/internal/storage"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockDeletionStorage implements storage.ImageStorage for deletion tests
type MockDeletionStorage struct {
	deleteCalled      bool
	deleteError       error
	deleteFolderError error
}

func (m *MockDeletionStorage) Upload(ctx context.Context, key string, reader io.Reader, size int64, mimeType string) error {
	return nil
}

func (m *MockDeletionStorage) Get(ctx context.Context, key string) ([]byte, error) {
	return nil, nil
}

func (m *MockDeletionStorage) Delete(ctx context.Context, key string) error {
	m.deleteCalled = true
	return m.deleteError
}

func (m *MockDeletionStorage) DeleteFolder(ctx context.Context, prefix string) error {
	return m.deleteFolderError
}

func (m *MockDeletionStorage) Exists(ctx context.Context, key string) (bool, error) {
	return false, nil
}

func (m *MockDeletionStorage) GeneratePresignedURL(ctx context.Context, key string, duration time.Duration) (string, error) {
	return "", nil
}

func (m *MockDeletionStorage) HealthCheck(ctx context.Context) error {
	return nil
}

func (m *MockDeletionStorage) CopyObject(ctx context.Context, sourceKey, destKey string) error {
	return nil
}

func (m *MockDeletionStorage) Download(ctx context.Context, key string) (io.ReadCloser, error) {
	return nil, nil
}

func (m *MockDeletionStorage) GetMetadata(ctx context.Context, key string) (*storage.FileMetadata, error) {
	return nil, nil
}

func (m *MockDeletionStorage) GetURL(key string) string {
	return ""
}

func (m *MockDeletionStorage) Health(ctx context.Context) error {
	return nil
}

func (m *MockDeletionStorage) ListObjects(ctx context.Context, prefix string, maxKeys int) ([]storage.ObjectInfo, error) {
	return nil, nil
}

// MockDeletionRepository with atomic reference methods
type MockDeletionRepository struct {
	metadata        *models.ImageMetadata
	dedupInfo       *models.DeduplicationInfo
	decrementResult map[string]*repository.ResolutionRefCount
	decrementError  error
	addRefError     error
}

func (m *MockDeletionRepository) Store(ctx context.Context, metadata *models.ImageMetadata) error {
	m.metadata = metadata
	return nil
}

func (m *MockDeletionRepository) Get(ctx context.Context, id string) (*models.ImageMetadata, error) {
	if m.metadata != nil {
		return m.metadata, nil
	}
	return nil, models.NotFoundError{Resource: "image", ID: id}
}

func (m *MockDeletionRepository) Update(ctx context.Context, metadata *models.ImageMetadata) error {
	m.metadata = metadata
	return nil
}

func (m *MockDeletionRepository) Delete(ctx context.Context, id string) error {
	m.metadata = nil
	return nil
}

func (m *MockDeletionRepository) List(ctx context.Context, offset, limit int) ([]*models.ImageMetadata, error) {
	return nil, nil
}

func (m *MockDeletionRepository) Exists(ctx context.Context, id string) (bool, error) {
	return m.metadata != nil, nil
}

func (m *MockDeletionRepository) HealthCheck(ctx context.Context) error {
	return nil
}

func (m *MockDeletionRepository) Health(ctx context.Context) error {
	return nil
}

func (m *MockDeletionRepository) Close() error {
	return nil
}

func (m *MockDeletionRepository) GetStats(ctx context.Context) (*repository.RepositoryStats, error) {
	return nil, nil
}

func (m *MockDeletionRepository) UpdateResolutions(ctx context.Context, id string, resolutions []string) error {
	return nil
}

func (m *MockDeletionRepository) GetImageStatistics(ctx context.Context) (*models.ImageStatistics, error) {
	return nil, nil
}

func (m *MockDeletionRepository) GetImageCountByFormat(ctx context.Context) (map[string]int64, error) {
	return nil, nil
}

func (m *MockDeletionRepository) GetTotalStorageUsed(ctx context.Context) (int64, error) {
	return 0, nil
}

func (m *MockDeletionRepository) GetTotalImageCount(ctx context.Context) (int64, error) {
	return 0, nil
}

func (m *MockDeletionRepository) StoreDeduplicationInfo(ctx context.Context, info *models.DeduplicationInfo) error {
	m.dedupInfo = info
	return nil
}

func (m *MockDeletionRepository) GetDeduplicationInfo(ctx context.Context, hash models.ImageHash) (*models.DeduplicationInfo, error) {
	if m.dedupInfo != nil {
		return m.dedupInfo, nil
	}
	return nil, models.NotFoundError{Resource: "dedup_info", ID: hash.String()}
}

func (m *MockDeletionRepository) UpdateDeduplicationInfo(ctx context.Context, info *models.DeduplicationInfo) error {
	m.dedupInfo = info
	return nil
}

func (m *MockDeletionRepository) DeleteDeduplicationInfo(ctx context.Context, hash models.ImageHash) error {
	m.dedupInfo = nil
	return nil
}

func (m *MockDeletionRepository) FindImageByHash(ctx context.Context, hash models.ImageHash) (*models.DeduplicationInfo, error) {
	return m.GetDeduplicationInfo(ctx, hash)
}

func (m *MockDeletionRepository) AddHashReference(ctx context.Context, hash models.ImageHash, imageID string) error {
	return nil
}

func (m *MockDeletionRepository) RemoveHashReference(ctx context.Context, hash models.ImageHash, imageID string) error {
	return nil
}

func (m *MockDeletionRepository) GetOrphanedHashes(ctx context.Context) ([]models.ImageHash, error) {
	return nil, nil
}

func (m *MockDeletionRepository) GetDeduplicationStatistics(ctx context.Context) (*models.DeduplicationStatistics, error) {
	return nil, nil
}

func (m *MockDeletionRepository) GetDuplicateCount(ctx context.Context) (int64, error) {
	return 0, nil
}

func (m *MockDeletionRepository) GetUniqueHashCount(ctx context.Context) (int64, error) {
	return 0, nil
}

func (m *MockDeletionRepository) GetStorageSavedByDeduplication(ctx context.Context) (int64, error) {
	return 0, nil
}

func (m *MockDeletionRepository) GetHashStatistics(ctx context.Context) ([]models.HashStat, error) {
	return nil, nil
}

func (m *MockDeletionRepository) DecrementResolutionRefs(ctx context.Context, hash models.ImageHash, imageID string, resolutions []string) (map[string]*repository.ResolutionRefCount, error) {
	if m.decrementError != nil {
		return nil, m.decrementError
	}
	if m.decrementResult != nil {
		return m.decrementResult, nil
	}
	// Default: all resolutions should be deleted
	result := make(map[string]*repository.ResolutionRefCount)
	for _, res := range resolutions {
		result[res] = &repository.ResolutionRefCount{
			Count:          0,
			ReferencingIDs: []string{},
			ShouldDelete:   true,
		}
	}
	return result, nil
}

func (m *MockDeletionRepository) AddResolutionRef(ctx context.Context, hash models.ImageHash, resolution, imageID string) error {
	return m.addRefError
}

func (m *MockDeletionRepository) GetImagesByTimeRange(ctx context.Context, start, end time.Time) (int64, error) {
	return 0, nil
}

func (m *MockDeletionRepository) GetResolutionStatistics(ctx context.Context) ([]models.ResolutionStat, error) {
	return nil, nil
}

func (m *MockDeletionRepository) GetStorageUsageByResolution(ctx context.Context) (map[string]int64, error) {
	return nil, nil
}

func (m *MockDeletionRepository) GetStorageStatistics(ctx context.Context) (*models.StorageStatistics, error) {
	return nil, nil
}

func TestDeleteImage_WithDeduplication_LastReference(t *testing.T) {
	storage := &MockDeletionStorage{}
	repo := &MockDeletionRepository{}
	dedupRepo := &MockDeletionRepository{}

	hash := models.ImageHash{
		Algorithm: "SHA256",
		Value:     "test-hash",
		Size:      1024,
	}

	repo.metadata = &models.ImageMetadata{
		ID:            "image-1",
		Filename:      "test.jpg",
		Hash:          hash,
		IsDeduped:     true,
		SharedImageID: "image-master",
		Resolutions:   []string{"thumbnail", "1920x1080"},
	}

	dedupRepo.dedupInfo = &models.DeduplicationInfo{
		Hash:           hash,
		MasterImageID:  "image-master",
		ReferencingIDs: []string{"image-1"},
		ReferenceCount: 1,
	}

	// Last reference - all resolutions should be deleted
	dedupRepo.decrementResult = map[string]*repository.ResolutionRefCount{
		"original":  {Count: 0, ShouldDelete: true},
		"thumbnail": {Count: 0, ShouldDelete: true},
		"1920x1080": {Count: 0, ShouldDelete: true},
	}

	cfg := &config.Config{
		Deletion: config.DeletionConfig{
			AsyncMode: false, // Sync for testing
		},
	}

	service := NewImageService(repo, dedupRepo, storage, &mockProcessorService{}, cfg)
	ctx := context.Background()

	err := service.DeleteImage(ctx, "image-1")
	require.NoError(t, err)

	// Verify storage delete was called
	assert.True(t, storage.deleteCalled)
}

func TestDeleteImage_WithDeduplication_SharedReferences(t *testing.T) {
	storage := &MockDeletionStorage{}
	repo := &MockDeletionRepository{}
	dedupRepo := &MockDeletionRepository{}

	hash := models.ImageHash{
		Algorithm: "SHA256",
		Value:     "test-hash",
		Size:      1024,
	}

	repo.metadata = &models.ImageMetadata{
		ID:            "image-1",
		Filename:      "test.jpg",
		Hash:          hash,
		IsDeduped:     true,
		SharedImageID: "image-master",
		Resolutions:   []string{"thumbnail"},
	}

	dedupRepo.dedupInfo = &models.DeduplicationInfo{
		Hash:           hash,
		MasterImageID:  "image-master",
		ReferencingIDs: []string{"image-1", "image-2"},
		ReferenceCount: 2,
	}

	// Still has references - nothing should be physically deleted
	dedupRepo.decrementResult = map[string]*repository.ResolutionRefCount{
		"original":  {Count: 1, ReferencingIDs: []string{"image-2"}, ShouldDelete: false},
		"thumbnail": {Count: 1, ReferencingIDs: []string{"image-2"}, ShouldDelete: false},
	}

	cfg := &config.Config{
		Deletion: config.DeletionConfig{
			AsyncMode: false,
		},
	}

	service := NewImageService(repo, dedupRepo, storage, &mockProcessorService{}, cfg)
	ctx := context.Background()

	err := service.DeleteImage(ctx, "image-1")
	require.NoError(t, err)

	// Verify storage delete was NOT called (files still shared)
	assert.False(t, storage.deleteCalled)
}

func TestDeleteResolution_WithDeduplication(t *testing.T) {
	storage := &MockDeletionStorage{}
	repo := &MockDeletionRepository{}
	dedupRepo := &MockDeletionRepository{}

	hash := models.ImageHash{
		Algorithm: "SHA256",
		Value:     "test-hash",
		Size:      1024,
	}

	repo.metadata = &models.ImageMetadata{
		ID:            "image-1",
		Filename:      "test.jpg",
		Hash:          hash,
		IsDeduped:     true,
		SharedImageID: "image-master",
		Resolutions:   []string{"thumbnail", "1920x1080"},
	}

	// Last reference to thumbnail - should be deleted
	dedupRepo.decrementResult = map[string]*repository.ResolutionRefCount{
		"thumbnail": {Count: 0, ShouldDelete: true},
	}

	cfg := &config.Config{
		Deletion: config.DeletionConfig{
			AsyncMode: false,
		},
	}

	service := NewImageService(repo, dedupRepo, storage, &mockProcessorService{}, cfg)
	ctx := context.Background()

	err := service.DeleteResolution(ctx, "image-1", "thumbnail")
	require.NoError(t, err)

	// Verify storage delete was called
	assert.True(t, storage.deleteCalled)

	// Verify resolution removed from metadata
	assert.NotContains(t, repo.metadata.Resolutions, "thumbnail")
	assert.Contains(t, repo.metadata.Resolutions, "1920x1080")
}

func TestDeleteResolution_CannotDeleteOriginal(t *testing.T) {
	storage := &MockDeletionStorage{}
	repo := &MockDeletionRepository{}
	dedupRepo := &MockDeletionRepository{}

	repo.metadata = &models.ImageMetadata{
		ID:          "image-1",
		Filename:    "test.jpg",
		Resolutions: []string{"thumbnail"},
	}

	cfg := &config.Config{
		Deletion: config.DeletionConfig{
			AsyncMode: false,
		},
	}

	service := NewImageService(repo, dedupRepo, storage, &mockProcessorService{}, cfg)
	ctx := context.Background()

	err := service.DeleteResolution(ctx, "image-1", "original")
	require.Error(t, err)

	// Should be validation error
	_, ok := err.(models.ValidationError)
	assert.True(t, ok)
}

func TestDeleteResolution_NotFound(t *testing.T) {
	storage := &MockDeletionStorage{}
	repo := &MockDeletionRepository{}
	dedupRepo := &MockDeletionRepository{}

	repo.metadata = &models.ImageMetadata{
		ID:          "image-1",
		Filename:    "test.jpg",
		Resolutions: []string{"thumbnail"},
	}

	cfg := &config.Config{
		Deletion: config.DeletionConfig{
			AsyncMode: false,
		},
	}

	service := NewImageService(repo, dedupRepo, storage, &mockProcessorService{}, cfg)
	ctx := context.Background()

	err := service.DeleteResolution(ctx, "image-1", "nonexistent")
	require.Error(t, err)

	// Should be not found error
	_, ok := err.(models.NotFoundError)
	assert.True(t, ok)
}

type mockProcessorService struct{}

func (m *mockProcessorService) DetectFormat(data []byte) (string, error) {
	return "image/jpeg", nil
}

func (m *mockProcessorService) GetDimensions(data []byte) (width, height int, err error) {
	return 1920, 1080, nil
}

func (m *mockProcessorService) ProcessImage(data []byte, config ResizeConfig) ([]byte, error) {
	return data, nil
}

func (m *mockProcessorService) ValidateImage(data []byte, maxSize int64) error {
	return nil
}
