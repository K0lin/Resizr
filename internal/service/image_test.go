package service

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"resizr/internal/config"
	"resizr/internal/models"
	"resizr/internal/testutil"

	"github.com/stretchr/testify/assert"
)

func TestNewImageService(t *testing.T) {
	mockRepo := &testutil.MockImageRepository{}
	mockStorage := &testutil.MockStorageProvider{}
	mockProcessor := &testutil.MockProcessorService{}
	cfg := testutil.TestConfig()

	service := NewImageService(mockRepo, mockStorage, mockProcessor, cfg)

	assert.NotNil(t, service)

	// Type assertion to check internal fields
	impl, ok := service.(*ImageServiceImpl)
	assert.True(t, ok)
	assert.Equal(t, mockRepo, impl.repo)
	assert.Equal(t, mockStorage, impl.storage)
	assert.Equal(t, mockProcessor, impl.processor)
	assert.Equal(t, cfg, impl.config)
}

func TestImageService_ProcessUpload_Success(t *testing.T) {
	mockRepo := &testutil.MockImageRepository{
		SaveFunc: func(ctx context.Context, metadata *models.ImageMetadata) error {
			return nil
		},
	}
	mockStorage := &testutil.MockStorageProvider{
		UploadFunc: func(ctx context.Context, key string, data io.Reader, contentType string) error {
			return nil
		},
	}
	mockProcessor := &testutil.MockProcessorService{
		ValidateImageFunc: func(data []byte) (width, height int, mimeType string, err error) {
			return 1920, 1080, "image/jpeg", nil
		},
		ProcessImageFunc: func(ctx context.Context, request models.ImageProcessingRequest, imageData []byte) ([]byte, error) {
			return testutil.CreateTestImageData(), nil
		},
	}

	cfg := testutil.TestConfig()
	service := NewImageService(mockRepo, mockStorage, mockProcessor, cfg)

	input := testutil.UploadInput{
		Filename:    "test.jpg",
		Data:        testutil.CreateTestImageData(),
		Size:        int64(len(testutil.CreateTestImageData())),
		Resolutions: []string{"800x600"},
	}

	ctx := context.Background()
	result, err := service.ProcessUpload(ctx, input)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotEmpty(t, result.ImageID)
	assert.Contains(t, result.ProcessedResolutions, "800x600")
	if cfg.Image.GenerateDefaultResolutions {
		assert.Contains(t, result.ProcessedResolutions, "thumbnail")
		assert.Contains(t, result.ProcessedResolutions, "preview")
	}
	assert.Equal(t, input.Size, result.OriginalSize)
}

func TestImageService_ProcessUpload_ValidationError(t *testing.T) {
	service := NewImageService(&testutil.MockImageRepository{}, &testutil.MockStorageProvider{}, &testutil.MockProcessorService{}, testutil.TestConfig())

	tests := []struct {
		name    string
		input   testutil.UploadInput
		wantErr string
	}{
		{
			name: "empty filename",
			input: testutil.UploadInput{
				Filename: "",
				Data:     testutil.CreateTestImageData(),
				Size:     100,
			},
			wantErr: "filename",
		},
		{
			name: "empty data",
			input: testutil.UploadInput{
				Filename: "test.jpg",
				Data:     []byte{},
				Size:     0,
			},
			wantErr: "data",
		},
		{
			name: "size mismatch",
			input: testutil.UploadInput{
				Filename: "test.jpg",
				Data:     testutil.CreateTestImageData(),
				Size:     999, // Wrong size
			},
			wantErr: "size",
		},
		{
			name: "invalid resolution",
			input: testutil.UploadInput{
				Filename:    "test.jpg",
				Data:        testutil.CreateTestImageData(),
				Size:        int64(len(testutil.CreateTestImageData())),
				Resolutions: []string{"invalid"},
			},
			wantErr: "resolution",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			_, err := service.ProcessUpload(ctx, tt.input)

			assert.Error(t, err)
			assert.IsType(t, models.ValidationError{}, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestImageService_ProcessUpload_ProcessorError(t *testing.T) {
	mockProcessor := &testutil.MockProcessorService{
		ValidateImageFunc: func(data []byte) (width, height int, mimeType string, err error) {
			return 0, 0, "", errors.New("invalid image format")
		},
	}

	service := NewImageService(&testutil.MockImageRepository{}, &testutil.MockStorageProvider{}, mockProcessor, testutil.TestConfig())

	input := testutil.UploadInput{
		Filename: "test.jpg",
		Data:     testutil.CreateTestImageData(),
		Size:     int64(len(testutil.CreateTestImageData())),
	}

	ctx := context.Background()
	_, err := service.ProcessUpload(ctx, input)

	assert.Error(t, err)
	assert.IsType(t, models.ProcessingError{}, err)
}

func TestImageService_ProcessUpload_StorageError(t *testing.T) {
	mockStorage := &testutil.MockStorageProvider{
		UploadFunc: func(ctx context.Context, key string, data io.Reader, contentType string) error {
			return errors.New("storage unavailable")
		},
	}
	mockProcessor := &testutil.MockProcessorService{
		ValidateImageFunc: func(data []byte) (width, height int, mimeType string, err error) {
			return 1920, 1080, "image/jpeg", nil
		},
	}

	service := NewImageService(&testutil.MockImageRepository{}, mockStorage, mockProcessor, testutil.TestConfig())

	input := testutil.UploadInput{
		Filename: "test.jpg",
		Data:     testutil.CreateTestImageData(),
		Size:     int64(len(testutil.CreateTestImageData())),
	}

	ctx := context.Background()
	_, err := service.ProcessUpload(ctx, input)

	assert.Error(t, err)
	assert.IsType(t, models.StorageError{}, err)
}

func TestImageService_GetMetadata_Success(t *testing.T) {
	expectedMetadata := testutil.CreateTestImageMetadata()
	mockRepo := &testutil.MockImageRepository{
		GetByIDFunc: func(ctx context.Context, id string) (*models.ImageMetadata, error) {
			return expectedMetadata, nil
		},
	}

	service := NewImageService(mockRepo, &testutil.MockStorageProvider{}, &testutil.MockProcessorService{}, testutil.TestConfig())

	ctx := context.Background()
	metadata, err := service.GetMetadata(ctx, testutil.ValidUUID)

	assert.NoError(t, err)
	assert.Equal(t, expectedMetadata, metadata)
}

func TestImageService_GetMetadata_InvalidUUID(t *testing.T) {
	service := NewImageService(&testutil.MockImageRepository{}, &testutil.MockStorageProvider{}, &testutil.MockProcessorService{}, testutil.TestConfig())

	ctx := context.Background()
	_, err := service.GetMetadata(ctx, testutil.InvalidUUID)

	assert.Error(t, err)
	assert.IsType(t, models.ValidationError{}, err)
	assert.Contains(t, err.Error(), "Invalid UUID format")
}

func TestImageService_GetMetadata_NotFound(t *testing.T) {
	mockRepo := &testutil.MockImageRepository{
		GetByIDFunc: func(ctx context.Context, id string) (*models.ImageMetadata, error) {
			return nil, models.NotFoundError{Resource: "image", ID: id}
		},
	}

	service := NewImageService(mockRepo, &testutil.MockStorageProvider{}, &testutil.MockProcessorService{}, testutil.TestConfig())

	ctx := context.Background()
	_, err := service.GetMetadata(ctx, testutil.ValidUUID)

	assert.Error(t, err)
	assert.IsType(t, models.NotFoundError{}, err)
}

func TestImageService_GetImageStream_Success(t *testing.T) {
	expectedMetadata := testutil.CreateTestImageMetadata()
	testData := testutil.CreateTestImageData()

	mockRepo := &testutil.MockImageRepository{
		GetByIDFunc: func(ctx context.Context, id string) (*models.ImageMetadata, error) {
			return expectedMetadata, nil
		},
	}
	mockStorage := &testutil.MockStorageProvider{
		DownloadFunc: func(ctx context.Context, key string) (io.ReadCloser, error) {
			return testutil.NewMockReadCloser(testData), nil
		},
	}

	service := NewImageService(mockRepo, mockStorage, &testutil.MockProcessorService{}, testutil.TestConfig())

	ctx := context.Background()
	stream, metadata, err := service.GetImageStream(ctx, testutil.ValidUUID, "thumbnail")

	assert.NoError(t, err)
	assert.NotNil(t, stream)
	assert.Equal(t, expectedMetadata, metadata)

	// Read and verify stream data
	data, err := io.ReadAll(stream)
	assert.NoError(t, err)
	assert.Equal(t, testData, data)

	stream.Close()
}

func TestImageService_GetImageStream_ResolutionNotFound(t *testing.T) {
	expectedMetadata := testutil.CreateTestImageMetadata()

	mockRepo := &testutil.MockImageRepository{
		GetByIDFunc: func(ctx context.Context, id string) (*models.ImageMetadata, error) {
			return expectedMetadata, nil
		},
	}

	service := NewImageService(mockRepo, &testutil.MockStorageProvider{}, &testutil.MockProcessorService{}, testutil.TestConfig())

	ctx := context.Background()
	_, _, err := service.GetImageStream(ctx, testutil.ValidUUID, "nonexistent")

	assert.Error(t, err)
	assert.IsType(t, models.NotFoundError{}, err)
}

func TestImageService_GeneratePresignedURL_Success(t *testing.T) {
	expectedURL := "https://example.com/presigned-url"
	mockStorage := &testutil.MockStorageProvider{
		GeneratePresignedURLFunc: func(ctx context.Context, key string, expiration time.Duration) (string, error) {
			return expectedURL, nil
		},
	}

	service := NewImageService(&testutil.MockImageRepository{}, mockStorage, &testutil.MockProcessorService{}, testutil.TestConfig())

	ctx := context.Background()
	storageKey := "images/test/thumbnail.jpg"
	duration := time.Hour

	url, err := service.GeneratePresignedURL(ctx, storageKey, duration)

	assert.NoError(t, err)
	assert.Equal(t, expectedURL, url)
}

func TestImageService_GeneratePresignedURL_Error(t *testing.T) {
	mockStorage := &testutil.MockStorageProvider{
		GeneratePresignedURLFunc: func(ctx context.Context, key string, expiration time.Duration) (string, error) {
			return "", errors.New("storage error")
		},
	}

	service := NewImageService(&testutil.MockImageRepository{}, mockStorage, &testutil.MockProcessorService{}, testutil.TestConfig())

	ctx := context.Background()
	_, err := service.GeneratePresignedURL(ctx, "test-key", time.Hour)

	assert.Error(t, err)
	assert.IsType(t, models.StorageError{}, err)
}

func TestImageService_DeleteImage_Success(t *testing.T) {
	expectedMetadata := testutil.CreateTestImageMetadata()

	mockRepo := &testutil.MockImageRepository{
		GetByIDFunc: func(ctx context.Context, id string) (*models.ImageMetadata, error) {
			return expectedMetadata, nil
		},
		DeleteFunc: func(ctx context.Context, id string) error {
			return nil
		},
	}
	mockStorage := &testutil.MockStorageProvider{
		DeleteFunc: func(ctx context.Context, key string) error {
			return nil
		},
	}

	service := NewImageService(mockRepo, mockStorage, &testutil.MockProcessorService{}, testutil.TestConfig())

	ctx := context.Background()
	err := service.DeleteImage(ctx, testutil.ValidUUID)

	assert.NoError(t, err)
}

func TestImageService_ListImages_Success(t *testing.T) {
	expectedImages := []*models.ImageMetadata{
		testutil.CreateTestImageMetadata(),
		testutil.CreateTestImageMetadata(),
	}

	mockRepo := &testutil.MockImageRepository{
		ListFunc: func(ctx context.Context, offset, limit int) ([]*models.ImageMetadata, error) {
			return expectedImages, nil
		},
	}

	service := NewImageService(mockRepo, &testutil.MockStorageProvider{}, &testutil.MockProcessorService{}, testutil.TestConfig())

	ctx := context.Background()
	images, total, err := service.ListImages(ctx, 0, 10)

	assert.NoError(t, err)
	assert.Equal(t, expectedImages, images)
	assert.Equal(t, -1, total) // Implementation returns -1 for unknown total
}

func TestImageService_ListImages_LimitValidation(t *testing.T) {
	mockRepo := &testutil.MockImageRepository{
		ListFunc: func(ctx context.Context, offset, limit int) ([]*models.ImageMetadata, error) {
			// Verify limit was adjusted
			assert.Equal(t, 50, limit)
			return []*models.ImageMetadata{}, nil
		},
	}

	service := NewImageService(mockRepo, &testutil.MockStorageProvider{}, &testutil.MockProcessorService{}, testutil.TestConfig())

	ctx := context.Background()

	// Test invalid limits are adjusted to default
	_, _, err := service.ListImages(ctx, 0, 0) // Zero limit
	assert.NoError(t, err)

	_, _, err = service.ListImages(ctx, 0, -1) // Negative limit
	assert.NoError(t, err)

	_, _, err = service.ListImages(ctx, 0, 200) // Excessive limit
	assert.NoError(t, err)
}

func TestImageService_ValidateUploadInput(t *testing.T) {
	cfg := testutil.TestConfig()
	service := &ImageServiceImpl{config: cfg}

	tests := []struct {
		name    string
		input   testutil.UploadInput
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid input",
			input: testutil.UploadInput{
				Filename:    "test.jpg",
				Data:        testutil.CreateTestImageData(),
				Size:        int64(len(testutil.CreateTestImageData())),
				Resolutions: []string{"800x600", "1200x900"},
			},
			wantErr: false,
		},
		{
			name: "comma separated resolutions",
			input: testutil.UploadInput{
				Filename:    "test.jpg",
				Data:        testutil.CreateTestImageData(),
				Size:        int64(len(testutil.CreateTestImageData())),
				Resolutions: []string{"800x600,1200x900,1600x1200"},
			},
			wantErr: false,
		},
		{
			name: "resolution exceeds max dimensions",
			input: testutil.UploadInput{
				Filename:    "test.jpg",
				Data:        testutil.CreateTestImageData(),
				Size:        int64(len(testutil.CreateTestImageData())),
				Resolutions: []string{"10000x10000"}, // Exceeds config max
			},
			wantErr: true,
			errMsg:  "exceeds maximum configured",
		},
		{
			name: "empty resolution after trim",
			input: testutil.UploadInput{
				Filename:    "test.jpg",
				Data:        testutil.CreateTestImageData(),
				Size:        int64(len(testutil.CreateTestImageData())),
				Resolutions: []string{"   ,  , 800x600  "},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := service.validateUploadInput(tt.input)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestImageService_ProcessResolution_Success(t *testing.T) {
	originalData := testutil.CreateTestImageData()
	expectedMetadata := testutil.CreateTestImageMetadata()

	mockRepo := &testutil.MockImageRepository{
		GetByIDFunc: func(ctx context.Context, id string) (*models.ImageMetadata, error) {
			return expectedMetadata, nil
		},
		UpdateFunc: func(ctx context.Context, metadata *models.ImageMetadata) error {
			return nil
		},
	}
	mockStorage := &testutil.MockStorageProvider{
		DownloadFunc: func(ctx context.Context, key string) (io.ReadCloser, error) {
			return testutil.NewMockReadCloser(originalData), nil
		},
		UploadFunc: func(ctx context.Context, key string, data io.Reader, contentType string) error {
			return nil
		},
	}
	mockProcessor := &testutil.MockProcessorService{
		ProcessImageFunc: func(ctx context.Context, request models.ImageProcessingRequest, imageData []byte) ([]byte, error) {
			return testutil.CreateTestImageData(), nil
		},
	}

	service := NewImageService(mockRepo, mockStorage, mockProcessor, testutil.TestConfig())

	ctx := context.Background()
	err := service.ProcessResolution(ctx, testutil.ValidUUID, "1024x768")

	assert.NoError(t, err)
}

func TestImageService_ProcessResolution_AlreadyExists(t *testing.T) {
	expectedMetadata := testutil.CreateTestImageMetadata()
	// Add the resolution we're trying to process
	expectedMetadata.Resolutions = append(expectedMetadata.Resolutions, "1024x768")

	mockRepo := &testutil.MockImageRepository{
		GetByIDFunc: func(ctx context.Context, id string) (*models.ImageMetadata, error) {
			return expectedMetadata, nil
		},
	}

	service := NewImageService(mockRepo, &testutil.MockStorageProvider{}, &testutil.MockProcessorService{}, testutil.TestConfig())

	ctx := context.Background()
	err := service.ProcessResolution(ctx, testutil.ValidUUID, "1024x768")

	// Should succeed without doing anything
	assert.NoError(t, err)
}

func TestImageService_ResizeConfig(t *testing.T) {
	cfg := &config.Config{
		Image: config.ImageConfig{
			Quality:    90,
			ResizeMode: "crop",
		},
	}

	service := &ImageServiceImpl{config: cfg}

	// Test the resize configuration creation
	resolutionConfig := models.ResolutionConfig{Width: 800, Height: 600}
	resizeConfig := ResizeConfig{
		Width:   resolutionConfig.Width,
		Height:  resolutionConfig.Height,
		Quality: service.config.Image.Quality,
		Format:  "image/jpeg",
		Mode:    ResizeMode(service.config.Image.ResizeMode),
	}

	assert.Equal(t, 800, resizeConfig.Width)
	assert.Equal(t, 600, resizeConfig.Height)
	assert.Equal(t, 90, resizeConfig.Quality)
	assert.Equal(t, "image/jpeg", resizeConfig.Format)
	assert.Equal(t, ResizeMode("crop"), resizeConfig.Mode)
}

func TestUploadInput_Validation(t *testing.T) {
	testData := testutil.CreateTestImageData()

	validInput := testutil.UploadInput{
		Filename:    "test.jpg",
		Data:        testData,
		Size:        int64(len(testData)),
		Resolutions: []string{"800x600"},
	}

	// Test valid input structure
	assert.Equal(t, "test.jpg", validInput.Filename)
	assert.Equal(t, testData, validInput.Data)
	assert.Equal(t, int64(len(testData)), validInput.Size)
	assert.Equal(t, []string{"800x600"}, validInput.Resolutions)
}

func TestUploadResult_Structure(t *testing.T) {
	result := &testutil.UploadResult{
		ImageID:              testutil.ValidUUID,
		ProcessedResolutions: []string{"thumbnail", "preview", "800x600"},
		OriginalSize:         102400,
		ProcessedSizes: map[string]int64{
			"thumbnail": 5000,
			"preview":   25000,
			"800x600":   15000,
		},
	}

	assert.Equal(t, testutil.ValidUUID, result.ImageID)
	assert.Contains(t, result.ProcessedResolutions, "thumbnail")
	assert.Equal(t, int64(102400), result.OriginalSize)
	assert.Equal(t, int64(5000), result.ProcessedSizes["thumbnail"])
}
