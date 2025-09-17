package middleware

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
	"time"

	"resizr/internal/config"
	"resizr/internal/testutil"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"golang.org/x/time/rate"
)

func TestRateLimit_Upload(t *testing.T) {
	cfg := &config.Config{
		RateLimit: config.RateLimitConfig{
			Upload:   2,
			Download: 100,
			Info:     50,
		},
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(RateLimit(cfg))
	router.POST("/api/v1/images", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	// Reset global rate limiter for clean test
	globalRateLimiter = nil
	once = sync.Once{}

	// First request should succeed
	req := httptest.NewRequest("POST", "/api/v1/images", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// Second request should succeed (within burst)
	req2 := httptest.NewRequest("POST", "/api/v1/images", nil)
	req2.RemoteAddr = "192.168.1.1:12345"
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusOK, w2.Code)

	// Third request should be rate limited
	req3 := httptest.NewRequest("POST", "/api/v1/images", nil)
	req3.RemoteAddr = "192.168.1.1:12345"
	w3 := httptest.NewRecorder()
	router.ServeHTTP(w3, req3)
	assert.Equal(t, http.StatusTooManyRequests, w3.Code)

	var response map[string]interface{}
	err := testutil.ParseJSONResponse(w3, &response)
	assert.NoError(t, err)
	assert.Equal(t, "Rate limit exceeded", response["error"])
	assert.Contains(t, response["message"], "Too many requests")
}

func TestRateLimit_Download(t *testing.T) {
	cfg := &config.Config{
		RateLimit: config.RateLimitConfig{
			Upload:   10,
			Download: 2,
			Info:     50,
		},
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(RateLimit(cfg))
	router.GET("/api/v1/images/:id/thumbnail", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	// Reset global rate limiter
	globalRateLimiter = nil
	once = sync.Once{}

	// Multiple requests within limit should succeed
	for i := 0; i < 4; i++ { // Burst is 2x rate = 4
		req := httptest.NewRequest("GET", "/api/v1/images/123/thumbnail", nil)
		req.RemoteAddr = "192.168.1.2:12345"
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if i < 4 {
			assert.Equal(t, http.StatusOK, w.Code, "Request %d should succeed", i+1)
		}
	}

	// Next request should be rate limited
	req := httptest.NewRequest("GET", "/api/v1/images/123/thumbnail", nil)
	req.RemoteAddr = "192.168.1.2:12345"
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusTooManyRequests, w.Code)
}

func TestRateLimit_Info(t *testing.T) {
	cfg := &config.Config{
		RateLimit: config.RateLimitConfig{
			Upload:   10,
			Download: 100,
			Info:     2,
		},
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(RateLimit(cfg))
	router.GET("/api/v1/images/:id/info", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	router.GET("/health", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	// Reset global rate limiter
	globalRateLimiter = nil
	once = sync.Once{}

	// Test info endpoint
	for i := 0; i < 4; i++ { // Burst is 2x rate = 4
		req := httptest.NewRequest("GET", "/api/v1/images/123/info", nil)
		req.RemoteAddr = "192.168.1.3:12345"
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if i < 4 {
			assert.Equal(t, http.StatusOK, w.Code, "Info request %d should succeed", i+1)
		}
	}

	// Test health endpoint (different key)
	req := httptest.NewRequest("GET", "/health", nil)
	req.RemoteAddr = "192.168.1.3:12345"
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRateLimit_DifferentIPs(t *testing.T) {
	cfg := &config.Config{
		RateLimit: config.RateLimitConfig{
			Upload: 1,
		},
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(RateLimit(cfg))
	router.POST("/api/v1/images", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	// Reset global rate limiter
	globalRateLimiter = nil
	once = sync.Once{}

	// Different IPs should have separate limits
	ips := []string{"192.168.1.1:12345", "192.168.1.2:12345", "192.168.1.3:12345"}

	for _, ip := range ips {
		req := httptest.NewRequest("POST", "/api/v1/images", nil)
		req.RemoteAddr = ip
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code, "Request from IP %s should succeed", ip)
	}
}

func TestRateLimit_NoLimit(t *testing.T) {
	cfg := &config.Config{
		RateLimit: config.RateLimitConfig{
			Upload:   10,
			Download: 100,
			Info:     50,
		},
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(RateLimit(cfg))
	router.GET("/some/other/endpoint", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	// Reset global rate limiter
	globalRateLimiter = nil
	once = sync.Once{}

	// Endpoints not covered by rate limiting should not be limited
	for i := 0; i < 100; i++ {
		req := httptest.NewRequest("GET", "/some/other/endpoint", nil)
		req.RemoteAddr = "192.168.1.1:12345"
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	}
}

func TestRateLimit_Headers(t *testing.T) {
	cfg := &config.Config{
		RateLimit: config.RateLimitConfig{
			Upload: 5,
		},
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(RateLimit(cfg))
	router.POST("/api/v1/images", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	// Reset global rate limiter
	globalRateLimiter = nil
	once = sync.Once{}

	// Make a request and check headers
	req := httptest.NewRequest("POST", "/api/v1/images", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "5", w.Header().Get("X-RateLimit-Limit"))
	assert.Equal(t, "10", w.Header().Get("X-RateLimit-Burst"))
	assert.NotEmpty(t, w.Header().Get("X-RateLimit-Reset"))

	// Parse reset time
	resetTime, err := strconv.ParseInt(w.Header().Get("X-RateLimit-Reset"), 10, 64)
	assert.NoError(t, err)
	assert.Greater(t, resetTime, time.Now().Unix())
}

func TestRateLimit_ExceededHeaders(t *testing.T) {
	cfg := &config.Config{
		RateLimit: config.RateLimitConfig{
			Upload: 1,
		},
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(RateLimit(cfg))
	router.POST("/api/v1/images", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	// Reset global rate limiter
	globalRateLimiter = nil
	once = sync.Once{}

	// Exhaust the rate limit
	for i := 0; i < 3; i++ { // Burst = 2, so 3rd request should be limited
		req := httptest.NewRequest("POST", "/api/v1/images", nil)
		req.RemoteAddr = "192.168.1.1:12345"
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if i < 2 {
			assert.Equal(t, http.StatusOK, w.Code)
		} else {
			assert.Equal(t, http.StatusTooManyRequests, w.Code)
			assert.Equal(t, "1", w.Header().Get("X-RateLimit-Limit"))
			assert.Equal(t, "0", w.Header().Get("X-RateLimit-Remaining"))
			assert.Equal(t, "60", w.Header().Get("Retry-After"))
			assert.NotEmpty(t, w.Header().Get("X-RateLimit-Reset"))
		}
	}
}

func TestGetRateLimit(t *testing.T) {
	cfg := &config.Config{
		RateLimit: config.RateLimitConfig{
			Upload:   10,
			Download: 100,
			Info:     50,
		},
	}

	rl := &RateLimiter{config: cfg}

	tests := []struct {
		method       string
		path         string
		expectedRate int
	}{
		{"POST", "/api/v1/images", 10},
		{"GET", "/api/v1/images/123/thumbnail", 100},
		{"GET", "/api/v1/images/123/preview", 100},
		{"GET", "/api/v1/images/123/original", 100},
		{"GET", "/api/v1/images/123/info", 50},
		{"GET", "/health", 50},
		{"GET", "/some/other/endpoint", 0},
		{"POST", "/some/other/endpoint", 0},
	}

	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			rate := rl.getRateLimit(tt.method, tt.path)
			assert.Equal(t, tt.expectedRate, rate)
		})
	}
}

func TestRateLimiter_GetLimiter(t *testing.T) {
	cfg := &config.Config{}
	rl := &RateLimiter{
		limiters: make(map[string]*rate.Limiter),
		config:   cfg,
	}

	// Test creating new limiter
	limiter1 := rl.getLimiter("key1", 10)
	assert.NotNil(t, limiter1)
	assert.Equal(t, 10, limiter1.Burst()) // Burst equals rate

	// Test getting existing limiter
	limiter2 := rl.getLimiter("key1", 10)
	assert.Equal(t, limiter1, limiter2) // Should be same instance

	// Test different key creates different limiter
	limiter3 := rl.getLimiter("key2", 10)
	assert.NotEqual(t, limiter1, limiter3)
}

func TestRateLimiter_Cleanup(t *testing.T) {
	cfg := &config.Config{}
	rl := &RateLimiter{
		limiters: make(map[string]*rate.Limiter),
		config:   cfg,
	}

	// Add many limiters to trigger cleanup
	for i := 0; i < 15000; i++ {
		key := "key" + strconv.Itoa(i)
		rl.limiters[key] = rate.NewLimiter(rate.Every(time.Minute), 10)
	}

	assert.Greater(t, len(rl.limiters), 10000)

	// Run cleanup
	rl.cleanupOldLimiters()

	// Should have cleaned up some limiters
	assert.LessOrEqual(t, len(rl.limiters), 5000)
}

func TestRateLimiter_Stop(t *testing.T) {
	rl := &RateLimiter{
		cleanup:     time.NewTicker(time.Second),
		stopCleanup: make(chan struct{}),
	}

	// Should not panic
	assert.NotPanics(t, func() {
		rl.Stop()
	})
}

func TestRateLimit_RequestID(t *testing.T) {
	cfg := &config.Config{
		RateLimit: config.RateLimitConfig{
			Upload: 1,
		},
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()

	// Add request ID middleware first
	router.Use(func(c *gin.Context) {
		c.Set("request_id", "test-request-id")
		c.Next()
	})

	router.Use(RateLimit(cfg))
	router.POST("/api/v1/images", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	// Reset global rate limiter
	globalRateLimiter = nil
	once = sync.Once{}

	// Exhaust rate limit to trigger rate limit exceeded handler
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest("POST", "/api/v1/images", nil)
		req.RemoteAddr = "192.168.1.1:12345"
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if i >= 2 {
			assert.Equal(t, http.StatusTooManyRequests, w.Code)
			// Verify the handler ran (by checking response structure)
			var response map[string]interface{}
			err := testutil.ParseJSONResponse(w, &response)
			assert.NoError(t, err)
			assert.Equal(t, "Rate limit exceeded", response["error"])
		}
	}
}

func TestRateLimit_EdgeCases(t *testing.T) {
	t.Run("zero rate limit", func(t *testing.T) {
		cfg := &config.Config{
			RateLimit: config.RateLimitConfig{
				Upload: 0,
			},
		}

		gin.SetMode(gin.TestMode)
		router := gin.New()
		router.Use(RateLimit(cfg))
		router.POST("/api/v1/images", func(c *gin.Context) {
			c.Status(http.StatusOK)
		})

		// Reset global rate limiter
		globalRateLimiter = nil
		once = sync.Once{}

		// Should not rate limit when rate is 0
		req := httptest.NewRequest("POST", "/api/v1/images", nil)
		req.RemoteAddr = "192.168.1.1:12345"
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("negative rate limit", func(t *testing.T) {
		cfg := &config.Config{
			RateLimit: config.RateLimitConfig{
				Upload: -1,
			},
		}

		rl := &RateLimiter{config: cfg}
		rate := rl.getRateLimit("POST", "/api/v1/images")
		assert.Equal(t, -1, rate)
	})
}
