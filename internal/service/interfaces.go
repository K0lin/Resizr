package service

import (
	"context"
	"io"
	"time"

	"resizr/internal/models"
)

// ImageService defines the interface for image business logic
type ImageService interface {
	// ProcessUpload handles the complete image upload workflow
	ProcessUpload(ctx context.Context, input UploadInput) (*UploadResult, error)

	// GetMetadata retrieves image metadata by ID
	GetMetadata(ctx context.Context, imageID string) (*models.ImageMetadata, error)

	// GetImageStream retrieves image data as a stream
	GetImageStream(ctx context.Context, imageID, resolution string) (io.ReadCloser, *models.ImageMetadata, error)

	// ProcessResolution generates a specific resolution for an existing image
	ProcessResolution(ctx context.Context, imageID, resolution string) error

	// DeleteImage removes an image and all its resolutions
	DeleteImage(ctx context.Context, imageID string) error

	// ListImages retrieves paginated list of images
	ListImages(ctx context.Context, offset, limit int) ([]*models.ImageMetadata, int, error)

	// GeneratePresignedURL generates a pre-signed URL for direct access to storage
	GeneratePresignedURL(ctx context.Context, storageKey string, duration time.Duration) (string, error)
}

// HealthService defines the interface for health checking
type HealthService interface {
	// CheckHealth performs comprehensive health check
	CheckHealth(ctx context.Context) (*HealthStatus, error)

	// GetMetrics retrieves system metrics
	GetMetrics(ctx context.Context) (map[string]interface{}, error)
}

// ProcessorService defines the interface for image processing
type ProcessorService interface {
	// DetectFormat detects image format from data
	DetectFormat(data []byte) (string, error)

	// GetDimensions extracts image dimensions
	GetDimensions(data []byte) (width, height int, err error)

	// ProcessImage resizes image to specified resolution
	ProcessImage(data []byte, config ResizeConfig) ([]byte, error)

	// ValidateImage checks if image data is valid
	ValidateImage(data []byte, maxSize int64) error
}

// Input/Output Types

// UploadInput represents input for image upload
type UploadInput struct {
	Filename    string   `json:"filename"`
	Data        []byte   `json:"-"`
	Size        int64    `json:"size"`
	Resolutions []string `json:"resolutions"`
}

// UploadResult represents the result of image upload
type UploadResult struct {
	ImageID              string           `json:"image_id"`
	ProcessedResolutions []string         `json:"processed_resolutions"`
	OriginalSize         int64            `json:"original_size"`
	ProcessedSizes       map[string]int64 `json:"processed_sizes"`
}

// ResizeConfig represents image resizing configuration
type ResizeConfig struct {
	Width   int        `json:"width"`
	Height  int        `json:"height"`
	Quality int        `json:"quality"`
	Format  string     `json:"format"`
	Mode    ResizeMode `json:"mode"`
}

// ResizeMode defines how image should be resized
type ResizeMode string

const (
	ResizeModeSmartFit ResizeMode = "smart_fit" // Maintain aspect ratio with background
	ResizeModeCrop     ResizeMode = "crop"      // Crop to exact dimensions
	ResizeModeStretch  ResizeMode = "stretch"   // Stretch to exact dimensions
)

// HealthStatus represents system health status
type HealthStatus struct {
	Services map[string]string `json:"services"`
	Uptime   int64             `json:"uptime_seconds"`
	Version  string            `json:"version"`
}

// Processing Statistics
type ProcessingStats struct {
	TotalImages      int64            `json:"total_images"`
	TotalSize        int64            `json:"total_size_bytes"`
	ProcessingTime   int64            `json:"avg_processing_time_ms"`
	ResolutionCounts map[string]int64 `json:"resolution_counts"`
	FormatCounts     map[string]int64 `json:"format_counts"`
}
