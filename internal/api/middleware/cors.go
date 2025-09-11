package middleware

import (
	"net/http"
	"strings"

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

		// Set CORS headers
		if origin != "" {
			if isAllowedOrigin(origin, cfg) {
				c.Header("Access-Control-Allow-Origin", origin)
			}
		} else {
			// For requests without Origin header (e.g., same-origin, Postman)
			if cfg.IsDevelopment() || cfg.CORS.AllowAllOrigins {
				c.Header("Access-Control-Allow-Origin", "*")
			}
		}

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

		// Handle preflight requests
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

// isAllowedOrigin checks if the origin is allowed
func isAllowedOrigin(origin string, cfg *config.Config) bool {
	// In development or if allow all origins is enabled, allow all origins
	if cfg.IsDevelopment() || cfg.CORS.AllowAllOrigins {
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

	// Check subdomain pattern (*.resizr.dev)
	if strings.HasSuffix(origin, ".resizr.dev") && strings.HasPrefix(origin, "https://") {
		return true
	}

	return false
}
