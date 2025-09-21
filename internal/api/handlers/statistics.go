package handlers

import (
	"net/http"

	"resizr/internal/models"
	"resizr/pkg/logger"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// StatisticsHandler handles statistics-related HTTP requests
type StatisticsHandler struct {
	statisticsService models.StatisticsService
}

// NewStatisticsHandler creates a new statistics handler
func NewStatisticsHandler(statisticsService models.StatisticsService) *StatisticsHandler {
	return &StatisticsHandler{
		statisticsService: statisticsService,
	}
}

// GetComprehensiveStatistics returns complete system statistics
// GET /api/v1/statistics
func (h *StatisticsHandler) GetComprehensiveStatistics(c *gin.Context) {
	ctx := c.Request.Context()
	requestID := c.GetString("request_id")

	logger.InfoWithContext(ctx, "Processing comprehensive statistics request",
		zap.String("request_id", requestID),
		zap.String("client_ip", c.ClientIP()))

	// Parse query parameters for options
	options := &models.StatisticsOptions{
		IncludeDetailedBreakdown:  c.DefaultQuery("detailed", "false") == "true",
		IncludePerformanceMetrics: c.DefaultQuery("performance", "true") == "true",
		IncludeSystemMetrics:      c.DefaultQuery("system", "true") == "true",
	}

	// Get comprehensive statistics
	stats, err := h.statisticsService.GetComprehensiveStatistics(options)
	if err != nil {
		logger.ErrorWithContext(ctx, "Failed to get comprehensive statistics",
			zap.Error(err),
			zap.String("request_id", requestID))
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "Statistics retrieval failed",
			Message: "Failed to retrieve system statistics",
			Code:    http.StatusInternalServerError,
		})
		return
	}

	logger.InfoWithContext(ctx, "Comprehensive statistics retrieved successfully",
		zap.Int64("total_images", stats.Images.TotalImages),
		zap.Int64("storage_used", stats.Storage.TotalStorageUsed),
		zap.Int64("deduped_images", stats.Deduplication.DedupedImages),
		zap.String("request_id", requestID))

	c.JSON(http.StatusOK, stats)
}

// GetImageStatistics returns only image-related statistics
// GET /api/v1/statistics/images
func (h *StatisticsHandler) GetImageStatistics(c *gin.Context) {
	ctx := c.Request.Context()
	requestID := c.GetString("request_id")

	logger.DebugWithContext(ctx, "Processing image statistics request",
		zap.String("request_id", requestID))

	stats, err := h.statisticsService.GetImageStatistics()
	if err != nil {
		logger.ErrorWithContext(ctx, "Failed to get image statistics",
			zap.Error(err),
			zap.String("request_id", requestID))
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "Image statistics retrieval failed",
			Message: "Failed to retrieve image statistics",
			Code:    http.StatusInternalServerError,
		})
		return
	}

	c.JSON(http.StatusOK, stats)
}

// GetStorageStatistics returns only storage-related statistics
// GET /api/v1/statistics/storage
func (h *StatisticsHandler) GetStorageStatistics(c *gin.Context) {
	ctx := c.Request.Context()
	requestID := c.GetString("request_id")

	logger.DebugWithContext(ctx, "Processing storage statistics request",
		zap.String("request_id", requestID))

	stats, err := h.statisticsService.GetStorageStatistics()
	if err != nil {
		logger.ErrorWithContext(ctx, "Failed to get storage statistics",
			zap.Error(err),
			zap.String("request_id", requestID))
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "Storage statistics retrieval failed",
			Message: "Failed to retrieve storage statistics",
			Code:    http.StatusInternalServerError,
		})
		return
	}

	c.JSON(http.StatusOK, stats)
}

// GetDeduplicationStatistics returns only deduplication-related statistics
// GET /api/v1/statistics/deduplication
func (h *StatisticsHandler) GetDeduplicationStatistics(c *gin.Context) {
	ctx := c.Request.Context()
	requestID := c.GetString("request_id")

	logger.DebugWithContext(ctx, "Processing deduplication statistics request",
		zap.String("request_id", requestID))

	stats, err := h.statisticsService.GetDeduplicationStatistics()
	if err != nil {
		logger.ErrorWithContext(ctx, "Failed to get deduplication statistics",
			zap.Error(err),
			zap.String("request_id", requestID))
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "Deduplication statistics retrieval failed",
			Message: "Failed to retrieve deduplication statistics",
			Code:    http.StatusInternalServerError,
		})
		return
	}

	c.JSON(http.StatusOK, stats)
}

// RefreshStatistics forces a refresh of cached statistics
// POST /api/v1/statistics/refresh
func (h *StatisticsHandler) RefreshStatistics(c *gin.Context) {
	ctx := c.Request.Context()
	requestID := c.GetString("request_id")

	logger.InfoWithContext(ctx, "Processing statistics refresh request",
		zap.String("request_id", requestID),
		zap.String("client_ip", c.ClientIP()))

	err := h.statisticsService.RefreshStatistics()
	if err != nil {
		logger.ErrorWithContext(ctx, "Failed to refresh statistics",
			zap.Error(err),
			zap.String("request_id", requestID))
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "Statistics refresh failed",
			Message: "Failed to refresh system statistics",
			Code:    http.StatusInternalServerError,
		})
		return
	}

	logger.InfoWithContext(ctx, "Statistics refreshed successfully",
		zap.String("request_id", requestID))

	c.JSON(http.StatusOK, gin.H{
		"message": "Statistics refreshed successfully",
		"status":  "success",
	})
}
