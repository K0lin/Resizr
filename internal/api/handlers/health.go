package handlers

import (
	"net/http"
	"time"

	"resizr/internal/models"
	"resizr/internal/service"
	"resizr/pkg/logger"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// HealthHandler handles health check requests
type HealthHandler struct {
	healthService service.HealthService
}

// NewHealthHandler creates a new health handler
func NewHealthHandler(healthService service.HealthService) *HealthHandler {
	return &HealthHandler{
		healthService: healthService,
	}
}

// Health handles the main health check endpoint
// GET /health
func (h *HealthHandler) Health(c *gin.Context) {
	ctx := c.Request.Context()
	requestID := c.GetString("request_id")

	logger.DebugWithContext(ctx, "Processing health check",
		zap.String("request_id", requestID))

	// Check all services
	healthStatus, err := h.healthService.CheckHealth(ctx)
	if err != nil {
		logger.ErrorWithContext(ctx, "Health check failed",
			zap.Error(err),
			zap.String("request_id", requestID))

		c.JSON(http.StatusServiceUnavailable, models.HealthResponse{
			Status:    "unhealthy",
			Services:  map[string]string{"error": err.Error()},
			Timestamp: time.Now(),
		})
		return
	}

	// Determine overall status
	overallStatus := "healthy"
	statusCode := http.StatusOK

	for service, status := range healthStatus.Services {
		if status != "connected" && status != "healthy" {
			overallStatus = "degraded"
			if statusCode == http.StatusOK {
				statusCode = http.StatusPartialContent // 206
			}
			logger.WarnWithContext(ctx, "Service unhealthy",
				zap.String("service", service),
				zap.String("status", status),
				zap.String("request_id", requestID))
		}
	}

	response := models.HealthResponse{
		Status:    overallStatus,
		Services:  healthStatus.Services,
		Timestamp: time.Now(),
	}

	logger.InfoWithContext(ctx, "Health check completed",
		zap.String("overall_status", overallStatus),
		zap.Any("services", healthStatus.Services),
		zap.String("request_id", requestID))

	c.JSON(statusCode, response)
}

// Metrics handles the metrics endpoint (debug only)
// GET /debug/vars
func (h *HealthHandler) Metrics(c *gin.Context) {
	ctx := c.Request.Context()
	requestID := c.GetString("request_id")

	logger.DebugWithContext(ctx, "Processing metrics request",
		zap.String("request_id", requestID))

	metrics, err := h.healthService.GetMetrics(ctx)
	if err != nil {
		logger.ErrorWithContext(ctx, "Failed to get metrics",
			zap.Error(err),
			zap.String("request_id", requestID))

		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "Metrics unavailable",
			Message: err.Error(),
			Code:    http.StatusInternalServerError,
		})
		return
	}

	c.JSON(http.StatusOK, metrics)
}
