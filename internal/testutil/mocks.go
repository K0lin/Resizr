package testutil

import (
	"context"
	"io"
	"time"

	"resizr/internal/models"
	"resizr/internal/repository"
	"resizr/internal/service"
	"resizr/internal/storage"
)

// ServiceUploadInput represents input for image upload (matches service.UploadInput)
type ServiceUploadInput struct {
	Filename    string   `json:"filename"`
	Data        []byte   `json:"-"`
	Size        int64    `json:"size"`
	Resolutions []string `json:"resolutions"`
}

// ServiceUploadResult represents the result of image upload (matches service.UploadResult)
type ServiceUploadResult struct {
	ImageID              string           `json:"image_id"`
	ProcessedResolutions []string         `json:"processed_resolutions"`
	OriginalSize         int64            `json:"original_size"`
	ProcessedSizes       map[string]int64 `json:"processed_sizes"`
}

// ...existing code...

// HealthStatus represents health check status - duplicated to avoid import cycles
type HealthStatus struct {
	Services map[string]string `json:"services"`
	Uptime   int64             `json:"uptime_seconds"`
	Version  string            `json:"version"`
}

// MockImageService is a mock implementation of ImageService
type MockImageService struct {
	ProcessUploadFunc        func(ctx context.Context, input interface{}) (interface{}, error)
	GetMetadataFunc          func(ctx context.Context, imageID string) (*models.ImageMetadata, error)
	GetImageStreamFunc       func(ctx context.Context, imageID, resolution string) (io.ReadCloser, *models.ImageMetadata, error)
	ProcessResolutionFunc    func(ctx context.Context, imageID, resolution string) error
	GeneratePresignedURLFunc func(ctx context.Context, storageKey string, expiration time.Duration) (string, error)
	DeleteImageFunc          func(ctx context.Context, imageID string) error
	ListImagesFunc           func(ctx context.Context, offset, limit int) ([]*models.ImageMetadata, int, error)
}

func (m *MockImageService) ProcessUpload(ctx context.Context, input interface{}) (interface{}, error) {
	if m.ProcessUploadFunc != nil {
		return m.ProcessUploadFunc(ctx, input)
	}
	return nil, nil
}

func (m *MockImageService) GetMetadata(ctx context.Context, imageID string) (*models.ImageMetadata, error) {
	if m.GetMetadataFunc != nil {
		return m.GetMetadataFunc(ctx, imageID)
	}
	return nil, nil
}

func (m *MockImageService) GetImageStream(ctx context.Context, imageID, resolution string) (io.ReadCloser, *models.ImageMetadata, error) {
	if m.GetImageStreamFunc != nil {
		return m.GetImageStreamFunc(ctx, imageID, resolution)
	}
	return nil, nil, nil
}

func (m *MockImageService) ProcessResolution(ctx context.Context, imageID, resolution string) error {
	if m.ProcessResolutionFunc != nil {
		return m.ProcessResolutionFunc(ctx, imageID, resolution)
	}
	return nil
}

func (m *MockImageService) GeneratePresignedURL(ctx context.Context, storageKey string, expiration time.Duration) (string, error) {
	if m.GeneratePresignedURLFunc != nil {
		return m.GeneratePresignedURLFunc(ctx, storageKey, expiration)
	}
	return "", nil
}

func (m *MockImageService) DeleteImage(ctx context.Context, imageID string) error {
	if m.DeleteImageFunc != nil {
		return m.DeleteImageFunc(ctx, imageID)
	}
	return nil
}

func (m *MockImageService) ListImages(ctx context.Context, offset, limit int) ([]*models.ImageMetadata, int, error) {
	if m.ListImagesFunc != nil {
		return m.ListImagesFunc(ctx, offset, limit)
	}
	return nil, 0, nil
}

// ServiceHealthStatus represents health check status (matches service.HealthStatus)
type ServiceHealthStatus struct {
	Services map[string]string `json:"services"`
	Uptime   int64             `json:"uptime_seconds"`
	Version  string            `json:"version"`
}

// MockHealthService is a mock implementation of HealthService
type MockHealthService struct {
	CheckHealthFunc func(ctx context.Context) (interface{}, error)
	GetMetricsFunc  func(ctx context.Context) (map[string]interface{}, error)
}

func (m *MockHealthService) CheckHealth(ctx context.Context) (interface{}, error) {
	if m.CheckHealthFunc != nil {
		return m.CheckHealthFunc(ctx)
	}
	return nil, nil
}

func (m *MockHealthService) GetMetrics(ctx context.Context) (map[string]interface{}, error) {
	if m.GetMetricsFunc != nil {
		return m.GetMetricsFunc(ctx)
	}
	return nil, nil
}

// MockImageRepository is a mock implementation of ImageRepository
type MockImageRepository struct {
	SaveFunc        func(ctx context.Context, metadata *models.ImageMetadata) error
	GetByIDFunc     func(ctx context.Context, id string) (*models.ImageMetadata, error)
	UpdateFunc      func(ctx context.Context, metadata *models.ImageMetadata) error
	DeleteFunc      func(ctx context.Context, id string) error
	ExistsFunc      func(ctx context.Context, id string) (bool, error)
	HealthCheckFunc func(ctx context.Context) error
	GetFunc         func(ctx context.Context, id string) (*models.ImageMetadata, error)
	StoreFunc       func(ctx context.Context, metadata *models.ImageMetadata) error
	ListFunc        func(ctx context.Context, offset, limit int) ([]*models.ImageMetadata, error)
	HealthFunc      func(ctx context.Context) error
	CloseFunc       func() error
	GetStatsFunc    func(ctx context.Context) (*repository.RepositoryStats, error)
}

func (m *MockImageRepository) Save(ctx context.Context, metadata *models.ImageMetadata) error {
	if m.SaveFunc != nil {
		return m.SaveFunc(ctx, metadata)
	}
	return nil
}

func (m *MockImageRepository) GetByID(ctx context.Context, id string) (*models.ImageMetadata, error) {
	if m.GetByIDFunc != nil {
		return m.GetByIDFunc(ctx, id)
	}
	return nil, nil
}

func (m *MockImageRepository) Get(ctx context.Context, id string) (*models.ImageMetadata, error) {
	if m.GetFunc != nil {
		return m.GetFunc(ctx, id)
	}
	if m.GetByIDFunc != nil {
		return m.GetByIDFunc(ctx, id)
	}
	return nil, nil
}

func (m *MockImageRepository) Store(ctx context.Context, metadata *models.ImageMetadata) error {
	if m.StoreFunc != nil {
		return m.StoreFunc(ctx, metadata)
	}
	if m.SaveFunc != nil {
		return m.SaveFunc(ctx, metadata)
	}
	return nil
}

func (m *MockImageRepository) Update(ctx context.Context, metadata *models.ImageMetadata) error {
	if m.UpdateFunc != nil {
		return m.UpdateFunc(ctx, metadata)
	}
	return nil
}

func (m *MockImageRepository) Delete(ctx context.Context, id string) error {
	if m.DeleteFunc != nil {
		return m.DeleteFunc(ctx, id)
	}
	return nil
}

func (m *MockImageRepository) Exists(ctx context.Context, id string) (bool, error) {
	if m.ExistsFunc != nil {
		return m.ExistsFunc(ctx, id)
	}
	return false, nil
}

func (m *MockImageRepository) List(ctx context.Context, offset, limit int) ([]*models.ImageMetadata, error) {
	if m.ListFunc != nil {
		return m.ListFunc(ctx, offset, limit)
	}
	return nil, nil
}

func (m *MockImageRepository) HealthCheck(ctx context.Context) error {
	if m.HealthCheckFunc != nil {
		return m.HealthCheckFunc(ctx)
	}
	if m.HealthFunc != nil {
		return m.HealthFunc(ctx)
	}
	return nil
}

func (m *MockImageRepository) Health(ctx context.Context) error {
	return m.HealthCheck(ctx)
}

func (m *MockImageRepository) Close() error {
	if m.CloseFunc != nil {
		return m.CloseFunc()
	}
	return nil
}

func (m *MockImageRepository) GetStats(ctx context.Context) (*repository.RepositoryStats, error) {
	if m.GetStatsFunc != nil {
		return m.GetStatsFunc(ctx)
	}
	return nil, nil
}

func (m *MockImageRepository) UpdateResolutions(ctx context.Context, id string, resolutions []string) error {
	// Simple implementation for mock
	return nil
}

// MockStorageProvider is a mock implementation of StorageProvider
type MockStorageProvider struct {
	UploadFunc               func(ctx context.Context, key string, data io.Reader, contentType string) error
	DownloadFunc             func(ctx context.Context, key string) (io.ReadCloser, error)
	DeleteFunc               func(ctx context.Context, key string) error
	ExistsFunc               func(ctx context.Context, key string) (bool, error)
	GeneratePresignedURLFunc func(ctx context.Context, key string, expiration time.Duration) (string, error)
	HealthCheckFunc          func(ctx context.Context) error
	HealthFunc               func(ctx context.Context) error
	GetMetadataFunc          func(ctx context.Context, key string) (*storage.FileMetadata, error)
	CopyObjectFunc           func(ctx context.Context, srcKey, destKey string) error
}

func (m *MockStorageProvider) Upload(ctx context.Context, key string, data io.Reader, _size int64, contentType string) error {
	if m.UploadFunc != nil {
		return m.UploadFunc(ctx, key, data, contentType)
	}
	return nil
}

func (m *MockStorageProvider) Download(ctx context.Context, key string) (io.ReadCloser, error) {
	if m.DownloadFunc != nil {
		return m.DownloadFunc(ctx, key)
	}
	return nil, nil
}

func (m *MockStorageProvider) Delete(ctx context.Context, key string) error {
	if m.DeleteFunc != nil {
		return m.DeleteFunc(ctx, key)
	}
	return nil
}

func (m *MockStorageProvider) Exists(ctx context.Context, key string) (bool, error) {
	if m.ExistsFunc != nil {
		return m.ExistsFunc(ctx, key)
	}
	return false, nil
}

func (m *MockStorageProvider) GeneratePresignedURL(ctx context.Context, key string, expiration time.Duration) (string, error) {
	if m.GeneratePresignedURLFunc != nil {
		return m.GeneratePresignedURLFunc(ctx, key, expiration)
	}
	return "", nil
}

func (m *MockStorageProvider) HealthCheck(ctx context.Context) error {
	if m.HealthCheckFunc != nil {
		return m.HealthCheckFunc(ctx)
	}
	if m.HealthFunc != nil {
		return m.HealthFunc(ctx)
	}
	return nil
}

func (m *MockStorageProvider) Health(ctx context.Context) error {
	return m.HealthCheck(ctx)
}

func (m *MockStorageProvider) GetMetadata(ctx context.Context, key string) (*storage.FileMetadata, error) {
	if m.GetMetadataFunc != nil {
		return m.GetMetadataFunc(ctx, key)
	}
	return nil, nil
}

func (m *MockStorageProvider) CopyObject(ctx context.Context, srcKey, destKey string) error {
	if m.CopyObjectFunc != nil {
		return m.CopyObjectFunc(ctx, srcKey, destKey)
	}
	return nil
}

func (m *MockStorageProvider) DeleteFolder(ctx context.Context, prefix string) error {
	// Simple implementation for mock
	return nil
}

func (m *MockStorageProvider) ListObjects(ctx context.Context, prefix string, maxKeys int) ([]storage.ObjectInfo, error) {
	// Simple implementation for mock
	return []storage.ObjectInfo{}, nil
}

func (m *MockStorageProvider) GetURL(key string) string {
	// Simple implementation for mock
	return "http://mock-url/" + key
}

// MockProcessorService is a mock implementation of ProcessorService
type MockProcessorService struct {
	ProcessImageFunc  func(data []byte, config service.ResizeConfig) ([]byte, error)
	ValidateImageFunc func(data []byte, maxSize int64) error
	DetectFormatFunc  func(data []byte) (string, error)
	GetDimensionsFunc func(data []byte) (width, height int, err error)
}

func (m *MockProcessorService) ProcessImage(data []byte, config service.ResizeConfig) ([]byte, error) {
	if m.ProcessImageFunc != nil {
		return m.ProcessImageFunc(data, config)
	}
	return nil, nil
}

func (m *MockProcessorService) ValidateImage(data []byte, maxSize int64) error {
	if m.ValidateImageFunc != nil {
		return m.ValidateImageFunc(data, maxSize)
	}
	return nil
}

func (m *MockProcessorService) DetectFormat(data []byte) (string, error) {
	if m.DetectFormatFunc != nil {
		return m.DetectFormatFunc(data)
	}
	return "image/jpeg", nil
}

func (m *MockProcessorService) GetDimensions(data []byte) (width, height int, err error) {
	if m.GetDimensionsFunc != nil {
		return m.GetDimensionsFunc(data)
	}
	return 1920, 1080, nil
}

// MockDeduplicationRepository is a mock implementation of DeduplicationRepository
type MockDeduplicationRepository struct {
	StoreDeduplicationInfoFunc  func(ctx context.Context, info *models.DeduplicationInfo) error
	GetDeduplicationInfoFunc    func(ctx context.Context, hash models.ImageHash) (*models.DeduplicationInfo, error)
	UpdateDeduplicationInfoFunc func(ctx context.Context, info *models.DeduplicationInfo) error
	DeleteDeduplicationInfoFunc func(ctx context.Context, hash models.ImageHash) error
	FindImageByHashFunc         func(ctx context.Context, hash models.ImageHash) (*models.DeduplicationInfo, error)
	AddHashReferenceFunc        func(ctx context.Context, hash models.ImageHash, imageID string) error
	RemoveHashReferenceFunc     func(ctx context.Context, hash models.ImageHash, imageID string) error
	GetOrphanedHashesFunc       func(ctx context.Context) ([]models.ImageHash, error)
}

func (m *MockDeduplicationRepository) StoreDeduplicationInfo(ctx context.Context, info *models.DeduplicationInfo) error {
	if m.StoreDeduplicationInfoFunc != nil {
		return m.StoreDeduplicationInfoFunc(ctx, info)
	}
	return nil
}

func (m *MockDeduplicationRepository) GetDeduplicationInfo(ctx context.Context, hash models.ImageHash) (*models.DeduplicationInfo, error) {
	if m.GetDeduplicationInfoFunc != nil {
		return m.GetDeduplicationInfoFunc(ctx, hash)
	}
	return nil, nil
}

func (m *MockDeduplicationRepository) UpdateDeduplicationInfo(ctx context.Context, info *models.DeduplicationInfo) error {
	if m.UpdateDeduplicationInfoFunc != nil {
		return m.UpdateDeduplicationInfoFunc(ctx, info)
	}
	return nil
}

func (m *MockDeduplicationRepository) DeleteDeduplicationInfo(ctx context.Context, hash models.ImageHash) error {
	if m.DeleteDeduplicationInfoFunc != nil {
		return m.DeleteDeduplicationInfoFunc(ctx, hash)
	}
	return nil
}

func (m *MockDeduplicationRepository) FindImageByHash(ctx context.Context, hash models.ImageHash) (*models.DeduplicationInfo, error) {
	if m.FindImageByHashFunc != nil {
		return m.FindImageByHashFunc(ctx, hash)
	}
	return m.GetDeduplicationInfo(ctx, hash)
}

func (m *MockDeduplicationRepository) AddHashReference(ctx context.Context, hash models.ImageHash, imageID string) error {
	if m.AddHashReferenceFunc != nil {
		return m.AddHashReferenceFunc(ctx, hash, imageID)
	}
	return nil
}

func (m *MockDeduplicationRepository) RemoveHashReference(ctx context.Context, hash models.ImageHash, imageID string) error {
	if m.RemoveHashReferenceFunc != nil {
		return m.RemoveHashReferenceFunc(ctx, hash, imageID)
	}
	return nil
}

func (m *MockDeduplicationRepository) GetOrphanedHashes(ctx context.Context) ([]models.ImageHash, error) {
	if m.GetOrphanedHashesFunc != nil {
		return m.GetOrphanedHashesFunc(ctx)
	}
	return []models.ImageHash{}, nil
}
