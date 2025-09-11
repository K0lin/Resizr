package middleware

import (
	"net/http"

	"resizr/internal/config"

	"github.com/gin-gonic/gin"
)

// CORS middleware handles Cross-Origin Resource Sharing
func CORS(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip CORS if disabled
		if !cfg.CORS.Enabled {
			c.Next()
			return
		}

		origin := c.Request.Header.Get("Origin")
		var allowedOrigin bool

		// Check if origin is allowed and set CORS headers only for allowed origins
		if origin != "" && isAllowedOrigin(origin, cfg) {
			c.Header("Access-Control-Allow-Origin", origin)
			allowedOrigin = true
		} else if origin == "" && cfg.CORS.AllowAllOrigins {
			// Only set wildcard for non-origin requests if explicitly allowing all origins
			c.Header("Access-Control-Allow-Origin", "*")
			allowedOrigin = true
		}

		// Only set other CORS headers if origin is allowed
		if allowedOrigin {
			c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Request-ID, X-Requested-With")
			c.Header("Access-Control-Expose-Headers", "X-Request-ID, Content-Length, Content-Type")

			// Set credentials header based on configuration
			if cfg.CORS.AllowCredentials {
				c.Header("Access-Control-Allow-Credentials", "true")
			} else {
				c.Header("Access-Control-Allow-Credentials", "false")
			}

			c.Header("Access-Control-Max-Age", "86400") // 24 hours
		}

		// Handle preflight requests
		if c.Request.Method == "OPTIONS" {
			if allowedOrigin {
				c.AbortWithStatus(http.StatusNoContent)
			} else {
				c.AbortWithStatus(http.StatusForbidden)
			}
			return
		}

		c.Next()
	}
}

// isAllowedOrigin checks if the origin is allowed
func isAllowedOrigin(origin string, cfg *config.Config) bool {
	// If allow all origins is enabled, allow all origins
	if cfg.CORS.AllowAllOrigins {
		return true
	}

	// Check exact match using configured allowed origins
	for _, allowed := range cfg.CORS.AllowedOrigins {
		if allowed == "*" {
			return true
		}
		if origin == allowed {
			return true
		}
	}

	return false
}
