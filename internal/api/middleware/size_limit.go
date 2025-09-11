package middleware

import (
	"fmt"
	"net/http"
	"strconv"

	"resizr/internal/models"
	"resizr/pkg/logger"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// RequestSizeLimit middleware limits the size of incoming requests
func RequestSizeLimit(maxSize int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip size check for GET requests
		if c.Request.Method == "GET" || c.Request.Method == "HEAD" {
			c.Next()
			return
		}

		// Check Content-Length header
		if contentLengthStr := c.Request.Header.Get("Content-Length"); contentLengthStr != "" {
			contentLength, err := strconv.ParseInt(contentLengthStr, 10, 64)
			if err != nil {
				logger.WarnWithContext(c.Request.Context(), "Invalid Content-Length header",
					zap.String("content_length", contentLengthStr),
					zap.Error(err),
					zap.String("request_id", c.GetString("request_id")))

				c.JSON(http.StatusBadRequest, models.ErrorResponse{
					Error:   "Invalid Content-Length",
					Message: "Content-Length header contains invalid value",
					Code:    http.StatusBadRequest,
				})
				c.Abort()
				return
			}

			if contentLength > maxSize {
				logger.WarnWithContext(c.Request.Context(), "Request size exceeds limit",
					zap.Int64("content_length", contentLength),
					zap.Int64("max_size", maxSize),
					zap.String("client_ip", c.ClientIP()),
					zap.String("request_id", c.GetString("request_id")))

				c.JSON(http.StatusRequestEntityTooLarge, models.ErrorResponse{
					Error:   "Request too large",
					Message: fmt.Sprintf("Request size %d bytes exceeds maximum allowed size of %d bytes", contentLength, maxSize),
					Code:    http.StatusRequestEntityTooLarge,
				})
				c.Abort()
				return
			}
		}

		// Limit the request body reader
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxSize)

		c.Next()
	}
}
