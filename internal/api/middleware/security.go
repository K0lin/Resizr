package middleware

import (
	"resizr/internal/config"

	"github.com/gin-gonic/gin"
)

// SecurityHeaders middleware adds security headers to responses
func SecurityHeaders(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Prevent MIME type sniffing
		c.Header("X-Content-Type-Options", "nosniff")

		// Prevent clickjacking
		c.Header("X-Frame-Options", "DENY")

		// Enable XSS protection
		c.Header("X-XSS-Protection", "1; mode=block")

		// Referrer policy
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")

		// Content Security Policy (basic)
		if cfg.IsProduction() {
			csp := "default-src 'self'; " +
				"img-src 'self' data: https:; " +
				"script-src 'self'; " +
				"style-src 'self' 'unsafe-inline'; " +
				"object-src 'none'; " +
				"base-uri 'self'; " +
				"form-action 'self'"
			c.Header("Content-Security-Policy", csp)
		}

		// HSTS (HTTP Strict Transport Security) for production
		if cfg.IsProduction() {
			c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}

		// Remove server information
		c.Header("Server", "RESIZR")

		c.Next()
	}
}
