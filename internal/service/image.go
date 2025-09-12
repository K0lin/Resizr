package service

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"resizr/internal/config"
	"resizr/internal/models"
	"resizr/internal/repository"
	"resizr/internal/storage"
	"resizr/pkg/logger"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// ImageServiceImpl implements the ImageService interface
type ImageServiceImpl struct {
	repo      repository.ImageRepository
	storage   storage.ImageStorage
	processor ProcessorService
	config    *config.Config
}

// NewImageService creates a new image service
func NewImageService(
	repo repository.ImageRepository,
	storage storage.ImageStorage,
	processor ProcessorService,
	config *config.Config,
) ImageService {
	return &ImageServiceImpl{
		repo:      repo,
		storage:   storage,
		processor: processor,
		config:    config,
	}
}

// ProcessUpload handles the complete image upload workflow
func (s *ImageServiceImpl) ProcessUpload(ctx context.Context, input UploadInput) (*UploadResult, error) {
	logger.InfoWithContext(ctx, "Starting image upload processing",
		zap.String("filename", input.Filename),
		zap.Int64("size", input.Size),
		zap.Strings("requested_resolutions", input.Resolutions))

	// Generate unique ID for the image
	imageID := uuid.New().String()

	// Validate input
	if err := s.validateUploadInput(input); err != nil {
		return nil, err
	}

	// Validate and process original image
	if err := s.processor.ValidateImage(input.Data, s.config.Image.MaxFileSize); err != nil {
		return nil, models.ProcessingError{
			Operation: "validate",
			Reason:    err.Error(),
		}
	}

	// Detect format and dimensions
	mimeType, err := s.processor.DetectFormat(input.Data)
	if err != nil {
		return nil, models.ProcessingError{
			Operation: "format_detection",
			Reason:    err.Error(),
		}
	}

	width, height, err := s.processor.GetDimensions(input.Data)
	if err != nil {
		return nil, models.ProcessingError{
			Operation: "dimension_extraction",
			Reason:    err.Error(),
		}
	}

	// Create image metadata
	metadata := models.NewImageMetadata(imageID, input.Filename, mimeType, input.Size, width, height)

	// Store original image
	originalKey := metadata.GetStorageKey("original")
	if err := s.storage.Upload(ctx, originalKey, bytes.NewReader(input.Data), input.Size, mimeType); err != nil {
		return nil, models.StorageError{
			Operation: "upload",
			Backend:   "S3",
			Reason:    err.Error(),
		}
	}

	logger.InfoWithContext(ctx, "Original image uploaded successfully",
		zap.String("image_id", imageID),
		zap.String("storage_key", originalKey))

	// Process requested resolutions
	processedResolutions := []string{}
	processedSizes := make(map[string]int64)

	// Add predefined resolutions based on configuration
	var allResolutions []string
	if s.config.Image.GenerateDefaultResolutions {
		allResolutions = append([]string{"thumbnail", "preview"}, input.Resolutions...)
	} else {
		allResolutions = input.Resolutions
	}

	for _, resolutionName := range allResolutions {
		// Skip duplicates
		if metadata.HasResolution(resolutionName) {
			continue
		}

		if err := s.processResolution(ctx, imageID, resolutionName, input.Data, mimeType); err != nil {
			logger.ErrorWithContext(ctx, "Failed to process resolution",
				zap.String("image_id", imageID),
				zap.String("resolution", resolutionName),
				zap.Error(err))
			// Continue with other resolutions instead of failing completely
			continue
		}

		metadata.AddResolution(resolutionName)
		processedResolutions = append(processedResolutions, resolutionName)

		// Get size of processed image (optional - for statistics)
		if size, err := s.getProcessedImageSize(ctx, imageID, resolutionName); err == nil {
			processedSizes[resolutionName] = size
		}
	}

	// Store metadata in repository
	if err := s.repo.Store(ctx, metadata); err != nil {
		// If metadata storage fails, cleanup uploaded images
		s.cleanupUploadedImages(ctx, imageID, append(processedResolutions, "original"))
		return nil, models.StorageError{
			Operation: "store_metadata",
			Backend:   "Redis",
			Reason:    err.Error(),
		}
	}

	logger.InfoWithContext(ctx, "Image upload processing completed",
		zap.String("image_id", imageID),
		zap.Strings("processed_resolutions", processedResolutions),
		zap.Int("total_resolutions", len(processedResolutions)))

	return &UploadResult{
		ImageID:              imageID,
		ProcessedResolutions: processedResolutions,
		OriginalSize:         input.Size,
		ProcessedSizes:       processedSizes,
	}, nil
}

// GetMetadata retrieves image metadata by ID
func (s *ImageServiceImpl) GetMetadata(ctx context.Context, imageID string) (*models.ImageMetadata, error) {
	logger.DebugWithContext(ctx, "Retrieving image metadata",
		zap.String("image_id", imageID))

	// Validate UUID format
	if _, err := uuid.Parse(imageID); err != nil {
		return nil, models.ValidationError{
			Field:   "image_id",
			Message: "Invalid UUID format",
		}
	}

	metadata, err := s.repo.Get(ctx, imageID)
	if err != nil {
		if _, ok := err.(models.NotFoundError); ok {
			return nil, err // Pass through not found errors
		}
		return nil, models.StorageError{
			Operation: "get_metadata",
			Backend:   "Redis",
			Reason:    err.Error(),
		}
	}

	return metadata, nil
}

// GetImageStream retrieves image data as a stream
func (s *ImageServiceImpl) GetImageStream(ctx context.Context, imageID, resolution string) (io.ReadCloser, *models.ImageMetadata, error) {
	logger.DebugWithContext(ctx, "Retrieving image stream",
		zap.String("image_id", imageID),
		zap.String("resolution", resolution))

	// Get metadata first
	metadata, err := s.GetMetadata(ctx, imageID)
	if err != nil {
		return nil, nil, err
	}

	// Validate resolution exists (except for original)
	if resolution != "original" && !metadata.HasResolution(resolution) {
		return nil, nil, models.NotFoundError{
			Resource: "resolution",
			ID:       fmt.Sprintf("%s/%s", imageID, resolution),
		}
	}

	// Get storage key and download stream
	storageKey := metadata.GetStorageKey(resolution)
	stream, err := s.storage.Download(ctx, storageKey)
	if err != nil {
		return nil, nil, models.StorageError{
			Operation: "download",
			Backend:   "S3",
			Reason:    err.Error(),
		}
	}

	return stream, metadata, nil
}

// ProcessResolution generates a specific resolution for an existing image
func (s *ImageServiceImpl) ProcessResolution(ctx context.Context, imageID, resolution string) error {
	logger.InfoWithContext(ctx, "Processing additional resolution",
		zap.String("image_id", imageID),
		zap.String("resolution", resolution))

	// Get metadata
	metadata, err := s.GetMetadata(ctx, imageID)
	if err != nil {
		return err
	}

	// Check if resolution already exists
	if metadata.HasResolution(resolution) {
		return nil // Already exists, no need to process
	}

	// Download original image data
	originalStream, _, err := s.GetImageStream(ctx, imageID, "original")
	if err != nil {
		return err
	}
	defer originalStream.Close()

	// Read original data
	originalData, err := io.ReadAll(originalStream)
	if err != nil {
		return models.ProcessingError{
			Operation: "read_original",
			Reason:    err.Error(),
		}
	}

	// Process the resolution
	if err := s.processResolution(ctx, imageID, resolution, originalData, metadata.MimeType); err != nil {
		return err
	}

	// Update metadata
	metadata.AddResolution(resolution)
	return s.repo.Update(ctx, metadata)
}

// DeleteImage removes an image and all its resolutions
func (s *ImageServiceImpl) DeleteImage(ctx context.Context, imageID string) error {
	logger.InfoWithContext(ctx, "Deleting image",
		zap.String("image_id", imageID))

	// Get metadata to know what to delete
	metadata, err := s.GetMetadata(ctx, imageID)
	if err != nil {
		return err
	}

	// Delete all resolutions from storage
	resolutionsToDelete := append([]string{"original"}, metadata.Resolutions...)

	for _, resolution := range resolutionsToDelete {
		storageKey := metadata.GetStorageKey(resolution)
		if err := s.storage.Delete(ctx, storageKey); err != nil {
			logger.WarnWithContext(ctx, "Failed to delete resolution from storage",
				zap.String("image_id", imageID),
				zap.String("resolution", resolution),
				zap.String("storage_key", storageKey),
				zap.Error(err))
			// Continue deleting other resolutions
		}
	}

	// Delete metadata from repository
	if err := s.repo.Delete(ctx, imageID); err != nil {
		return models.StorageError{
			Operation: "delete_metadata",
			Backend:   "Redis",
			Reason:    err.Error(),
		}
	}

	logger.InfoWithContext(ctx, "Image deleted successfully",
		zap.String("image_id", imageID),
		zap.Int("deleted_resolutions", len(resolutionsToDelete)))

	return nil
}

// ListImages retrieves paginated list of images
func (s *ImageServiceImpl) ListImages(ctx context.Context, offset, limit int) ([]*models.ImageMetadata, int, error) {
	logger.DebugWithContext(ctx, "Listing images",
		zap.Int("offset", offset),
		zap.Int("limit", limit))

	if limit <= 0 || limit > 100 {
		limit = 50 // Default limit
	}

	images, err := s.repo.List(ctx, offset, limit)
	if err != nil {
		return nil, 0, models.StorageError{
			Operation: "list_images",
			Backend:   "Redis",
			Reason:    err.Error(),
		}
	}

	// Get total count (this could be cached for better performance)
	// For now, return -1 to indicate total is unknown
	total := -1

	return images, total, nil
}

// GeneratePresignedURL generates a pre-signed URL for direct access to storage
func (s *ImageServiceImpl) GeneratePresignedURL(ctx context.Context, storageKey string, duration time.Duration) (string, error) {
	logger.DebugWithContext(ctx, "Generating presigned URL",
		zap.String("storage_key", storageKey),
		zap.Duration("duration", duration))

	presignedURL, err := s.storage.GeneratePresignedURL(ctx, storageKey, duration)
	if err != nil {
		logger.ErrorWithContext(ctx, "Failed to generate presigned URL",
			zap.String("storage_key", storageKey),
			zap.Error(err))
		return "", models.StorageError{
			Operation: "generate_presigned_url",
			Backend:   "S3",
			Reason:    err.Error(),
		}
	}

	logger.InfoWithContext(ctx, "Presigned URL generated successfully",
		zap.String("storage_key", storageKey),
		zap.Duration("duration", duration))

	return presignedURL, nil
}

// Helper methods

// validateUploadInput validates the upload input
func (s *ImageServiceImpl) validateUploadInput(input UploadInput) error {
	if input.Filename == "" {
		return models.ValidationError{
			Field:   "filename",
			Message: "Filename is required",
		}
	}

	if len(input.Data) == 0 {
		return models.ValidationError{
			Field:   "data",
			Message: "Image data is required",
		}
	}

	if input.Size != int64(len(input.Data)) {
		return models.ValidationError{
			Field:   "size",
			Message: "Size mismatch with actual data length",
		}
	}

	// Validate requested resolutions - support comma-separated values
	validatedResolutions := []string{}
	for _, resolution := range input.Resolutions {
		// Handle comma-separated resolutions in a single field
		resolutions := strings.Split(resolution, ",")
		for _, res := range resolutions {
			res = strings.TrimSpace(res) // Remove whitespace
			if res == "" {
				continue // Skip empty strings
			}
			if _, err := models.ParseResolution(res); err != nil {
				return models.ValidationError{
					Field:   "resolutions",
					Message: fmt.Sprintf("Invalid resolution format '%s': %s", res, err.Error()),
				}
			}
			validatedResolutions = append(validatedResolutions, res)
		}
	}
	// Update input with parsed resolutions
	input.Resolutions = validatedResolutions

	return nil
}

// processResolution processes a single resolution
func (s *ImageServiceImpl) processResolution(ctx context.Context, imageID, resolutionName string, originalData []byte, mimeType string) error {
	// Parse resolution configuration
	resolutionConfig, err := models.ParseResolution(resolutionName)
	if err != nil {
		return models.ValidationError{
			Field:   "resolution",
			Message: err.Error(),
		}
	}

	// Configure resize parameters
	resizeConfig := ResizeConfig{
		Width:   resolutionConfig.Width,
		Height:  resolutionConfig.Height,
		Quality: s.config.Image.Quality,
		Format:  mimeType,
		Mode:    ResizeMode(s.config.Image.ResizeMode),
	}

	// Process the image
	processedData, err := s.processor.ProcessImage(originalData, resizeConfig)
	if err != nil {
		return models.ProcessingError{
			Operation: "resize",
			Reason:    err.Error(),
		}
	}

	// Upload processed image
	storageKey := fmt.Sprintf("images/%s/%s.%s", imageID, resolutionName, models.GetExtensionFromMimeType(mimeType))
	if err := s.storage.Upload(ctx, storageKey, bytes.NewReader(processedData), int64(len(processedData)), mimeType); err != nil {
		return models.StorageError{
			Operation: "upload_processed",
			Backend:   "S3",
			Reason:    err.Error(),
		}
	}

	logger.DebugWithContext(ctx, "Resolution processed successfully",
		zap.String("image_id", imageID),
		zap.String("resolution", resolutionName),
		zap.String("storage_key", storageKey),
		zap.Int("processed_size", len(processedData)))

	return nil
}

// getProcessedImageSize gets the size of a processed image
func (s *ImageServiceImpl) getProcessedImageSize(ctx context.Context, imageID, resolution string) (int64, error) {
	// This is optional - for statistics only
	// Implementation would query storage for object size
	return 0, fmt.Errorf("not implemented")
}

// cleanupUploadedImages cleans up images if upload fails
func (s *ImageServiceImpl) cleanupUploadedImages(ctx context.Context, imageID string, resolutions []string) {
	logger.WarnWithContext(ctx, "Cleaning up uploaded images due to failure",
		zap.String("image_id", imageID),
		zap.Strings("resolutions", resolutions))

	for _, resolution := range resolutions {
		storageKey := fmt.Sprintf("images/%s/%s.jpg", imageID, resolution) // Simplified
		if err := s.storage.Delete(ctx, storageKey); err != nil {
			logger.ErrorWithContext(ctx, "Failed to cleanup uploaded image",
				zap.String("image_id", imageID),
				zap.String("resolution", resolution),
				zap.String("storage_key", storageKey),
				zap.Error(err))
		}
	}
}
