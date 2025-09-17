package service

import (
	"context"
	"errors"
	"io"
	"runtime"
	"testing"
	"time"

	"resizr/internal/models"

	"github.com/stretchr/testify/assert"
)

// Local mocks for health service testing
type mockImageRepository struct {
	healthFunc   func(ctx context.Context) error
	closeFunc    func() error
	getStatsFunc func(ctx context.Context) (*RepositoryStats, error)
}

func (m *mockImageRepository) Save(ctx context.Context, metadata *models.ImageMetadata) error {
	return nil
}
func (m *mockImageRepository) GetByID(ctx context.Context, id string) (*models.ImageMetadata, error) {
	return nil, nil
}
func (m *mockImageRepository) Get(ctx context.Context, id string) (*models.ImageMetadata, error) {
	return nil, nil
}
func (m *mockImageRepository) Store(ctx context.Context, metadata *models.ImageMetadata) error {
	return nil
}
func (m *mockImageRepository) Update(ctx context.Context, metadata *models.ImageMetadata) error {
	return nil
}
func (m *mockImageRepository) Delete(ctx context.Context, id string) error         { return nil }
func (m *mockImageRepository) Exists(ctx context.Context, id string) (bool, error) { return false, nil }
func (m *mockImageRepository) List(ctx context.Context, offset, limit int) ([]*models.ImageMetadata, error) {
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
func (m *mockImageRepository) GetStats(ctx context.Context) (*RepositoryStats, error) {
	if m.getStatsFunc != nil {
		return m.getStatsFunc(ctx)
	}
	return nil, nil
}
func (m *mockImageRepository) UpdateResolutions(ctx context.Context, imageID string, resolutions []string) error {
	return nil
}

type mockStorageProvider struct {
	healthFunc func(ctx context.Context) error
}

func (m *mockStorageProvider) Upload(ctx context.Context, key string, data interface{}, size int64, contentType string) error {
	return nil
}
func (m *mockStorageProvider) Download(ctx context.Context, key string) (io.ReadCloser, error) {
	return nil, nil
}
func (m *mockStorageProvider) Delete(ctx context.Context, key string) error { return nil }
func (m *mockStorageProvider) Exists(ctx context.Context, key string) (bool, error) {
	return false, nil
}
func (m *mockStorageProvider) GeneratePresignedURL(ctx context.Context, key string, expiration time.Duration) (string, error) {
	return "", nil
}
func (m *mockStorageProvider) HealthCheck(ctx context.Context) error {
	if m.healthFunc != nil {
		return m.healthFunc(ctx)
	}
	return nil
}
func (m *mockStorageProvider) Health(ctx context.Context) error { return m.HealthCheck(ctx) }
func (m *mockStorageProvider) CopyObject(ctx context.Context, srcKey, destKey string) error {
	return nil
}
func (m *mockStorageProvider) GetMetadata(ctx context.Context, key string) (*models.ImageMetadata, error) {
	return nil, nil
}

func TestNewHealthService(t *testing.T) {
	mockRepo := &mockImageRepository{}
	mockStorage := &mockStorageProvider{}
	version := "1.0.0"

	service := NewHealthService(mockRepo, mockStorage, version)

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
		HealthCheckFunc: func(ctx context.Context) error {
			return nil
		},
	}
	mockStorage := &mockStorageProvider{
		HealthCheckFunc: func(ctx context.Context) error {
			return nil
		},
	}

	service := NewHealthService(mockRepo, mockStorage, "1.0.0")
	ctx := context.Background()

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
		HealthCheckFunc: func(ctx context.Context) error {
			return repoError
		},
	}
	mockStorage := &mockStorageProvider{
		HealthCheckFunc: func(ctx context.Context) error {
			return nil
		},
	}

	service := NewHealthService(mockRepo, mockStorage, "1.0.0")
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
		HealthCheckFunc: func(ctx context.Context) error {
			return nil
		},
	}
	mockStorage := &mockStorageProvider{
		HealthCheckFunc: func(ctx context.Context) error {
			return s3Error
		},
	}

	service := NewHealthService(mockRepo, mockStorage, "1.0.0")
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
		HealthCheckFunc: func(ctx context.Context) error {
			return repoError
		},
	}
	mockStorage := &mockStorageProvider{
		HealthCheckFunc: func(ctx context.Context) error {
			return s3Error
		},
	}

	service := NewHealthService(mockRepo, mockStorage, "1.0.0")
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
		GetStatsFunc: func(ctx context.Context) (*RepositoryStats, error) {
			return &RepositoryStats{
				TotalImages: 150,
				CacheHits:   1000,
				CacheMisses: 50,
			}, nil
		},
	}
	mockStorage := &mockStorageProvider{}

	service := NewHealthService(mockRepo, mockStorage, "1.0.0")
	ctx := context.Background()

	metrics, err := service.GetMetrics(ctx)

	assert.NoError(t, err)
	assert.NotNil(t, metrics)

	// Check system metrics
	systemMetrics, ok := metrics["system"].(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, "1.0.0", systemMetrics["version"])
	assert.Greater(t, systemMetrics["uptime_seconds"].(int64), int64(0))
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
		GetStatsFunc: func(ctx context.Context) (*RepositoryStats, error) {
			return nil, errors.New("stats unavailable")
		},
	}
	mockStorage := &mockStorageProvider{}

	service := NewHealthService(mockRepo, mockStorage, "1.0.0")
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
		HealthCheckFunc: func(ctx context.Context) error { return nil },
	}
	mockStorage := &mockStorageProvider{
		HealthCheckFunc: func(ctx context.Context) error { return nil },
	}

	service := NewHealthService(mockRepo, mockStorage, "1.0.0")

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
		HealthCheckFunc: func(ctx context.Context) error {
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
		HealthCheckFunc: func(ctx context.Context) error {
			return nil
		},
	}

	service := NewHealthService(mockRepo, mockStorage, "1.0.0")

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
		HealthCheckFunc: func(ctx context.Context) error {
			callCount++
			if callCount%2 == 0 {
				return errors.New("intermittent failure")
			}
			return nil
		},
	}
	mockStorage := &mockStorageProvider{
		HealthCheckFunc: func(ctx context.Context) error {
			return nil
		},
	}

	service := NewHealthService(mockRepo, mockStorage, "1.0.0")
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

	service := NewHealthService(mockRepo, mockStorage, "1.0.0")
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
				HealthCheckFunc: func(ctx context.Context) error { return nil },
			}
			mockStorage := &mockStorageProvider{
				HealthCheckFunc: func(ctx context.Context) error { return nil },
			}

			service := NewHealthService(mockRepo, mockStorage, version)
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
type MockImageRepositoryWithStats struct {
	*mockImageRepository
	GetStatsFunc func(ctx context.Context) (*RepositoryStats, error)
}

func (m *MockImageRepositoryWithStats) GetStats(ctx context.Context) (*RepositoryStats, error) {
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
