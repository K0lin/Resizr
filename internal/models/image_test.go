package models

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewImageMetadata(t *testing.T) {
	id := "f47ac10b-58cc-4372-a567-0e02b2c3d479"
	filename := "test.jpg"
	mimeType := "image/jpeg"
	size := int64(102400)
	width := 1920
	height := 1080

	metadata := NewImageMetadata(id, filename, mimeType, size, width, height)

	assert.Equal(t, id, metadata.ID)
	assert.Equal(t, filename, metadata.Filename)
	assert.Equal(t, mimeType, metadata.MimeType)
	assert.Equal(t, size, metadata.Size)
	assert.Equal(t, width, metadata.Width)
	assert.Equal(t, height, metadata.Height)
	assert.Equal(t, "images/f47ac10b-58cc-4372-a567-0e02b2c3d479/original.jpg", metadata.OriginalKey)
	assert.Empty(t, metadata.Resolutions)
	assert.True(t, time.Since(metadata.CreatedAt) < time.Second)
	assert.True(t, time.Since(metadata.UpdatedAt) < time.Second)
}

func TestImageMetadata_GetDimensions(t *testing.T) {
	metadata := &ImageMetadata{
		Width:  1920,
		Height: 1080,
	}

	dimensions := metadata.GetDimensions()

	assert.Equal(t, 1920, dimensions.Width)
	assert.Equal(t, 1080, dimensions.Height)
}

func TestImageMetadata_HasResolution(t *testing.T) {
	metadata := &ImageMetadata{
		Resolutions: []string{"thumbnail", "preview", "800x600"},
	}

	assert.True(t, metadata.HasResolution("thumbnail"))
	assert.True(t, metadata.HasResolution("preview"))
	assert.True(t, metadata.HasResolution("800x600"))
	assert.False(t, metadata.HasResolution("1200x900"))
	assert.False(t, metadata.HasResolution(""))
}

func TestImageMetadata_AddResolution(t *testing.T) {
	metadata := &ImageMetadata{
		Resolutions: []string{"thumbnail"},
		UpdatedAt:   time.Now().Add(-time.Hour),
	}
	oldUpdatedAt := metadata.UpdatedAt

	// Add new resolution
	metadata.AddResolution("preview")
	assert.Contains(t, metadata.Resolutions, "preview")
	assert.True(t, metadata.UpdatedAt.After(oldUpdatedAt))

	// Try to add existing resolution
	resolutionCount := len(metadata.Resolutions)
	metadata.AddResolution("thumbnail")
	assert.Equal(t, resolutionCount, len(metadata.Resolutions)) // Should not change
}

func TestImageMetadata_GetFileExtension(t *testing.T) {
	tests := []struct {
		filename string
		expected string
	}{
		{"test.jpg", "jpg"},
		{"image.png", "png"},
		{"photo.JPEG", "jpeg"},
		{"document.pdf", "pdf"},
		{"noextension", ""},
		{"multiple.dots.gif", "gif"},
		{"", ""},
	}

	for _, tt := range tests {
		metadata := &ImageMetadata{Filename: tt.filename}
		ext := metadata.GetFileExtension()
		assert.Equal(t, tt.expected, ext, "filename: %s", tt.filename)
	}
}

func TestImageMetadata_GetStorageKey(t *testing.T) {
	metadata := &ImageMetadata{
		ID:       "test-uuid",
		Filename: "test.jpg",
	}

	tests := []struct {
		resolution string
		expected   string
	}{
		{"original", "images/test-uuid/original.jpg"},
		{"thumbnail", "images/test-uuid/thumbnail.jpg"},
		{"preview", "images/test-uuid/preview.jpg"},
		{"800x600", "images/test-uuid/800x600.jpg"},
	}

	for _, tt := range tests {
		key := metadata.GetStorageKey(tt.resolution)
		assert.Equal(t, tt.expected, key)
	}
}

func TestImageMetadata_ToInfoResponse(t *testing.T) {
	metadata := &ImageMetadata{
		ID:          "test-uuid",
		Filename:    "test.jpg",
		MimeType:    "image/jpeg",
		Size:        102400,
		Width:       1920,
		Height:      1080,
		Resolutions: []string{"thumbnail", "preview"},
		CreatedAt:   time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC),
	}

	response := metadata.ToInfoResponse()

	assert.Equal(t, "test-uuid", response.ID)
	assert.Equal(t, "test.jpg", response.Filename)
	assert.Equal(t, "image/jpeg", response.MimeType)
	assert.Equal(t, int64(102400), response.Size)
	assert.Equal(t, 1920, response.Dimensions.Width)
	assert.Equal(t, 1080, response.Dimensions.Height)
	assert.Contains(t, response.AvailableResolutions, "original")
	assert.Contains(t, response.AvailableResolutions, "thumbnail")
	assert.Contains(t, response.AvailableResolutions, "preview")
	assert.Equal(t, metadata.CreatedAt, response.CreatedAt)
}

func TestImageMetadata_IsValidUUID(t *testing.T) {
	tests := []struct {
		id    string
		valid bool
	}{
		{"f47ac10b-58cc-4372-a567-0e02b2c3d479", true},
		{"550e8400-e29b-41d4-a716-446655440000", true},
		{"invalid-uuid", false},
		{"f47ac10b-58cc-4372-a567-0e02b2c3d47", false},   // Too short
		{"f47ac10b-58cc-4372-a567-0e02b2c3d4799", false}, // Too long
		{"", false},
		{"f47ac10b-58cc-5372-a567-0e02b2c3d479", false}, // Wrong version
	}

	for _, tt := range tests {
		metadata := &ImageMetadata{ID: tt.id}
		assert.Equal(t, tt.valid, metadata.IsValidUUID(), "UUID: %s", tt.id)
	}
}

func TestImageMetadata_IsValidMimeType(t *testing.T) {
	validTypes := []string{"image/jpeg", "image/png", "image/gif", "image/webp"}
	invalidTypes := []string{"text/plain", "application/pdf", "image/bmp", "video/mp4", ""}

	for _, mimeType := range validTypes {
		metadata := &ImageMetadata{MimeType: mimeType}
		assert.True(t, metadata.IsValidMimeType(), "MIME type: %s", mimeType)
	}

	for _, mimeType := range invalidTypes {
		metadata := &ImageMetadata{MimeType: mimeType}
		assert.False(t, metadata.IsValidMimeType(), "MIME type: %s", mimeType)
	}
}

func TestImageMetadata_Validate(t *testing.T) {
	validMetadata := &ImageMetadata{
		ID:       "f47ac10b-58cc-4372-a567-0e02b2c3d479",
		Filename: "test.jpg",
		MimeType: "image/jpeg",
		Size:     102400,
		Width:    1920,
		Height:   1080,
	}

	// Valid metadata should pass
	err := validMetadata.Validate()
	assert.NoError(t, err)

	// Test various invalid cases
	tests := []struct {
		name     string
		modify   func(*ImageMetadata)
		errField string
	}{
		{
			"empty ID",
			func(m *ImageMetadata) { m.ID = "" },
			"id",
		},
		{
			"invalid UUID",
			func(m *ImageMetadata) { m.ID = "invalid-uuid" },
			"id",
		},
		{
			"empty filename",
			func(m *ImageMetadata) { m.Filename = "" },
			"filename",
		},
		{
			"empty MIME type",
			func(m *ImageMetadata) { m.MimeType = "" },
			"mime_type",
		},
		{
			"invalid MIME type",
			func(m *ImageMetadata) { m.MimeType = "text/plain" },
			"mime_type",
		},
		{
			"zero size",
			func(m *ImageMetadata) { m.Size = 0 },
			"size",
		},
		{
			"negative size",
			func(m *ImageMetadata) { m.Size = -1 },
			"size",
		},
		{
			"zero width",
			func(m *ImageMetadata) { m.Width = 0 },
			"dimensions",
		},
		{
			"zero height",
			func(m *ImageMetadata) { m.Height = 0 },
			"dimensions",
		},
		{
			"negative width",
			func(m *ImageMetadata) { m.Width = -1 },
			"dimensions",
		},
		{
			"negative height",
			func(m *ImageMetadata) { m.Height = -1 },
			"dimensions",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a copy of valid metadata
			metadata := *validMetadata
			tt.modify(&metadata)

			err := metadata.Validate()
			assert.Error(t, err)
			assert.IsType(t, ValidationError{}, err)
			validationErr := err.(ValidationError)
			assert.Equal(t, tt.errField, validationErr.Field)
		})
	}
}

func TestParseResolution(t *testing.T) {
	tests := []struct {
		resolution string
		expected   ResolutionConfig
		expectErr  bool
	}{
		{"thumbnail", ResolutionConfig{Width: 150, Height: 150}, false},
		{"preview", ResolutionConfig{Width: 800, Height: 600}, false},
		{"800x600", ResolutionConfig{Width: 800, Height: 600}, false},
		{"1920x1080", ResolutionConfig{Width: 1920, Height: 1080}, false},
		{"1x1", ResolutionConfig{Width: 1, Height: 1}, false},
		{"4096x4096", ResolutionConfig{Width: 4096, Height: 4096}, false},
		{"original", ResolutionConfig{}, true},
		{"8193x8193", ResolutionConfig{Width: 8193, Height: 8193}, false}, // Large dimensions are valid at parsing level
		{"invalid", ResolutionConfig{}, true},
		{"800", ResolutionConfig{}, true},
		{"800x", ResolutionConfig{}, true},
		{"x600", ResolutionConfig{}, true},
		{"800X600", ResolutionConfig{}, true}, // Wrong case
		{"0x600", ResolutionConfig{}, true},
		{"800x0", ResolutionConfig{}, true},
		{"-800x600", ResolutionConfig{}, true},
		{"800x-600", ResolutionConfig{}, true},
		{"", ResolutionConfig{}, true},
		{"abc x def", ResolutionConfig{}, true},
	}

	for _, tt := range tests {
		t.Run(tt.resolution, func(t *testing.T) {
			config, err := ParseResolution(tt.resolution)

			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected.Width, config.Width)
				assert.Equal(t, tt.expected.Height, config.Height)
			}
		})
	}
}

func TestResolutionConfig_String(t *testing.T) {
	config := ResolutionConfig{Width: 800, Height: 600}
	assert.Equal(t, "800x600", config.String())
}

func TestResolutionConfig_IsSquare(t *testing.T) {
	tests := []struct {
		config   ResolutionConfig
		expected bool
	}{
		{ResolutionConfig{Width: 800, Height: 800}, true},
		{ResolutionConfig{Width: 150, Height: 150}, true},
		{ResolutionConfig{Width: 800, Height: 600}, false},
		{ResolutionConfig{Width: 1920, Height: 1080}, false},
		{ResolutionConfig{Width: 0, Height: 0}, true},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.expected, tt.config.IsSquare(), "config: %dx%d", tt.config.Width, tt.config.Height)
	}
}

func TestResolutionConfig_AspectRatio(t *testing.T) {
	tests := []struct {
		config   ResolutionConfig
		expected float64
	}{
		{ResolutionConfig{Width: 800, Height: 600}, 800.0 / 600.0},
		{ResolutionConfig{Width: 1920, Height: 1080}, 1920.0 / 1080.0},
		{ResolutionConfig{Width: 150, Height: 150}, 1.0},
		{ResolutionConfig{Width: 100, Height: 0}, 0.0}, // Division by zero
	}

	for _, tt := range tests {
		assert.InDelta(t, tt.expected, tt.config.AspectRatio(), 0.001, "config: %dx%d", tt.config.Width, tt.config.Height)
	}
}

func TestGetMimeTypeFromExtension(t *testing.T) {
	tests := []struct {
		filename string
		expected string
	}{
		{"test.jpg", "image/jpeg"},
		{"test.jpeg", "image/jpeg"},
		{"test.JPG", "image/jpeg"},
		{"test.JPEG", "image/jpeg"},
		{"test.png", "image/png"},
		{"test.PNG", "image/png"},
		{"test.gif", "image/gif"},
		{"test.GIF", "image/gif"},
		{"test.webp", "image/webp"},
		{"test.WEBP", "image/webp"},
		{"test.bmp", ""},
		{"test.pdf", ""},
		{"test", ""},
		{"", ""},
		{"test.", ""},
	}

	for _, tt := range tests {
		mimeType := GetMimeTypeFromExtension(tt.filename)
		assert.Equal(t, tt.expected, mimeType, "filename: %s", tt.filename)
	}
}

func TestGetExtensionFromMimeType(t *testing.T) {
	tests := []struct {
		mimeType string
		expected string
	}{
		{"image/jpeg", "jpg"},
		{"image/png", "png"},
		{"image/gif", "gif"},
		{"image/webp", "webp"},
		{"image/bmp", ""},
		{"text/plain", ""},
		{"application/pdf", ""},
		{"", ""},
	}

	for _, tt := range tests {
		ext := GetExtensionFromMimeType(tt.mimeType)
		assert.Equal(t, tt.expected, ext, "MIME type: %s", tt.mimeType)
	}
}

func TestCustomErrorTypes(t *testing.T) {
	t.Run("ValidationError", func(t *testing.T) {
		err := ValidationError{
			Field:   "test_field",
			Message: "test message",
		}
		expected := "validation error on field 'test_field': test message"
		assert.Equal(t, expected, err.Error())
	})

	t.Run("NotFoundError", func(t *testing.T) {
		err := NotFoundError{
			Resource: "image",
			ID:       "123",
		}
		expected := "image with ID '123' not found"
		assert.Equal(t, expected, err.Error())
	})

	t.Run("ProcessingError", func(t *testing.T) {
		err := ProcessingError{
			Operation: "resize",
			Reason:    "invalid format",
		}
		expected := "processing error during resize: invalid format"
		assert.Equal(t, expected, err.Error())
	})

	t.Run("StorageError", func(t *testing.T) {
		err := StorageError{
			Operation: "upload",
			Backend:   "S3",
			Reason:    "connection failed",
		}
		expected := "storage error during upload on S3: connection failed"
		assert.Equal(t, expected, err.Error())
	})
}

func TestResponseStructures(t *testing.T) {
	t.Run("UploadResponse", func(t *testing.T) {
		response := UploadResponse{
			ID:          "test-id",
			Message:     "Success",
			Resolutions: []string{"thumbnail", "preview"},
		}

		assert.Equal(t, "test-id", response.ID)
		assert.Equal(t, "Success", response.Message)
		assert.Equal(t, []string{"thumbnail", "preview"}, response.Resolutions)
	})

	t.Run("InfoResponse", func(t *testing.T) {
		response := InfoResponse{
			ID:       "test-id",
			Filename: "test.jpg",
			MimeType: "image/jpeg",
			Size:     102400,
			Dimensions: DimensionInfo{
				Width:  1920,
				Height: 1080,
			},
			AvailableResolutions: []string{"original", "thumbnail"},
			CreatedAt:            time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC),
		}

		assert.Equal(t, "test-id", response.ID)
		assert.Equal(t, 1920, response.Dimensions.Width)
		assert.Equal(t, 1080, response.Dimensions.Height)
		assert.Contains(t, response.AvailableResolutions, "original")
	})

	t.Run("PresignedURLResponse", func(t *testing.T) {
		expiresAt := time.Now().Add(time.Hour)
		response := PresignedURLResponse{
			URL:       "https://example.com/presigned",
			ExpiresAt: expiresAt,
			ExpiresIn: 3600,
		}

		assert.Equal(t, "https://example.com/presigned", response.URL)
		assert.Equal(t, expiresAt, response.ExpiresAt)
		assert.Equal(t, 3600, response.ExpiresIn)
	})

	t.Run("ErrorResponse", func(t *testing.T) {
		response := ErrorResponse{
			Error:   "TestError",
			Message: "Test message",
			Code:    400,
		}

		assert.Equal(t, "TestError", response.Error)
		assert.Equal(t, "Test message", response.Message)
		assert.Equal(t, 400, response.Code)
	})

	t.Run("HealthResponse", func(t *testing.T) {
		response := HealthResponse{
			Status:    "healthy",
			Services:  map[string]string{"redis": "connected"},
			Timestamp: time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC),
		}

		assert.Equal(t, "healthy", response.Status)
		assert.Equal(t, "connected", response.Services["redis"])
		assert.False(t, response.Timestamp.IsZero())
	})
}

func TestDimensionInfo(t *testing.T) {
	dimensions := DimensionInfo{
		Width:  1920,
		Height: 1080,
	}

	assert.Equal(t, 1920, dimensions.Width)
	assert.Equal(t, 1080, dimensions.Height)
}

func TestStorageInfo(t *testing.T) {
	storageInfo := StorageInfo{
		Bucket: "test-bucket",
		Key:    "test-key",
		URL:    "https://example.com/test",
	}

	assert.Equal(t, "test-bucket", storageInfo.Bucket)
	assert.Equal(t, "test-key", storageInfo.Key)
	assert.Equal(t, "https://example.com/test", storageInfo.URL)
}

func TestImageProcessingRequest(t *testing.T) {
	request := ImageProcessingRequest{
		ImageID: "test-id",
		Resolution: ResolutionConfig{
			Width:  800,
			Height: 600,
		},
		Quality: 85,
	}

	assert.Equal(t, "test-id", request.ImageID)
	assert.Equal(t, 800, request.Resolution.Width)
	assert.Equal(t, 600, request.Resolution.Height)
	assert.Equal(t, 85, request.Quality)
}

func TestUploadRequest(t *testing.T) {
	request := UploadRequest{
		Resolutions: []string{"800x600", "1200x900"},
	}

	assert.Equal(t, 2, len(request.Resolutions))
	assert.Contains(t, request.Resolutions, "800x600")
	assert.Contains(t, request.Resolutions, "1200x900")
}

func TestEdgeCases(t *testing.T) {
	t.Run("empty filename extension", func(t *testing.T) {
		metadata := &ImageMetadata{Filename: ""}
		assert.Equal(t, "", metadata.GetFileExtension())
	})

	t.Run("metadata with nil resolutions", func(t *testing.T) {
		metadata := &ImageMetadata{}
		assert.False(t, metadata.HasResolution("test"))

		// Should not panic
		metadata.AddResolution("test")
		assert.Contains(t, metadata.Resolutions, "test")
	})

	t.Run("parse resolution edge cases", func(t *testing.T) {
		// Test maximum valid dimension (should succeed)
		config, err := ParseResolution("8192x8192")
		assert.NoError(t, err)
		assert.Equal(t, 8192, config.Width)
		assert.Equal(t, 8192, config.Height)

		// Test large dimension (should succeed at parsing level)
		config, err = ParseResolution("8193x8192")
		assert.NoError(t, err)
		assert.Equal(t, 8193, config.Width)
		assert.Equal(t, 8192, config.Height)
	})

	t.Run("storage key with different extensions", func(t *testing.T) {
		testCases := []struct {
			filename string
			expected string
		}{
			{"test.jpg", "jpg"},
			{"test.png", "png"},
			{"test.GIF", "gif"},
			{"test.WEBP", "webp"},
			{"noext", ""},
		}

		for _, tc := range testCases {
			metadata := &ImageMetadata{
				ID:       "test-id",
				Filename: tc.filename,
			}
			key := metadata.GetStorageKey("thumbnail")

			if tc.expected != "" {
				assert.Contains(t, key, "thumbnail."+tc.expected)
			}
		}
	})
}
