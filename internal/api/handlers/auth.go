package handlers

import (
	"net/http"

	"resizr/internal/api/middleware"
	"resizr/internal/config"
	"resizr/internal/models"
	"resizr/pkg/logger"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// AuthHandler handles authentication-related HTTP requests
type AuthHandler struct {
	config *config.Config
}

// NewAuthHandler creates a new auth handler
func NewAuthHandler(config *config.Config) *AuthHandler {
	return &AuthHandler{
		config: config,
	}
}

// GenerateAPIKeyResponse represents the response after generating an API key
type GenerateAPIKeyResponse struct {
	APIKey  string `json:"api_key"`
	Message string `json:"message"`
}

// GenerateAPIKey handles API key generation requests
// GET /api/v1/auth/generate-key?permission=read-write
func (h *AuthHandler) GenerateAPIKey(c *gin.Context) {
	ctx := c.Request.Context()
	requestID := c.GetString("request_id")

	logger.InfoWithContext(ctx, "Processing API key generation request",
		zap.String("request_id", requestID),
		zap.String("client_ip", c.ClientIP()))

	logger.InfoWithContext(ctx, "Generating new API key",
		zap.String("request_id", requestID),
		zap.Bool("auth_enabled", h.config.Auth.Enabled))

	// Generate new API key
	apiKey, err := middleware.GenerateAPIKey()
	if err != nil {
		logger.ErrorWithContext(ctx, "Failed to generate API key",
			zap.Error(err),
			zap.String("request_id", requestID))

		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "Key generation failed",
			Message: "Failed to generate API key",
			Code:    http.StatusInternalServerError,
		})
		return
	}

	logger.InfoWithContext(ctx, "API key generated successfully",
		zap.String("request_id", requestID),
		zap.String("api_key_prefix", middleware.MaskAPIKey(apiKey)))

	// Return the generated key
	response := GenerateAPIKeyResponse{
		APIKey:  apiKey,
		Message: "API key generated successfully. Add it to AUTH_READWRITE_KEYS or AUTH_READONLY_KEYS environment variable to activate.",
	}

	c.JSON(http.StatusCreated, response)
}

// GetAuthStatus returns the current authentication status
// GET /api/v1/auth/status
func (h *AuthHandler) GetAuthStatus(c *gin.Context) {
	ctx := c.Request.Context()
	requestID := c.GetString("request_id")

	logger.DebugWithContext(ctx, "Processing auth status request",
		zap.String("request_id", requestID))

	status := map[string]interface{}{
		"auth_enabled": h.config.Auth.Enabled,
		"key_header":   h.config.Auth.KeyHeader,
	}

	// If auth is enabled, show counts (but not the actual keys)
	if h.config.Auth.Enabled {
		status["read_write_keys_count"] = len(h.config.Auth.ReadWriteKeys)
		status["read_only_keys_count"] = len(h.config.Auth.ReadOnlyKeys)
	}

	c.JSON(http.StatusOK, status)
}
