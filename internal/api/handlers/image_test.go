package handlers

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"testing"
	"time"

	"resizr/internal/models"
	"resizr/internal/service"
	"resizr/internal/testutil"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// Local mock to avoid import cycles
type mockImageService struct {
	processUploadFunc        func(ctx context.Context, input service.UploadInput) (*service.UploadResult, error)
	getMetadataFunc          func(ctx context.Context, imageID string) (*models.ImageMetadata, error)
	getImageStreamFunc       func(ctx context.Context, imageID, resolution string) (io.ReadCloser, *models.ImageMetadata, error)
	processResolutionFunc    func(ctx context.Context, imageID, resolution string) error
	generatePresignedURLFunc func(ctx context.Context, storageKey string, expiration time.Duration) (string, error)
	deleteImageFunc          func(ctx context.Context, imageID string) error
	listImagesFunc           func(ctx context.Context, offset, limit int) ([]*models.ImageMetadata, int, error)
}

func (m *mockImageService) ProcessUpload(ctx context.Context, input service.UploadInput) (*service.UploadResult, error) {
	if m.processUploadFunc != nil {
		return m.processUploadFunc(ctx, input)
	}
	return nil, nil
}

func (m *mockImageService) GetMetadata(ctx context.Context, imageID string) (*models.ImageMetadata, error) {
	if m.getMetadataFunc != nil {
		return m.getMetadataFunc(ctx, imageID)
	}
	return nil, nil
}

func (m *mockImageService) GetImageStream(ctx context.Context, imageID, resolution string) (io.ReadCloser, *models.ImageMetadata, error) {
	if m.getImageStreamFunc != nil {
		return m.getImageStreamFunc(ctx, imageID, resolution)
	}
	return nil, nil, nil
}

func (m *mockImageService) ProcessResolution(ctx context.Context, imageID, resolution string) error {
	if m.processResolutionFunc != nil {
		return m.processResolutionFunc(ctx, imageID, resolution)
	}
	return nil
}

func (m *mockImageService) GeneratePresignedURL(ctx context.Context, storageKey string, expiration time.Duration) (string, error) {
	if m.generatePresignedURLFunc != nil {
		return m.generatePresignedURLFunc(ctx, storageKey, expiration)
	}
	return "", nil
}

func (m *mockImageService) DeleteImage(ctx context.Context, imageID string) error {
	if m.deleteImageFunc != nil {
		return m.deleteImageFunc(ctx, imageID)
	}
	return nil
}

func (m *mockImageService) ListImages(ctx context.Context, offset, limit int) ([]*models.ImageMetadata, int, error) {
	if m.listImagesFunc != nil {
		return m.listImagesFunc(ctx, offset, limit)
	}
	return nil, 0, nil
}

func TestImageHandler_Upload(t *testing.T) {
	cfg := testutil.TestConfig()

	tests := []struct {
		name           string
		formData       map[string]string
		fileContent    []byte
		filename       string
		setupMock      func(*mockImageService)
		expectedStatus int
		expectError    bool
	}{
		{
			name:        "successful upload",
			formData:    map[string]string{"resolutions": "800x600,1200x900"},
			fileContent: testutil.CreateTestImageData(),
			filename:    "test.jpg",
			setupMock: func(mock *mockImageService) {
				mock.processUploadFunc = func(ctx context.Context, input service.UploadInput) (*service.UploadResult, error) {
					return &service.UploadResult{
						ImageID:              testutil.ValidUUID,
						ProcessedResolutions: []string{"original", "thumbnail", "preview", "800x600", "1200x900"},
					}, nil
				}
			},
			expectedStatus: http.StatusCreated,
			expectError:    false,
		},
		{
			name:        "upload without resolutions",
			formData:    map[string]string{},
			fileContent: testutil.CreateTestImageData(),
			filename:    "test.jpg",
			setupMock: func(mock *mockImageService) {
				mock.processUploadFunc = func(ctx context.Context, input service.UploadInput) (*service.UploadResult, error) {
					return &service.UploadResult{
						ImageID:              testutil.ValidUUID,
						ProcessedResolutions: []string{"original", "thumbnail", "preview"},
					}, nil
				}
			},
			expectedStatus: http.StatusCreated,
			expectError:    false,
		},
		{
			name:           "file too large",
			formData:       map[string]string{},
			fileContent:    testutil.CreateLargeTestImageData(int(cfg.Image.MaxFileSize + 1)),
			filename:       "large.jpg",
			setupMock:      func(mock *mockImageService) {},
			expectedStatus: http.StatusRequestEntityTooLarge,
			expectError:    true,
		},
		{
			name:        "service processing error",
			formData:    map[string]string{},
			fileContent: testutil.CreateTestImageData(),
			filename:    "test.jpg",
			setupMock: func(mock *mockImageService) {
				mock.processUploadFunc = func(ctx context.Context, input service.UploadInput) (*service.UploadResult, error) {
					return nil, models.ProcessingError{
						Operation: "upload",
						Reason:    "invalid image format",
					}
				}
			},
			expectedStatus: http.StatusUnprocessableEntity,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			mockService := &mockImageService{}
			tt.setupMock(mockService)

			handler := NewImageHandler(mockService, cfg)

			// Create multipart request
			req := testutil.CreateMultipartRequest("POST", "/api/v1/images", tt.formData, "image", tt.filename, tt.fileContent)
			c, w := testutil.SetupTestContext(req)

			// Execute
			handler.Upload(c)

			// Assert
			assert.Equal(t, tt.expectedStatus, w.Code)

			var response map[string]interface{}
			err := testutil.ParseJSONResponse(w, &response)
			assert.NoError(t, err)

			if tt.expectError {
				assert.Contains(t, response, "error")
				assert.Contains(t, response, "message")
			} else {
				assert.Contains(t, response, "id")
				assert.Contains(t, response, "message")
				assert.Contains(t, response, "resolutions")
				assert.Equal(t, testutil.ValidUUID, response["id"])
				assert.Equal(t, "Image uploaded successfully", response["message"])
			}
		})
	}
}

func TestImageHandler_Upload_EdgeCases(t *testing.T) {
	cfg := testutil.TestConfig()
	mockService := &mockImageService{}
	handler := NewImageHandler(mockService, cfg)

	t.Run("no file in request", func(t *testing.T) {
		req := testutil.CreateTestRequest("POST", "/api/v1/images", nil)
		req.Header.Set("Content-Type", "multipart/form-data; boundary=test")
		c, w := testutil.SetupTestContext(req)

		handler.Upload(c)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("invalid form data", func(t *testing.T) {
		req := testutil.CreateTestRequest("POST", "/api/v1/images", nil)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		c, w := testutil.SetupTestContext(req)

		handler.Upload(c)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("multiple resolution fields", func(t *testing.T) {
		formData := map[string]string{
			"resolutions": "800x600",
		}

		mockService.processUploadFunc = func(ctx context.Context, input service.UploadInput) (*service.UploadResult, error) {
			assert.Contains(t, input.Resolutions, "800x600")
			return &service.UploadResult{
				ImageID:              testutil.ValidUUID,
				ProcessedResolutions: []string{"original", "800x600"},
			}, nil
		}

		req := testutil.CreateMultipartRequest("POST", "/api/v1/images", formData, "image", "test.jpg", testutil.CreateTestImageData())
		c, w := testutil.SetupTestContext(req)

		handler.Upload(c)

		assert.Equal(t, http.StatusCreated, w.Code)
	})
}

func TestImageHandler_Info(t *testing.T) {
	tests := []struct {
		name           string
		imageID        string
		setupMock      func(*mockImageService)
		expectedStatus int
		expectError    bool
	}{
		{
			name:    "successful info retrieval",
			imageID: testutil.ValidUUID,
			setupMock: func(mock *mockImageService) {
				mock.getMetadataFunc = func(ctx context.Context, imageID string) (*models.ImageMetadata, error) {
					return testutil.CreateTestImageMetadata(), nil
				}
			},
			expectedStatus: http.StatusOK,
			expectError:    false,
		},
		{
			name:           "invalid UUID",
			imageID:        testutil.InvalidUUID,
			setupMock:      func(mock *mockImageService) {},
			expectedStatus: http.StatusBadRequest,
			expectError:    true,
		},
		{
			name:    "image not found",
			imageID: testutil.ValidUUID,
			setupMock: func(mock *mockImageService) {
				mock.getMetadataFunc = func(ctx context.Context, imageID string) (*models.ImageMetadata, error) {
					return nil, models.NotFoundError{
						Resource: "image",
						ID:       imageID,
					}
				}
			},
			expectedStatus: http.StatusNotFound,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			mockService := &mockImageService{}
			tt.setupMock(mockService)

			handler := NewImageHandler(mockService, testutil.TestConfig())

			// Create test context with URL parameter
			req := testutil.CreateTestRequest("GET", fmt.Sprintf("/api/v1/images/%s/info", tt.imageID), nil)
			c, w := testutil.SetupTestContext(req)
			c.AddParam("id", tt.imageID)

			// Execute
			handler.Info(c)

			// Assert
			assert.Equal(t, tt.expectedStatus, w.Code)

			var response map[string]interface{}
			err := testutil.ParseJSONResponse(w, &response)
			assert.NoError(t, err)

			if tt.expectError {
				assert.Contains(t, response, "error")
				assert.Contains(t, response, "message")
			} else {
				assert.Equal(t, tt.imageID, response["id"])
				assert.Contains(t, response, "filename")
				assert.Contains(t, response, "mime_type")
				assert.Contains(t, response, "dimensions")
				assert.Contains(t, response, "available_resolutions")
			}
		})
	}
}

func TestImageHandler_DownloadMethods(t *testing.T) {
	mockMetadata := testutil.CreateTestImageMetadata()
	testImageData := testutil.CreateTestImageData()

	tests := []struct {
		name       string
		method     func(*ImageHandler, *gin.Context)
		resolution string
	}{
		{"DownloadOriginal", (*ImageHandler).DownloadOriginal, "original"},
		{"DownloadThumbnail", (*ImageHandler).DownloadThumbnail, "thumbnail"},
		{"DownloadPreview", (*ImageHandler).DownloadPreview, "preview"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := &mockImageService{
				getImageStreamFunc: func(ctx context.Context, imageID, resolution string) (io.ReadCloser, *models.ImageMetadata, error) {
					assert.Equal(t, testutil.ValidUUID, imageID)
					assert.Equal(t, tt.resolution, resolution)
					return testutil.NewMockReadCloser(testImageData), mockMetadata, nil
				},
			}

			handler := NewImageHandler(mockService, testutil.TestConfig())

			req := testutil.CreateTestRequest("GET", fmt.Sprintf("/api/v1/images/%s/%s", testutil.ValidUUID, tt.resolution), nil)
			c, w := testutil.SetupTestContext(req)
			c.AddParam("id", testutil.ValidUUID)

			tt.method(handler, c)

			assert.Equal(t, http.StatusOK, w.Code)
			assert.Equal(t, mockMetadata.MimeType, w.Header().Get("Content-Type"))
			assert.Contains(t, w.Header().Get("Cache-Control"), "public")
			assert.NotEmpty(t, w.Header().Get("ETag"))
		})
	}
}

func TestImageHandler_DownloadCustomResolution(t *testing.T) {
	tests := []struct {
		name           string
		resolution     string
		setupMock      func(*mockImageService)
		expectedStatus int
		expectError    bool
	}{
		{
			name:       "valid custom resolution",
			resolution: "800x600",
			setupMock: func(mock *mockImageService) {
				mock.getImageStreamFunc = func(ctx context.Context, imageID, resolution string) (io.ReadCloser, *models.ImageMetadata, error) {
					return testutil.NewMockReadCloser(testutil.CreateTestImageData()), testutil.CreateTestImageMetadata(), nil
				}
			},
			expectedStatus: http.StatusOK,
			expectError:    false,
		},
		{
			name:           "invalid resolution format",
			resolution:     "invalid",
			setupMock:      func(mock *mockImageService) {},
			expectedStatus: http.StatusBadRequest,
			expectError:    true,
		},
		{
			name:       "service error",
			resolution: "800x600",
			setupMock: func(mock *mockImageService) {
				mock.getImageStreamFunc = func(ctx context.Context, imageID, resolution string) (io.ReadCloser, *models.ImageMetadata, error) {
					return nil, nil, models.NotFoundError{Resource: "image", ID: imageID}
				}
			},
			expectedStatus: http.StatusNotFound,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := &mockImageService{}
			tt.setupMock(mockService)

			handler := NewImageHandler(mockService, testutil.TestConfig())

			req := testutil.CreateTestRequest("GET", fmt.Sprintf("/api/v1/images/%s/%s", testutil.ValidUUID, tt.resolution), nil)
			c, w := testutil.SetupTestContext(req)
			c.AddParam("id", testutil.ValidUUID)
			c.AddParam("resolution", tt.resolution)

			handler.DownloadCustomResolution(c)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectError && w.Code >= 400 {
				var response map[string]interface{}
				err := testutil.ParseJSONResponse(w, &response)
				assert.NoError(t, err)
				assert.Contains(t, response, "error")
			}
		})
	}
}

func TestImageHandler_GeneratePresignedURL(t *testing.T) {
	tests := []struct {
		name           string
		imageID        string
		resolution     string
		expiresIn      string
		setupMock      func(*mockImageService)
		expectedStatus int
		expectError    bool
	}{
		{
			name:       "successful presigned URL generation",
			imageID:    testutil.ValidUUID,
			resolution: "thumbnail",
			expiresIn:  "3600",
			setupMock: func(mock *mockImageService) {
				mockMetadata := testutil.CreateTestImageMetadata()
				mock.getMetadataFunc = func(ctx context.Context, imageID string) (*models.ImageMetadata, error) {
					return mockMetadata, nil
				}
				mock.generatePresignedURLFunc = func(ctx context.Context, storageKey string, expiration time.Duration) (string, error) {
					return "https://example.com/presigned-url", nil
				}
			},
			expectedStatus: http.StatusOK,
			expectError:    false,
		},
		{
			name:           "invalid UUID",
			imageID:        testutil.InvalidUUID,
			resolution:     "thumbnail",
			setupMock:      func(mock *mockImageService) {},
			expectedStatus: http.StatusBadRequest,
			expectError:    true,
		},
		{
			name:           "invalid expires_in",
			imageID:        testutil.ValidUUID,
			resolution:     "thumbnail",
			expiresIn:      "invalid",
			setupMock:      func(mock *mockImageService) {},
			expectedStatus: http.StatusBadRequest,
			expectError:    true,
		},
		{
			name:           "expires_in too large",
			imageID:        testutil.ValidUUID,
			resolution:     "thumbnail",
			expiresIn:      strconv.Itoa(8 * 24 * 3600), // 8 days
			setupMock:      func(mock *mockImageService) {},
			expectedStatus: http.StatusBadRequest,
			expectError:    true,
		},
		{
			name:       "image not found",
			imageID:    testutil.ValidUUID,
			resolution: "thumbnail",
			expiresIn:  "3600",
			setupMock: func(mock *mockImageService) {
				mock.getMetadataFunc = func(ctx context.Context, imageID string) (*models.ImageMetadata, error) {
					return nil, models.NotFoundError{Resource: "image", ID: imageID}
				}
			},
			expectedStatus: http.StatusNotFound,
			expectError:    true,
		},
		{
			name:       "resolution not available",
			imageID:    testutil.ValidUUID,
			resolution: "nonexistent",
			expiresIn:  "3600",
			setupMock: func(mock *mockImageService) {
				mockMetadata := testutil.CreateTestImageMetadata()
				mock.getMetadataFunc = func(ctx context.Context, imageID string) (*models.ImageMetadata, error) {
					return mockMetadata, nil
				}
			},
			expectedStatus: http.StatusNotFound,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := &mockImageService{}
			tt.setupMock(mockService)

			handler := NewImageHandler(mockService, testutil.TestConfig())

			url := fmt.Sprintf("/api/v1/images/%s/%s/presigned-url", tt.imageID, tt.resolution)
			if tt.expiresIn != "" {
				url += "?expires_in=" + tt.expiresIn
			}

			req := testutil.CreateTestRequest("GET", url, nil)
			c, w := testutil.SetupTestContext(req)
			c.AddParam("id", tt.imageID)
			c.AddParam("resolution", tt.resolution)

			handler.GeneratePresignedURL(c)

			assert.Equal(t, tt.expectedStatus, w.Code)

			var response map[string]interface{}
			err := testutil.ParseJSONResponse(w, &response)
			assert.NoError(t, err)

			if tt.expectError {
				assert.Contains(t, response, "error")
				assert.Contains(t, response, "message")
			} else {
				assert.Contains(t, response, "url")
				assert.Contains(t, response, "expires_at")
				assert.Contains(t, response, "expires_in")
			}
		})
	}
}

func TestImageHandler_ValidationHelpers(t *testing.T) {
	handler := &ImageHandler{}

	// Test UUID validation
	assert.True(t, handler.isValidUUID(testutil.ValidUUID))
	assert.False(t, handler.isValidUUID(testutil.InvalidUUID))
	assert.False(t, handler.isValidUUID(""))
	assert.False(t, handler.isValidUUID("too-short"))

	// Test custom resolution validation
	assert.True(t, handler.isValidCustomResolution("800x600"))
	assert.True(t, handler.isValidCustomResolution("1920x1080"))
	assert.False(t, handler.isValidCustomResolution("800"))
	assert.False(t, handler.isValidCustomResolution("800x"))
	assert.False(t, handler.isValidCustomResolution("x600"))
	assert.False(t, handler.isValidCustomResolution("800X600"))
	assert.False(t, handler.isValidCustomResolution("abc x def"))

	// Test size validation
	assert.True(t, handler.isValidSize("original"))
	assert.True(t, handler.isValidSize("thumbnail"))
	assert.True(t, handler.isValidSize("preview"))
	assert.True(t, handler.isValidSize("800x600"))
	assert.False(t, handler.isValidSize("invalid"))
}

func TestImageHandler_FilenameGeneration(t *testing.T) {
	handler := &ImageHandler{}

	tests := []struct {
		originalFilename string
		resolution       string
		expectedFilename string
	}{
		{"test.jpg", "original", "test.jpg"},
		{"test.jpg", "thumbnail", "test_thumbnail.jpg"},
		{"test.jpg", "800x600", "test_800x600.jpg"},
		{"image.png", "preview", "image_preview.png"},
		{"noext", "thumbnail", "noext_thumbnail.jpg"},
	}

	for _, tt := range tests {
		result := handler.generateDownloadFilename(tt.originalFilename, tt.resolution)
		assert.Equal(t, tt.expectedFilename, result)
	}
}

func TestImageHandler_ErrorHandling(t *testing.T) {
	handler := &ImageHandler{}

	tests := []struct {
		name         string
		err          error
		expectedCode int
	}{
		{
			"validation error",
			models.ValidationError{Field: "test", Message: "invalid"},
			http.StatusBadRequest,
		},
		{
			"not found error",
			models.NotFoundError{Resource: "image", ID: "123"},
			http.StatusNotFound,
		},
		{
			"processing error",
			models.ProcessingError{Operation: "resize", Reason: "invalid format"},
			http.StatusUnprocessableEntity,
		},
		{
			"storage error",
			models.StorageError{Operation: "upload", Backend: "s3", Reason: "connection failed"},
			http.StatusServiceUnavailable,
		},
		{
			"unknown error",
			errors.New("unknown error"),
			http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := testutil.CreateTestRequest("GET", "/test", nil)
			c, w := testutil.SetupTestContext(req)

			handler.handleServiceError(c, tt.err, "test-request-id", "test operation")

			assert.Equal(t, tt.expectedCode, w.Code)

			var response map[string]interface{}
			err := testutil.ParseJSONResponse(w, &response)
			assert.NoError(t, err)
			assert.Contains(t, response, "error")
			assert.Contains(t, response, "message")
			assert.Equal(t, float64(tt.expectedCode), response["code"])
		})
	}
}

func TestNewImageHandler(t *testing.T) {
	mockService := &mockImageService{}
	cfg := testutil.TestConfig()

	handler := NewImageHandler(mockService, cfg)

	assert.NotNil(t, handler)
	assert.Equal(t, mockService, handler.imageService)
	assert.Equal(t, cfg, handler.config)
}
