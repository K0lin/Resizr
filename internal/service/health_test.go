package service

import (
	"context"
	"errors"
	"io"
	"runtime"
	"testing"
	"time"

	"resizr/internal/models"
	"resizr/internal/repository"
	"resizr/internal/storage"
	"resizr/internal/testutil"

	"github.com/stretchr/testify/assert"
)

// Local mocks for health service testing
type mockImageRepository struct {
	healthFunc   func(ctx context.Context) error
	closeFunc    func() error
	getStatsFunc func(ctx context.Context) (*repository.RepositoryStats, error)
}

func (m *mockImageRepository) Save(_ctx context.Context, _metadata *models.ImageMetadata) error {
	return nil
}
func (m *mockImageRepository) GetByID(_ctx context.Context, _id string) (*models.ImageMetadata, error) {
	return nil, nil
}
func (m *mockImageRepository) Get(_ctx context.Context, _id string) (*models.ImageMetadata, error) {
	return nil, nil
}
func (m *mockImageRepository) Store(_ctx context.Context, _metadata *models.ImageMetadata) error {
	return nil
}
func (m *mockImageRepository) Update(_ctx context.Context, _metadata *models.ImageMetadata) error {
	return nil
}
func (m *mockImageRepository) Delete(_ctx context.Context, _id string) error { return nil }
func (m *mockImageRepository) Exists(_ctx context.Context, _id string) (bool, error) {
	return false, nil
}
func (m *mockImageRepository) List(_ctx context.Context, _offset, _limit int) ([]*models.ImageMetadata, error) {
	return nil, nil
}
func (m *mockImageRepository) HealthCheck(ctx context.Context) error {
	if m.healthFunc != nil {
		return m.healthFunc(ctx)
	}
	return nil
}
func (m *mockImageRepository) Health(ctx context.Context) error { return m.HealthCheck(ctx) }
func (m *mockImageRepository) Close() error {
	if m.closeFunc != nil {
		return m.closeFunc()
	}
	return nil
}
func (m *mockImageRepository) GetStats(ctx context.Context) (*repository.RepositoryStats, error) {
	if m.getStatsFunc != nil {
		return m.getStatsFunc(ctx)
	}
	return nil, nil
}
func (m *mockImageRepository) UpdateResolutions(ctx context.Context, imageID string, resolutions []string) error {
	return nil
}

// Deduplication methods
func (m *mockImageRepository) StoreDeduplicationInfo(ctx context.Context, info *models.DeduplicationInfo) error {
	return nil
}
func (m *mockImageRepository) GetDeduplicationInfo(ctx context.Context, hash models.ImageHash) (*models.DeduplicationInfo, error) {
	return nil, nil
}
func (m *mockImageRepository) UpdateDeduplicationInfo(ctx context.Context, info *models.DeduplicationInfo) error {
	return nil
}
func (m *mockImageRepository) DeleteDeduplicationInfo(ctx context.Context, hash models.ImageHash) error {
	return nil
}
func (m *mockImageRepository) FindImageByHash(ctx context.Context, hash models.ImageHash) (*models.DeduplicationInfo, error) {
	return nil, nil
}
func (m *mockImageRepository) AddHashReference(ctx context.Context, hash models.ImageHash, imageID string) error {
	return nil
}
func (m *mockImageRepository) RemoveHashReference(ctx context.Context, hash models.ImageHash, imageID string) error {
	return nil
}
func (m *mockImageRepository) GetOrphanedHashes(ctx context.Context) ([]models.ImageHash, error) {
	return nil, nil
}

// Statistics methods
func (m *mockImageRepository) GetImageStatistics(ctx context.Context) (*models.ImageStatistics, error) {
	return &models.ImageStatistics{}, nil
}
func (m *mockImageRepository) GetStorageStatistics(ctx context.Context) (*models.StorageStatistics, error) {
	return &models.StorageStatistics{}, nil
}
func (m *mockImageRepository) GetImageCountByFormat(ctx context.Context) (map[string]int64, error) {
	return map[string]int64{}, nil
}
func (m *mockImageRepository) GetResolutionStatistics(ctx context.Context) ([]models.ResolutionStat, error) {
	return []models.ResolutionStat{}, nil
}
func (m *mockImageRepository) GetImagesByTimeRange(ctx context.Context, start, end time.Time) (int64, error) {
	return 0, nil
}
func (m *mockImageRepository) GetStorageUsageByResolution(ctx context.Context) (map[string]int64, error) {
	return map[string]int64{}, nil
}
func (m *mockImageRepository) GetDeduplicationStatistics(ctx context.Context) (*models.DeduplicationStatistics, error) {
	return &models.DeduplicationStatistics{}, nil
}
func (m *mockImageRepository) GetHashStatistics(ctx context.Context) ([]models.HashStat, error) {
	return []models.HashStat{}, nil
}
func (m *mockImageRepository) GetDuplicateCount(ctx context.Context) (int64, error) {
	return 0, nil
}
func (m *mockImageRepository) GetUniqueHashCount(ctx context.Context) (int64, error) {
	return 0, nil
}
func (m *mockImageRepository) GetStorageSavedByDeduplication(ctx context.Context) (int64, error) {
	return 0, nil
}

// Cache methods
func (m *mockImageRepository) SetCachedURL(ctx context.Context, imageID, resolution, url string, ttl time.Duration) error {
	return nil
}
func (m *mockImageRepository) GetCachedURL(ctx context.Context, imageID, resolution string) (string, error) {
	return "", nil
}
func (m *mockImageRepository) DeleteCachedURL(ctx context.Context, imageID, resolution string) error {
	return nil
}
func (m *mockImageRepository) DeleteAllCachedURLs(ctx context.Context, imageID string) error {
	return nil
}
func (m *mockImageRepository) SetCache(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	return nil
}
func (m *mockImageRepository) GetCache(ctx context.Context, key string) (string, error) {
	return "", nil
}
func (m *mockImageRepository) DeleteCache(ctx context.Context, key string) error {
	return nil
}

type mockStorageProvider struct {
	healthFunc func(ctx context.Context) error
}

func (m *mockStorageProvider) Upload(_ctx context.Context, _key string, _data io.Reader, _size int64, _contentType string) error {
	return nil
}
func (m *mockStorageProvider) Download(_ctx context.Context, _key string) (io.ReadCloser, error) {
	return nil, nil
}
func (m *mockStorageProvider) Delete(_ctx context.Context, _key string) error          { return nil }
func (m *mockStorageProvider) DeleteFolder(_ctx context.Context, _prefix string) error { return nil }
func (m *mockStorageProvider) Exists(_ctx context.Context, _key string) (bool, error) {
	return false, nil
}
func (m *mockStorageProvider) GeneratePresignedURL(_ctx context.Context, _key string, _expiration time.Duration) (string, error) {
	return "", nil
}
func (m *mockStorageProvider) HealthCheck(ctx context.Context) error {
	if m.healthFunc != nil {
		return m.healthFunc(ctx)
	}
	return nil
}
func (m *mockStorageProvider) Health(ctx context.Context) error { return m.HealthCheck(ctx) }
func (m *mockStorageProvider) CopyObject(_ctx context.Context, _srcKey, _destKey string) error {
	return nil
}
func (m *mockStorageProvider) GetMetadata(_ctx context.Context, _key string) (*storage.FileMetadata, error) {
	return nil, nil
}
func (m *mockStorageProvider) ListObjects(_ctx context.Context, _prefix string, _maxKeys int) ([]storage.ObjectInfo, error) {
	return nil, nil
}
func (m *mockStorageProvider) GetURL(_key string) string {
	return ""
}

func TestNewHealthService(t *testing.T) {
	mockRepo := &mockImageRepository{}
	mockStorage := &mockStorageProvider{}
	version := "1.0.0"

	service := NewHealthService(mockRepo, mockStorage, testutil.TestConfig(), version)

	assert.NotNil(t, service)

	// Type assertion to access internal fields
	impl, ok := service.(*HealthServiceImpl)
	assert.True(t, ok)
	assert.Equal(t, mockRepo, impl.repo)
	assert.Equal(t, mockStorage, impl.storage)
	assert.Equal(t, version, impl.version)
	assert.True(t, time.Since(impl.startTime) < time.Second) // Recently created
}

func TestHealthService_CheckHealth_AllHealthy(t *testing.T) {
	mockRepo := &mockImageRepository{
		healthFunc: func(ctx context.Context) error {
			return nil
		},
	}
	mockStorage := &mockStorageProvider{
		healthFunc: func(ctx context.Context) error {
			return nil
		},
	}

	service := NewHealthService(mockRepo, mockStorage, testutil.TestConfig(), "1.0.0")
	ctx := context.Background()

	// Sleep briefly to ensure uptime > 0
	time.Sleep(1 * time.Millisecond)

	status, err := service.CheckHealth(ctx)

	assert.NoError(t, err)
	assert.NotNil(t, status)
	assert.Equal(t, "connected", status.Services["redis"])
	assert.Equal(t, "connected", status.Services["s3"])
	assert.Equal(t, "healthy", status.Services["application"])
	assert.Equal(t, "1.0.0", status.Version)
	assert.Greater(t, status.Uptime, int64(0))
}

func TestHealthService_CheckHealth_RedisUnhealthy(t *testing.T) {
	repoError := errors.New("redis connection failed")
	mockRepo := &mockImageRepository{
		healthFunc: func(ctx context.Context) error {
			return repoError
		},
	}
	mockStorage := &mockStorageProvider{
		healthFunc: func(ctx context.Context) error {
			return nil
		},
	}

	service := NewHealthService(mockRepo, mockStorage, testutil.TestConfig(), "1.0.0")
	ctx := context.Background()

	status, err := service.CheckHealth(ctx)

	assert.NoError(t, err)
	assert.NotNil(t, status)
	assert.Contains(t, status.Services["redis"], "unhealthy: redis connection failed")
	assert.Equal(t, "connected", status.Services["s3"])
	assert.Equal(t, "healthy", status.Services["application"])
}

func TestHealthService_CheckHealth_S3Unhealthy(t *testing.T) {
	s3Error := errors.New("s3 bucket not accessible")
	mockRepo := &mockImageRepository{
		healthFunc: func(ctx context.Context) error {
			return nil
		},
	}
	mockStorage := &mockStorageProvider{
		healthFunc: func(ctx context.Context) error {
			return s3Error
		},
	}

	service := NewHealthService(mockRepo, mockStorage, testutil.TestConfig(), "1.0.0")
	ctx := context.Background()

	status, err := service.CheckHealth(ctx)

	assert.NoError(t, err)
	assert.NotNil(t, status)
	assert.Equal(t, "connected", status.Services["redis"])
	assert.Contains(t, status.Services["s3"], "unhealthy: s3 bucket not accessible")
	assert.Equal(t, "healthy", status.Services["application"])
}

func TestHealthService_CheckHealth_AllUnhealthy(t *testing.T) {
	repoError := errors.New("redis down")
	s3Error := errors.New("s3 down")

	mockRepo := &mockImageRepository{
		healthFunc: func(ctx context.Context) error {
			return repoError
		},
	}
	mockStorage := &mockStorageProvider{
		healthFunc: func(ctx context.Context) error {
			return s3Error
		},
	}

	service := NewHealthService(mockRepo, mockStorage, testutil.TestConfig(), "1.0.0")
	ctx := context.Background()

	status, err := service.CheckHealth(ctx)

	assert.NoError(t, err)
	assert.NotNil(t, status)
	assert.Contains(t, status.Services["redis"], "unhealthy: redis down")
	assert.Contains(t, status.Services["s3"], "unhealthy: s3 down")
	assert.Equal(t, "healthy", status.Services["application"])
}

func TestHealthService_GetMetrics_Success(t *testing.T) {
	mockRepo := &mockImageRepository{
		getStatsFunc: func(ctx context.Context) (*repository.RepositoryStats, error) {
			return &repository.RepositoryStats{
				TotalImages: 150,
				CacheHits:   1000,
				CacheMisses: 50,
			}, nil
		},
	}
	mockStorage := &mockStorageProvider{}

	service := NewHealthService(mockRepo, mockStorage, testutil.TestConfig(), "1.0.0")
	ctx := context.Background()

	// Sleep briefly to ensure uptime > 0
	time.Sleep(1 * time.Millisecond)

	metrics, err := service.GetMetrics(ctx)

	assert.NoError(t, err)
	assert.NotNil(t, metrics)

	// Check system metrics
	systemMetrics, ok := metrics["system"].(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, "1.0.0", systemMetrics["version"])
	assert.Greater(t, systemMetrics["uptime_milliseconds"].(int64), int64(0))
	assert.Equal(t, runtime.Version(), systemMetrics["go_version"])
	assert.Greater(t, systemMetrics["goroutines"].(int), 0)
	assert.Equal(t, runtime.NumCPU(), systemMetrics["cpu_count"])

	// Check memory metrics
	memoryMetrics, ok := metrics["memory"].(map[string]interface{})
	assert.True(t, ok)
	assert.Greater(t, memoryMetrics["alloc_bytes"].(uint64), uint64(0))
	assert.Greater(t, memoryMetrics["total_alloc_bytes"].(uint64), uint64(0))
	assert.Greater(t, memoryMetrics["sys_bytes"].(uint64), uint64(0))

	// Check repository metrics
	repoMetrics, ok := metrics["repository"].(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, int64(150), repoMetrics["total_images"])
	assert.Equal(t, int64(1000), repoMetrics["cache_hits"])
	assert.Equal(t, int64(50), repoMetrics["cache_misses"])

	// Check timestamp
	assert.Greater(t, metrics["timestamp"].(int64), int64(0))
}

func TestHealthService_GetMetrics_RepositoryStatsError(t *testing.T) {
	mockRepo := &mockImageRepository{
		getStatsFunc: func(ctx context.Context) (*repository.RepositoryStats, error) {
			return nil, errors.New("stats unavailable")
		},
	}
	mockStorage := &mockStorageProvider{}

	service := NewHealthService(mockRepo, mockStorage, testutil.TestConfig(), "1.0.0")
	ctx := context.Background()

	metrics, err := service.GetMetrics(ctx)

	assert.NoError(t, err)
	assert.NotNil(t, metrics)

	// System and memory metrics should still be present
	assert.Contains(t, metrics, "system")
	assert.Contains(t, metrics, "memory")
	assert.Contains(t, metrics, "timestamp")

	// Repository metrics should be absent due to error
	assert.NotContains(t, metrics, "repository")
}

func TestHealthService_Uptime(t *testing.T) {
	mockRepo := &mockImageRepository{
		healthFunc: func(ctx context.Context) error { return nil },
	}
	mockStorage := &mockStorageProvider{
		healthFunc: func(ctx context.Context) error { return nil },
	}

	service := NewHealthService(mockRepo, mockStorage, testutil.TestConfig(), "1.0.0")

	// Wait a small amount of time to ensure uptime > 0
	time.Sleep(10 * time.Millisecond)

	ctx := context.Background()

	status1, err := service.CheckHealth(ctx)
	assert.NoError(t, err)
	assert.Greater(t, status1.Uptime, int64(0))

	// Wait longer and check uptime increases
	time.Sleep(100 * time.Millisecond)

	status2, err := service.CheckHealth(ctx)
	assert.NoError(t, err)
	assert.Greater(t, status2.Uptime, status1.Uptime)
}

func TestHealthService_Context_Cancellation(t *testing.T) {
	mockRepo := &mockImageRepository{
		healthFunc: func(ctx context.Context) error {
			// Simulate slow health check
			select {
			case <-time.After(100 * time.Millisecond):
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		},
	}
	mockStorage := &mockStorageProvider{
		healthFunc: func(ctx context.Context) error {
			return nil
		},
	}

	service := NewHealthService(mockRepo, mockStorage, testutil.TestConfig(), "1.0.0")

	// Create context with short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	status, err := service.CheckHealth(ctx)

	// Health check should still succeed but redis might be marked unhealthy due to timeout
	assert.NoError(t, err)
	assert.NotNil(t, status)
	// The exact status depends on timing, but the method should not fail
}

func TestHealthService_MultipleChecks(t *testing.T) {
	callCount := 0
	mockRepo := &mockImageRepository{
		healthFunc: func(ctx context.Context) error {
			callCount++
			if callCount%2 == 0 {
				return errors.New("intermittent failure")
			}
			return nil
		},
	}
	mockStorage := &mockStorageProvider{
		healthFunc: func(ctx context.Context) error {
			return nil
		},
	}

	service := NewHealthService(mockRepo, mockStorage, testutil.TestConfig(), "1.0.0")
	ctx := context.Background()

	// First call - should succeed
	status1, err := service.CheckHealth(ctx)
	assert.NoError(t, err)
	assert.Equal(t, "connected", status1.Services["redis"])

	// Second call - should show unhealthy
	status2, err := service.CheckHealth(ctx)
	assert.NoError(t, err)
	assert.Contains(t, status2.Services["redis"], "unhealthy")

	// Third call - should succeed again
	status3, err := service.CheckHealth(ctx)
	assert.NoError(t, err)
	assert.Equal(t, "connected", status3.Services["redis"])
}

func TestHealthService_MemoryMetrics(t *testing.T) {
	mockRepo := &mockImageRepository{}
	mockStorage := &mockStorageProvider{}

	service := NewHealthService(mockRepo, mockStorage, testutil.TestConfig(), "1.0.0")
	ctx := context.Background()

	metrics, err := service.GetMetrics(ctx)
	assert.NoError(t, err)

	memoryMetrics, ok := metrics["memory"].(map[string]interface{})
	assert.True(t, ok)

	// Check all expected memory fields are present
	expectedFields := []string{
		"alloc_bytes",
		"total_alloc_bytes",
		"sys_bytes",
		"heap_alloc_bytes",
		"heap_sys_bytes",
		"heap_objects",
		"gc_runs",
		"gc_pause_ns",
	}

	for _, field := range expectedFields {
		assert.Contains(t, memoryMetrics, field, "Memory metrics should contain %s", field)
	}
}

func TestHealthService_Version(t *testing.T) {
	testVersions := []string{"1.0.0", "2.1.3-beta", "dev", ""}

	for _, version := range testVersions {
		t.Run("version_"+version, func(t *testing.T) {
			mockRepo := &mockImageRepository{
				healthFunc: func(ctx context.Context) error { return nil },
			}
			mockStorage := &mockStorageProvider{
				healthFunc: func(ctx context.Context) error { return nil },
			}

			service := NewHealthService(mockRepo, mockStorage, testutil.TestConfig(), version)
			ctx := context.Background()

			status, err := service.CheckHealth(ctx)
			assert.NoError(t, err)
			assert.Equal(t, version, status.Version)

			metrics, err := service.GetMetrics(ctx)
			assert.NoError(t, err)
			systemMetrics := metrics["system"].(map[string]interface{})
			assert.Equal(t, version, systemMetrics["version"])
		})
	}
}

// MockImageRepository extension to add GetStats
type _MockImageRepositoryWithStats struct {
	*mockImageRepository
	GetStatsFunc func(ctx context.Context) (*RepositoryStats, error)
}

func (m *_MockImageRepositoryWithStats) GetStats(ctx context.Context) (*RepositoryStats, error) {
	if m.GetStatsFunc != nil {
		return m.GetStatsFunc(ctx)
	}
	return &RepositoryStats{}, nil
}

func TestRepositoryStats_Struct(t *testing.T) {
	stats := &RepositoryStats{
		TotalImages: 100,
		CacheHits:   500,
		CacheMisses: 25,
	}

	assert.Equal(t, int64(100), stats.TotalImages)
	assert.Equal(t, int64(500), stats.CacheHits)
	assert.Equal(t, int64(25), stats.CacheMisses)
}

func TestHealthStatus_Struct(t *testing.T) {
	status := &HealthStatus{
		Services: map[string]string{
			"redis": "connected",
			"s3":    "connected",
		},
		Uptime:  3600,
		Version: "1.2.3",
	}

	assert.Equal(t, "connected", status.Services["redis"])
	assert.Equal(t, "connected", status.Services["s3"])
	assert.Equal(t, int64(3600), status.Uptime)
	assert.Equal(t, "1.2.3", status.Version)
}

func TestHealthService_S3CacheConfiguration(t *testing.T) {
	tests := []struct {
		name              string
		s3ChecksDisabled  bool
		s3ChecksInterval  time.Duration
		expectedS3Checked bool
		description       string
	}{
		{
			name:              "S3 checks enabled",
			s3ChecksDisabled:  false,
			s3ChecksInterval:  30 * time.Second,
			expectedS3Checked: true,
			description:       "When S3 checks are enabled, S3 should be checked",
		},
		{
			name:              "S3 checks disabled",
			s3ChecksDisabled:  true,
			s3ChecksInterval:  30 * time.Second,
			expectedS3Checked: false,
			description:       "When S3 checks are disabled, S3 should not be checked to save API costs",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create config with specific health check settings
			config := testutil.TestConfig()
			config.Health.S3ChecksDisabled = tt.s3ChecksDisabled
			config.Health.S3ChecksInterval = tt.s3ChecksInterval

			var s3CheckCalled bool
			mockStorage := &mockStorageProvider{
				healthFunc: func(ctx context.Context) error {
					s3CheckCalled = true
					return nil
				},
			}

			mockRepo := &mockImageRepository{
				healthFunc: func(ctx context.Context) error {
					return nil
				},
			}

			service := NewHealthService(mockRepo, mockStorage, config, "1.0.0")

			ctx := context.Background()
			_, err := service.CheckHealth(ctx)
			assert.NoError(t, err)

			assert.Equal(t, tt.expectedS3Checked, s3CheckCalled, tt.description)
		})
	}
}

func TestHealthService_S3CachingBehavior(t *testing.T) {
	// Create config with short caching interval for testing
	config := testutil.TestConfig()
	config.Health.S3ChecksDisabled = false
	config.Health.S3ChecksInterval = 100 * time.Millisecond // Short interval for testing

	var s3CheckCount int
	mockStorage := &mockStorageProvider{
		healthFunc: func(ctx context.Context) error {
			s3CheckCount++
			return nil
		},
	}

	mockRepo := &mockImageRepository{
		healthFunc: func(ctx context.Context) error {
			return nil
		},
	}

	service := NewHealthService(mockRepo, mockStorage, config, "1.0.0")
	ctx := context.Background()

	// First check should call S3
	_, err := service.CheckHealth(ctx)
	assert.NoError(t, err)
	assert.Equal(t, 1, s3CheckCount, "First check should call S3")

	// Immediate second check should use cache
	_, err = service.CheckHealth(ctx)
	assert.NoError(t, err)
	assert.Equal(t, 1, s3CheckCount, "Second check should use cached result")

	// Wait for cache to expire
	time.Sleep(150 * time.Millisecond)

	// Third check should call S3 again
	_, err = service.CheckHealth(ctx)
	assert.NoError(t, err)
	assert.Equal(t, 2, s3CheckCount, "Third check after cache expiry should call S3 again")
}

func TestHealthService_S3ErrorCaching(t *testing.T) {
	// Create config with short caching interval for testing
	config := testutil.TestConfig()
	config.Health.S3ChecksDisabled = false
	config.Health.S3ChecksInterval = 100 * time.Millisecond

	var s3CheckCount int
	mockStorage := &mockStorageProvider{
		healthFunc: func(ctx context.Context) error {
			s3CheckCount++
			return errors.New("S3 connection failed")
		},
	}

	mockRepo := &mockImageRepository{
		healthFunc: func(ctx context.Context) error {
			return nil
		},
	}

	service := NewHealthService(mockRepo, mockStorage, config, "1.0.0")
	ctx := context.Background()

	// First check should call S3 and cache the error
	status, err := service.CheckHealth(ctx)
	assert.NoError(t, err) // Service should not fail overall
	assert.Contains(t, status.Services["s3"], "unhealthy: S3 connection failed")
	assert.Equal(t, 1, s3CheckCount, "First check should call S3")

	// Immediate second check should use cached error result
	status, err = service.CheckHealth(ctx)
	assert.NoError(t, err)
	assert.Contains(t, status.Services["s3"], "unhealthy: S3 connection failed")
	assert.Equal(t, 1, s3CheckCount, "Second check should use cached error result")

	// Wait for cache to expire
	time.Sleep(150 * time.Millisecond)

	// Third check should call S3 again
	status, err = service.CheckHealth(ctx)
	assert.NoError(t, err)
	assert.Contains(t, status.Services["s3"], "unhealthy: S3 connection failed")
	assert.Equal(t, 2, s3CheckCount, "Third check after cache expiry should call S3 again")
}
