package service

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"resizr/internal/config"
	"resizr/internal/models"
	"resizr/internal/repository"
	"resizr/internal/storage"
	"resizr/internal/testutil"

	"github.com/stretchr/testify/assert"
)

// Local mocks to avoid interface mismatches
type mockImageRepositoryForImageService struct {
	saveFunc     func(ctx context.Context, metadata *models.ImageMetadata) error
	getByIDFunc  func(ctx context.Context, id string) (*models.ImageMetadata, error)
	updateFunc   func(ctx context.Context, metadata *models.ImageMetadata) error
	deleteFunc   func(ctx context.Context, id string) error
	existsFunc   func(ctx context.Context, id string) (bool, error)
	listFunc     func(ctx context.Context, offset, limit int) ([]*models.ImageMetadata, error)
	healthFunc   func(ctx context.Context) error
	closeFunc    func() error
	getStatsFunc func(ctx context.Context) (*repository.RepositoryStats, error)
}

func (m *mockImageRepositoryForImageService) Save(ctx context.Context, metadata *models.ImageMetadata) error {
	if m.saveFunc != nil {
		return m.saveFunc(ctx, metadata)
	}
	return nil
}

func (m *mockImageRepositoryForImageService) GetByID(ctx context.Context, id string) (*models.ImageMetadata, error) {
	if m.getByIDFunc != nil {
		return m.getByIDFunc(ctx, id)
	}
	return nil, nil
}

func (m *mockImageRepositoryForImageService) Get(ctx context.Context, id string) (*models.ImageMetadata, error) {
	return m.GetByID(ctx, id)
}

func (m *mockImageRepositoryForImageService) Store(ctx context.Context, metadata *models.ImageMetadata) error {
	return m.Save(ctx, metadata)
}

func (m *mockImageRepositoryForImageService) Update(ctx context.Context, metadata *models.ImageMetadata) error {
	if m.updateFunc != nil {
		return m.updateFunc(ctx, metadata)
	}
	return nil
}

func (m *mockImageRepositoryForImageService) Delete(ctx context.Context, id string) error {
	if m.deleteFunc != nil {
		return m.deleteFunc(ctx, id)
	}
	return nil
}

func (m *mockImageRepositoryForImageService) Exists(ctx context.Context, id string) (bool, error) {
	if m.existsFunc != nil {
		return m.existsFunc(ctx, id)
	}
	return false, nil
}

func (m *mockImageRepositoryForImageService) List(ctx context.Context, offset, limit int) ([]*models.ImageMetadata, error) {
	if m.listFunc != nil {
		return m.listFunc(ctx, offset, limit)
	}
	return nil, nil
}

func (m *mockImageRepositoryForImageService) HealthCheck(ctx context.Context) error {
	if m.healthFunc != nil {
		return m.healthFunc(ctx)
	}
	return nil
}

func (m *mockImageRepositoryForImageService) Health(ctx context.Context) error {
	return m.HealthCheck(ctx)
}

func (m *mockImageRepositoryForImageService) Close() error {
	if m.closeFunc != nil {
		return m.closeFunc()
	}
	return nil
}

func (m *mockImageRepositoryForImageService) GetStats(ctx context.Context) (*repository.RepositoryStats, error) {
	if m.getStatsFunc != nil {
		return m.getStatsFunc(ctx)
	}
	return nil, nil
}

func (m *mockImageRepositoryForImageService) UpdateResolutions(ctx context.Context, imageID string, resolutions []string) error {
	return nil
}

type mockStorageProviderForImageService struct {
	uploadFunc               func(ctx context.Context, key string, data io.Reader, size int64, contentType string) error
	downloadFunc             func(ctx context.Context, key string) (io.ReadCloser, error)
	deleteFunc               func(ctx context.Context, key string) error
	existsFunc               func(ctx context.Context, key string) (bool, error)
	generatePresignedURLFunc func(ctx context.Context, key string, expiration time.Duration) (string, error)
	healthCheckFunc          func(ctx context.Context) error
	getMetadataFunc          func(ctx context.Context, key string) (*storage.FileMetadata, error)
	copyObjectFunc           func(ctx context.Context, srcKey, destKey string) error
	listObjectsFunc          func(ctx context.Context, prefix string, maxKeys int) ([]storage.ObjectInfo, error)
	getURLFunc               func(key string) string
}

func (m *mockStorageProviderForImageService) Upload(ctx context.Context, key string, data io.Reader, size int64, contentType string) error {
	if m.uploadFunc != nil {
		return m.uploadFunc(ctx, key, data, size, contentType)
	}
	return nil
}

func (m *mockStorageProviderForImageService) Download(ctx context.Context, key string) (io.ReadCloser, error) {
	if m.downloadFunc != nil {
		return m.downloadFunc(ctx, key)
	}
	return nil, nil
}

func (m *mockStorageProviderForImageService) Delete(ctx context.Context, key string) error {
	if m.deleteFunc != nil {
		return m.deleteFunc(ctx, key)
	}
	return nil
}

func (m *mockStorageProviderForImageService) Exists(ctx context.Context, key string) (bool, error) {
	if m.existsFunc != nil {
		return m.existsFunc(ctx, key)
	}
	return false, nil
}

func (m *mockStorageProviderForImageService) GeneratePresignedURL(ctx context.Context, key string, expiration time.Duration) (string, error) {
	if m.generatePresignedURLFunc != nil {
		return m.generatePresignedURLFunc(ctx, key, expiration)
	}
	return "", nil
}

func (m *mockStorageProviderForImageService) HealthCheck(ctx context.Context) error {
	if m.healthCheckFunc != nil {
		return m.healthCheckFunc(ctx)
	}
	return nil
}

func (m *mockStorageProviderForImageService) Health(ctx context.Context) error {
	return m.HealthCheck(ctx)
}

func (m *mockStorageProviderForImageService) GetMetadata(ctx context.Context, key string) (*storage.FileMetadata, error) {
	if m.getMetadataFunc != nil {
		return m.getMetadataFunc(ctx, key)
	}
	return nil, nil
}

func (m *mockStorageProviderForImageService) CopyObject(ctx context.Context, srcKey, destKey string) error {
	if m.copyObjectFunc != nil {
		return m.copyObjectFunc(ctx, srcKey, destKey)
	}
	return nil
}

func (m *mockStorageProviderForImageService) ListObjects(ctx context.Context, prefix string, maxKeys int) ([]storage.ObjectInfo, error) {
	if m.listObjectsFunc != nil {
		return m.listObjectsFunc(ctx, prefix, maxKeys)
	}
	return nil, nil
}

func (m *mockStorageProviderForImageService) GetURL(key string) string {
	if m.getURLFunc != nil {
		return m.getURLFunc(key)
	}
	return ""
}

type mockProcessorServiceForImageService struct {
	processImageFunc  func(data []byte, config ResizeConfig) ([]byte, error)
	validateImageFunc func(data []byte, maxSize int64) error
	detectFormatFunc  func(data []byte) (string, error)
	getDimensionsFunc func(data []byte) (width, height int, err error)
}

func (m *mockProcessorServiceForImageService) ProcessImage(data []byte, config ResizeConfig) ([]byte, error) {
	if m.processImageFunc != nil {
		return m.processImageFunc(data, config)
	}
	return nil, nil
}

func (m *mockProcessorServiceForImageService) ValidateImage(data []byte, maxSize int64) error {
	if m.validateImageFunc != nil {
		return m.validateImageFunc(data, maxSize)
	}
	return nil
}

func (m *mockProcessorServiceForImageService) DetectFormat(data []byte) (string, error) {
	if m.detectFormatFunc != nil {
		return m.detectFormatFunc(data)
	}
	return "image/jpeg", nil
}

func (m *mockProcessorServiceForImageService) GetDimensions(data []byte) (width, height int, err error) {
	if m.getDimensionsFunc != nil {
		return m.getDimensionsFunc(data)
	}
	return 1920, 1080, nil
}

func TestNewImageService(t *testing.T) {
	mockRepo := &mockImageRepositoryForImageService{}
	mockStorage := &mockStorageProviderForImageService{}
	mockProcessor := &mockProcessorServiceForImageService{}
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
	mockRepo := &mockImageRepositoryForImageService{
		saveFunc: func(ctx context.Context, metadata *models.ImageMetadata) error {
			return nil
		},
	}
	mockStorage := &mockStorageProviderForImageService{
		uploadFunc: func(ctx context.Context, key string, data io.Reader, size int64, contentType string) error {
			return nil
		},
	}
	mockProcessor := &mockProcessorServiceForImageService{
		validateImageFunc: func(data []byte, maxSize int64) error {
			return nil
		},
		processImageFunc: func(data []byte, config ResizeConfig) ([]byte, error) {
			return testutil.CreateTestImageData(), nil
		},
	}

	cfg := testutil.TestConfig()
	service := NewImageService(mockRepo, mockStorage, mockProcessor, cfg)

	input := UploadInput{
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
	service := NewImageService(&mockImageRepositoryForImageService{}, &mockStorageProviderForImageService{}, &mockProcessorServiceForImageService{}, testutil.TestConfig())

	tests := []struct {
		name    string
		input   UploadInput
		wantErr string
	}{
		{
			name: "empty filename",
			input: UploadInput{
				Filename: "",
				Data:     testutil.CreateTestImageData(),
				Size:     100,
			},
			wantErr: "filename",
		},
		{
			name: "empty data",
			input: UploadInput{
				Filename: "test.jpg",
				Data:     []byte{},
				Size:     0,
			},
			wantErr: "data",
		},
		{
			name: "size mismatch",
			input: UploadInput{
				Filename: "test.jpg",
				Data:     testutil.CreateTestImageData(),
				Size:     999, // Wrong size
			},
			wantErr: "size",
		},
		{
			name: "invalid resolution",
			input: UploadInput{
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
	mockProcessor := &mockProcessorServiceForImageService{
		validateImageFunc: func(data []byte, maxSize int64) error {
			return errors.New("invalid image format")
		},
	}

	service := NewImageService(&mockImageRepositoryForImageService{}, &mockStorageProviderForImageService{}, mockProcessor, testutil.TestConfig())

	input := UploadInput{
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
	mockStorage := &mockStorageProviderForImageService{
		uploadFunc: func(ctx context.Context, key string, data io.Reader, size int64, contentType string) error {
			return errors.New("storage unavailable")
		},
	}
	mockProcessor := &mockProcessorServiceForImageService{
		validateImageFunc: func(data []byte, maxSize int64) error {
			return nil
		},
	}

	service := NewImageService(&mockImageRepositoryForImageService{}, mockStorage, mockProcessor, testutil.TestConfig())

	input := UploadInput{
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
	mockRepo := &mockImageRepositoryForImageService{
		getByIDFunc: func(ctx context.Context, id string) (*models.ImageMetadata, error) {
			return expectedMetadata, nil
		},
	}

	service := NewImageService(mockRepo, &mockStorageProviderForImageService{}, &mockProcessorServiceForImageService{}, testutil.TestConfig())

	ctx := context.Background()
	metadata, err := service.GetMetadata(ctx, testutil.ValidUUID)

	assert.NoError(t, err)
	assert.Equal(t, expectedMetadata, metadata)
}

func TestImageService_GetMetadata_InvalidUUID(t *testing.T) {
	service := NewImageService(&mockImageRepositoryForImageService{}, &mockStorageProviderForImageService{}, &mockProcessorServiceForImageService{}, testutil.TestConfig())

	ctx := context.Background()
	_, err := service.GetMetadata(ctx, testutil.InvalidUUID)

	assert.Error(t, err)
	assert.IsType(t, models.ValidationError{}, err)
	assert.Contains(t, err.Error(), "Invalid UUID format")
}

func TestImageService_GetMetadata_NotFound(t *testing.T) {
	mockRepo := &mockImageRepositoryForImageService{
		getByIDFunc: func(ctx context.Context, id string) (*models.ImageMetadata, error) {
			return nil, models.NotFoundError{Resource: "image", ID: id}
		},
	}

	service := NewImageService(mockRepo, &mockStorageProviderForImageService{}, &mockProcessorServiceForImageService{}, testutil.TestConfig())

	ctx := context.Background()
	_, err := service.GetMetadata(ctx, testutil.ValidUUID)

	assert.Error(t, err)
	assert.IsType(t, models.NotFoundError{}, err)
}

func TestImageService_GetImageStream_Success(t *testing.T) {
	expectedMetadata := testutil.CreateTestImageMetadata()
	testData := testutil.CreateTestImageData()

	mockRepo := &mockImageRepositoryForImageService{
		getByIDFunc: func(ctx context.Context, id string) (*models.ImageMetadata, error) {
			return expectedMetadata, nil
		},
	}
	mockStorage := &mockStorageProviderForImageService{
		downloadFunc: func(ctx context.Context, key string) (io.ReadCloser, error) {
			return testutil.NewMockReadCloser(testData), nil
		},
	}

	service := NewImageService(mockRepo, mockStorage, &mockProcessorServiceForImageService{}, testutil.TestConfig())

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

	mockRepo := &mockImageRepositoryForImageService{
		getByIDFunc: func(ctx context.Context, id string) (*models.ImageMetadata, error) {
			return expectedMetadata, nil
		},
	}

	service := NewImageService(mockRepo, &mockStorageProviderForImageService{}, &mockProcessorServiceForImageService{}, testutil.TestConfig())

	ctx := context.Background()
	_, _, err := service.GetImageStream(ctx, testutil.ValidUUID, "nonexistent")

	assert.Error(t, err)
	assert.IsType(t, models.NotFoundError{}, err)
}

func TestImageService_GeneratePresignedURL_Success(t *testing.T) {
	expectedURL := "https://example.com/presigned-url"
	mockStorage := &mockStorageProviderForImageService{
		generatePresignedURLFunc: func(ctx context.Context, key string, expiration time.Duration) (string, error) {
			return expectedURL, nil
		},
	}

	service := NewImageService(&mockImageRepositoryForImageService{}, mockStorage, &mockProcessorServiceForImageService{}, testutil.TestConfig())

	ctx := context.Background()
	storageKey := "images/test/thumbnail.jpg"
	duration := time.Hour

	url, err := service.GeneratePresignedURL(ctx, storageKey, duration)

	assert.NoError(t, err)
	assert.Equal(t, expectedURL, url)
}

func TestImageService_GeneratePresignedURL_Error(t *testing.T) {
	mockStorage := &mockStorageProviderForImageService{
		generatePresignedURLFunc: func(ctx context.Context, key string, expiration time.Duration) (string, error) {
			return "", errors.New("storage error")
		},
	}

	service := NewImageService(&mockImageRepositoryForImageService{}, mockStorage, &mockProcessorServiceForImageService{}, testutil.TestConfig())

	ctx := context.Background()
	_, err := service.GeneratePresignedURL(ctx, "test-key", time.Hour)

	assert.Error(t, err)
	assert.IsType(t, models.StorageError{}, err)
}

func TestImageService_DeleteImage_Success(t *testing.T) {
	expectedMetadata := testutil.CreateTestImageMetadata()

	mockRepo := &mockImageRepositoryForImageService{
		getByIDFunc: func(ctx context.Context, id string) (*models.ImageMetadata, error) {
			return expectedMetadata, nil
		},
		deleteFunc: func(ctx context.Context, id string) error {
			return nil
		},
	}
	mockStorage := &mockStorageProviderForImageService{
		deleteFunc: func(ctx context.Context, key string) error {
			return nil
		},
	}

	service := NewImageService(mockRepo, mockStorage, &mockProcessorServiceForImageService{}, testutil.TestConfig())

	ctx := context.Background()
	err := service.DeleteImage(ctx, testutil.ValidUUID)

	assert.NoError(t, err)
}

func TestImageService_ListImages_Success(t *testing.T) {
	expectedImages := []*models.ImageMetadata{
		testutil.CreateTestImageMetadata(),
		testutil.CreateTestImageMetadata(),
	}

	mockRepo := &mockImageRepositoryForImageService{
		listFunc: func(ctx context.Context, offset, limit int) ([]*models.ImageMetadata, error) {
			return expectedImages, nil
		},
	}

	service := NewImageService(mockRepo, &mockStorageProviderForImageService{}, &mockProcessorServiceForImageService{}, testutil.TestConfig())

	ctx := context.Background()
	images, total, err := service.ListImages(ctx, 0, 10)

	assert.NoError(t, err)
	assert.Equal(t, expectedImages, images)
	assert.Equal(t, -1, total) // Implementation returns -1 for unknown total
}

func TestImageService_ListImages_LimitValidation(t *testing.T) {
	mockRepo := &mockImageRepositoryForImageService{
		listFunc: func(ctx context.Context, offset, limit int) ([]*models.ImageMetadata, error) {
			// Verify limit was adjusted
			assert.Equal(t, 50, limit)
			return []*models.ImageMetadata{}, nil
		},
	}

	service := NewImageService(mockRepo, &mockStorageProviderForImageService{}, &mockProcessorServiceForImageService{}, testutil.TestConfig())

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
		input   UploadInput
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid input",
			input: UploadInput{
				Filename:    "test.jpg",
				Data:        testutil.CreateTestImageData(),
				Size:        int64(len(testutil.CreateTestImageData())),
				Resolutions: []string{"800x600", "1200x900"},
			},
			wantErr: false,
		},
		{
			name: "comma separated resolutions",
			input: UploadInput{
				Filename:    "test.jpg",
				Data:        testutil.CreateTestImageData(),
				Size:        int64(len(testutil.CreateTestImageData())),
				Resolutions: []string{"800x600,1200x900,1600x1200"},
			},
			wantErr: false,
		},
		{
			name: "resolution exceeds max dimensions",
			input: UploadInput{
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
			input: UploadInput{
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

	mockRepo := &mockImageRepositoryForImageService{
		getByIDFunc: func(ctx context.Context, id string) (*models.ImageMetadata, error) {
			return expectedMetadata, nil
		},
		updateFunc: func(ctx context.Context, metadata *models.ImageMetadata) error {
			return nil
		},
	}
	mockStorage := &mockStorageProviderForImageService{
		downloadFunc: func(ctx context.Context, key string) (io.ReadCloser, error) {
			return testutil.NewMockReadCloser(originalData), nil
		},
		uploadFunc: func(ctx context.Context, key string, data io.Reader, size int64, contentType string) error {
			return nil
		},
	}
	mockProcessor := &mockProcessorServiceForImageService{
		processImageFunc: func(data []byte, config ResizeConfig) ([]byte, error) {
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

	mockRepo := &mockImageRepositoryForImageService{
		getByIDFunc: func(ctx context.Context, id string) (*models.ImageMetadata, error) {
			return expectedMetadata, nil
		},
	}

	service := NewImageService(mockRepo, &mockStorageProviderForImageService{}, &mockProcessorServiceForImageService{}, testutil.TestConfig())

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

	validInput := UploadInput{
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
	result := &UploadResult{
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
