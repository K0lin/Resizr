package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"resizr/internal/config"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestCORS_Enabled(t *testing.T) {
	tests := []struct {
		name            string
		config          *config.Config
		origin          string
		method          string
		expectedStatus  int
		expectedHeaders map[string]string
	}{
		{
			name: "allowed origin with credentials",
			config: &config.Config{
				CORS: config.CORSConfig{
					Enabled:          true,
					AllowAllOrigins:  false,
					AllowedOrigins:   []string{"https://example.com"},
					AllowCredentials: true,
				},
			},
			origin:         "https://example.com",
			method:         "GET",
			expectedStatus: http.StatusOK,
			expectedHeaders: map[string]string{
				"Access-Control-Allow-Origin":      "https://example.com",
				"Access-Control-Allow-Credentials": "true",
				"Access-Control-Allow-Methods":     "GET, POST, PUT, DELETE, OPTIONS",
			},
		},
		{
			name: "allowed origin without credentials",
			config: &config.Config{
				CORS: config.CORSConfig{
					Enabled:          true,
					AllowAllOrigins:  false,
					AllowedOrigins:   []string{"https://example.com"},
					AllowCredentials: false,
				},
			},
			origin:         "https://example.com",
			method:         "GET",
			expectedStatus: http.StatusOK,
			expectedHeaders: map[string]string{
				"Access-Control-Allow-Origin":      "https://example.com",
				"Access-Control-Allow-Credentials": "false",
			},
		},
		{
			name: "allow all origins",
			config: &config.Config{
				CORS: config.CORSConfig{
					Enabled:         true,
					AllowAllOrigins: true,
				},
			},
			origin:         "https://anydomain.com",
			method:         "GET",
			expectedStatus: http.StatusOK,
			expectedHeaders: map[string]string{
				"Access-Control-Allow-Origin": "https://anydomain.com",
			},
		},
		{
			name: "wildcard in allowed origins",
			config: &config.Config{
				CORS: config.CORSConfig{
					Enabled:        true,
					AllowedOrigins: []string{"*"},
				},
			},
			origin:         "https://anydomain.com",
			method:         "GET",
			expectedStatus: http.StatusOK,
			expectedHeaders: map[string]string{
				"Access-Control-Allow-Origin": "https://anydomain.com",
			},
		},
		{
			name: "disallowed origin",
			config: &config.Config{
				CORS: config.CORSConfig{
					Enabled:        true,
					AllowedOrigins: []string{"https://allowed.com"},
				},
			},
			origin:         "https://forbidden.com",
			method:         "GET",
			expectedStatus: http.StatusOK,
			expectedHeaders: map[string]string{
				"Access-Control-Allow-Origin": "", // Should not be set
			},
		},
		{
			name: "no origin header with allow all",
			config: &config.Config{
				CORS: config.CORSConfig{
					Enabled:         true,
					AllowAllOrigins: true,
				},
			},
			origin:         "",
			method:         "GET",
			expectedStatus: http.StatusOK,
			expectedHeaders: map[string]string{
				"Access-Control-Allow-Origin": "*",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			gin.SetMode(gin.TestMode)
			router := gin.New()
			router.Use(CORS(tt.config))
			router.GET("/test", func(c *gin.Context) {
				c.Status(http.StatusOK)
			})

			// Create request
			req := httptest.NewRequest(tt.method, "/test", nil)
			if tt.origin != "" {
				req.Header.Set("Origin", tt.origin)
			}

			w := httptest.NewRecorder()

			// Execute
			router.ServeHTTP(w, req)

			// Assert
			assert.Equal(t, tt.expectedStatus, w.Code)

			for headerName, expectedValue := range tt.expectedHeaders {
				if expectedValue == "" {
					// Header should not be set
					assert.Empty(t, w.Header().Get(headerName), "Header %s should not be set", headerName)
				} else {
					assert.Equal(t, expectedValue, w.Header().Get(headerName), "Header %s mismatch", headerName)
				}
			}
		})
	}
}

func TestCORS_Disabled(t *testing.T) {
	config := &config.Config{
		CORS: config.CORSConfig{
			Enabled: false,
		},
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(CORS(config))
	router.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Origin", "https://example.com")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// No CORS headers should be set
	assert.Empty(t, w.Header().Get("Access-Control-Allow-Origin"))
	assert.Empty(t, w.Header().Get("Access-Control-Allow-Methods"))
	assert.Empty(t, w.Header().Get("Access-Control-Allow-Headers"))
}

func TestCORS_PreflightRequests(t *testing.T) {
	tests := []struct {
		name           string
		config         *config.Config
		origin         string
		expectedStatus int
	}{
		{
			name: "allowed origin preflight",
			config: &config.Config{
				CORS: config.CORSConfig{
					Enabled:        true,
					AllowedOrigins: []string{"https://example.com"},
				},
			},
			origin:         "https://example.com",
			expectedStatus: http.StatusNoContent,
		},
		{
			name: "disallowed origin preflight",
			config: &config.Config{
				CORS: config.CORSConfig{
					Enabled:        true,
					AllowedOrigins: []string{"https://allowed.com"},
				},
			},
			origin:         "https://forbidden.com",
			expectedStatus: http.StatusForbidden,
		},
		{
			name: "allow all origins preflight",
			config: &config.Config{
				CORS: config.CORSConfig{
					Enabled:         true,
					AllowAllOrigins: true,
				},
			},
			origin:         "https://anydomain.com",
			expectedStatus: http.StatusNoContent,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			router := gin.New()
			router.Use(CORS(tt.config))
			router.GET("/test", func(c *gin.Context) {
				c.Status(http.StatusOK)
			})

			req := httptest.NewRequest("OPTIONS", "/test", nil)
			req.Header.Set("Origin", tt.origin)
			req.Header.Set("Access-Control-Request-Method", "POST")
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedStatus == http.StatusNoContent {
				assert.Equal(t, tt.origin, w.Header().Get("Access-Control-Allow-Origin"))
				assert.NotEmpty(t, w.Header().Get("Access-Control-Allow-Methods"))
			}
		})
	}
}

func TestIsAllowedOrigin(t *testing.T) {
	tests := []struct {
		name     string
		origin   string
		config   *config.Config
		expected bool
	}{
		{
			name:   "allow all origins enabled",
			origin: "https://example.com",
			config: &config.Config{
				CORS: config.CORSConfig{
					AllowAllOrigins: true,
				},
			},
			expected: true,
		},
		{
			name:   "wildcard in allowed origins",
			origin: "https://example.com",
			config: &config.Config{
				CORS: config.CORSConfig{
					AllowedOrigins: []string{"*"},
				},
			},
			expected: true,
		},
		{
			name:   "exact match",
			origin: "https://example.com",
			config: &config.Config{
				CORS: config.CORSConfig{
					AllowedOrigins: []string{"https://example.com", "https://test.com"},
				},
			},
			expected: true,
		},
		{
			name:   "no match",
			origin: "https://forbidden.com",
			config: &config.Config{
				CORS: config.CORSConfig{
					AllowedOrigins: []string{"https://example.com", "https://test.com"},
				},
			},
			expected: false,
		},
		{
			name:   "empty allowed origins",
			origin: "https://example.com",
			config: &config.Config{
				CORS: config.CORSConfig{
					AllowedOrigins: []string{},
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isAllowedOrigin(tt.origin, tt.config)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCORS_Headers(t *testing.T) {
	config := &config.Config{
		CORS: config.CORSConfig{
			Enabled:        true,
			AllowedOrigins: []string{"https://example.com"},
		},
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(CORS(config))
	router.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Origin", "https://example.com")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// Check all expected headers are set
	assert.Equal(t, "https://example.com", w.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "GET, POST, PUT, DELETE, OPTIONS", w.Header().Get("Access-Control-Allow-Methods"))
	assert.Equal(t, "Content-Type, Authorization, X-Request-ID, X-Requested-With", w.Header().Get("Access-Control-Allow-Headers"))
	assert.Equal(t, "X-Request-ID, Content-Length, Content-Type", w.Header().Get("Access-Control-Expose-Headers"))
	assert.Equal(t, "86400", w.Header().Get("Access-Control-Max-Age"))
}

func TestCORS_MultipleOrigins(t *testing.T) {
	config := &config.Config{
		CORS: config.CORSConfig{
			Enabled: true,
			AllowedOrigins: []string{
				"https://example.com",
				"https://test.com",
				"http://localhost:3000",
			},
		},
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(CORS(config))
	router.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	testOrigins := []struct {
		origin   string
		expected bool
	}{
		{"https://example.com", true},
		{"https://test.com", true},
		{"http://localhost:3000", true},
		{"https://forbidden.com", false},
	}

	for _, tt := range testOrigins {
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Origin", tt.origin)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		if tt.expected {
			assert.Equal(t, tt.origin, w.Header().Get("Access-Control-Allow-Origin"))
		} else {
			assert.Empty(t, w.Header().Get("Access-Control-Allow-Origin"))
		}
	}
}

func TestCORS_EdgeCases(t *testing.T) {
	t.Run("empty origin with strict config", func(t *testing.T) {
		config := &config.Config{
			CORS: config.CORSConfig{
				Enabled:         false,
				AllowAllOrigins: false,
				AllowedOrigins:  []string{"https://example.com"},
			},
		}

		gin.SetMode(gin.TestMode)
		router := gin.New()
		router.Use(CORS(config))
		router.GET("/test", func(c *gin.Context) {
			c.Status(http.StatusOK)
		})

		req := httptest.NewRequest("GET", "/test", nil)
		// No Origin header
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Empty(t, w.Header().Get("Access-Control-Allow-Origin"))
	})

	t.Run("case sensitivity", func(t *testing.T) {
		config := &config.Config{
			CORS: config.CORSConfig{
				Enabled:        true,
				AllowedOrigins: []string{"https://Example.com"},
			},
		}

		gin.SetMode(gin.TestMode)
		router := gin.New()
		router.Use(CORS(config))
		router.GET("/test", func(c *gin.Context) {
			c.Status(http.StatusOK)
		})

		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Origin", "https://example.com") // Different case
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		// Should not match due to case sensitivity
		assert.Empty(t, w.Header().Get("Access-Control-Allow-Origin"))
	})
}
