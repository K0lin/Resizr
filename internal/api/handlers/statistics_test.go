package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"resizr/internal/models"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockStatisticsService implements models.StatisticsService for testing
type MockStatisticsService struct {
	mock.Mock
}

func (m *MockStatisticsService) GetComprehensiveStatistics(options *models.StatisticsOptions) (*models.ResizrStatistics, error) {
	args := m.Called(options)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.ResizrStatistics), args.Error(1)
}

func (m *MockStatisticsService) GetImageStatistics() (*models.ImageStatistics, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.ImageStatistics), args.Error(1)
}

func (m *MockStatisticsService) GetStorageStatistics() (*models.StorageStatistics, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.StorageStatistics), args.Error(1)
}

func (m *MockStatisticsService) GetDeduplicationStatistics() (*models.DeduplicationStatistics, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.DeduplicationStatistics), args.Error(1)
}

func (m *MockStatisticsService) RefreshStatistics() error {
	args := m.Called()
	return args.Error(0)
}

func createTestStatisticsHandler() (*StatisticsHandler, *MockStatisticsService) {
	mockService := &MockStatisticsService{}
	handler := NewStatisticsHandler(mockService)
	return handler, mockService
}

func createTestContext(method, path string) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(method, path, nil)
	c.Set("request_id", "test-request-id")
	return c, w
}

func TestNewStatisticsHandler(t *testing.T) {
	handler, _ := createTestStatisticsHandler()
	assert.NotNil(t, handler)
}

func TestGetComprehensiveStatistics_Success(t *testing.T) {
	handler, mockService := createTestStatisticsHandler()
	c, w := createTestContext("GET", "/api/v1/statistics")

	expectedStats := &models.ResizrStatistics{
		Images: models.ImageStatistics{
			TotalImages:    100,
			ImagesByFormat: map[string]int64{"jpeg": 60, "png": 40},
		},
		Storage: models.StorageStatistics{
			TotalStorageUsed: 1024000,
		},
		Deduplication: models.DeduplicationStatistics{
			UniqueImages: 75,
		},
		System: models.SystemStatistics{
			CPUCount: 4,
		},
		Timestamp: time.Now(),
	}

	mockService.On("GetComprehensiveStatistics", mock.AnythingOfType("*models.StatisticsOptions")).Return(expectedStats, nil)

	handler.GetComprehensiveStatistics(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var result models.ResizrStatistics
	err := json.Unmarshal(w.Body.Bytes(), &result)
	assert.NoError(t, err)
	assert.Equal(t, int64(100), result.Images.TotalImages)
	assert.Equal(t, int64(1024000), result.Storage.TotalStorageUsed)

	mockService.AssertExpectations(t)
}

func TestGetComprehensiveStatistics_WithQueryParams(t *testing.T) {
	handler, mockService := createTestStatisticsHandler()
	c, w := createTestContext("GET", "/api/v1/statistics?detailed=true&performance=false&system=false")

	// Add query parameters
	c.Request.URL.RawQuery = "detailed=true&performance=false&system=false"

	expectedStats := &models.ResizrStatistics{
		Images:    models.ImageStatistics{TotalImages: 100},
		Timestamp: time.Now(),
	}

	mockService.On("GetComprehensiveStatistics", mock.MatchedBy(func(opts *models.StatisticsOptions) bool {
		return opts != nil &&
			opts.IncludeDetailedBreakdown &&
			!opts.IncludePerformanceMetrics &&
			!opts.IncludeSystemMetrics
	})).Return(expectedStats, nil)

	handler.GetComprehensiveStatistics(c)

	assert.Equal(t, http.StatusOK, w.Code)
	mockService.AssertExpectations(t)
}

func TestGetComprehensiveStatistics_ServiceError(t *testing.T) {
	handler, mockService := createTestStatisticsHandler()
	c, w := createTestContext("GET", "/api/v1/statistics")

	mockService.On("GetComprehensiveStatistics", mock.AnythingOfType("*models.StatisticsOptions")).Return(nil, errors.New("service error"))

	handler.GetComprehensiveStatistics(c)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var errorResponse models.ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &errorResponse)
	assert.NoError(t, err)
	assert.Equal(t, "Statistics retrieval failed", errorResponse.Error)

	mockService.AssertExpectations(t)
}

func TestGetImageStatistics_Success(t *testing.T) {
	handler, mockService := createTestStatisticsHandler()
	c, w := createTestContext("GET", "/api/v1/statistics/images")

	expectedStats := &models.ImageStatistics{
		TotalImages:      100,
		ImagesByFormat:   map[string]int64{"jpeg": 60, "png": 40},
		ResolutionCounts: map[string]int64{"800x600": 30, "1920x1080": 20},
	}

	mockService.On("GetImageStatistics").Return(expectedStats, nil)

	handler.GetImageStatistics(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var result models.ImageStatistics
	err := json.Unmarshal(w.Body.Bytes(), &result)
	assert.NoError(t, err)
	assert.Equal(t, int64(100), result.TotalImages)
	assert.Equal(t, map[string]int64{"jpeg": 60, "png": 40}, result.ImagesByFormat)

	mockService.AssertExpectations(t)
}

func TestGetImageStatistics_ServiceError(t *testing.T) {
	handler, mockService := createTestStatisticsHandler()
	c, w := createTestContext("GET", "/api/v1/statistics/images")

	mockService.On("GetImageStatistics").Return(nil, errors.New("service error"))

	handler.GetImageStatistics(c)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var errorResponse models.ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &errorResponse)
	assert.NoError(t, err)
	assert.Equal(t, "Image statistics retrieval failed", errorResponse.Error)

	mockService.AssertExpectations(t)
}

func TestGetStorageStatistics_Success(t *testing.T) {
	handler, mockService := createTestStatisticsHandler()
	c, w := createTestContext("GET", "/api/v1/statistics/storage")

	expectedStats := &models.StorageStatistics{
		TotalStorageUsed:        1024000,
		OriginalImagesSize:      512000,
		ProcessedImagesSize:     512000,
		StorageByResolution:     map[string]int64{"original": 512000, "thumbnail": 256000},
		AverageCompressionRatio: 0.75,
	}

	mockService.On("GetStorageStatistics").Return(expectedStats, nil)

	handler.GetStorageStatistics(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var result models.StorageStatistics
	err := json.Unmarshal(w.Body.Bytes(), &result)
	assert.NoError(t, err)
	assert.Equal(t, int64(1024000), result.TotalStorageUsed)

	mockService.AssertExpectations(t)
}

func TestGetDeduplicationStatistics_Success(t *testing.T) {
	handler, mockService := createTestStatisticsHandler()
	c, w := createTestContext("GET", "/api/v1/statistics/deduplication")

	expectedStats := &models.DeduplicationStatistics{
		TotalDuplicatesFound:     25,
		DedupedImages:            25,
		UniqueImages:             75,
		DeduplicationRate:        25.0,
		AverageReferencesPerHash: 2,
	}

	mockService.On("GetDeduplicationStatistics").Return(expectedStats, nil)

	handler.GetDeduplicationStatistics(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var result models.DeduplicationStatistics
	err := json.Unmarshal(w.Body.Bytes(), &result)
	assert.NoError(t, err)
	assert.Equal(t, int64(25), result.TotalDuplicatesFound)
	assert.Equal(t, float64(25.0), result.DeduplicationRate)

	mockService.AssertExpectations(t)
}

func TestRefreshStatistics_Success(t *testing.T) {
	handler, mockService := createTestStatisticsHandler()
	c, w := createTestContext("POST", "/api/v1/statistics/refresh")

	mockService.On("RefreshStatistics").Return(nil)

	handler.RefreshStatistics(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var result map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &result)
	assert.NoError(t, err)
	assert.Equal(t, "Statistics refreshed successfully", result["message"])
	assert.Equal(t, "success", result["status"])

	mockService.AssertExpectations(t)
}

func TestRefreshStatistics_ServiceError(t *testing.T) {
	handler, mockService := createTestStatisticsHandler()
	c, w := createTestContext("POST", "/api/v1/statistics/refresh")

	mockService.On("RefreshStatistics").Return(errors.New("refresh failed"))

	handler.RefreshStatistics(c)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var errorResponse models.ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &errorResponse)
	assert.NoError(t, err)
	assert.Equal(t, "Statistics refresh failed", errorResponse.Error)

	mockService.AssertExpectations(t)
}

func TestGetStorageStatistics_ServiceError(t *testing.T) {
	handler, mockService := createTestStatisticsHandler()
	c, w := createTestContext("GET", "/api/v1/statistics/storage")

	mockService.On("GetStorageStatistics").Return(nil, errors.New("storage error"))

	handler.GetStorageStatistics(c)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var errorResponse models.ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &errorResponse)
	assert.NoError(t, err)
	assert.Equal(t, "Storage statistics retrieval failed", errorResponse.Error)

	mockService.AssertExpectations(t)
}

func TestGetDeduplicationStatistics_ServiceError(t *testing.T) {
	handler, mockService := createTestStatisticsHandler()
	c, w := createTestContext("GET", "/api/v1/statistics/deduplication")

	mockService.On("GetDeduplicationStatistics").Return(nil, errors.New("dedup error"))

	handler.GetDeduplicationStatistics(c)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var errorResponse models.ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &errorResponse)
	assert.NoError(t, err)
	assert.Equal(t, "Deduplication statistics retrieval failed", errorResponse.Error)

	mockService.AssertExpectations(t)
}
