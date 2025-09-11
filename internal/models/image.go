package models

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// ImageMetadata represents image metadata stored in Redis
type ImageMetadata struct {
	ID          string    `json:"id" redis:"id"`
	OriginalKey string    `json:"original_key" redis:"original_key"`
	Filename    string    `json:"filename" redis:"filename"`
	MimeType    string    `json:"mime_type" redis:"mime_type"`
	Size        int64     `json:"size" redis:"size"`
	Width       int       `json:"width" redis:"width"`
	Height      int       `json:"height" redis:"height"`
	Resolutions []string  `json:"resolutions" redis:"resolutions"`
	CreatedAt   time.Time `json:"created_at" redis:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" redis:"updated_at"`
}

// ResolutionConfig defines image resolution parameters
type ResolutionConfig struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

// UploadRequest represents the request payload for image upload
type UploadRequest struct {
	Resolutions []string `form:"resolutions" json:"resolutions" binding:"omitempty"`
}

// UploadResponse represents the response after successful image upload
type UploadResponse struct {
	ID          string   `json:"id"`
	Message     string   `json:"message"`
	Resolutions []string `json:"resolutions"`
}

// InfoResponse represents the response for image info endpoint
type InfoResponse struct {
	ID                   string        `json:"id"`
	Filename             string        `json:"filename"`
	MimeType             string        `json:"mime_type"`
	Size                 int64         `json:"size"`
	Dimensions           DimensionInfo `json:"dimensions"`
	AvailableResolutions []string      `json:"available_resolutions"`
	CreatedAt            time.Time     `json:"created_at"`
}

// DimensionInfo represents image dimensions
type DimensionInfo struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

// ErrorResponse represents a standardized error response
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
	Code    int    `json:"code"`
}

// HealthResponse represents the health check response
type HealthResponse struct {
	Status    string            `json:"status"`
	Services  map[string]string `json:"services"`
	Timestamp time.Time         `json:"timestamp"`
}

// ImageProcessingRequest represents an image processing request
type ImageProcessingRequest struct {
	ImageID    string           `json:"image_id"`
	Resolution ResolutionConfig `json:"resolution"`
	Quality    int              `json:"quality"`
}

// StorageInfo represents storage location information
type StorageInfo struct {
	Bucket string `json:"bucket"`
	Key    string `json:"key"`
	URL    string `json:"url,omitempty"`
}

// Custom error types for better error handling
type (
	// ValidationError represents a validation error
	ValidationError struct {
		Field   string `json:"field"`
		Message string `json:"message"`
	}

	// NotFoundError represents a resource not found error
	NotFoundError struct {
		Resource string `json:"resource"`
		ID       string `json:"id"`
	}

	// ProcessingError represents an image processing error
	ProcessingError struct {
		Operation string `json:"operation"`
		Reason    string `json:"reason"`
	}

	// StorageError represents a storage operation error
	StorageError struct {
		Operation string `json:"operation"`
		Backend   string `json:"backend"`
		Reason    string `json:"reason"`
	}
)

// Error implementations for custom error types
func (e ValidationError) Error() string {
	return fmt.Sprintf("validation error on field '%s': %s", e.Field, e.Message)
}

func (e NotFoundError) Error() string {
	return fmt.Sprintf("%s with ID '%s' not found", e.Resource, e.ID)
}

func (e ProcessingError) Error() string {
	return fmt.Sprintf("processing error during %s: %s", e.Operation, e.Reason)
}

func (e StorageError) Error() string {
	return fmt.Sprintf("storage error during %s on %s: %s", e.Operation, e.Backend, e.Reason)
}

// Methods for ImageMetadata

// GetDimensions returns the image dimensions
func (im *ImageMetadata) GetDimensions() DimensionInfo {
	return DimensionInfo{
		Width:  im.Width,
		Height: im.Height,
	}
}

// HasResolution checks if a specific resolution exists
func (im *ImageMetadata) HasResolution(resolution string) bool {
	for _, res := range im.Resolutions {
		if res == resolution {
			return true
		}
	}
	return false
}

// AddResolution adds a new resolution to the list
func (im *ImageMetadata) AddResolution(resolution string) {
	if !im.HasResolution(resolution) {
		im.Resolutions = append(im.Resolutions, resolution)
		im.UpdatedAt = time.Now()
	}
}

// GetFileExtension extracts file extension from filename
func (im *ImageMetadata) GetFileExtension() string {
	parts := strings.Split(im.Filename, ".")
	if len(parts) > 1 {
		return strings.ToLower(parts[len(parts)-1])
	}
	return ""
}

// GetStorageKey generates the storage key for a specific resolution
func (im *ImageMetadata) GetStorageKey(resolution string) string {
	ext := im.GetFileExtension()
	if resolution == "original" {
		return fmt.Sprintf("images/%s/original.%s", im.ID, ext)
	}
	return fmt.Sprintf("images/%s/%s.%s", im.ID, resolution, ext)
}

// ToInfoResponse converts ImageMetadata to InfoResponse
func (im *ImageMetadata) ToInfoResponse() InfoResponse {
	return InfoResponse{
		ID:                   im.ID,
		Filename:             im.Filename,
		MimeType:             im.MimeType,
		Size:                 im.Size,
		Dimensions:           im.GetDimensions(),
		AvailableResolutions: append([]string{"original"}, im.Resolutions...),
		CreatedAt:            im.CreatedAt,
	}
}

// Validation methods

// IsValidUUID checks if the ID is a valid UUID format
func (im *ImageMetadata) IsValidUUID() bool {
	uuidRegex := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	return uuidRegex.MatchString(im.ID)
}

// IsValidMimeType checks if the MIME type is supported
func (im *ImageMetadata) IsValidMimeType() bool {
	validTypes := []string{
		"image/jpeg",
		"image/png",
		"image/gif",
		"image/webp",
	}

	for _, validType := range validTypes {
		if im.MimeType == validType {
			return true
		}
	}
	return false
}

// Validate validates the ImageMetadata
func (im *ImageMetadata) Validate() error {
	if im.ID == "" {
		return ValidationError{Field: "id", Message: "ID is required"}
	}

	if !im.IsValidUUID() {
		return ValidationError{Field: "id", Message: "ID must be a valid UUID"}
	}

	if im.Filename == "" {
		return ValidationError{Field: "filename", Message: "filename is required"}
	}

	if im.MimeType == "" {
		return ValidationError{Field: "mime_type", Message: "MIME type is required"}
	}

	if !im.IsValidMimeType() {
		return ValidationError{Field: "mime_type", Message: "unsupported MIME type"}
	}

	if im.Size <= 0 {
		return ValidationError{Field: "size", Message: "size must be positive"}
	}

	if im.Width <= 0 || im.Height <= 0 {
		return ValidationError{Field: "dimensions", Message: "width and height must be positive"}
	}

	return nil
}

// Utility functions

// ParseResolution parses a resolution string like "800x600" into ResolutionConfig
func ParseResolution(resolution string) (ResolutionConfig, error) {
	// Handle predefined resolutions
	switch resolution {
	case "thumbnail":
		return ResolutionConfig{Width: 150, Height: 150}, nil
	case "preview":
		return ResolutionConfig{Width: 800, Height: 600}, nil
	case "original":
		return ResolutionConfig{}, fmt.Errorf("original resolution cannot be parsed")
	}

	// Parse custom resolution format: "WIDTHxHEIGHT"
	resolutionRegex := regexp.MustCompile(`^(\d+)x(\d+)$`)
	matches := resolutionRegex.FindStringSubmatch(resolution)

	if len(matches) != 3 {
		return ResolutionConfig{}, fmt.Errorf("invalid resolution format: %s (expected format: WIDTHxHEIGHT)", resolution)
	}

	width, err := strconv.Atoi(matches[1])
	if err != nil {
		return ResolutionConfig{}, fmt.Errorf("invalid width: %s", matches[1])
	}

	height, err := strconv.Atoi(matches[2])
	if err != nil {
		return ResolutionConfig{}, fmt.Errorf("invalid height: %s", matches[2])
	}

	// Validate reasonable dimensions
	if width <= 0 || height <= 0 {
		return ResolutionConfig{}, fmt.Errorf("width and height must be positive")
	}

	if width > 10000 || height > 10000 {
		return ResolutionConfig{}, fmt.Errorf("width and height cannot exceed 10000 pixels")
	}

	return ResolutionConfig{Width: width, Height: height}, nil
}

// FormatResolution formats a ResolutionConfig into a string
func (rc ResolutionConfig) String() string {
	return fmt.Sprintf("%dx%d", rc.Width, rc.Height)
}

// IsSquare checks if the resolution is square (width == height)
func (rc ResolutionConfig) IsSquare() bool {
	return rc.Width == rc.Height
}

// AspectRatio calculates the aspect ratio
func (rc ResolutionConfig) AspectRatio() float64 {
	if rc.Height == 0 {
		return 0
	}
	return float64(rc.Width) / float64(rc.Height)
}

// GetMimeTypeFromExtension returns MIME type based on file extension
func GetMimeTypeFromExtension(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	default:
		return ""
	}
}

// GetExtensionFromMimeType returns file extension based on MIME type
func GetExtensionFromMimeType(mimeType string) string {
	switch mimeType {
	case "image/jpeg":
		return "jpg"
	case "image/png":
		return "png"
	case "image/gif":
		return "gif"
	case "image/webp":
		return "webp"
	default:
		return ""
	}
}

// NewImageMetadata creates a new ImageMetadata with current timestamp
func NewImageMetadata(id, filename, mimeType string, size int64, width, height int) *ImageMetadata {
	now := time.Now()
	return &ImageMetadata{
		ID:          id,
		OriginalKey: fmt.Sprintf("images/%s/original.%s", id, GetExtensionFromMimeType(mimeType)),
		Filename:    filename,
		MimeType:    mimeType,
		Size:        size,
		Width:       width,
		Height:      height,
		Resolutions: []string{},
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}
