package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestRequestID_GenerateNew(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(RequestID())
	router.GET("/test", func(c *gin.Context) {
		requestID := c.GetString(RequestIDKey)
		assert.NotEmpty(t, requestID)

		// Verify it's a valid UUID
		_, err := uuid.Parse(requestID)
		assert.NoError(t, err)

		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Check response header
	responseID := w.Header().Get(RequestIDHeader)
	assert.NotEmpty(t, responseID)

	// Verify it's a valid UUID
	_, err := uuid.Parse(responseID)
	assert.NoError(t, err)
}

func TestRequestID_UseExisting(t *testing.T) {
	existingID := "existing-request-id-123"

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(RequestID())
	router.GET("/test", func(c *gin.Context) {
		requestID := c.GetString(RequestIDKey)
		assert.Equal(t, existingID, requestID)
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set(RequestIDHeader, existingID)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Check response header matches the existing ID
	responseID := w.Header().Get(RequestIDHeader)
	assert.Equal(t, existingID, responseID)
}

func TestRequestID_ContextPropagation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(RequestID())
	router.GET("/test", func(c *gin.Context) {
		// Check context contains request ID
		requestID := c.GetString(RequestIDKey)
		assert.NotEmpty(t, requestID)

		// Check request context has been updated
		assert.NotNil(t, c.Request.Context())

		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRequestID_MultipleRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(RequestID())

	var requestIDs []string

	router.GET("/test", func(c *gin.Context) {
		requestID := c.GetString(RequestIDKey)
		requestIDs = append(requestIDs, requestID)
		c.Status(http.StatusOK)
	})

	// Make multiple requests
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	}

	// All request IDs should be different
	assert.Len(t, requestIDs, 3)
	assert.NotEqual(t, requestIDs[0], requestIDs[1])
	assert.NotEqual(t, requestIDs[1], requestIDs[2])
	assert.NotEqual(t, requestIDs[0], requestIDs[2])
}

func TestRequestID_EmptyHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(RequestID())
	router.GET("/test", func(c *gin.Context) {
		requestID := c.GetString(RequestIDKey)
		assert.NotEmpty(t, requestID)

		// Should be a generated UUID, not empty
		_, err := uuid.Parse(requestID)
		assert.NoError(t, err)

		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set(RequestIDHeader, "") // Empty header
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Should generate new ID when header is empty
	responseID := w.Header().Get(RequestIDHeader)
	assert.NotEmpty(t, responseID)
	_, err := uuid.Parse(responseID)
	assert.NoError(t, err)
}

func TestRequestID_Constants(t *testing.T) {
	assert.Equal(t, "X-Request-ID", RequestIDHeader)
	assert.Equal(t, "request_id", RequestIDKey)
}

func TestRequestID_Integration(t *testing.T) {
	// Test integration with other middleware
	gin.SetMode(gin.TestMode)
	router := gin.New()

	// Add request ID middleware first
	router.Use(RequestID())

	// Add a second middleware that uses the request ID
	router.Use(func(c *gin.Context) {
		requestID := c.GetString(RequestIDKey)
		assert.NotEmpty(t, requestID)
		c.Header("Test-Header", "processed-"+requestID)
		c.Next()
	})

	router.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Verify both headers are set
	assert.NotEmpty(t, w.Header().Get(RequestIDHeader))
	assert.Contains(t, w.Header().Get("Test-Header"), "processed-")
}

func TestRequestID_CaseInsensitive(t *testing.T) {
	// Test different case variations of the header
	testCases := []string{
		"X-Request-ID",
		"x-request-id",
		"X-REQUEST-ID",
	}

	for _, headerName := range testCases {
		t.Run("header_"+headerName, func(t *testing.T) {
			existingID := "test-id-" + headerName

			gin.SetMode(gin.TestMode)
			router := gin.New()
			router.Use(RequestID())
			router.GET("/test", func(c *gin.Context) {
				// The middleware should handle case variations through Gin's GetHeader
				requestID := c.GetString(RequestIDKey)
				c.JSON(http.StatusOK, gin.H{"request_id": requestID})
			})

			req := httptest.NewRequest("GET", "/test", nil)
			req.Header.Set(headerName, existingID)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)

			// Response header should always use canonical form
			assert.Equal(t, existingID, w.Header().Get(RequestIDHeader))
		})
	}
}

func TestRequestID_LongExistingID(t *testing.T) {
	// Test with a very long existing ID
	longID := "very-long-request-id-" + uuid.New().String() + "-" + uuid.New().String()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(RequestID())
	router.GET("/test", func(c *gin.Context) {
		requestID := c.GetString(RequestIDKey)
		assert.Equal(t, longID, requestID)
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set(RequestIDHeader, longID)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, longID, w.Header().Get(RequestIDHeader))
}

func TestRequestID_SpecialCharacters(t *testing.T) {
	// Test with special characters in existing ID
	specialID := "req-123_abc-def.test@example"

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(RequestID())
	router.GET("/test", func(c *gin.Context) {
		requestID := c.GetString(RequestIDKey)
		assert.Equal(t, specialID, requestID)
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set(RequestIDHeader, specialID)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, specialID, w.Header().Get(RequestIDHeader))
}
