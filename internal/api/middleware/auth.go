package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"slices"
	"strings"

	"resizr/internal/config"
	"resizr/internal/models"
	"resizr/pkg/logger"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Permission levels
const (
	PermissionRead      = "read"
	PermissionReadWrite = "read-write"
)

// APIKeyAuth middleware validates API keys and sets permission level
func APIKeyAuth(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Set config in context so RequirePermission can access it
		c.Set("config", cfg)

		// Skip authentication if disabled
		if !cfg.Auth.Enabled {
			c.Next()
			return
		}

		requestID := c.GetString("request_id")

		// Get API key from header
		apiKey := c.GetHeader(cfg.Auth.KeyHeader)
		if apiKey == "" {
			logger.WarnWithContext(c.Request.Context(), "Missing API key",
				zap.String("request_id", requestID),
				zap.String("header", cfg.Auth.KeyHeader))

			c.JSON(http.StatusUnauthorized, models.ErrorResponse{
				Error:   "Missing API key",
				Message: "API key must be provided in " + cfg.Auth.KeyHeader + " header",
				Code:    http.StatusUnauthorized,
			})
			c.Abort()
			return
		}

		// Validate API key and determine permission level
		permission := validateAPIKey(apiKey, cfg.Auth)
		if permission == "" {
			logger.WarnWithContext(c.Request.Context(), "Invalid API key",
				zap.String("request_id", requestID),
				zap.String("api_key_prefix", MaskAPIKey(apiKey)))

			c.JSON(http.StatusUnauthorized, models.ErrorResponse{
				Error:   "Invalid API key",
				Message: "The provided API key is not valid",
				Code:    http.StatusUnauthorized,
			})
			c.Abort()
			return
		}

		// Set permission in context for use by other middleware/handlers
		c.Set("auth_permission", permission)

		logger.DebugWithContext(c.Request.Context(), "API key authenticated",
			zap.String("request_id", requestID),
			zap.String("permission", permission),
			zap.String("api_key_prefix", MaskAPIKey(apiKey)))

		c.Next()
	}
}

// RequirePermission middleware checks if the authenticated user has the required permission
func RequirePermission(required string) gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetString("request_id")

		// Skip permission check if auth is disabled
		cfg, exists := c.Get("config")
		if exists {
			if configData, ok := cfg.(*config.Config); ok && !configData.Auth.Enabled {
				c.Next()
				return
			}
		}

		// Get permission from context (set by APIKeyAuth middleware)
		permission := c.GetString("auth_permission")
		if permission == "" {
			logger.ErrorWithContext(c.Request.Context(), "Permission not found in context",
				zap.String("request_id", requestID),
				zap.String("required_permission", required))

			c.JSON(http.StatusInternalServerError, models.ErrorResponse{
				Error:   "Authentication error",
				Message: "Internal authentication error",
				Code:    http.StatusInternalServerError,
			})
			c.Abort()
			return
		}

		// Check if user has required permission
		if !hasPermission(permission, required) {
			logger.WarnWithContext(c.Request.Context(), "Insufficient permissions",
				zap.String("request_id", requestID),
				zap.String("user_permission", permission),
				zap.String("required_permission", required))

			c.JSON(http.StatusForbidden, models.ErrorResponse{
				Error:   "Insufficient permissions",
				Message: "This operation requires " + required + " permissions",
				Code:    http.StatusForbidden,
			})
			c.Abort()
			return
		}

		logger.DebugWithContext(c.Request.Context(), "Permission check passed",
			zap.String("request_id", requestID),
			zap.String("user_permission", permission),
			zap.String("required_permission", required))

		c.Next()
	}
}

// validateAPIKey validates an API key and returns the permission level
func validateAPIKey(apiKey string, authConfig config.AuthConfig) string {
	// Check read-write keys
	if slices.Contains(authConfig.ReadWriteKeys, apiKey) {
		return PermissionReadWrite
	}

	// Check read-only keys
	if slices.Contains(authConfig.ReadOnlyKeys, apiKey) {
		return PermissionRead
	}

	return ""
}

// hasPermission checks if a user permission level satisfies the required permission
func hasPermission(userPermission, requiredPermission string) bool {
	switch requiredPermission {
	case PermissionRead:
		// Both read and read-write permissions can access read operations
		return userPermission == PermissionRead || userPermission == PermissionReadWrite
	case PermissionReadWrite:
		// Only read-write permission can access write operations
		return userPermission == PermissionReadWrite
	default:
		return false
	}
}

// MaskAPIKey masks an API key for logging (shows only first 8 characters)
func MaskAPIKey(apiKey string) string {
	if len(apiKey) <= 8 {
		return strings.Repeat("*", len(apiKey))
	}
	return apiKey[:8] + strings.Repeat("*", len(apiKey)-8)
}

// GenerateAPIKey generates a cryptographically secure API key
func GenerateAPIKey() (string, error) {
	// Generate 32 random bytes (256 bits)
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}

	// Convert to hex string (64 characters)
	return hex.EncodeToString(bytes), nil
}

// ValidateAPIKeyFormat validates that an API key has the correct format
func ValidateAPIKeyFormat(apiKey string) bool {
	// API key should be 64 character hex string
	if len(apiKey) != 64 {
		return false
	}

	// Check if it's valid hex
	_, err := hex.DecodeString(apiKey)
	return err == nil
}
