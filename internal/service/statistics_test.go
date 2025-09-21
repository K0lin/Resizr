package service

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"resizr/internal/config"
	"resizr/internal/models"
	"resizr/internal/repository"
	"resizr/internal/storage"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockImageRepository implements repository.ImageRepository for testing
type MockImageRepository struct {
	mock.Mock
}

func (m *MockImageRepository) Store(ctx context.Context, img *models.ImageMetadata) error {
	args := m.Called(ctx, img)
	return args.Error(0)
}

func (m *MockImageRepository) Get(ctx context.Context, id string) (*models.ImageMetadata, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(*models.ImageMetadata), args.Error(1)
}

func (m *MockImageRepository) Update(ctx context.Context, img *models.ImageMetadata) error {
	args := m.Called(ctx, img)
	return args.Error(0)
}

func (m *MockImageRepository) Delete(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockImageRepository) Exists(ctx context.Context, id string) (bool, error) {
	args := m.Called(ctx, id)
	return args.Bool(0), args.Error(1)
}

func (m *MockImageRepository) List(ctx context.Context, offset, limit int) ([]*models.ImageMetadata, error) {
	args := m.Called(ctx, offset, limit)
	return args.Get(0).([]*models.ImageMetadata), args.Error(1)
}

func (m *MockImageRepository) UpdateResolutions(ctx context.Context, id string, resolutions []string) error {
	args := m.Called(ctx, id, resolutions)
	return args.Error(0)
}

func (m *MockImageRepository) GetStats(ctx context.Context) (*repository.RepositoryStats, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.RepositoryStats), args.Error(1)
}

func (m *MockImageRepository) GetImageStatistics(ctx context.Context) (*models.ImageStatistics, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.ImageStatistics), args.Error(1)
}

func (m *MockImageRepository) GetStorageStatistics(ctx context.Context) (*models.StorageStatistics, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.StorageStatistics), args.Error(1)
}

func (m *MockImageRepository) GetImageCountByFormat(ctx context.Context) (map[string]int64, error) {
	args := m.Called(ctx)
	return args.Get(0).(map[string]int64), args.Error(1)
}

func (m *MockImageRepository) GetResolutionStatistics(ctx context.Context) ([]models.ResolutionStat, error) {
	args := m.Called(ctx)
	return args.Get(0).([]models.ResolutionStat), args.Error(1)
}

func (m *MockImageRepository) GetImagesByTimeRange(ctx context.Context, start, end time.Time) (int64, error) {
	args := m.Called(ctx, start, end)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockImageRepository) GetStorageUsageByResolution(ctx context.Context) (map[string]int64, error) {
	args := m.Called(ctx)
	return args.Get(0).(map[string]int64), args.Error(1)
}

func (m *MockImageRepository) Health(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockImageRepository) Close() error {
	args := m.Called()
	return args.Error(0)
}

// MockDeduplicationRepository implements repository.DeduplicationRepository for testing
type MockDeduplicationRepository struct {
	mock.Mock
}

func (m *MockDeduplicationRepository) StoreDeduplicationInfo(ctx context.Context, info *models.DeduplicationInfo) error {
	args := m.Called(ctx, info)
	return args.Error(0)
}

func (m *MockDeduplicationRepository) GetDeduplicationInfo(ctx context.Context, hash models.ImageHash) (*models.DeduplicationInfo, error) {
	args := m.Called(ctx, hash)
	return args.Get(0).(*models.DeduplicationInfo), args.Error(1)
}

func (m *MockDeduplicationRepository) UpdateDeduplicationInfo(ctx context.Context, info *models.DeduplicationInfo) error {
	args := m.Called(ctx, info)
	return args.Error(0)
}

func (m *MockDeduplicationRepository) DeleteDeduplicationInfo(ctx context.Context, hash models.ImageHash) error {
	args := m.Called(ctx, hash)
	return args.Error(0)
}

func (m *MockDeduplicationRepository) FindImageByHash(ctx context.Context, hash models.ImageHash) (*models.DeduplicationInfo, error) {
	args := m.Called(ctx, hash)
	return args.Get(0).(*models.DeduplicationInfo), args.Error(1)
}

func (m *MockDeduplicationRepository) AddHashReference(ctx context.Context, hash models.ImageHash, imageID string) error {
	args := m.Called(ctx, hash, imageID)
	return args.Error(0)
}

func (m *MockDeduplicationRepository) RemoveHashReference(ctx context.Context, hash models.ImageHash, imageID string) error {
	args := m.Called(ctx, hash, imageID)
	return args.Error(0)
}

func (m *MockDeduplicationRepository) GetOrphanedHashes(ctx context.Context) ([]models.ImageHash, error) {
	args := m.Called(ctx)
	return args.Get(0).([]models.ImageHash), args.Error(1)
}

func (m *MockDeduplicationRepository) GetDeduplicationStatistics(ctx context.Context) (*models.DeduplicationStatistics, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.DeduplicationStatistics), args.Error(1)
}

func (m *MockDeduplicationRepository) GetHashStatistics(ctx context.Context) ([]models.HashStat, error) {
	args := m.Called(ctx)
	return args.Get(0).([]models.HashStat), args.Error(1)
}

func (m *MockDeduplicationRepository) GetDuplicateCount(ctx context.Context) (int64, error) {
	args := m.Called(ctx)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockDeduplicationRepository) GetUniqueHashCount(ctx context.Context) (int64, error) {
	args := m.Called(ctx)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockDeduplicationRepository) GetStorageSavedByDeduplication(ctx context.Context) (int64, error) {
	args := m.Called(ctx)
	return args.Get(0).(int64), args.Error(1)
}

// MockImageStorage implements storage.ImageStorage for testing
type MockImageStorage struct {
	mock.Mock
}

func (m *MockImageStorage) Store(ctx context.Context, key string, data []byte) error {
	args := m.Called(ctx, key, data)
	return args.Error(0)
}

func (m *MockImageStorage) Get(ctx context.Context, key string) ([]byte, error) {
	args := m.Called(ctx, key)
	return args.Get(0).([]byte), args.Error(1)
}

func (m *MockImageStorage) Delete(ctx context.Context, key string) error {
	args := m.Called(ctx, key)
	return args.Error(0)
}

func (m *MockImageStorage) Exists(ctx context.Context, key string) (bool, error) {
	args := m.Called(ctx, key)
	return args.Bool(0), args.Error(1)
}

func (m *MockImageStorage) GeneratePresignedURL(ctx context.Context, key string, duration time.Duration) (string, error) {
	args := m.Called(ctx, key, duration)
	return args.String(0), args.Error(1)
}

func (m *MockImageStorage) Health(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockImageStorage) GetMetadata(ctx context.Context, key string) (*storage.FileMetadata, error) {
	args := m.Called(ctx, key)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*storage.FileMetadata), args.Error(1)
}

func (m *MockImageStorage) ListObjects(ctx context.Context, prefix string, maxKeys int) ([]storage.ObjectInfo, error) {
	args := m.Called(ctx, prefix, maxKeys)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]storage.ObjectInfo), args.Error(1)
}

func (m *MockImageStorage) GetObjectSize(ctx context.Context, key string) (int64, error) {
	args := m.Called(ctx, key)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockImageStorage) CopyObject(ctx context.Context, srcKey, destKey string) error {
	args := m.Called(ctx, srcKey, destKey)
	return args.Error(0)
}

func (m *MockImageStorage) DeleteFolder(ctx context.Context, prefix string) error {
	args := m.Called(ctx, prefix)
	return args.Error(0)
}

func (m *MockImageStorage) Download(ctx context.Context, key string) (io.ReadCloser, error) {
	args := m.Called(ctx, key)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(io.ReadCloser), args.Error(1)
}

func (m *MockImageStorage) Upload(ctx context.Context, key string, reader io.Reader, size int64, contentType string) error {
	args := m.Called(ctx, key, reader, size, contentType)
	return args.Error(0)
}

func (m *MockImageStorage) GetURL(key string) string {
	args := m.Called(key)
	return args.String(0)
}

// Test fixtures
func createTestConfig() *config.Config {
	return &config.Config{
		Statistics: config.StatisticsConfig{
			CacheEnabled: true,
			CacheTTL:     5 * time.Minute,
		},
	}
}

func createTestService() (*StatisticsServiceImpl, *MockImageRepository, *MockDeduplicationRepository, *MockImageStorage) {
	mockImageRepo := &MockImageRepository{}
	mockDedupRepo := &MockDeduplicationRepository{}
	mockStorage := &MockImageStorage{}
	cfg := createTestConfig()

	service := NewStatisticsService(
		mockImageRepo,
		mockDedupRepo,
		mockStorage,
		cfg,
	).(*StatisticsServiceImpl)

	return service, mockImageRepo, mockDedupRepo, mockStorage
}

func TestNewStatisticsService(t *testing.T) {
	service, _, _, _ := createTestService()

	assert.NotNil(t, service)
	assert.NotNil(t, service.cache)
	assert.True(t, service.config.Statistics.CacheEnabled)
	assert.Equal(t, 5*time.Minute, service.config.Statistics.CacheTTL)
}

func TestGetImageStatistics_Success(t *testing.T) {
	service, mockImageRepo, _, _ := createTestService()

	expectedStats := &models.ImageStatistics{
		TotalImages:      100,
		ImagesByFormat:   map[string]int64{"jpeg": 60, "png": 40},
		ResolutionCounts: map[string]int64{},
		TopResolutions:   []models.ResolutionStat{},
	}

	mockImageRepo.On("GetImageStatistics", mock.Anything).Return(expectedStats, nil)

	result, err := service.GetImageStatistics()

	assert.NoError(t, err)
	assert.Equal(t, expectedStats, result)
	mockImageRepo.AssertExpectations(t)
}

func TestGetImageStatistics_FallbackToBasicStats(t *testing.T) {
	service, mockImageRepo, _, _ := createTestService()

	// First call fails, should fallback to GetStats and calculate basic stats
	mockImageRepo.On("GetImageStatistics", mock.Anything).Return(nil, errors.New("detailed stats not available"))

	repoStats := &repository.RepositoryStats{
		TotalImages: 50,
		CacheHits:   100,
		CacheMisses: 20,
		StorageUsed: 1024000,
		Connections: repository.ConnectionStats{Active: 1, Idle: 0, Total: 1},
		KeyCounts:   map[string]int64{"metadata": 50, "cache": 10},
	}
	mockImageRepo.On("GetStats", mock.Anything).Return(repoStats, nil).Once()

	// Mock the additional calls that GetImageStatistics makes in fallback mode
	mockImageRepo.On("GetImagesByTimeRange", mock.Anything, mock.Anything, mock.Anything).Return(int64(5), nil).Times(3)
	mockImageRepo.On("GetImageCountByFormat", mock.Anything).Return(map[string]int64{"jpeg": 30, "png": 20}, nil)
	mockImageRepo.On("GetResolutionStatistics", mock.Anything).Return([]models.ResolutionStat{}, nil)

	result, err := service.GetImageStatistics()

	assert.NoError(t, err)
	assert.Equal(t, int64(50), result.TotalImages)
	assert.Equal(t, map[string]int64{"jpeg": 30, "png": 20}, result.ImagesByFormat)
	mockImageRepo.AssertExpectations(t)
}

func TestGetStorageStatistics_Success(t *testing.T) {
	service, mockImageRepo, _, _ := createTestService()

	expectedStats := &models.StorageStatistics{
		TotalStorageUsed:    1024000,
		OriginalImagesSize:  512000,
		ProcessedImagesSize: 512000,
		StorageByResolution: map[string]int64{"original": 512000, "thumbnail": 256000},
	}

	mockImageRepo.On("GetStorageStatistics", mock.Anything).Return(expectedStats, nil)

	result, err := service.GetStorageStatistics()

	assert.NoError(t, err)
	assert.Equal(t, expectedStats, result)
	mockImageRepo.AssertExpectations(t)
}

func TestGetDeduplicationStatistics_Success(t *testing.T) {
	service, _, mockDedupRepo, _ := createTestService()

	expectedStats := &models.DeduplicationStatistics{
		TotalDuplicatesFound:     25,
		DedupedImages:            25,
		UniqueImages:             75,
		DeduplicationRate:        25.0,
		AverageReferencesPerHash: 2,
	}

	mockDedupRepo.On("GetDeduplicationStatistics", mock.Anything).Return(expectedStats, nil)

	result, err := service.GetDeduplicationStatistics()

	assert.NoError(t, err)
	assert.Equal(t, expectedStats, result)
	mockDedupRepo.AssertExpectations(t)
}

func TestGetComprehensiveStatistics_WithCacheDisabled(t *testing.T) {
	service, mockImageRepo, mockDedupRepo, _ := createTestService()
	service.config.Statistics.CacheEnabled = false

	// Mock all individual statistics calls
	imageStats := &models.ImageStatistics{TotalImages: 100}
	storageStats := &models.StorageStatistics{TotalStorageUsed: 1024000}
	dedupStats := &models.DeduplicationStatistics{UniqueImages: 75}
	mockImageRepo.On("GetImageStatistics", mock.Anything).Return(imageStats, nil).Once()
	mockImageRepo.On("GetStorageStatistics", mock.Anything).Return(storageStats, nil).Once()
	mockDedupRepo.On("GetDeduplicationStatistics", mock.Anything).Return(dedupStats, nil).Once()
	// Since all stats calls succeed, GetStats should not be called

	result, err := service.GetComprehensiveStatistics(nil)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, int64(100), result.Images.TotalImages)
	assert.Equal(t, int64(1024000), result.Storage.TotalStorageUsed)
	assert.Equal(t, int64(75), result.Deduplication.UniqueImages)
	mockImageRepo.AssertExpectations(t)
	mockDedupRepo.AssertExpectations(t)
}

func TestGetComprehensiveStatistics_WithCacheEnabled(t *testing.T) {
	service, mockImageRepo, mockDedupRepo, _ := createTestService()

	// Mock all individual statistics calls for first request (cache miss)
	imageStats := &models.ImageStatistics{TotalImages: 100}
	storageStats := &models.StorageStatistics{TotalStorageUsed: 1024000}
	dedupStats := &models.DeduplicationStatistics{UniqueImages: 75}
	mockImageRepo.On("GetImageStatistics", mock.Anything).Return(imageStats, nil).Once()
	mockImageRepo.On("GetStorageStatistics", mock.Anything).Return(storageStats, nil).Once()
	mockDedupRepo.On("GetDeduplicationStatistics", mock.Anything).Return(dedupStats, nil).Once()
	// Since all stats calls succeed, GetStats should not be called

	// First call - should generate and cache
	result1, err1 := service.GetComprehensiveStatistics(nil)
	assert.NoError(t, err1)
	assert.NotNil(t, result1)

	// Second call - should return cached result (no mock calls expected)
	result2, err2 := service.GetComprehensiveStatistics(nil)
	assert.NoError(t, err2)
	assert.NotNil(t, result2)
	assert.Equal(t, result1.Images.TotalImages, result2.Images.TotalImages)

	mockImageRepo.AssertExpectations(t)
	mockDedupRepo.AssertExpectations(t)
}

func TestGetComprehensiveStatistics_CacheExpiry(t *testing.T) {
	service, mockImageRepo, mockDedupRepo, _ := createTestService()
	service.config.Statistics.CacheTTL = 100 * time.Millisecond // Very short TTL

	// Mock statistics calls
	imageStats := &models.ImageStatistics{TotalImages: 100}
	storageStats := &models.StorageStatistics{TotalStorageUsed: 1024000}
	dedupStats := &models.DeduplicationStatistics{UniqueImages: 75}

	// Mock calls should happen twice due to cache expiry
	mockImageRepo.On("GetImageStatistics", mock.Anything).Return(imageStats, nil).Twice()
	mockImageRepo.On("GetStorageStatistics", mock.Anything).Return(storageStats, nil).Twice()
	mockDedupRepo.On("GetDeduplicationStatistics", mock.Anything).Return(dedupStats, nil).Twice()
	// Since all stats calls succeed, GetStats should not be called

	// First call
	result1, err1 := service.GetComprehensiveStatistics(nil)
	assert.NoError(t, err1)
	assert.NotNil(t, result1)

	// Wait for cache to expire
	time.Sleep(150 * time.Millisecond)

	// Second call - cache should be expired, should regenerate
	result2, err2 := service.GetComprehensiveStatistics(nil)
	assert.NoError(t, err2)
	assert.NotNil(t, result2)

	mockImageRepo.AssertExpectations(t)
	mockDedupRepo.AssertExpectations(t)
}

func TestRefreshStatistics_WithCacheEnabled(t *testing.T) {
	service, mockImageRepo, mockDedupRepo, _ := createTestService()

	// Pre-populate cache
	imageStats := &models.ImageStatistics{TotalImages: 100}
	mockImageRepo.On("GetImageStatistics", mock.Anything).Return(imageStats, nil).Once()
	mockImageRepo.On("GetStorageStatistics", mock.Anything).Return(&models.StorageStatistics{}, nil).Once()
	mockDedupRepo.On("GetDeduplicationStatistics", mock.Anything).Return(&models.DeduplicationStatistics{}, nil).Once()
	// Since GetImageStatistics and GetStorageStatistics succeed, GetStats should not be called
	// mockImageRepo.On("GetStats", mock.Anything).Return(&repository.RepositoryStats{Connections: repository.ConnectionStats{Active: 1}}, nil) - No expectation

	// Generate initial cached data
	_, err := service.GetComprehensiveStatistics(nil)
	assert.NoError(t, err)

	// Verify cache has data
	cached := service.getCachedStatistics()
	assert.NotNil(t, cached)

	// Refresh should clear cache
	err = service.RefreshStatistics()
	assert.NoError(t, err)

	// Cache should be empty after refresh
	cached = service.getCachedStatistics()
	assert.Nil(t, cached)

	mockImageRepo.AssertExpectations(t)
	mockDedupRepo.AssertExpectations(t)
}

func TestRefreshStatistics_WithCacheDisabled(t *testing.T) {
	service, _, _, _ := createTestService()
	service.config.Statistics.CacheEnabled = false

	err := service.RefreshStatistics()
	assert.NoError(t, err)
}

func TestGetComprehensiveStatistics_WithOptions(t *testing.T) {
	service, mockImageRepo, mockDedupRepo, _ := createTestService()
	service.config.Statistics.CacheEnabled = false // Disable cache for this test

	// Options to exclude performance and system metrics
	options := &models.StatisticsOptions{
		IncludePerformanceMetrics: false,
		IncludeSystemMetrics:      false,
	}

	imageStats := &models.ImageStatistics{TotalImages: 100}
	storageStats := &models.StorageStatistics{TotalStorageUsed: 1024000}
	dedupStats := &models.DeduplicationStatistics{UniqueImages: 75}

	mockImageRepo.On("GetImageStatistics", mock.Anything).Return(imageStats, nil)
	mockImageRepo.On("GetStorageStatistics", mock.Anything).Return(storageStats, nil)
	mockDedupRepo.On("GetDeduplicationStatistics", mock.Anything).Return(dedupStats, nil)
	// Note: GetStats should NOT be called since performance metrics are excluded

	result, err := service.GetComprehensiveStatistics(options)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, int64(100), result.Images.TotalImages)
	// System stats should have values
	assert.Equal(t, 0, result.System.CPUCount)

	mockImageRepo.AssertExpectations(t)
	mockDedupRepo.AssertExpectations(t)
}

func TestGetComprehensiveStatistics_HandleErrors(t *testing.T) {
	service, mockImageRepo, mockDedupRepo, _ := createTestService()
	service.config.Statistics.CacheEnabled = false

	// Mock all calls to return errors for comprehensive statistics
	mockImageRepo.On("GetImageStatistics", mock.Anything).Return(nil, errors.New("image stats error"))
	mockImageRepo.On("GetStats", mock.Anything).Return(nil, errors.New("repo stats error"))
	mockImageRepo.On("GetStorageStatistics", mock.Anything).Return(nil, errors.New("storage stats error"))
	mockDedupRepo.On("GetDeduplicationStatistics", mock.Anything).Return(nil, errors.New("dedup stats error"))

	// Mock the fallback calls for deduplication statistics
	mockDedupRepo.On("GetDuplicateCount", mock.Anything).Return(int64(0), errors.New("duplicate count error"))
	mockDedupRepo.On("GetUniqueHashCount", mock.Anything).Return(int64(0), errors.New("unique count error"))
	mockDedupRepo.On("GetHashStatistics", mock.Anything).Return([]models.HashStat{}, errors.New("hash stats error"))

	result, err := service.GetComprehensiveStatistics(nil)

	// Should not return error, but provide partial stats
	assert.NoError(t, err)
	assert.NotNil(t, result)
	// All stats should be zero/empty due to errors
	assert.Equal(t, int64(0), result.Images.TotalImages)
	assert.Equal(t, int64(0), result.Storage.TotalStorageUsed)
	assert.Equal(t, int64(0), result.Deduplication.TotalDuplicatesFound)

	mockImageRepo.AssertExpectations(t)
	mockDedupRepo.AssertExpectations(t)
}
