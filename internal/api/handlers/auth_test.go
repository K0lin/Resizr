package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"resizr/internal/api/middleware"
	"resizr/internal/config"
	"resizr/internal/models"
	"resizr/internal/testutil"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuthHandler_GenerateAPIKey(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		authEnabled    bool
		expectedStatus int
		expectKey      bool
	}{
		{
			name:           "generate key with auth enabled",
			authEnabled:    true,
			expectedStatus: http.StatusCreated,
			expectKey:      true,
		},
		{
			name:           "generate key with auth disabled",
			authEnabled:    false,
			expectedStatus: http.StatusCreated,
			expectKey:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			cfg := testutil.TestConfig()
			cfg.Auth.Enabled = tt.authEnabled
			handler := NewAuthHandler(cfg)

			// Create GET request
			req := httptest.NewRequest("GET", "/api/v1/auth/generate-key", nil)
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = req
			c.Set("request_id", "test-request-id")

			// Execute
			handler.GenerateAPIKey(c)

			// Verify
			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectKey {
				var response GenerateAPIKeyResponse
				err := testutil.ParseJSONResponse(w, &response)
				require.NoError(t, err)

				assert.NotEmpty(t, response.APIKey)
				assert.NotEmpty(t, response.Message)
				assert.True(t, middleware.ValidateAPIKeyFormat(response.APIKey))
				assert.Contains(t, response.Message, "AUTH_READWRITE_KEYS or AUTH_READONLY_KEYS")
			} else {
				var response models.ErrorResponse
				err := testutil.ParseJSONResponse(w, &response)
				require.NoError(t, err)
				assert.NotEmpty(t, response.Error)
			}
		})
	}
}

func TestAuthHandler_GenerateAPIKey_EdgeCases(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		authEnabled    bool
		expectedStatus int
		expectKey      bool
	}{
		{
			name:           "auth enabled generates key",
			authEnabled:    true,
			expectedStatus: http.StatusCreated,
			expectKey:      true,
		},
		{
			name:           "auth disabled generates key",
			authEnabled:    false,
			expectedStatus: http.StatusCreated,
			expectKey:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := testutil.TestConfig()
			cfg.Auth.Enabled = tt.authEnabled
			handler := NewAuthHandler(cfg)

			req := httptest.NewRequest("GET", "/api/v1/auth/generate-key", nil)
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = req
			c.Set("request_id", "test-request-id")

			handler.GenerateAPIKey(c)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectKey {
				var response GenerateAPIKeyResponse
				err := testutil.ParseJSONResponse(w, &response)
				require.NoError(t, err)
				assert.NotEmpty(t, response.APIKey)
				assert.Contains(t, response.Message, "AUTH_READWRITE_KEYS or AUTH_READONLY_KEYS")
			}
		})
	}
}

func TestAuthHandler_GetAuthStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name        string
		authEnabled bool
		rwKeysCount int
		roKeysCount int
		keyHeader   string
	}{
		{
			name:        "auth disabled",
			authEnabled: false,
			rwKeysCount: 0,
			roKeysCount: 0,
			keyHeader:   "X-API-Key",
		},
		{
			name:        "auth enabled with keys",
			authEnabled: true,
			rwKeysCount: 2,
			roKeysCount: 3,
			keyHeader:   "X-API-Key",
		},
		{
			name:        "auth enabled no keys",
			authEnabled: true,
			rwKeysCount: 0,
			roKeysCount: 0,
			keyHeader:   "Authorization",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup config
			cfg := &config.Config{
				Auth: config.AuthConfig{
					Enabled:       tt.authEnabled,
					ReadWriteKeys: make([]string, tt.rwKeysCount),
					ReadOnlyKeys:  make([]string, tt.roKeysCount),
					KeyHeader:     tt.keyHeader,
				},
			}

			handler := NewAuthHandler(cfg)

			// Create request
			req := httptest.NewRequest("GET", "/api/v1/auth/status", nil)
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = req
			c.Set("request_id", "test-request-id")

			// Execute
			handler.GetAuthStatus(c)

			// Verify
			assert.Equal(t, http.StatusOK, w.Code)

			var response map[string]interface{}
			err := testutil.ParseJSONResponse(w, &response)
			require.NoError(t, err)

			assert.Equal(t, tt.authEnabled, response["auth_enabled"])
			assert.Equal(t, tt.keyHeader, response["key_header"])

			if tt.authEnabled {
				assert.Equal(t, float64(tt.rwKeysCount), response["read_write_keys_count"])
				assert.Equal(t, float64(tt.roKeysCount), response["read_only_keys_count"])
			} else {
				assert.NotContains(t, response, "read_write_keys_count")
				assert.NotContains(t, response, "read_only_keys_count")
			}
		})
	}
}

func TestNewAuthHandler(t *testing.T) {
	cfg := testutil.TestConfig()
	handler := NewAuthHandler(cfg)

	assert.NotNil(t, handler)
	assert.Equal(t, cfg, handler.config)
}
