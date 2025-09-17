package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"resizr/internal/config"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestSecurityHeaders_Development(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			GinMode: "debug", // Development mode
		},
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(SecurityHeaders(cfg))
	router.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Check basic security headers
	assert.Equal(t, "nosniff", w.Header().Get("X-Content-Type-Options"))
	assert.Equal(t, "DENY", w.Header().Get("X-Frame-Options"))
	assert.Equal(t, "1; mode=block", w.Header().Get("X-XSS-Protection"))
	assert.Equal(t, "strict-origin-when-cross-origin", w.Header().Get("Referrer-Policy"))
	assert.Equal(t, "RESIZR", w.Header().Get("Server"))

	// Production-only headers should not be set in development
	assert.Empty(t, w.Header().Get("Content-Security-Policy"))
	assert.Empty(t, w.Header().Get("Strict-Transport-Security"))
}

func TestSecurityHeaders_Production(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			GinMode: "release", // Production mode
		},
		Logger: config.LoggerConfig{
			Format: "json", // Production mode requires json format
		},
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(SecurityHeaders(cfg))
	router.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Check all security headers including production-only ones
	assert.Equal(t, "nosniff", w.Header().Get("X-Content-Type-Options"))
	assert.Equal(t, "DENY", w.Header().Get("X-Frame-Options"))
	assert.Equal(t, "1; mode=block", w.Header().Get("X-XSS-Protection"))
	assert.Equal(t, "strict-origin-when-cross-origin", w.Header().Get("Referrer-Policy"))
	assert.Equal(t, "RESIZR", w.Header().Get("Server"))

	// Production-specific headers
	csp := w.Header().Get("Content-Security-Policy")
	assert.NotEmpty(t, csp)
	assert.Contains(t, csp, "default-src 'self'")
	assert.Contains(t, csp, "img-src 'self' data: https:")
	assert.Contains(t, csp, "script-src 'self'")
	assert.Contains(t, csp, "style-src 'self' 'unsafe-inline'")
	assert.Contains(t, csp, "object-src 'none'")
	assert.Contains(t, csp, "base-uri 'self'")
	assert.Contains(t, csp, "form-action 'self'")

	hsts := w.Header().Get("Strict-Transport-Security")
	assert.Equal(t, "max-age=31536000; includeSubDomains", hsts)
}

func TestSecurityHeaders_CSPContent(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			GinMode: "release",
		},
		Logger: config.LoggerConfig{
			Format: "json",
		},
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(SecurityHeaders(cfg))
	router.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	csp := w.Header().Get("Content-Security-Policy")

	// Verify all CSP directives are present
	expectedDirectives := []string{
		"default-src 'self'",
		"img-src 'self' data: https:",
		"script-src 'self'",
		"style-src 'self' 'unsafe-inline'",
		"object-src 'none'",
		"base-uri 'self'",
		"form-action 'self'",
	}

	for _, directive := range expectedDirectives {
		assert.Contains(t, csp, directive, "CSP should contain directive: %s", directive)
	}
}

func TestSecurityHeaders_MultipleRequests(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			GinMode: "release",
		},
		Logger: config.LoggerConfig{
			Format: "json",
		},
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(SecurityHeaders(cfg))
	router.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	// Make multiple requests to ensure consistency
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "nosniff", w.Header().Get("X-Content-Type-Options"))
		assert.Equal(t, "DENY", w.Header().Get("X-Frame-Options"))
		assert.NotEmpty(t, w.Header().Get("Content-Security-Policy"))
		assert.NotEmpty(t, w.Header().Get("Strict-Transport-Security"))
	}
}

func TestSecurityHeaders_DifferentMethods(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			GinMode: "release",
		},
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(SecurityHeaders(cfg))
	router.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	router.POST("/test", func(c *gin.Context) {
		c.Status(http.StatusCreated)
	})
	router.PUT("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	router.DELETE("/test", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	methods := []struct {
		method string
		status int
	}{
		{"GET", http.StatusOK},
		{"POST", http.StatusCreated},
		{"PUT", http.StatusOK},
		{"DELETE", http.StatusNoContent},
	}

	for _, m := range methods {
		t.Run(m.method, func(t *testing.T) {
			req := httptest.NewRequest(m.method, "/test", nil)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			assert.Equal(t, m.status, w.Code)

			// All methods should get security headers
			assert.Equal(t, "nosniff", w.Header().Get("X-Content-Type-Options"))
			assert.Equal(t, "DENY", w.Header().Get("X-Frame-Options"))
			assert.Equal(t, "1; mode=block", w.Header().Get("X-XSS-Protection"))
			assert.Equal(t, "RESIZR", w.Header().Get("Server"))
		})
	}
}

func TestSecurityHeaders_WithOtherMiddleware(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			GinMode: "debug",
		},
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()

	// Add security headers first
	router.Use(SecurityHeaders(cfg))

	// Add another middleware that also sets headers
	router.Use(func(c *gin.Context) {
		c.Header("Custom-Header", "test-value")
		c.Next()
	})

	router.GET("/test", func(c *gin.Context) {
		c.Header("Handler-Header", "handler-value")
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Security headers should be present
	assert.Equal(t, "nosniff", w.Header().Get("X-Content-Type-Options"))
	assert.Equal(t, "DENY", w.Header().Get("X-Frame-Options"))

	// Other headers should also be present
	assert.Equal(t, "test-value", w.Header().Get("Custom-Header"))
	assert.Equal(t, "handler-value", w.Header().Get("Handler-Header"))
}

func TestSecurityHeaders_ServerHeaderOverride(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			GinMode: "debug",
		},
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(SecurityHeaders(cfg))
	router.GET("/test", func(c *gin.Context) {
		// Try to override server header in handler
		c.Header("Server", "CustomServer")
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// The middleware sets the Server header, but handler can override it
	// Gin processes headers in order, so the last one wins
	assert.Equal(t, "CustomServer", w.Header().Get("Server"))
}

func TestSecurityHeaders_EdgeCases(t *testing.T) {
	t.Run("nil config", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		router := gin.New()

		// This should not panic even with nil config
		assert.NotPanics(t, func() {
			router.Use(SecurityHeaders(nil))
			router.GET("/test", func(c *gin.Context) {
				c.Status(http.StatusOK)
			})
		})

		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()

		// This would panic if not handled properly
		assert.Panics(t, func() {
			router.ServeHTTP(w, req)
		})
	})

	t.Run("test mode gin mode", func(t *testing.T) {
		cfg := &config.Config{
			Server: config.ServerConfig{
				GinMode: "test",
			},
		}

		gin.SetMode(gin.TestMode)
		router := gin.New()
		router.Use(SecurityHeaders(cfg))
		router.GET("/test", func(c *gin.Context) {
			c.Status(http.StatusOK)
		})

		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		// Test mode should behave like development (no production headers)
		assert.Equal(t, "nosniff", w.Header().Get("X-Content-Type-Options"))
		assert.Empty(t, w.Header().Get("Content-Security-Policy"))
		assert.Empty(t, w.Header().Get("Strict-Transport-Security"))
	})
}

func TestSecurityHeaders_AllHeaders(t *testing.T) {
	// Test that all expected headers are set with correct values
	cfg := &config.Config{
		Server: config.ServerConfig{
			GinMode: "release",
		},
		Logger: config.LoggerConfig{
			Format: "json",
		},
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(SecurityHeaders(cfg))
	router.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	expectedHeaders := map[string]string{
		"X-Content-Type-Options":    "nosniff",
		"X-Frame-Options":           "DENY",
		"X-XSS-Protection":          "1; mode=block",
		"Referrer-Policy":           "strict-origin-when-cross-origin",
		"Server":                    "RESIZR",
		"Strict-Transport-Security": "max-age=31536000; includeSubDomains",
	}

	for headerName, expectedValue := range expectedHeaders {
		assert.Equal(t, expectedValue, w.Header().Get(headerName), "Header %s should have value %s", headerName, expectedValue)
	}

	// CSP is more complex, just check it exists and has key parts
	csp := w.Header().Get("Content-Security-Policy")
	assert.NotEmpty(t, csp)
	assert.Contains(t, csp, "default-src 'self'")
}
