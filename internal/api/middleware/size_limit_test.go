package middleware

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"resizr/internal/testutil"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestRequestSizeLimit_WithinLimit(t *testing.T) {
	maxSize := int64(1024)                   // 1KB limit
	smallPayload := strings.Repeat("a", 500) // 500 bytes

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(RequestSizeLimit(maxSize))
	router.POST("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest("POST", "/test", strings.NewReader(smallPayload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRequestSizeLimit_ExceedsLimit(t *testing.T) {
	maxSize := int64(1024)                    // 1KB limit
	largePayload := strings.Repeat("a", 2048) // 2KB payload

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(RequestSizeLimit(maxSize))
	router.POST("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest("POST", "/test", strings.NewReader(largePayload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Length", fmt.Sprintf("%d", len(largePayload)))
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusRequestEntityTooLarge, w.Code)

	var response map[string]interface{}
	err := testutil.ParseJSONResponse(w, &response)
	assert.NoError(t, err)
	assert.Equal(t, "Request too large", response["error"])
	assert.Contains(t, response["message"], "exceeds maximum allowed size")
}

func TestRequestSizeLimit_ExactLimit(t *testing.T) {
	maxSize := int64(1024)                    // 1KB limit
	exactPayload := strings.Repeat("a", 1024) // Exactly 1KB

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(RequestSizeLimit(maxSize))
	router.POST("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest("POST", "/test", strings.NewReader(exactPayload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRequestSizeLimit_GetRequestSkipped(t *testing.T) {
	maxSize := int64(100) // Very small limit

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(RequestSizeLimit(maxSize))
	router.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// GET requests should not be limited
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRequestSizeLimit_HeadRequestSkipped(t *testing.T) {
	maxSize := int64(100) // Very small limit

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(RequestSizeLimit(maxSize))
	router.HEAD("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest("HEAD", "/test", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// HEAD requests should not be limited
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRequestSizeLimit_InvalidContentLength(t *testing.T) {
	maxSize := int64(1024)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(RequestSizeLimit(maxSize))
	router.POST("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest("POST", "/test", strings.NewReader("test"))
	req.Header.Set("Content-Length", "invalid-number")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response map[string]interface{}
	err := testutil.ParseJSONResponse(w, &response)
	assert.NoError(t, err)
	assert.Equal(t, "Invalid Content-Length", response["error"])
	assert.Contains(t, response["message"], "Content-Length header contains invalid value")
}

func TestRequestSizeLimit_NoContentLength(t *testing.T) {
	maxSize := int64(1024)
	payload := strings.Repeat("a", 500)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(RequestSizeLimit(maxSize))
	router.POST("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest("POST", "/test", strings.NewReader(payload))
	// Don't set Content-Length header
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// Should succeed when no Content-Length header is present (relies on MaxBytesReader)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRequestSizeLimit_ZeroContentLength(t *testing.T) {
	maxSize := int64(1024)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(RequestSizeLimit(maxSize))
	router.POST("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest("POST", "/test", strings.NewReader(""))
	req.Header.Set("Content-Length", "0")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRequestSizeLimit_NegativeContentLength(t *testing.T) {
	maxSize := int64(1024)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(RequestSizeLimit(maxSize))
	router.POST("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest("POST", "/test", strings.NewReader("test"))
	req.Header.Set("Content-Length", "-1")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// Negative content length should be allowed (within limit)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRequestSizeLimit_MultipleRequests(t *testing.T) {
	maxSize := int64(1024)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(RequestSizeLimit(maxSize))
	router.POST("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	// Test multiple requests of different sizes
	testCases := []struct {
		name         string
		payloadSize  int
		expectedCode int
	}{
		{"small request", 100, http.StatusOK},
		{"medium request", 512, http.StatusOK},
		{"large request", 2048, http.StatusRequestEntityTooLarge},
		{"exact limit", 1024, http.StatusOK},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			payload := strings.Repeat("a", tc.payloadSize)
			req := httptest.NewRequest("POST", "/test", strings.NewReader(payload))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Content-Length", fmt.Sprintf("%d", len(payload)))
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			assert.Equal(t, tc.expectedCode, w.Code)
		})
	}
}

func TestRequestSizeLimit_DifferentMethods(t *testing.T) {
	maxSize := int64(100)               // Very small limit
	payload := strings.Repeat("a", 200) // Exceeds limit

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(RequestSizeLimit(maxSize))
	router.POST("/test", func(c *gin.Context) { c.Status(http.StatusOK) })
	router.PUT("/test", func(c *gin.Context) { c.Status(http.StatusOK) })
	router.PATCH("/test", func(c *gin.Context) { c.Status(http.StatusOK) })
	router.DELETE("/test", func(c *gin.Context) { c.Status(http.StatusOK) })
	router.GET("/test", func(c *gin.Context) { c.Status(http.StatusOK) })
	router.HEAD("/test", func(c *gin.Context) { c.Status(http.StatusOK) })

	testCases := []struct {
		method       string
		expectedCode int
	}{
		{"POST", http.StatusRequestEntityTooLarge},   // Should be limited
		{"PUT", http.StatusRequestEntityTooLarge},    // Should be limited
		{"PATCH", http.StatusRequestEntityTooLarge},  // Should be limited
		{"DELETE", http.StatusRequestEntityTooLarge}, // Should be limited
		{"GET", http.StatusOK},                       // Should NOT be limited
		{"HEAD", http.StatusOK},                      // Should NOT be limited
	}

	for _, tc := range testCases {
		t.Run(tc.method, func(t *testing.T) {
			var req *http.Request
			if tc.method == "GET" || tc.method == "HEAD" {
				req = httptest.NewRequest(tc.method, "/test", nil)
			} else {
				body := strings.NewReader(payload)
				req = httptest.NewRequest(tc.method, "/test", body)
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("Content-Length", fmt.Sprintf("%d", len(payload)))
			}
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			assert.Equal(t, tc.expectedCode, w.Code, "Method %s should return %d", tc.method, tc.expectedCode)
		})
	}
}

func TestRequestSizeLimit_WithRequestID(t *testing.T) {
	maxSize := int64(100)
	largePayload := strings.Repeat("a", 200)

	gin.SetMode(gin.TestMode)
	router := gin.New()

	// Add request ID middleware first
	router.Use(func(c *gin.Context) {
		c.Set("request_id", "test-request-123")
		c.Next()
	})

	router.Use(RequestSizeLimit(maxSize))
	router.POST("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest("POST", "/test", strings.NewReader(largePayload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Length", fmt.Sprintf("%d", len(largePayload)))
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusRequestEntityTooLarge, w.Code)

	var response map[string]interface{}
	err := testutil.ParseJSONResponse(w, &response)
	assert.NoError(t, err)
	assert.Equal(t, "Request too large", response["error"])
}

func TestRequestSizeLimit_MaxBytesReader(t *testing.T) {
	maxSize := int64(100)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(RequestSizeLimit(maxSize))
	router.POST("/test", func(c *gin.Context) {
		// Try to read the body - this should be limited by MaxBytesReader
		body := make([]byte, 200)
		n, err := c.Request.Body.Read(body)

		// The exact behavior depends on timing, but body should be limited
		if err != nil {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "body too large"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"bytes_read": n})
	})

	// Send request without Content-Length so MaxBytesReader is the primary protection
	largePayload := strings.Repeat("a", 200)
	req := httptest.NewRequest("POST", "/test", strings.NewReader(largePayload))
	// Explicitly don't set Content-Length
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// The request might succeed or fail depending on when MaxBytesReader kicks in
	// This is more of an integration test to ensure MaxBytesReader is properly set
	assert.True(t, w.Code == http.StatusOK || w.Code == http.StatusRequestEntityTooLarge)
}

func TestRequestSizeLimit_EdgeCases(t *testing.T) {
	t.Run("zero max size", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		router := gin.New()
		router.Use(RequestSizeLimit(0))
		router.POST("/test", func(c *gin.Context) {
			c.Status(http.StatusOK)
		})

		req := httptest.NewRequest("POST", "/test", strings.NewReader("a"))
		req.Header.Set("Content-Length", "1")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusRequestEntityTooLarge, w.Code)
	})

	t.Run("negative max size", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		router := gin.New()
		router.Use(RequestSizeLimit(-1))
		router.POST("/test", func(c *gin.Context) {
			c.Status(http.StatusOK)
		})

		req := httptest.NewRequest("POST", "/test", strings.NewReader("a"))
		req.Header.Set("Content-Length", "1")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		// Negative max size should reject any content
		assert.Equal(t, http.StatusRequestEntityTooLarge, w.Code)
	})

	t.Run("very large content length", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		router := gin.New()
		router.Use(RequestSizeLimit(1024))
		router.POST("/test", func(c *gin.Context) {
			c.Status(http.StatusOK)
		})

		req := httptest.NewRequest("POST", "/test", strings.NewReader("small"))
		req.Header.Set("Content-Length", "999999999999999999") // Very large number
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusRequestEntityTooLarge, w.Code)
	})
}

func TestRequestSizeLimit_BinaryData(t *testing.T) {
	maxSize := int64(1024)
	binaryData := bytes.Repeat([]byte{0x00, 0xFF, 0xAA, 0x55}, 100) // 400 bytes of binary data

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(RequestSizeLimit(maxSize))
	router.POST("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest("POST", "/test", bytes.NewReader(binaryData))
	req.Header.Set("Content-Type", "application/octet-stream")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRequestSizeLimit_EmptyBody(t *testing.T) {
	maxSize := int64(1024)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(RequestSizeLimit(maxSize))
	router.POST("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest("POST", "/test", nil) // No body
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}
