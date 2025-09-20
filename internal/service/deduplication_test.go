package service

import (
	"context"
	"testing"

	"resizr/internal/config"
	"resizr/internal/models"
	"resizr/internal/testutil"

	"github.com/stretchr/testify/assert"
)

// testConfig returns a test configuration without import cycle
func testConfig() *config.Config {
	return &config.Config{
		Image: config.ImageConfig{
			MaxFileSize:                10485760, // 10MB
			Quality:                    85,
			GenerateDefaultResolutions: true,
			ResizeMode:                 "smart_fit",
			MaxWidth:                   4096,
			MaxHeight:                  4096,
		},
	}
}

// testProcessorService implements ProcessorService for testing
type testProcessorService struct{}

func (t *testProcessorService) ProcessImage(data []byte, config ResizeConfig) ([]byte, error) {
	return data, nil
}

func (t *testProcessorService) ValidateImage(data []byte, maxSize int64) error {
	return nil
}

func (t *testProcessorService) DetectFormat(data []byte) (string, error) {
	return "image/jpeg", nil
}

func (t *testProcessorService) GetDimensions(data []byte) (width, height int, err error) {
	return 1920, 1080, nil
}

// TestDeduplicationInfo_ResolutionReferenceTracking tests the resolution reference tracking functionality
func TestDeduplicationInfo_ResolutionReferenceTracking(t *testing.T) {
	t.Run("add_resolution_reference", func(t *testing.T) {
		hash := models.ImageHash{
			Value:     "test-hash",
			Algorithm: "SHA256",
			Size:      1024,
		}
		dedupInfo := models.NewDeduplicationInfo(hash, "550e8400-e29b-41d4-a716-446655440000", "images/550e8400-e29b-41d4-a716-446655440000/original.jpg")

		// Add resolution reference
		dedupInfo.AddResolutionReference("800x600", "user-image-1")
		dedupInfo.AddResolutionReference("800x600", "user-image-2")
		dedupInfo.AddResolutionReference("100x100", "user-image-1")

		// Verify references
		assert.Equal(t, 2, dedupInfo.GetResolutionReferenceCount("800x600"))
		assert.Equal(t, 1, dedupInfo.GetResolutionReferenceCount("100x100"))
		assert.Equal(t, 0, dedupInfo.GetResolutionReferenceCount("1920x1080"))
	})

	t.Run("remove_resolution_reference", func(t *testing.T) {
		hash := models.ImageHash{
			Value:     "test-hash",
			Algorithm: "SHA256",
			Size:      1024,
		}
		dedupInfo := models.NewDeduplicationInfo(hash, "550e8400-e29b-41d4-a716-446655440000", "images/550e8400-e29b-41d4-a716-446655440000/original.jpg")

		// Add references
		dedupInfo.AddResolutionReference("800x600", "user-image-1")
		dedupInfo.AddResolutionReference("800x600", "user-image-2")
		dedupInfo.AddResolutionReference("100x100", "user-image-1")

		// Remove one reference
		dedupInfo.RemoveResolutionReference("800x600", "user-image-1")

		// Verify remaining references
		assert.Equal(t, 1, dedupInfo.GetResolutionReferenceCount("800x600"))
		assert.Equal(t, 1, dedupInfo.GetResolutionReferenceCount("100x100"))
	})

	t.Run("remove_last_resolution_reference", func(t *testing.T) {
		hash := models.ImageHash{
			Value:     "test-hash",
			Algorithm: "SHA256",
			Size:      1024,
		}
		dedupInfo := models.NewDeduplicationInfo(hash, "550e8400-e29b-41d4-a716-446655440000", "images/550e8400-e29b-41d4-a716-446655440000/original.jpg")

		// Add single reference
		dedupInfo.AddResolutionReference("100x100", "user-image-1")

		// Remove last reference
		dedupInfo.RemoveResolutionReference("100x100", "user-image-1")

		// Verify no references remain
		assert.Equal(t, 0, dedupInfo.GetResolutionReferenceCount("100x100"))
	})

	t.Run("remove_nonexistent_reference", func(t *testing.T) {
		hash := models.ImageHash{
			Value:     "test-hash",
			Algorithm: "SHA256",
			Size:      1024,
		}
		dedupInfo := models.NewDeduplicationInfo(hash, "550e8400-e29b-41d4-a716-446655440000", "images/550e8400-e29b-41d4-a716-446655440000/original.jpg")

		// Try to remove non-existent reference
		assert.NotPanics(t, func() {
			dedupInfo.RemoveResolutionReference("800x600", "nonexistent-image")
		})

		// Verify no change
		assert.Equal(t, 0, dedupInfo.GetResolutionReferenceCount("800x600"))
	})
}

// TestImageService_DeduplicationCleanup tests the deduplication cleanup logic
func TestImageService_DeduplicationCleanup(t *testing.T) {
	t.Run("deduplicated_image_cleanup", func(t *testing.T) {
		mockRepo := &testutil.MockImageRepository{
			GetFunc: func(ctx context.Context, id string) (*models.ImageMetadata, error) {
				return &models.ImageMetadata{
					ID:            id,
					Hash:          models.ImageHash{Value: "test-hash", Algorithm: "SHA256", Size: 1024},
					IsDeduped:     true,
					SharedImageID: "550e8400-e29b-41d4-a716-446655440000",
					Resolutions:   []string{"original", "800x600", "100x100"},
				}, nil
			},
			DeleteFunc: func(ctx context.Context, id string) error {
				return nil
			},
		}

		mockDeduplicationRepo := &testutil.MockDeduplicationRepository{
			GetDeduplicationInfoFunc: func(ctx context.Context, hash models.ImageHash) (*models.DeduplicationInfo, error) {
				return &models.DeduplicationInfo{
					MasterImageID:  "550e8400-e29b-41d4-a716-446655440000",
					Hash:           models.ImageHash{Value: "test-hash", Algorithm: "SHA256", Size: 1024},
					ReferencingIDs: []string{"550e8400-e29b-41d4-a716-446655440003"}, // Still has other references
					ResolutionRefs: map[string]*models.ResolutionReference{
						"800x600": {ReferencingIDs: []string{"550e8400-e29b-41d4-a716-446655440003"}}, // 800x600 still used by others
						"100x100": {ReferencingIDs: []string{"f47ac10b-58cc-4372-a567-0e02b2c3d479"}}, // 100x100 only used by this image
					},
				}, nil
			},
			UpdateDeduplicationInfoFunc: func(ctx context.Context, info *models.DeduplicationInfo) error {
				return nil
			},
		}

		mockStorage := &testutil.MockStorageProvider{
			DeleteFunc: func(ctx context.Context, key string) error {
				return nil
			},
		}

		mockProcessor := &testProcessorService{}

		service := NewImageService(mockRepo, mockDeduplicationRepo, mockStorage, mockProcessor, testConfig())

		// Execute deletion
		err := service.DeleteImage(context.Background(), "f47ac10b-58cc-4372-a567-0e02b2c3d479")

		// Verify results
		assert.NoError(t, err)
	})

	t.Run("last_reference_cleanup", func(t *testing.T) {
		mockRepo := &testutil.MockImageRepository{
			GetFunc: func(ctx context.Context, id string) (*models.ImageMetadata, error) {
				return &models.ImageMetadata{
					ID:            id,
					Hash:          models.ImageHash{Value: "test-hash", Algorithm: "SHA256", Size: 1024},
					IsDeduped:     true,
					SharedImageID: "550e8400-e29b-41d4-a716-446655440000",
					Resolutions:   []string{"original", "800x600"},
				}, nil
			},
			DeleteFunc: func(ctx context.Context, id string) error {
				return nil
			},
		}

		mockDeduplicationRepo := &testutil.MockDeduplicationRepository{
			GetDeduplicationInfoFunc: func(ctx context.Context, hash models.ImageHash) (*models.DeduplicationInfo, error) {
				return &models.DeduplicationInfo{
					MasterImageID:  "550e8400-e29b-41d4-a716-446655440000",
					Hash:           models.ImageHash{Value: "test-hash", Algorithm: "SHA256", Size: 1024},
					ReferencingIDs: []string{"f47ac10b-58cc-4372-a567-0e02b2c3d479"}, // This is the last reference
					ResolutionRefs: map[string]*models.ResolutionReference{
						"original": {ReferencingIDs: []string{"f47ac10b-58cc-4372-a567-0e02b2c3d479"}},
						"800x600":  {ReferencingIDs: []string{"f47ac10b-58cc-4372-a567-0e02b2c3d479"}},
					},
				}, nil
			},
			DeleteDeduplicationInfoFunc: func(ctx context.Context, hash models.ImageHash) error {
				return nil
			},
		}

		mockStorage := &testutil.MockStorageProvider{
			DeleteFunc: func(ctx context.Context, key string) error {
				return nil
			},
		}

		mockProcessor := &testProcessorService{}

		service := NewImageService(mockRepo, mockDeduplicationRepo, mockStorage, mockProcessor, testConfig())

		// Execute deletion
		err := service.DeleteImage(context.Background(), "f47ac10b-58cc-4372-a567-0e02b2c3d479")

		// Verify results
		assert.NoError(t, err)
	})

	t.Run("non_deduplicated_image_cleanup", func(t *testing.T) {
		mockRepo := &testutil.MockImageRepository{
			GetFunc: func(ctx context.Context, id string) (*models.ImageMetadata, error) {
				return &models.ImageMetadata{
					ID:          id,
					Hash:        models.ImageHash{}, // Empty hash means non-deduplicated
					IsDeduped:   false,
					Resolutions: []string{"original", "800x600", "100x100"},
				}, nil
			},
			DeleteFunc: func(ctx context.Context, id string) error {
				return nil
			},
		}

		mockDeduplicationRepo := &testutil.MockDeduplicationRepository{}

		mockStorage := &testutil.MockStorageProvider{
			DeleteFunc: func(ctx context.Context, key string) error {
				return nil
			},
		}

		mockProcessor := &testProcessorService{}

		service := NewImageService(mockRepo, mockDeduplicationRepo, mockStorage, mockProcessor, testConfig())

		// Execute deletion
		err := service.DeleteImage(context.Background(), "f47ac10b-58cc-4372-a567-0e02b2c3d479")

		// Verify results
		assert.NoError(t, err)
	})
}

// TestImageService_ResolutionTrackingPerUser tests the per-user resolution tracking
func TestImageService_ResolutionTrackingPerUser(t *testing.T) {
	t.Run("shared_resolution_preservation", func(t *testing.T) {
		mockRepo := &testutil.MockImageRepository{
			GetFunc: func(ctx context.Context, id string) (*models.ImageMetadata, error) {
				return &models.ImageMetadata{
					ID:            id,
					Hash:          models.ImageHash{Value: "test-hash", Algorithm: "SHA256", Size: 1024},
					IsDeduped:     true,
					SharedImageID: "550e8400-e29b-41d4-a716-446655440000",
					Resolutions:   []string{"original", "800x600", "100x100"},
				}, nil
			},
			DeleteFunc: func(ctx context.Context, id string) error {
				return nil
			},
		}

		mockDeduplicationRepo := &testutil.MockDeduplicationRepository{
			GetDeduplicationInfoFunc: func(ctx context.Context, hash models.ImageHash) (*models.DeduplicationInfo, error) {
				return &models.DeduplicationInfo{
					MasterImageID:  "550e8400-e29b-41d4-a716-446655440000",
					Hash:           models.ImageHash{Value: "test-hash", Algorithm: "SHA256", Size: 1024},
					ReferencingIDs: []string{"550e8400-e29b-41d4-a716-446655440001", "550e8400-e29b-41d4-a716-446655440002"}, // User A still has reference
					ResolutionRefs: map[string]*models.ResolutionReference{
						"original": {ReferencingIDs: []string{"550e8400-e29b-41d4-a716-446655440001", "550e8400-e29b-41d4-a716-446655440002"}}, // Both users use original
						"800x600":  {ReferencingIDs: []string{"550e8400-e29b-41d4-a716-446655440001", "550e8400-e29b-41d4-a716-446655440002"}}, // Both users use 800x600
						"100x100":  {ReferencingIDs: []string{"550e8400-e29b-41d4-a716-446655440002"}},                                         // Only User B uses 100x100
					},
				}, nil
			},
			UpdateDeduplicationInfoFunc: func(ctx context.Context, info *models.DeduplicationInfo) error {
				return nil
			},
		}

		mockStorage := &testutil.MockStorageProvider{
			DeleteFunc: func(ctx context.Context, key string) error {
				return nil
			},
		}

		mockProcessor := &testProcessorService{}

		service := NewImageService(mockRepo, mockDeduplicationRepo, mockStorage, mockProcessor, testConfig())

		// Execute deletion
		err := service.DeleteImage(context.Background(), "550e8400-e29b-41d4-a716-446655440002")

		// Verify results
		assert.NoError(t, err)
	})

	t.Run("final_cleanup_when_last_user_deletes", func(t *testing.T) {
		mockRepo := &testutil.MockImageRepository{
			GetFunc: func(ctx context.Context, id string) (*models.ImageMetadata, error) {
				return &models.ImageMetadata{
					ID:            id,
					Hash:          models.ImageHash{Value: "test-hash", Algorithm: "SHA256", Size: 1024},
					IsDeduped:     true,
					SharedImageID: "550e8400-e29b-41d4-a716-446655440000",
					Resolutions:   []string{"original", "800x600"},
				}, nil
			},
			DeleteFunc: func(ctx context.Context, id string) error {
				return nil
			},
		}

		mockDeduplicationRepo := &testutil.MockDeduplicationRepository{
			GetDeduplicationInfoFunc: func(ctx context.Context, hash models.ImageHash) (*models.DeduplicationInfo, error) {
				return &models.DeduplicationInfo{
					MasterImageID:  "550e8400-e29b-41d4-a716-446655440000",
					Hash:           models.ImageHash{Value: "test-hash", Algorithm: "SHA256", Size: 1024},
					ReferencingIDs: []string{"550e8400-e29b-41d4-a716-446655440001"}, // User A is the last reference
					ResolutionRefs: map[string]*models.ResolutionReference{
						"original": {ReferencingIDs: []string{"550e8400-e29b-41d4-a716-446655440001"}},
						"800x600":  {ReferencingIDs: []string{"550e8400-e29b-41d4-a716-446655440001"}},
					},
				}, nil
			},
			DeleteDeduplicationInfoFunc: func(ctx context.Context, hash models.ImageHash) error {
				return nil
			},
		}

		mockStorage := &testutil.MockStorageProvider{
			DeleteFunc: func(ctx context.Context, key string) error {
				return nil
			},
		}

		mockProcessor := &testProcessorService{}

		service := NewImageService(mockRepo, mockDeduplicationRepo, mockStorage, mockProcessor, testConfig())

		// Execute deletion
		err := service.DeleteImage(context.Background(), "550e8400-e29b-41d4-a716-446655440001")

		// Verify results
		assert.NoError(t, err)
	})
}

// TestImageService_ErrorHandling tests error handling in deduplication scenarios
func TestImageService_ErrorHandling(t *testing.T) {
	t.Run("deduplication_info_not_found", func(t *testing.T) {
		mockRepo := &testutil.MockImageRepository{
			GetFunc: func(ctx context.Context, id string) (*models.ImageMetadata, error) {
				return &models.ImageMetadata{
					ID:          id,
					Hash:        models.ImageHash{Value: "test-hash", Algorithm: "SHA256", Size: 1024},
					IsDeduped:   true,
					Resolutions: []string{"original", "800x600"},
				}, nil
			},
			DeleteFunc: func(ctx context.Context, id string) error {
				return nil
			},
		}

		mockDeduplicationRepo := &testutil.MockDeduplicationRepository{
			GetDeduplicationInfoFunc: func(ctx context.Context, hash models.ImageHash) (*models.DeduplicationInfo, error) {
				return nil, models.NotFoundError{Resource: "deduplication_info", ID: "test-hash"}
			},
		}

		mockStorage := &testutil.MockStorageProvider{
			DeleteFunc: func(ctx context.Context, key string) error {
				return nil
			},
		}

		mockProcessor := &testProcessorService{}

		service := NewImageService(mockRepo, mockDeduplicationRepo, mockStorage, mockProcessor, testConfig())

		// Execute deletion
		err := service.DeleteImage(context.Background(), "f47ac10b-58cc-4372-a567-0e02b2c3d479")

		// Should succeed despite deduplication info not being found
		assert.NoError(t, err)
	})
}
