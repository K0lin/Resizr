package middleware

import (
	"resizr/pkg/logger"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const (
	// RequestIDHeader is the header name for request ID
	RequestIDHeader = "X-Request-ID"
	// RequestIDKey is the context key for request ID
	RequestIDKey = "request_id"
)

// RequestID middleware generates or extracts request ID for tracing
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		var requestID string

		// Check if request ID is provided in header
		if existingID := c.GetHeader(RequestIDHeader); existingID != "" {
			requestID = existingID
		} else {
			// Generate new request ID
			requestID = uuid.New().String()
		}

		// Set in context for handlers and logging
		c.Set(RequestIDKey, requestID)

		// Set response header
		c.Header(RequestIDHeader, requestID)

		// Add to logger context
		ctx := logger.WithRequestID(c.Request.Context(), requestID)
		c.Request = c.Request.WithContext(ctx)

		c.Next()
	}
}
