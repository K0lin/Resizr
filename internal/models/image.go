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
	ID            string    `json:"id" redis:"id"`
	OriginalKey   string    `json:"original_key" redis:"original_key"`
	Filename      string    `json:"filename" redis:"filename"`
	MimeType      string    `json:"mime_type" redis:"mime_type"`
	Size          int64     `json:"size" redis:"size"`
	Width         int       `json:"width" redis:"width"`
	Height        int       `json:"height" redis:"height"`
	Resolutions   []string  `json:"resolutions" redis:"resolutions"`
	CreatedAt     time.Time `json:"created_at" redis:"created_at"`
	UpdatedAt     time.Time `json:"updated_at" redis:"updated_at"`
	Hash          ImageHash `json:"hash" redis:"hash"`                       // Hash for deduplication
	IsDeduped     bool      `json:"is_deduped" redis:"is_deduped"`           // True if this image shares storage with others
	SharedImageID string    `json:"shared_image_id" redis:"shared_image_id"` // ID of the master image (if deduplicated)
}

// ResolutionConfig defines image resolution parameters
type ResolutionConfig struct {
	Width  int    `json:"width"`
	Height int    `json:"height"`
	Alias  string `json:"alias,omitempty"` // Optional alias for the resolution
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

// PresignedURLResponse represents the response for presigned URL endpoint
type PresignedURLResponse struct {
	URL       string    `json:"url"`
	ExpiresAt time.Time `json:"expires_at"`
	ExpiresIn int       `json:"expires_in"` // seconds
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

// HasResolution checks if a specific resolution exists (by dimensions or alias)
func (im *ImageMetadata) HasResolution(resolution string) bool {
	// Don't allow access via the full "dimensions:alias" format from API
	if strings.Contains(resolution, ":") {
		return false
	}

	for _, res := range im.Resolutions {
		// Direct match for legacy resolutions (no colon)
		if res == resolution {
			return true
		}
		// Check if resolution matches an alias
		if alias := ExtractAlias(res); alias != "" && alias == resolution {
			return true
		}
		// Check if resolution matches dimensions part of an aliased resolution
		if dimensions := ExtractDimensions(res); dimensions != res && dimensions == resolution {
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

	// Always use dimensions for storage key to avoid duplicates
	dimensions := im.ResolveToDimensions(resolution)
	return fmt.Sprintf("images/%s/%s.%s", im.ID, dimensions, ext)
}

// ResolveToDimensions resolves any resolution (alias or dimensions) to pure dimensions for storage
func (im *ImageMetadata) ResolveToDimensions(resolution string) string {
	// If it's already in pure dimensions format, return as-is
	if IsValidDimensionFormat(resolution) {
		return resolution
	}

	// Check if it's a predefined resolution
	if resolution == "thumbnail" {
		return "thumbnail"
	}

	// Search for the resolution by alias and return its dimensions
	for _, res := range im.Resolutions {
		if alias := ExtractAlias(res); alias != "" && alias == resolution {
			return ExtractDimensions(res)
		}
	}

	// Fallback: return as-is (shouldn't happen if HasResolution was called first)
	return resolution
}

// FindStoredResolution finds the actual stored resolution string for a given access resolution
// Note: This is kept for backward compatibility but storage always uses dimensions
func (im *ImageMetadata) FindStoredResolution(resolution string) string {
	// For storage optimization, we always return the dimensions part
	return im.ResolveToDimensions(resolution)
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

// ParseResolution parses a resolution string like "800x600" or "800x600:alias" into ResolutionConfig
func ParseResolution(resolution string) (ResolutionConfig, error) {
	// Handle predefined resolutions
	switch resolution {
	case "thumbnail":
		return ResolutionConfig{Width: 150, Height: 150}, nil
	case "original":
		return ResolutionConfig{}, fmt.Errorf("original resolution cannot be parsed")
	}

	// Extract alias if present
	dimensions, alias := SplitResolutionAndAlias(resolution)

	// Parse custom resolution format: "WIDTHxHEIGHT"
	resolutionRegex := regexp.MustCompile(`^(\d+)x(\d+)$`)
	matches := resolutionRegex.FindStringSubmatch(dimensions)

	if len(matches) != 3 {
		return ResolutionConfig{}, fmt.Errorf("invalid resolution format: %s (expected format: WIDTHxHEIGHT or WIDTHxHEIGHT:alias)", resolution)
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

	// Note: Business logic validation (max dimensions) is handled at the service layer

	return ResolutionConfig{Width: width, Height: height, Alias: alias}, nil
}

// FormatResolution formats a ResolutionConfig into a string with optional alias
func (rc ResolutionConfig) String() string {
	if rc.Alias != "" {
		return fmt.Sprintf("%dx%d:%s", rc.Width, rc.Height, rc.Alias)
	}
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

// Utility functions for resolution alias handling

// SplitResolutionAndAlias splits a resolution string like "800x600:alias" into dimensions and alias
func SplitResolutionAndAlias(resolution string) (dimensions, alias string) {
	parts := strings.Split(resolution, ":")
	if len(parts) == 2 {
		return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
	}
	return resolution, ""
}

// ExtractAlias extracts the alias from a resolution string like "800x600:alias"
func ExtractAlias(resolution string) string {
	_, alias := SplitResolutionAndAlias(resolution)
	return alias
}

// ExtractDimensions extracts the dimensions part from a resolution string like "800x600:alias"
func ExtractDimensions(resolution string) string {
	dimensions, _ := SplitResolutionAndAlias(resolution)
	return dimensions
}

// IsValidDimensionFormat checks if a string is in the WIDTHxHEIGHT format
func IsValidDimensionFormat(resolution string) bool {
	resolutionRegex := regexp.MustCompile(`^(\d+)x(\d+)$`)
	return resolutionRegex.MatchString(resolution)
}

// FormatResolutionWithAlias creates a resolution string with alias if provided
func FormatResolutionWithAlias(width, height int, alias string) string {
	if alias != "" {
		return fmt.Sprintf("%dx%d:%s", width, height, alias)
	}
	return fmt.Sprintf("%dx%d", width, height)
}

// NewImageMetadata creates a new ImageMetadata with current timestamp
func NewImageMetadata(id, filename, mimeType string, size int64, width, height int) *ImageMetadata {
	now := time.Now()
	return &ImageMetadata{
		ID:            id,
		OriginalKey:   fmt.Sprintf("images/%s/original.%s", id, GetExtensionFromMimeType(mimeType)),
		Filename:      filename,
		MimeType:      mimeType,
		Size:          size,
		Width:         width,
		Height:        height,
		Resolutions:   []string{},
		CreatedAt:     now,
		UpdatedAt:     now,
		Hash:          ImageHash{}, // Will be set later
		IsDeduped:     false,
		SharedImageID: "",
	}
}

// NewImageMetadataWithHash creates a new ImageMetadata with hash information
func NewImageMetadataWithHash(id, filename, mimeType string, size int64, width, height int, hash ImageHash) *ImageMetadata {
	metadata := NewImageMetadata(id, filename, mimeType, size, width, height)
	metadata.Hash = hash
	return metadata
}

// GetActualStorageKey returns the actual storage key (considers deduplication)
func (im *ImageMetadata) GetActualStorageKey(resolution string) string {
	if im.IsDeduped && im.SharedImageID != "" {
		// Use shared image's storage key
		ext := im.GetFileExtension()
		if resolution == "original" {
			return fmt.Sprintf("images/%s/original.%s", im.SharedImageID, ext)
		}
		dimensions := im.ResolveToDimensions(resolution)
		return fmt.Sprintf("images/%s/%s.%s", im.SharedImageID, dimensions, ext)
	}
	// Use own storage key
	return im.GetStorageKey(resolution)
}

// MarkAsDeduped marks this image as sharing storage with another image
func (im *ImageMetadata) MarkAsDeduped(sharedImageID string) {
	im.IsDeduped = true
	im.SharedImageID = sharedImageID
	im.UpdatedAt = time.Now()
}
