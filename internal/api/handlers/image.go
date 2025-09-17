package handlers

import (
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"resizr/internal/config"
	"resizr/internal/models"
	"resizr/internal/service"
	"resizr/pkg/logger"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// ImageHandler handles image-related HTTP requests
type ImageHandler struct {
	imageService service.ImageService
	config       *config.Config
}

// NewImageHandler creates a new image handler
func NewImageHandler(imageService service.ImageService, config *config.Config) *ImageHandler {
	return &ImageHandler{
		imageService: imageService,
		config:       config,
	}
}

// Upload handles image upload requests
// POST /api/v1/images
func (h *ImageHandler) Upload(c *gin.Context) {
	ctx := c.Request.Context()
	requestID := c.GetString("request_id")

	logger.InfoWithContext(ctx, "Processing image upload",
		zap.String("request_id", requestID),
		zap.String("client_ip", c.ClientIP()))

	// Parse multipart form
	if err := c.Request.ParseMultipartForm(h.config.Image.MaxFileSize); err != nil {
		logger.ErrorWithContext(ctx, "Failed to parse multipart form",
			zap.Error(err),
			zap.String("request_id", requestID))
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Invalid form data",
			Message: "Failed to parse multipart form",
			Code:    http.StatusBadRequest,
		})
		return
	}

	// Get uploaded file
	file, header, err := c.Request.FormFile("image")
	if err != nil {
		logger.ErrorWithContext(ctx, "No image file in request",
			zap.Error(err),
			zap.String("request_id", requestID))
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Missing image file",
			Message: "Request must contain an 'image' file field",
			Code:    http.StatusBadRequest,
		})
		return
	}
	defer file.Close()

	// Validate file size
	if header.Size > h.config.Image.MaxFileSize {
		logger.WarnWithContext(ctx, "File size exceeds limit",
			zap.Int64("file_size", header.Size),
			zap.Int64("max_size", h.config.Image.MaxFileSize),
			zap.String("request_id", requestID))
		c.JSON(http.StatusRequestEntityTooLarge, models.ErrorResponse{
			Error:   "File too large",
			Message: fmt.Sprintf("File size %d bytes exceeds limit of %d bytes", header.Size, h.config.Image.MaxFileSize),
			Code:    http.StatusRequestEntityTooLarge,
		})
		return
	}

	// Parse additional resolutions from form
	var req models.UploadRequest

	// Get resolutions from form - handle both single and multiple field approaches
	if values := c.Request.Form["resolutions"]; len(values) > 0 {
		// Handle both multiple fields and comma-separated values
		var allResolutions []string
		for _, value := range values {
			// Split each value by comma in case it contains multiple resolutions
			splitValues := strings.Split(value, ",")
			for _, splitValue := range splitValues {
				trimmed := strings.TrimSpace(splitValue)
				if trimmed != "" {
					allResolutions = append(allResolutions, trimmed)
				}
			}
		}
		req.Resolutions = allResolutions
	} else if err := c.ShouldBind(&req); err != nil {
		logger.WarnWithContext(ctx, "Invalid resolution parameters",
			zap.Error(err),
			zap.String("request_id", requestID))
		// Continue with empty resolutions - this is optional
	}

	// Read file data
	fileData, err := io.ReadAll(file)
	if err != nil {
		logger.ErrorWithContext(ctx, "Failed to read file data",
			zap.Error(err),
			zap.String("request_id", requestID))
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "File read error",
			Message: "Failed to read uploaded file",
			Code:    http.StatusInternalServerError,
		})
		return
	}

	// Process upload through service layer
	result, err := h.imageService.ProcessUpload(ctx, service.UploadInput{
		Filename:    header.Filename,
		Data:        fileData,
		Size:        header.Size,
		Resolutions: req.Resolutions,
	})

	if err != nil {
		h.handleServiceError(c, err, requestID, "upload failed")
		return
	}

	logger.InfoWithContext(ctx, "Image upload completed successfully",
		zap.String("image_id", result.ImageID),
		zap.String("filename", header.Filename),
		zap.Int64("size", header.Size),
		zap.Strings("resolutions", result.ProcessedResolutions),
		zap.String("request_id", requestID))

	// Return success response
	response := models.UploadResponse{
		ID:          result.ImageID,
		Message:     "Image uploaded successfully",
		Resolutions: result.ProcessedResolutions,
	}

	c.JSON(http.StatusCreated, response)
}

// Info handles image metadata requests
// GET /api/v1/images/:id/info
func (h *ImageHandler) Info(c *gin.Context) {
	ctx := c.Request.Context()
	requestID := c.GetString("request_id")
	imageID := c.Param("id")

	logger.DebugWithContext(ctx, "Getting image info",
		zap.String("image_id", imageID),
		zap.String("request_id", requestID))

	// Validate UUID format
	if !h.isValidUUID(imageID) {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Invalid image ID",
			Message: "Image ID must be a valid UUID",
			Code:    http.StatusBadRequest,
		})
		return
	}

	// Get image metadata
	metadata, err := h.imageService.GetMetadata(ctx, imageID)
	if err != nil {
		h.handleServiceError(c, err, requestID, "get metadata failed")
		return
	}

	// Convert to API response
	response := metadata.ToInfoResponse()
	c.JSON(http.StatusOK, response)
}

// DownloadOriginal handles original image download
// GET /api/v1/images/:id/original
func (h *ImageHandler) DownloadOriginal(c *gin.Context) {
	h.downloadImage(c, "original")
}

// DownloadThumbnail handles thumbnail download
// GET /api/v1/images/:id/thumbnail
func (h *ImageHandler) DownloadThumbnail(c *gin.Context) {
	h.downloadImage(c, "thumbnail")
}

// DownloadPreview handles preview download
// GET /api/v1/images/:id/preview
func (h *ImageHandler) DownloadPreview(c *gin.Context) {
	h.downloadImage(c, "preview")
}

// DownloadCustomResolution handles custom resolution download
// GET /api/v1/images/:id/:resolution
func (h *ImageHandler) DownloadCustomResolution(c *gin.Context) {
	resolution := c.Param("resolution")

	// Validate resolution format (e.g., "800x600")
	if !h.isValidCustomResolution(resolution) {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Invalid resolution format",
			Message: "Resolution must be in format WIDTHxHEIGHT (e.g., 800x600)",
			Code:    http.StatusBadRequest,
		})
		return
	}

	h.downloadImage(c, resolution)
}

// GeneratePresignedURL generates a pre-signed URL for image access
// GET /api/v1/images/:id/:resolution/presigned-url
func (h *ImageHandler) GeneratePresignedURL(c *gin.Context) {
	ctx := c.Request.Context()
	requestID := c.GetString("request_id")
	imageID := c.Param("id")

	// Get resolution from URL path
	size := c.Param("resolution")

	// Handle predefined resolutions by detecting URL patterns
	fullPath := c.FullPath()
	if strings.Contains(fullPath, "/original/presigned-url") {
		size = "original"
	} else if strings.Contains(fullPath, "/thumbnail/presigned-url") {
		size = "thumbnail"
	} else if strings.Contains(fullPath, "/preview/presigned-url") {
		size = "preview"
	}

	// Get optional expires_in parameter (default: 1 hour)
	expiresInParam := c.Query("expires_in")
	expiresIn := 3600 // default: 1 hour
	if expiresInParam != "" {
		parsed, err := strconv.Atoi(expiresInParam)
		if err != nil || parsed <= 0 {
			c.JSON(http.StatusBadRequest, models.ErrorResponse{
				Error:   "Invalid expires_in parameter",
				Message: "expires_in must be a positive integer (seconds)",
				Code:    http.StatusBadRequest,
			})
			return
		}
		expiresIn = parsed
	}

	// Validate maximum expiration (7 days)
	maxExpiresIn := 7 * 24 * 3600 // 7 days in seconds
	if expiresIn > maxExpiresIn {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "expires_in too large",
			Message: fmt.Sprintf("Maximum expiration is %d seconds (7 days)", maxExpiresIn),
			Code:    http.StatusBadRequest,
		})
		return
	}

	logger.DebugWithContext(ctx, "Generating presigned URL",
		zap.String("image_id", imageID),
		zap.String("size", size),
		zap.Int("expires_in", expiresIn),
		zap.String("request_id", requestID))

	// Validate UUID format
	if !h.isValidUUID(imageID) {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Invalid image ID",
			Message: "Image ID must be a valid UUID",
			Code:    http.StatusBadRequest,
		})
		return
	}

	// Note: Size format validation is done after getting metadata to allow 404 for unavailable resolutions

	// Get image metadata to verify image exists
	metadata, err := h.imageService.GetMetadata(ctx, imageID)
	if err != nil {
		h.handleServiceError(c, err, requestID, "get metadata for presigned URL failed")
		return
	}

	// Validate size exists (except for original)
	if size != "original" && !metadata.HasResolution(size) {
		c.JSON(http.StatusNotFound, models.ErrorResponse{
			Error:   "Resolution not found",
			Message: fmt.Sprintf("Resolution '%s' not available for this image", size),
			Code:    http.StatusNotFound,
		})
		return
	}

	// Validate size format for custom resolutions (after checking availability)
	if size != "original" && size != "thumbnail" && size != "preview" && !h.isValidCustomResolution(size) {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Invalid size format",
			Message: "Custom resolution must be in format WIDTHxHEIGHT (e.g., 800x600)",
			Code:    http.StatusBadRequest,
		})
		return
	}

	// Generate storage key and presigned URL
	storageKey := metadata.GetStorageKey(size)
	duration := time.Duration(expiresIn) * time.Second

	presignedURL, err := h.imageService.GeneratePresignedURL(ctx, storageKey, duration)
	if err != nil {
		h.handleServiceError(c, err, requestID, "generate presigned URL failed")
		return
	}

	expiresAt := time.Now().Add(duration)

	logger.InfoWithContext(ctx, "Presigned URL generated successfully",
		zap.String("image_id", imageID),
		zap.String("size", size),
		zap.Int("expires_in", expiresIn),
		zap.Time("expires_at", expiresAt),
		zap.String("request_id", requestID))

	c.JSON(http.StatusOK, models.PresignedURLResponse{
		URL:       presignedURL,
		ExpiresAt: expiresAt,
		ExpiresIn: expiresIn,
	})
}

// downloadImage is a common handler for all image downloads
func (h *ImageHandler) downloadImage(c *gin.Context, resolution string) {
	ctx := c.Request.Context()
	requestID := c.GetString("request_id")
	imageID := c.Param("id")

	logger.DebugWithContext(ctx, "Processing image download",
		zap.String("image_id", imageID),
		zap.String("resolution", resolution),
		zap.String("request_id", requestID))

	// Validate UUID format
	if !h.isValidUUID(imageID) {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Invalid image ID",
			Message: "Image ID must be a valid UUID",
			Code:    http.StatusBadRequest,
		})
		return
	}

	// Get image stream from service
	stream, metadata, err := h.imageService.GetImageStream(ctx, imageID, resolution)
	if err != nil {
		h.handleServiceError(c, err, requestID, "get image stream failed")
		return
	}
	defer stream.Close()

	// Set response headers
	h.setImageResponseHeaders(c, metadata, resolution)

	// Stream image data to client
	logger.DebugWithContext(ctx, "Streaming image to client",
		zap.String("image_id", imageID),
		zap.String("resolution", resolution),
		zap.String("mime_type", metadata.MimeType),
		zap.String("request_id", requestID))

	// Copy stream to response
	bytesWritten, err := io.Copy(c.Writer, stream)
	if err != nil {
		logger.ErrorWithContext(ctx, "Failed to stream image data",
			zap.Error(err),
			zap.String("image_id", imageID),
			zap.String("resolution", resolution),
			zap.String("request_id", requestID))
		return
	}

	logger.InfoWithContext(ctx, "Image download completed",
		zap.String("image_id", imageID),
		zap.String("resolution", resolution),
		zap.Int64("bytes_streamed", bytesWritten),
		zap.String("request_id", requestID))
}

// setImageResponseHeaders sets appropriate headers for image responses
func (h *ImageHandler) setImageResponseHeaders(c *gin.Context, metadata *models.ImageMetadata, resolution string) {
	// Set content type based on image format
	c.Header("Content-Type", metadata.MimeType)

	// Set cache headers
	c.Header("Cache-Control", "public, max-age=3600, immutable")
	c.Header("ETag", fmt.Sprintf(`"%s-%s"`, metadata.ID, resolution))

	// Set content disposition for downloads
	filename := h.generateDownloadFilename(metadata.Filename, resolution)
	c.Header("Content-Disposition", fmt.Sprintf(`inline; filename="%s"`, filename))

	// Set additional headers for browser compatibility
	c.Header("Accept-Ranges", "bytes")
}

// generateDownloadFilename generates appropriate filename for downloads
func (h *ImageHandler) generateDownloadFilename(originalFilename, resolution string) string {
	// Extract file extension
	parts := strings.Split(originalFilename, ".")
	var ext string
	if len(parts) > 1 {
		ext = parts[len(parts)-1]
	} else {
		ext = "jpg" // default
	}

	// Extract base filename
	base := strings.TrimSuffix(originalFilename, "."+ext)

	if resolution == "original" {
		return fmt.Sprintf("%s.%s", base, ext)
	}

	return fmt.Sprintf("%s_%s.%s", base, resolution, ext)
}

// handleServiceError handles errors from the service layer
func (h *ImageHandler) handleServiceError(c *gin.Context, err error, requestID, operation string) {
	ctx := c.Request.Context()

	switch e := err.(type) {
	case models.ValidationError:
		logger.WarnWithContext(ctx, "Validation error",
			zap.String("field", e.Field),
			zap.String("message", e.Message),
			zap.String("request_id", requestID),
			zap.String("operation", operation))
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "Validation failed",
			Message: e.Error(),
			Code:    http.StatusBadRequest,
		})

	case models.NotFoundError:
		logger.WarnWithContext(ctx, "Resource not found",
			zap.String("resource", e.Resource),
			zap.String("id", e.ID),
			zap.String("request_id", requestID),
			zap.String("operation", operation))
		c.JSON(http.StatusNotFound, models.ErrorResponse{
			Error:   "Not found",
			Message: e.Error(),
			Code:    http.StatusNotFound,
		})

	case models.ProcessingError:
		logger.ErrorWithContext(ctx, "Processing error",
			zap.String("operation_type", e.Operation),
			zap.String("reason", e.Reason),
			zap.String("request_id", requestID),
			zap.String("operation", operation))
		c.JSON(http.StatusUnprocessableEntity, models.ErrorResponse{
			Error:   "Processing failed",
			Message: e.Error(),
			Code:    http.StatusUnprocessableEntity,
		})

	case models.StorageError:
		logger.ErrorWithContext(ctx, "Storage error",
			zap.String("storage_operation", e.Operation),
			zap.String("backend", e.Backend),
			zap.String("reason", e.Reason),
			zap.String("request_id", requestID),
			zap.String("operation", operation))
		c.JSON(http.StatusServiceUnavailable, models.ErrorResponse{
			Error:   "Storage unavailable",
			Message: "Temporary service unavailability",
			Code:    http.StatusServiceUnavailable,
		})

	default:
		logger.ErrorWithContext(ctx, "Unknown error",
			zap.Error(err),
			zap.String("request_id", requestID),
			zap.String("operation", operation))
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "Internal server error",
			Message: "An unexpected error occurred",
			Code:    http.StatusInternalServerError,
		})
	}
}

// Validation helpers

func (h *ImageHandler) isValidUUID(id string) bool {
	// Simple UUID validation - can be improved with regex
	return len(id) == 36 && strings.Count(id, "-") == 4
}

func (h *ImageHandler) isValidCustomResolution(resolution string) bool {
	// Validate format: numbers + 'x' + numbers (e.g., "800x600")
	parts := strings.Split(resolution, "x")
	if len(parts) != 2 {
		return false
	}

	// Check if both parts are numbers
	for _, part := range parts {
		if _, err := strconv.Atoi(part); err != nil {
			return false
		}
	}

	return true
}

func (h *ImageHandler) isValidSize(size string) bool {
	// Check predefined sizes
	if size == "original" || size == "thumbnail" || size == "preview" {
		return true
	}

	// Check custom resolution format
	return h.isValidCustomResolution(size)
}
