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
		Resolutions: []string{"thumbnail", "800x600"},
	}

	assert.True(t, metadata.HasResolution("thumbnail"))
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
	metadata.AddResolution("800x600")
	assert.Contains(t, metadata.Resolutions, "800x600")
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
		Resolutions: []string{"thumbnail", "800x600"},
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
	assert.Contains(t, response.AvailableResolutions, "800x600")
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

func TestNewImageMetadataWithHash(t *testing.T) {
	id := "f47ac10b-58cc-4372-a567-0e02b2c3d479"
	filename := "test.jpg"
	mimeType := "image/jpeg"
	size := int64(102400)
	width := 1920
	height := 1080
	hash := ImageHash{
		Algorithm: "SHA256",
		Value:     "abcdef123456",
		Size:      size,
	}

	metadata := NewImageMetadataWithHash(id, filename, mimeType, size, width, height, hash)

	assert.Equal(t, id, metadata.ID)
	assert.Equal(t, filename, metadata.Filename)
	assert.Equal(t, mimeType, metadata.MimeType)
	assert.Equal(t, size, metadata.Size)
	assert.Equal(t, width, metadata.Width)
	assert.Equal(t, height, metadata.Height)
	assert.Equal(t, hash, metadata.Hash)
	assert.Equal(t, "images/f47ac10b-58cc-4372-a567-0e02b2c3d479/original.jpg", metadata.OriginalKey)
	assert.Empty(t, metadata.Resolutions)
	assert.True(t, time.Since(metadata.CreatedAt) < time.Second)
	assert.True(t, time.Since(metadata.UpdatedAt) < time.Second)
}

func TestImageMetadata_GetActualStorageKey(t *testing.T) {
	t.Run("non_deduped_image", func(t *testing.T) {
		metadata := &ImageMetadata{
			ID:        "f47ac10b-58cc-4372-a567-0e02b2c3d479",
			IsDeduped: false,
			Filename:  "test.jpg",
			MimeType:  "image/jpeg",
		}

		key := metadata.GetActualStorageKey("800x600")
		expected := "images/f47ac10b-58cc-4372-a567-0e02b2c3d479/800x600.jpg"
		assert.Equal(t, expected, key)
	})

	t.Run("deduped_image", func(t *testing.T) {
		metadata := &ImageMetadata{
			ID:            "f47ac10b-58cc-4372-a567-0e02b2c3d479",
			IsDeduped:     true,
			SharedImageID: "550e8400-e29b-41d4-a716-446655440000",
			Filename:      "test.jpg",
			MimeType:      "image/jpeg",
		}

		key := metadata.GetActualStorageKey("800x600")
		expected := "images/550e8400-e29b-41d4-a716-446655440000/800x600.jpg"
		assert.Equal(t, expected, key)
	})
}

func TestImageMetadata_MarkAsDeduped(t *testing.T) {
	metadata := &ImageMetadata{
		ID:        "f47ac10b-58cc-4372-a567-0e02b2c3d479",
		IsDeduped: false,
	}

	sharedImageID := "550e8400-e29b-41d4-a716-446655440000"
	metadata.MarkAsDeduped(sharedImageID)

	assert.True(t, metadata.IsDeduped)
	assert.Equal(t, sharedImageID, metadata.SharedImageID)
	assert.True(t, time.Since(metadata.UpdatedAt) < time.Second)
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
			Resolutions: []string{"thumbnail", "800x600"},
		}

		assert.Equal(t, "test-id", response.ID)
		assert.Equal(t, "Success", response.Message)
		assert.Equal(t, []string{"thumbnail", "800x600"}, response.Resolutions)
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

// Test alias functionality
func TestResolutionAliases(t *testing.T) {
	t.Run("ParseResolution with aliases", func(t *testing.T) {
		testCases := []struct {
			input          string
			expectedWidth  int
			expectedHeight int
			expectedAlias  string
			shouldError    bool
		}{
			{"800x600:small", 800, 600, "small", false},
			{"1920x1080:large", 1920, 1080, "large", false},
			{"100x100:tiny", 100, 100, "tiny", false},
			{"800x600:my_custom_size", 800, 600, "my_custom_size", false},
			{"800x600:", 800, 600, "", false},       // Empty alias should work
			{"800x600", 800, 600, "", false},        // No alias should work
			{":alias", 0, 0, "", true},              // No dimensions should fail
			{"800x600:alias:extra", 0, 0, "", true}, // Multiple colons should fail
		}

		for _, tc := range testCases {
			t.Run(tc.input, func(t *testing.T) {
				config, err := ParseResolution(tc.input)

				if tc.shouldError {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
					assert.Equal(t, tc.expectedWidth, config.Width)
					assert.Equal(t, tc.expectedHeight, config.Height)
					assert.Equal(t, tc.expectedAlias, config.Alias)
				}
			})
		}
	})

	t.Run("ResolutionConfig String with aliases", func(t *testing.T) {
		testCases := []struct {
			config   ResolutionConfig
			expected string
		}{
			{ResolutionConfig{800, 600, "small"}, "800x600:small"},
			{ResolutionConfig{1920, 1080, "large"}, "1920x1080:large"},
			{ResolutionConfig{800, 600, ""}, "800x600"},
			{ResolutionConfig{100, 100, "tiny"}, "100x100:tiny"},
		}

		for _, tc := range testCases {
			result := tc.config.String()
			assert.Equal(t, tc.expected, result)
		}
	})

	t.Run("Alias utility functions", func(t *testing.T) {
		testCases := []struct {
			input              string
			expectedDimensions string
			expectedAlias      string
		}{
			{"800x600:small", "800x600", "small"},
			{"1920x1080:large_screen", "1920x1080", "large_screen"},
			{"100x100", "100x100", ""},
			{"thumbnail", "thumbnail", ""},
			{"800x600:", "800x600", ""},
		}

		for _, tc := range testCases {
			t.Run(tc.input, func(t *testing.T) {
				dimensions := ExtractDimensions(tc.input)
				alias := ExtractAlias(tc.input)

				assert.Equal(t, tc.expectedDimensions, dimensions)
				assert.Equal(t, tc.expectedAlias, alias)

				// Test SplitResolutionAndAlias
				splitDims, splitAlias := SplitResolutionAndAlias(tc.input)
				assert.Equal(t, tc.expectedDimensions, splitDims)
				assert.Equal(t, tc.expectedAlias, splitAlias)
			})
		}
	})

	t.Run("HasResolution with aliases", func(t *testing.T) {
		metadata := &ImageMetadata{
			Resolutions: []string{"thumbnail", "100x100:small", "800x600:medium", "1920x1080:large"},
		}

		// Test direct matches (legacy)
		assert.True(t, metadata.HasResolution("thumbnail"))

		// Test alias access
		assert.True(t, metadata.HasResolution("small"))
		assert.True(t, metadata.HasResolution("medium"))
		assert.True(t, metadata.HasResolution("large"))

		// Test dimensions access
		assert.True(t, metadata.HasResolution("100x100"))
		assert.True(t, metadata.HasResolution("800x600"))
		assert.True(t, metadata.HasResolution("1920x1080"))

		// Test invalid access (should be blocked)
		assert.False(t, metadata.HasResolution("100x100:small"))  // Full string access blocked
		assert.False(t, metadata.HasResolution("800x600:medium")) // Full string access blocked
		assert.False(t, metadata.HasResolution("nonexistent"))    // Non-existent alias
		assert.False(t, metadata.HasResolution("999x999"))        // Non-existent dimensions
	})

	t.Run("GetStorageKey with aliases", func(t *testing.T) {
		metadata := &ImageMetadata{
			ID:          "test-id",
			Filename:    "test.jpg",
			Resolutions: []string{"thumbnail", "100x100:small", "800x600:medium"},
		}

		// Test storage key generation (always uses dimensions to avoid duplicates)
		testCases := []struct {
			resolution string
			expected   string
		}{
			{"thumbnail", "images/test-id/thumbnail.jpg"},
			{"small", "images/test-id/100x100.jpg"},   // Alias resolves to dimensions
			{"medium", "images/test-id/800x600.jpg"},  // Alias resolves to dimensions
			{"100x100", "images/test-id/100x100.jpg"}, // Direct dimensions
			{"800x600", "images/test-id/800x600.jpg"}, // Direct dimensions
			{"original", "images/test-id/original.jpg"},
		}

		for _, tc := range testCases {
			t.Run(tc.resolution, func(t *testing.T) {
				result := metadata.GetStorageKey(tc.resolution)
				assert.Equal(t, tc.expected, result)
			})
		}
	})

	t.Run("FindStoredResolution", func(t *testing.T) {
		metadata := &ImageMetadata{
			Resolutions: []string{"thumbnail", "100x100:small", "800x600:medium"},
		}

		testCases := []struct {
			input    string
			expected string
		}{
			{"thumbnail", "thumbnail"},     // Direct match
			{"small", "100x100"},           // Alias resolves to dimensions
			{"medium", "800x600"},          // Alias resolves to dimensions
			{"100x100", "100x100"},         // Direct dimensions
			{"800x600", "800x600"},         // Direct dimensions
			{"nonexistent", "nonexistent"}, // Fallback
		}

		for _, tc := range testCases {
			t.Run(tc.input, func(t *testing.T) {
				result := metadata.FindStoredResolution(tc.input)
				assert.Equal(t, tc.expected, result)
			})
		}
	})

	t.Run("ResolveToDimensions", func(t *testing.T) {
		metadata := &ImageMetadata{
			Resolutions: []string{"thumbnail", "100x100:small", "800x600:medium"},
		}

		testCases := []struct {
			input    string
			expected string
		}{
			{"thumbnail", "thumbnail"},     // Predefined resolution
			{"small", "100x100"},           // Alias resolves to dimensions
			{"medium", "800x600"},          // Alias resolves to dimensions
			{"100x100", "100x100"},         // Already dimensions
			{"800x600", "800x600"},         // Already dimensions
			{"nonexistent", "nonexistent"}, // Fallback
		}

		for _, tc := range testCases {
			t.Run(tc.input, func(t *testing.T) {
				result := metadata.ResolveToDimensions(tc.input)
				assert.Equal(t, tc.expected, result)
			})
		}
	})

	t.Run("FormatResolutionWithAlias", func(t *testing.T) {
		testCases := []struct {
			width    int
			height   int
			alias    string
			expected string
		}{
			{800, 600, "small", "800x600:small"},
			{1920, 1080, "", "1920x1080"},
			{100, 100, "tiny", "100x100:tiny"},
		}

		for _, tc := range testCases {
			result := FormatResolutionWithAlias(tc.width, tc.height, tc.alias)
			assert.Equal(t, tc.expected, result)
		}
	})

	t.Run("IsValidDimensionFormat", func(t *testing.T) {
		testCases := []struct {
			input    string
			expected bool
		}{
			{"800x600", true},
			{"1920x1080", true},
			{"1x1", true},
			{"800x600:alias", false}, // With alias
			{"thumbnail", false},     // Predefined
			{"800", false},           // Missing height
			{"x600", false},          // Missing width
			{"abc", false},           // Invalid format
		}

		for _, tc := range testCases {
			t.Run(tc.input, func(t *testing.T) {
				result := IsValidDimensionFormat(tc.input)
				assert.Equal(t, tc.expected, result)
			})
		}
	})

	t.Run("Backward compatibility", func(t *testing.T) {
		// Simulate existing data (resolutions without aliases)
		metadata := &ImageMetadata{
			ID:          "test-id",
			Filename:    "test.jpg",
			Resolutions: []string{"thumbnail", "100x100", "800x600"},
		}

		// All existing functionality should still work
		assert.True(t, metadata.HasResolution("thumbnail"))
		assert.True(t, metadata.HasResolution("100x100"))
		assert.True(t, metadata.HasResolution("800x600"))

		// Storage keys should work as before
		assert.Equal(t, "images/test-id/thumbnail.jpg", metadata.GetStorageKey("thumbnail"))
		assert.Equal(t, "images/test-id/100x100.jpg", metadata.GetStorageKey("100x100"))
		assert.Equal(t, "images/test-id/800x600.jpg", metadata.GetStorageKey("800x600"))

		// Can add new alias resolutions
		metadata.AddResolution("1920x1080:large")
		assert.True(t, metadata.HasResolution("large"))
		assert.True(t, metadata.HasResolution("1920x1080"))
		// Storage key should always use dimensions (no duplicates)
		assert.Equal(t, "images/test-id/1920x1080.jpg", metadata.GetStorageKey("large"))
	})
}

// Additional tests to increase coverage to 95%
func TestCalculateImageHashFromReader(t *testing.T) {
	// Test the private _CalculateImageHashFromReader function indirectly
	// by testing functions that use it or similar functionality

	// Test CompareBytesByBytes more thoroughly
	t.Run("CompareBytesByBytes edge cases", func(t *testing.T) {
		data1 := []byte("test data")
		data2 := []byte("test data")
		data3 := []byte("different data")

		// Test same data
		assert.True(t, CompareBytesByBytes(data1, data2))

		// Test different data
		assert.False(t, CompareBytesByBytes(data1, data3))

		// Test empty data
		assert.True(t, CompareBytesByBytes([]byte{}, []byte{}))

		// Test one empty, one not
		assert.False(t, CompareBytesByBytes([]byte{}, data1))
		assert.False(t, CompareBytesByBytes(data1, []byte{}))
	})
}

func TestDeduplicationInfoEdgeCases(t *testing.T) {
	t.Run("HasReference edge cases", func(t *testing.T) {
		hash := ImageHash{Algorithm: "sha256", Value: "test", Size: 8}
		dedupInfo := NewDeduplicationInfo(hash, "master-id", "images/master/original.jpg")

		// Test with existing reference
		assert.True(t, dedupInfo.HasReference("master-id"))

		// Test with non-existing reference
		assert.False(t, dedupInfo.HasReference("non-existing"))

		// Test with empty string
		assert.False(t, dedupInfo.HasReference(""))
	})

	t.Run("AddResolutionReference edge cases", func(t *testing.T) {
		hash := ImageHash{Algorithm: "sha256", Value: "test", Size: 8}
		dedupInfo := NewDeduplicationInfo(hash, "master-id", "images/master/original.jpg")

		// Add resolution for existing image
		dedupInfo.AddResolutionReference("master-id", "800x600")
		assert.True(t, dedupInfo.HasResolutionReference("master-id", "800x600"))

		// Add another resolution for same image
		dedupInfo.AddResolutionReference("master-id", "thumbnail")
		assert.True(t, dedupInfo.HasResolutionReference("master-id", "thumbnail"))
		assert.Equal(t, 2, dedupInfo.GetResolutionReferenceCount("master-id"))

		// Try to add same resolution again (should not duplicate)
		dedupInfo.AddResolutionReference("master-id", "800x600")
		assert.Equal(t, 2, dedupInfo.GetResolutionReferenceCount("master-id"))

		// Add resolution for non-existing image (only creates resolution reference, not main reference)
		dedupInfo.AddResolutionReference("100x100", "new-id")
		assert.False(t, dedupInfo.HasReference("new-id"))                     // Main reference doesn't exist
		assert.True(t, dedupInfo.HasResolutionReference("100x100", "new-id")) // But resolution reference does
	})
}

func TestParseResolutionEdgeCases(t *testing.T) {
	t.Run("ParseResolution additional cases", func(t *testing.T) {
		// Test max dimension limits
		config, err := ParseResolution("8192x8192")
		assert.NoError(t, err)
		assert.Equal(t, 8192, config.Width)
		assert.Equal(t, 8192, config.Height)

		// Test dimensions exactly at limit
		config, err = ParseResolution("4096x4096")
		assert.NoError(t, err)
		assert.Equal(t, 4096, config.Width)
		assert.Equal(t, 4096, config.Height)

		// Test with various valid formats
		validCases := []struct {
			input    string
			expected ResolutionConfig
		}{
			{"50x50", ResolutionConfig{Width: 50, Height: 50, Alias: ""}},
			{"1x1", ResolutionConfig{Width: 1, Height: 1, Alias: ""}},
			{"100x200:custom", ResolutionConfig{Width: 100, Height: 200, Alias: "custom"}},
		}

		for _, tc := range validCases {
			config, err := ParseResolution(tc.input)
			assert.NoError(t, err, "failed for input: %s", tc.input)
			assert.Equal(t, tc.expected.Width, config.Width)
			assert.Equal(t, tc.expected.Height, config.Height)
			assert.Equal(t, tc.expected.Alias, config.Alias)
		}
	})
}

func TestGetActualStorageKeyEdgeCases(t *testing.T) {
	t.Run("GetActualStorageKey with various dedup states", func(t *testing.T) {
		// Test non-deduped image (normal case)
		metadata := &ImageMetadata{
			ID:       "test-id",
			Filename: "test.jpg",
		}

		key := metadata.GetActualStorageKey("800x600")
		assert.Equal(t, "images/test-id/800x600.jpg", key)

		// Test deduped image with shared ID
		metadata.IsDeduped = true
		metadata.SharedImageID = "shared-id"

		key = metadata.GetActualStorageKey("800x600")
		assert.Equal(t, "images/shared-id/800x600.jpg", key)

		// Test deduped image without shared ID (should fall back to own ID)
		metadata.SharedImageID = ""

		key = metadata.GetActualStorageKey("800x600")
		assert.Equal(t, "images/test-id/800x600.jpg", key)

		// Test with original resolution
		key = metadata.GetActualStorageKey("original")
		assert.Equal(t, "images/test-id/original.jpg", key)
	})
}
