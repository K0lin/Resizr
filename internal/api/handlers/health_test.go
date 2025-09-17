package handlers

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"resizr/internal/models"
	"resizr/internal/service"
	"resizr/internal/testutil"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// Local mock to avoid import cycles
type mockHealthService struct {
	checkHealthFunc func(ctx context.Context) (*service.HealthStatus, error)
	getMetricsFunc  func(ctx context.Context) (map[string]interface{}, error)
}

func (m *mockHealthService) CheckHealth(ctx context.Context) (*service.HealthStatus, error) {
	if m.checkHealthFunc != nil {
		return m.checkHealthFunc(ctx)
	}
	return nil, nil
}

func (m *mockHealthService) GetMetrics(ctx context.Context) (map[string]interface{}, error) {
	if m.getMetricsFunc != nil {
		return m.getMetricsFunc(ctx)
	}
	return nil, nil
}

func TestHealthHandler_Health(t *testing.T) {
	tests := []struct {
		name           string
		setupMock      func(*mockHealthService)
		expectedStatus int
		expectedBody   map[string]interface{}
	}{
		{
			name: "healthy services",
			setupMock: func(mock *mockHealthService) {
				mock.checkHealthFunc = func(ctx context.Context) (*service.HealthStatus, error) {
					return &service.HealthStatus{
						Services: map[string]string{
							"redis":       "connected",
							"s3":          "connected",
							"application": "healthy",
						},
						Uptime:  3600,
						Version: "1.0.0",
					}, nil
				}
			},
			expectedStatus: http.StatusOK,
			expectedBody: map[string]interface{}{
				"status": "healthy",
				"services": map[string]interface{}{
					"redis":       "connected",
					"s3":          "connected",
					"application": "healthy",
				},
			},
		},
		{
			name: "degraded services",
			setupMock: func(mock *mockHealthService) {
				mock.checkHealthFunc = func(ctx context.Context) (*service.HealthStatus, error) {
					return &service.HealthStatus{
						Services: map[string]string{
							"redis":       "connected",
							"s3":          "disconnected",
							"application": "healthy",
						},
						Uptime:  3600,
						Version: "1.0.0",
					}, nil
				}
			},
			expectedStatus: http.StatusPartialContent,
			expectedBody: map[string]interface{}{
				"status": "degraded",
				"services": map[string]interface{}{
					"redis":       "connected",
					"s3":          "disconnected",
					"application": "healthy",
				},
			},
		},
		{
			name: "health check error",
			setupMock: func(mock *mockHealthService) {
				mock.checkHealthFunc = func(ctx context.Context) (*service.HealthStatus, error) {
					return nil, errors.New("health check failed")
				}
			},
			expectedStatus: http.StatusServiceUnavailable,
			expectedBody: map[string]interface{}{
				"status": "unhealthy",
				"services": map[string]interface{}{
					"error": "health check failed",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			mockService := &mockHealthService{}
			tt.setupMock(mockService)

			handler := NewHealthHandler(mockService)

			// Create test context
			req := testutil.CreateTestRequest("GET", "/health", nil)
			c, w := testutil.SetupTestContext(req)

			// Execute
			handler.Health(c)

			// Assert
			assert.Equal(t, tt.expectedStatus, w.Code)

			var response map[string]interface{}
			err := testutil.ParseJSONResponse(w, &response)
			assert.NoError(t, err)

			// Check status
			assert.Equal(t, tt.expectedBody["status"], response["status"])

			// Check services
			expectedServices := tt.expectedBody["services"].(map[string]interface{})
			actualServices := response["services"].(map[string]interface{})

			for key, expectedValue := range expectedServices {
				assert.Equal(t, expectedValue, actualServices[key])
			}

			// Check timestamp exists
			assert.NotNil(t, response["timestamp"])
		})
	}
}

func TestHealthHandler_Health_ServiceDegradation(t *testing.T) {
	// Test specific degradation scenarios
	mockService := &mockHealthService{
		checkHealthFunc: func(ctx context.Context) (*service.HealthStatus, error) {
			return &service.HealthStatus{
				Services: map[string]string{
					"redis":       "connected",
					"s3":          "timeout", // This should cause degraded status
					"application": "healthy",
				},
				Uptime:  3600,
				Version: "1.0.0",
			}, nil
		},
	}

	handler := NewHealthHandler(mockService)
	req := testutil.CreateTestRequest("GET", "/health", nil)
	c, w := testutil.SetupTestContext(req)

	handler.Health(c)

	assert.Equal(t, http.StatusPartialContent, w.Code)

	var response map[string]interface{}
	err := testutil.ParseJSONResponse(w, &response)
	assert.NoError(t, err)
	assert.Equal(t, "degraded", response["status"])
}

func TestHealthHandler_Metrics(t *testing.T) {
	tests := []struct {
		name           string
		setupMock      func(*mockHealthService)
		expectedStatus int
		expectError    bool
	}{
		{
			name: "successful metrics retrieval",
			setupMock: func(mock *mockHealthService) {
				mock.getMetricsFunc = func(ctx context.Context) (map[string]interface{}, error) {
					return map[string]interface{}{
						"uptime":       "1h30m",
						"requests":     1000,
						"errors":       5,
						"memory_usage": "128MB",
					}, nil
				}
			},
			expectedStatus: http.StatusOK,
			expectError:    false,
		},
		{
			name: "metrics error",
			setupMock: func(mock *mockHealthService) {
				mock.getMetricsFunc = func(ctx context.Context) (map[string]interface{}, error) {
					return nil, errors.New("metrics collection failed")
				}
			},
			expectedStatus: http.StatusInternalServerError,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			mockService := &mockHealthService{}
			tt.setupMock(mockService)

			handler := NewHealthHandler(mockService)

			// Create test context
			req := testutil.CreateTestRequest("GET", "/debug/vars", nil)
			c, w := testutil.SetupTestContext(req)

			// Execute
			handler.Metrics(c)

			// Assert
			assert.Equal(t, tt.expectedStatus, w.Code)

			var response map[string]interface{}
			err := testutil.ParseJSONResponse(w, &response)
			assert.NoError(t, err)

			if tt.expectError {
				assert.Contains(t, response, "error")
				assert.Contains(t, response, "message")
				assert.Contains(t, response, "code")
			} else {
				assert.Contains(t, response, "uptime")
				assert.Contains(t, response, "requests")
				assert.Contains(t, response, "errors")
				assert.Contains(t, response, "memory_usage")
			}
		})
	}
}

func TestNewHealthHandler(t *testing.T) {
	mockService := &testutil.MockHealthService{}
	handler := NewHealthHandler(mockService)

	assert.NotNil(t, handler)
	assert.Equal(t, mockService, handler.healthService)
}

func TestHealthHandler_RequestIDExtraction(t *testing.T) {
	mockService := &testutil.MockHealthService{
		CheckHealthFunc: func(ctx context.Context) (*models.HealthResponse, error) {
			return &models.HealthResponse{
				Status:    "healthy",
				Services:  map[string]string{"test": "connected"},
				Timestamp: time.Now(),
			}, nil
		},
	}

	handler := NewHealthHandler(mockService)

	// Test without request_id
	req := testutil.CreateTestRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req
	// Don't set request_id

	handler.Health(c)

	assert.Equal(t, http.StatusOK, w.Code)

	// Test with request_id
	req2 := testutil.CreateTestRequest("GET", "/health", nil)
	w2 := httptest.NewRecorder()
	c2, _ := gin.CreateTestContext(w2)
	c2.Request = req2
	c2.Set("request_id", "test-request-123")

	handler.Health(c2)

	assert.Equal(t, http.StatusOK, w2.Code)
}

func TestHealthHandler_EdgeCases(t *testing.T) {
	t.Run("nil health service response", func(t *testing.T) {
		mockService := &testutil.MockHealthService{
			CheckHealthFunc: func(ctx context.Context) (*models.HealthResponse, error) {
				return nil, nil // This should not happen but we test defensive coding
			},
		}

		handler := NewHealthHandler(mockService)
		req := testutil.CreateTestRequest("GET", "/health", nil)
		c, w := testutil.SetupTestContext(req)

		handler.Health(c)

		// Should handle gracefully
		assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	})

	t.Run("empty services map", func(t *testing.T) {
		mockService := &testutil.MockHealthService{
			CheckHealthFunc: func(ctx context.Context) (*models.HealthResponse, error) {
				return &models.HealthResponse{
					Status:    "healthy",
					Services:  map[string]string{}, // Empty services
					Timestamp: time.Now(),
				}, nil
			},
		}

		handler := NewHealthHandler(mockService)
		req := testutil.CreateTestRequest("GET", "/health", nil)
		c, w := testutil.SetupTestContext(req)

		handler.Health(c)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := testutil.ParseJSONResponse(w, &response)
		assert.NoError(t, err)
		assert.Equal(t, "healthy", response["status"])
	})
}
