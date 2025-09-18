package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"resizr/internal/config"
	"resizr/internal/models"
	"resizr/internal/testutil"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAPIKeyAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		authEnabled    bool
		readWriteKeys  []string
		readOnlyKeys   []string
		keyHeader      string
		requestHeader  string
		requestValue   string
		expectedStatus int
		expectedPerm   string
	}{
		{
			name:           "auth disabled - should pass without key",
			authEnabled:    false,
			readWriteKeys:  []string{"rw-key"},
			readOnlyKeys:   []string{"ro-key"},
			keyHeader:      "X-API-Key",
			requestHeader:  "",
			requestValue:   "",
			expectedStatus: http.StatusOK,
			expectedPerm:   "",
		},
		{
			name:           "valid read-write key",
			authEnabled:    true,
			readWriteKeys:  []string{"rw-key-1", "rw-key-2"},
			readOnlyKeys:   []string{"ro-key-1"},
			keyHeader:      "X-API-Key",
			requestHeader:  "X-API-Key",
			requestValue:   "rw-key-1",
			expectedStatus: http.StatusOK,
			expectedPerm:   PermissionReadWrite,
		},
		{
			name:           "valid read-only key",
			authEnabled:    true,
			readWriteKeys:  []string{"rw-key-1"},
			readOnlyKeys:   []string{"ro-key-1", "ro-key-2"},
			keyHeader:      "X-API-Key",
			requestHeader:  "X-API-Key",
			requestValue:   "ro-key-2",
			expectedStatus: http.StatusOK,
			expectedPerm:   PermissionRead,
		},
		{
			name:           "invalid key",
			authEnabled:    true,
			readWriteKeys:  []string{"rw-key"},
			readOnlyKeys:   []string{"ro-key"},
			keyHeader:      "X-API-Key",
			requestHeader:  "X-API-Key",
			requestValue:   "invalid-key",
			expectedStatus: http.StatusUnauthorized,
			expectedPerm:   "",
		},
		{
			name:           "missing key header",
			authEnabled:    true,
			readWriteKeys:  []string{"rw-key"},
			readOnlyKeys:   []string{"ro-key"},
			keyHeader:      "X-API-Key",
			requestHeader:  "",
			requestValue:   "",
			expectedStatus: http.StatusUnauthorized,
			expectedPerm:   "",
		},
		{
			name:           "custom header name",
			authEnabled:    true,
			readWriteKeys:  []string{"rw-key"},
			readOnlyKeys:   []string{"ro-key"},
			keyHeader:      "Authorization",
			requestHeader:  "Authorization",
			requestValue:   "rw-key",
			expectedStatus: http.StatusOK,
			expectedPerm:   PermissionReadWrite,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup config
			cfg := &config.Config{
				Auth: config.AuthConfig{
					Enabled:       tt.authEnabled,
					ReadWriteKeys: tt.readWriteKeys,
					ReadOnlyKeys:  tt.readOnlyKeys,
					KeyHeader:     tt.keyHeader,
				},
			}

			// Setup router with middleware
			router := gin.New()
			router.Use(func(c *gin.Context) {
				c.Set("request_id", "test-request-id")
				c.Next()
			})
			router.Use(APIKeyAuth(cfg))
			router.GET("/test", func(c *gin.Context) {
				permission := c.GetString("auth_permission")
				c.JSON(http.StatusOK, gin.H{"permission": permission})
			})

			// Create request
			req := httptest.NewRequest("GET", "/test", nil)
			if tt.requestHeader != "" && tt.requestValue != "" {
				req.Header.Set(tt.requestHeader, tt.requestValue)
			}

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			// Verify response
			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedStatus == http.StatusOK {
				var response map[string]interface{}
				err := testutil.ParseJSONResponse(w, &response)
				require.NoError(t, err)
				assert.Equal(t, tt.expectedPerm, response["permission"])
			} else {
				var response models.ErrorResponse
				err := testutil.ParseJSONResponse(w, &response)
				require.NoError(t, err)
				assert.NotEmpty(t, response.Error)
			}
		})
	}
}

func TestRequirePermission(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name               string
		authEnabled        bool
		userPermission     string
		requiredPermission string
		expectedStatus     int
	}{
		{
			name:               "auth disabled - should pass",
			authEnabled:        false,
			userPermission:     "",
			requiredPermission: PermissionReadWrite,
			expectedStatus:     http.StatusOK,
		},
		{
			name:               "read-write user accessing read endpoint",
			authEnabled:        true,
			userPermission:     PermissionReadWrite,
			requiredPermission: PermissionRead,
			expectedStatus:     http.StatusOK,
		},
		{
			name:               "read-write user accessing write endpoint",
			authEnabled:        true,
			userPermission:     PermissionReadWrite,
			requiredPermission: PermissionReadWrite,
			expectedStatus:     http.StatusOK,
		},
		{
			name:               "read-only user accessing read endpoint",
			authEnabled:        true,
			userPermission:     PermissionRead,
			requiredPermission: PermissionRead,
			expectedStatus:     http.StatusOK,
		},
		{
			name:               "read-only user accessing write endpoint",
			authEnabled:        true,
			userPermission:     PermissionRead,
			requiredPermission: PermissionReadWrite,
			expectedStatus:     http.StatusForbidden,
		},
		{
			name:               "missing permission in context",
			authEnabled:        true,
			userPermission:     "",
			requiredPermission: PermissionRead,
			expectedStatus:     http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup config
			cfg := &config.Config{
				Auth: config.AuthConfig{
					Enabled: tt.authEnabled,
				},
			}

			// Setup router with middleware
			router := gin.New()
			router.Use(func(c *gin.Context) {
				c.Set("request_id", "test-request-id")
				c.Set("config", cfg)
				if tt.userPermission != "" {
					c.Set("auth_permission", tt.userPermission)
				}
				c.Next()
			})
			router.Use(RequirePermission(tt.requiredPermission))
			router.GET("/test", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"success": true})
			})

			// Create request
			req := httptest.NewRequest("GET", "/test", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			// Verify response
			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedStatus == http.StatusOK {
				var response map[string]interface{}
				err := testutil.ParseJSONResponse(w, &response)
				require.NoError(t, err)
				assert.True(t, response["success"].(bool))
			} else {
				var response models.ErrorResponse
				err := testutil.ParseJSONResponse(w, &response)
				require.NoError(t, err)
				assert.NotEmpty(t, response.Error)
			}
		})
	}
}

func TestValidateAPIKey(t *testing.T) {
	tests := []struct {
		name               string
		apiKey             string
		readWriteKeys      []string
		readOnlyKeys       []string
		expectedPermission string
	}{
		{
			name:               "valid read-write key",
			apiKey:             "rw-key-1",
			readWriteKeys:      []string{"rw-key-1", "rw-key-2"},
			readOnlyKeys:       []string{"ro-key-1"},
			expectedPermission: PermissionReadWrite,
		},
		{
			name:               "valid read-only key",
			apiKey:             "ro-key-1",
			readWriteKeys:      []string{"rw-key-1"},
			readOnlyKeys:       []string{"ro-key-1", "ro-key-2"},
			expectedPermission: PermissionRead,
		},
		{
			name:               "invalid key",
			apiKey:             "invalid-key",
			readWriteKeys:      []string{"rw-key-1"},
			readOnlyKeys:       []string{"ro-key-1"},
			expectedPermission: "",
		},
		{
			name:               "empty key",
			apiKey:             "",
			readWriteKeys:      []string{"rw-key-1"},
			readOnlyKeys:       []string{"ro-key-1"},
			expectedPermission: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			authConfig := config.AuthConfig{
				ReadWriteKeys: tt.readWriteKeys,
				ReadOnlyKeys:  tt.readOnlyKeys,
			}

			result := validateAPIKey(tt.apiKey, authConfig)
			assert.Equal(t, tt.expectedPermission, result)
		})
	}
}

func TestHasPermission(t *testing.T) {
	tests := []struct {
		name               string
		userPermission     string
		requiredPermission string
		expected           bool
	}{
		{
			name:               "read-write user can read",
			userPermission:     PermissionReadWrite,
			requiredPermission: PermissionRead,
			expected:           true,
		},
		{
			name:               "read-write user can write",
			userPermission:     PermissionReadWrite,
			requiredPermission: PermissionReadWrite,
			expected:           true,
		},
		{
			name:               "read-only user can read",
			userPermission:     PermissionRead,
			requiredPermission: PermissionRead,
			expected:           true,
		},
		{
			name:               "read-only user cannot write",
			userPermission:     PermissionRead,
			requiredPermission: PermissionReadWrite,
			expected:           false,
		},
		{
			name:               "invalid permission",
			userPermission:     "invalid",
			requiredPermission: PermissionRead,
			expected:           false,
		},
		{
			name:               "invalid required permission",
			userPermission:     PermissionRead,
			requiredPermission: "invalid",
			expected:           false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hasPermission(tt.userPermission, tt.requiredPermission)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMaskAPIKey(t *testing.T) {
	tests := []struct {
		name     string
		apiKey   string
		expected string
	}{
		{
			name:     "normal key",
			apiKey:   "abcdefghijklmnopqrstuvwxyz",
			expected: "abcdefgh******************",
		},
		{
			name:     "short key",
			apiKey:   "abc",
			expected: "***",
		},
		{
			name:     "8 character key",
			apiKey:   "abcdefgh",
			expected: "********",
		},
		{
			name:     "9 character key",
			apiKey:   "abcdefghi",
			expected: "abcdefgh*",
		},
		{
			name:     "empty key",
			apiKey:   "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MaskAPIKey(tt.apiKey)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGenerateAPIKey(t *testing.T) {
	// Test multiple generations to ensure uniqueness
	keys := make(map[string]bool)
	for i := 0; i < 10; i++ {
		key, err := GenerateAPIKey()
		require.NoError(t, err)
		assert.Len(t, key, 64) // 32 bytes * 2 hex chars = 64 characters
		assert.True(t, ValidateAPIKeyFormat(key))
		assert.False(t, keys[key], "Generated duplicate API key")
		keys[key] = true
	}
}

func TestValidateAPIKeyFormat(t *testing.T) {
	tests := []struct {
		name     string
		apiKey   string
		expected bool
	}{
		{
			name:     "valid 64-char hex key",
			apiKey:   "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
			expected: true,
		},
		{
			name:     "too short",
			apiKey:   "abcdef1234567890",
			expected: false,
		},
		{
			name:     "too long",
			apiKey:   "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890extra",
			expected: false,
		},
		{
			name:     "invalid hex characters",
			apiKey:   "ghijkl1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
			expected: false,
		},
		{
			name:     "empty key",
			apiKey:   "",
			expected: false,
		},
		{
			name:     "uppercase hex (should be valid)",
			apiKey:   "ABCDEF1234567890ABCDEF1234567890ABCDEF1234567890ABCDEF1234567890",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateAPIKeyFormat(tt.apiKey)
			assert.Equal(t, tt.expected, result)
		})
	}
}
