package service

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"resizr/internal/config"
	"resizr/internal/models"
	"resizr/internal/queue"
	"resizr/internal/repository"
	"resizr/internal/storage"
	"resizr/internal/worker"
	"resizr/pkg/logger"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// ImageServiceImpl implements the ImageService interface
type ImageServiceImpl struct {
	repo          repository.ImageRepository
	dedupRepo     repository.DeduplicationRepository
	storage       storage.ImageStorage
	processor     ProcessorService
	config        *config.Config
	deletionQueue queue.DeletionQueue  // Optional: for async deletion
	intentMgr     worker.IntentManager // Optional: for deletion intent logging
}

// NewImageService creates a new image service
func NewImageService(
	repo repository.ImageRepository,
	dedupRepo repository.DeduplicationRepository,
	storage storage.ImageStorage,
	processor ProcessorService,
	config *config.Config,
) ImageService {
	return &ImageServiceImpl{
		repo:      repo,
		dedupRepo: dedupRepo,
		storage:   storage,
		processor: processor,
		config:    config,
		// deletionQueue and intentMgr are optional, set via SetDeletionQueue if async mode enabled
	}
}

// SetDeletionQueue sets the deletion queue and intent manager (for async deletion)
// This is called during application initialization if DELETION_ASYNC_MODE=true
func (s *ImageServiceImpl) SetDeletionQueue(q queue.DeletionQueue, intent worker.IntentManager) {
	s.deletionQueue = q
	s.intentMgr = intent
}

// ProcessUpload handles the complete image upload workflow
func (s *ImageServiceImpl) ProcessUpload(ctx context.Context, input UploadInput) (*UploadResult, error) {
	logger.InfoWithContext(ctx, "Starting image upload processing",
		zap.String("filename", input.Filename),
		zap.Int64("size", input.Size),
		zap.Strings("requested_resolutions", input.Resolutions))

	// Generate unique ID for the image with collision detection
	imageID, err := s.generateUniqueImageID(ctx)
	if err != nil {
		return nil, models.ProcessingError{
			Operation: "uuid_generation",
			Reason:    err.Error(),
		}
	}

	// Validate input
	if err := s.validateUploadInput(input); err != nil {
		return nil, err
	}

	// Validate and process original image
	if err := s.processor.ValidateImage(input.Data, s.config.Image.MaxFileSize); err != nil {
		return nil, models.ProcessingError{
			Operation: "validate",
			Reason:    err.Error(),
		}
	}

	// Detect format and dimensions
	mimeType, err := s.processor.DetectFormat(input.Data)
	if err != nil {
		return nil, models.ProcessingError{
			Operation: "format_detection",
			Reason:    err.Error(),
		}
	}

	width, height, err := s.processor.GetDimensions(input.Data)
	if err != nil {
		return nil, models.ProcessingError{
			Operation: "dimension_extraction",
			Reason:    err.Error(),
		}
	}

	// Calculate hash for deduplication
	hash := models.CalculateImageHash(input.Data)

	logger.InfoWithContext(ctx, "Calculated image hash for deduplication",
		zap.String("hash", hash.String()),
		zap.Int64("size", hash.Size),
		zap.String("filename", input.Filename))

	// Check for deduplication (Stage 1: Hash comparison)
	existingDedupInfo, err := s.dedupRepo.FindImageByHash(ctx, hash)
	var metadata *models.ImageMetadata
	// ...existing code...

	logger.InfoWithContext(ctx, "Deduplication lookup result",
		zap.String("hash", hash.String()),
		zap.Bool("found_existing", err == nil && existingDedupInfo != nil),
		zap.String("lookup_error", func() string {
			if err != nil {
				return err.Error()
			}
			return "none"
		}()),
		zap.String("existing_master_id", func() string {
			if existingDedupInfo != nil {
				return existingDedupInfo.MasterImageID
			}
			return "none"
		}()))

	if err == nil && existingDedupInfo != nil {
		// Hash exists - perform Stage 2: Byte-to-byte comparison
		logger.InfoWithContext(ctx, "Found matching hash, performing byte-to-byte verification",
			zap.String("existing_master_id", existingDedupInfo.MasterImageID),
			zap.String("hash", hash.String()))

		isDuplicate, verifyErr := s.verifyDuplicateByBytes(ctx, existingDedupInfo.MasterImageID, input.Data)
		if verifyErr != nil {
			logger.WarnWithContext(ctx, "Failed to verify duplicate by bytes, treating as new image",
				zap.Error(verifyErr))
			isDuplicate = false
			// Create new metadata since verification failed
			metadata = models.NewImageMetadataWithHash(imageID, input.Filename, mimeType, input.Size, width, height, hash)
		}

		if isDuplicate {
			// It's a real duplicate - create metadata that references existing storage
			metadata = models.NewImageMetadataWithHash(imageID, input.Filename, mimeType, input.Size, width, height, hash)
			metadata.MarkAsDeduped(existingDedupInfo.MasterImageID)

			// Verify that the original file actually exists in storage
			originalKey := metadata.GetActualStorageKey("original")
			originalExists, existsErr := s.storage.Exists(ctx, originalKey)
			if existsErr != nil {
				logger.WarnWithContext(ctx, "Failed to check if original file exists, treating as new image",
					zap.String("original_key", originalKey),
					zap.Error(existsErr))
				isDuplicate = false
				// Reset metadata to non-deduped state
				metadata.IsDeduped = false
				metadata.SharedImageID = ""
			} else if !originalExists {
				logger.InfoWithContext(ctx, "Original file doesn't exist in storage, uploading new copy",
					zap.String("original_key", originalKey),
					zap.String("hash", hash.String()))

				// Upload the original file since it doesn't exist
				if err := s.storage.Upload(ctx, originalKey, bytes.NewReader(input.Data), input.Size, mimeType); err != nil {
					return nil, models.StorageError{
						Operation: "upload_original",
						Backend:   "S3",
						Reason:    err.Error(),
					}
				}

				logger.InfoWithContext(ctx, "Original image uploaded for deduplicated content",
					zap.String("image_id", imageID),
					zap.String("storage_key", originalKey))
			}

			if isDuplicate {
				// Ensure ResolutionRefs is initialized (for backward compatibility)
				if existingDedupInfo.ResolutionRefs == nil {
					existingDedupInfo.ResolutionRefs = make(map[string]*models.ResolutionReference)
					logger.InfoWithContext(ctx, "Initializing resolution references for existing deduplication info",
						zap.String("hash", hash.String()),
						zap.String("master_id", existingDedupInfo.MasterImageID))
				}

				// Add reference to existing deduplication info
				existingDedupInfo.AddReference(imageID)

				// Add atomic reference for original resolution (all images have original)
				if err := s.dedupRepo.AddResolutionRef(ctx, hash, "original", imageID); err != nil {
					logger.WarnWithContext(ctx, "Failed to add atomic resolution reference for original",
						zap.String("image_id", imageID),
						zap.Error(err))
				}

				if err := s.dedupRepo.UpdateDeduplicationInfo(ctx, existingDedupInfo); err != nil {
					return nil, models.StorageError{
						Operation: "update_dedup_info",
						Backend:   "Repository",
						Reason:    err.Error(),
					}
				}

				metadata.IsDeduped = true
				metadata.SharedImageID = existingDedupInfo.MasterImageID

				logger.InfoWithContext(ctx, "Image deduplicated successfully",
					zap.String("image_id", imageID),
					zap.String("shared_with", metadata.SharedImageID),
					zap.String("hash", hash.String()))
			}
		}
	} else {
		// No existing deduplication found, create metadata for new image
		metadata = models.NewImageMetadataWithHash(imageID, input.Filename, mimeType, input.Size, width, height, hash)
	}

	if metadata != nil && !metadata.IsDeduped {
		// New unique image - store file

		// Store original image
		originalKey := metadata.GetStorageKey("original")
		if err := s.storage.Upload(ctx, originalKey, bytes.NewReader(input.Data), input.Size, mimeType); err != nil {
			return nil, models.StorageError{
				Operation: "upload",
				Backend:   "S3",
				Reason:    err.Error(),
			}
		}

		logger.InfoWithContext(ctx, "Original image uploaded successfully",
			zap.String("image_id", imageID),
			zap.String("storage_key", originalKey))

		// Create deduplication info for this new image
		dedupInfo := models.NewDeduplicationInfo(hash, imageID, originalKey)

		logger.InfoWithContext(ctx, "Creating new deduplication info",
			zap.String("image_id", imageID),
			zap.String("hash", hash.String()),
			zap.String("storage_key", originalKey),
			zap.Int("reference_count", dedupInfo.ReferenceCount))

		if err := s.dedupRepo.StoreDeduplicationInfo(ctx, dedupInfo); err != nil {
			// Log warning but don't fail the upload
			logger.WarnWithContext(ctx, "Failed to store deduplication info",
				zap.String("image_id", imageID),
				zap.String("hash", hash.String()),
				zap.Error(err))
		} else {
			logger.InfoWithContext(ctx, "Deduplication info created successfully",
				zap.String("image_id", imageID),
				zap.String("hash", hash.String()),
				zap.String("storage_key", originalKey))

			// Add atomic reference for original resolution
			if err := s.dedupRepo.AddResolutionRef(ctx, hash, "original", imageID); err != nil {
				logger.WarnWithContext(ctx, "Failed to add atomic resolution reference for original",
					zap.String("image_id", imageID),
					zap.Error(err))
			}
		}
	}

	// Process requested resolutions
	processedResolutions := []string{}
	processedSizes := make(map[string]int64)

	// Add predefined resolutions based on configuration
	var allResolutions []string
	if s.config.Image.GenerateDefaultResolutions {
		allResolutions = append([]string{"thumbnail"}, input.Resolutions...)
	} else {
		allResolutions = input.Resolutions
	}

	for _, resolutionName := range allResolutions {
		// Skip duplicates
		if metadata.HasResolution(resolutionName) {
			continue
		}

		var shouldProcess = true

		// For deduplicated images, check if resolution already exists in shared storage
		if metadata != nil && metadata.IsDeduped {
			// Get deduplication info to check per-resolution references
			dedupInfo, err := s.dedupRepo.GetDeduplicationInfo(ctx, metadata.Hash)
			if err == nil {
				// Ensure ResolutionRefs is initialized (for backward compatibility)
				if dedupInfo.ResolutionRefs == nil {
					dedupInfo.ResolutionRefs = make(map[string]*models.ResolutionReference)
				}

				if dedupInfo.GetResolutionReferenceCount(resolutionName) > 0 {
					// Resolution already exists in shared storage, just add our reference
					shouldProcess = false
					logger.InfoWithContext(ctx, "Resolution already exists in shared storage",
						zap.String("image_id", imageID),
						zap.String("shared_with", metadata.SharedImageID),
						zap.String("resolution", resolutionName),
						zap.Int("existing_refs", dedupInfo.GetResolutionReferenceCount(resolutionName)))
				}
			}
		}

		var processingSucceeded = true
		if shouldProcess {
			if err := s.processResolutionWithMetadata(ctx, imageID, resolutionName, input.Data, mimeType, metadata); err != nil {
				logger.ErrorWithContext(ctx, "Failed to process resolution",
					zap.String("image_id", imageID),
					zap.String("resolution", resolutionName),
					zap.Error(err))
				// Continue with other resolutions instead of failing completely
				processingSucceeded = false
			}
		}

		// Only add to metadata and processed list if processing succeeded (or wasn't needed)
		if processingSucceeded {
			metadata.AddResolution(resolutionName)
			processedResolutions = append(processedResolutions, resolutionName)
		} else {
			// Skip adding to deduplication tracking if processing failed
			continue
		}

		// Add atomic resolution reference for deduplication tracking
		if err := s.dedupRepo.AddResolutionRef(ctx, metadata.Hash, resolutionName, imageID); err != nil {
			logger.WarnWithContext(ctx, "Failed to add atomic resolution reference",
				zap.String("image_id", imageID),
				zap.String("resolution", resolutionName),
				zap.Bool("is_deduped", metadata.IsDeduped),
				zap.Error(err))
		}

		// ...existing code...
	}

	// Store metadata in repository
	if err := s.repo.Store(ctx, metadata); err != nil {
		// If metadata storage fails, cleanup uploaded images
		s.cleanupUploadedImages(ctx, imageID, append(processedResolutions, "original"))
		return nil, models.StorageError{
			Operation: "store_metadata",
			Backend:   "Redis",
			Reason:    err.Error(),
		}
	}

	logger.InfoWithContext(ctx, "Image upload processing completed",
		zap.String("image_id", imageID),
		zap.Strings("processed_resolutions", processedResolutions),
		zap.Int("total_resolutions", len(processedResolutions)))

	return &UploadResult{
		ImageID:              imageID,
		ProcessedResolutions: processedResolutions,
		OriginalSize:         input.Size,
		ProcessedSizes:       processedSizes,
	}, nil
}

// GetMetadata retrieves image metadata by ID
func (s *ImageServiceImpl) GetMetadata(ctx context.Context, imageID string) (*models.ImageMetadata, error) {
	logger.DebugWithContext(ctx, "Retrieving image metadata",
		zap.String("image_id", imageID))

	// Validate UUID format
	if _, err := uuid.Parse(imageID); err != nil {
		return nil, models.ValidationError{
			Field:   "image_id",
			Message: "Invalid UUID format",
		}
	}

	metadata, err := s.repo.Get(ctx, imageID)
	if err != nil {
		if _, ok := err.(models.NotFoundError); ok {
			return nil, err // Pass through not found errors
		}
		return nil, models.StorageError{
			Operation: "get_metadata",
			Backend:   "Redis",
			Reason:    err.Error(),
		}
	}

	return metadata, nil
}

// GetImageStream retrieves image data as a stream
func (s *ImageServiceImpl) GetImageStream(ctx context.Context, imageID, resolution string) (io.ReadCloser, *models.ImageMetadata, error) {
	logger.DebugWithContext(ctx, "Retrieving image stream",
		zap.String("image_id", imageID),
		zap.String("resolution", resolution))

	// Get metadata first
	metadata, err := s.GetMetadata(ctx, imageID)
	if err != nil {
		return nil, nil, err
	}

	// Validate resolution exists (except for original)
	if resolution != "original" && !metadata.HasResolution(resolution) {
		return nil, nil, models.NotFoundError{
			Resource: "resolution",
			ID:       fmt.Sprintf("%s/%s", imageID, resolution),
		}
	}

	// Get actual storage key (handles deduplication)
	storageKey := metadata.GetActualStorageKey(resolution)
	stream, err := s.storage.Download(ctx, storageKey)
	if err != nil {
		return nil, nil, models.StorageError{
			Operation: "download",
			Backend:   "S3",
			Reason:    err.Error(),
		}
	}

	return stream, metadata, nil
}

// ProcessResolution generates a specific resolution for an existing image
func (s *ImageServiceImpl) ProcessResolution(ctx context.Context, imageID, resolution string) error {
	logger.InfoWithContext(ctx, "Processing additional resolution",
		zap.String("image_id", imageID),
		zap.String("resolution", resolution))

	// Get metadata
	metadata, err := s.GetMetadata(ctx, imageID)
	if err != nil {
		return err
	}

	// Check if resolution already exists
	if metadata.HasResolution(resolution) {
		return nil // Already exists, no need to process
	}

	// Download original image data
	originalStream, _, err := s.GetImageStream(ctx, imageID, "original")
	if err != nil {
		return err
	}
	defer func() {
		if err := originalStream.Close(); err != nil {
			logger.WarnWithContext(ctx, "Failed to close original stream", zap.String("error", err.Error()))
		}
	}()

	// Read original data
	originalData, err := io.ReadAll(originalStream)
	if err != nil {
		return models.ProcessingError{
			Operation: "read_original",
			Reason:    err.Error(),
		}
	}

	// Process the resolution
	if err := s.processResolution(ctx, imageID, resolution, originalData, metadata.MimeType); err != nil {
		return err
	}

	// Update metadata
	metadata.AddResolution(resolution)
	return s.repo.Update(ctx, metadata)
}

// DeleteImage removes an image and all its resolutions
// Uses atomic reference counting and async deletion queue
func (s *ImageServiceImpl) DeleteImage(ctx context.Context, imageID string) error {
	logger.InfoWithContext(ctx, "Deleting image",
		zap.String("image_id", imageID))

	// Get metadata to know what to delete
	metadata, err := s.GetMetadata(ctx, imageID)
	if err != nil {
		return err
	}

	// Collect all resolutions (original + custom)
	allResolutions := append([]string{"original"}, metadata.Resolutions...)

	// Variables to track deletion results
	var deletedResolutions []string
	var sharedResolutions []string
	var asyncDeletionIntents []string

	// Handle deduplication cleanup with atomic operations
	if metadata.Hash.Value != "" {
		// Atomically decrement references for all resolutions
		refCounts, err := s.dedupRepo.DecrementResolutionRefs(ctx, metadata.Hash, imageID, allResolutions)
		if err != nil {
			logger.ErrorWithContext(ctx, "Failed to decrement resolution references",
				zap.String("image_id", imageID),
				zap.String("hash", metadata.Hash.String()),
				zap.Error(err))
			// Continue with best-effort cleanup
			refCounts = make(map[string]*repository.ResolutionRefCount)
		}

		// Process each resolution based on reference count
		for _, resolution := range allResolutions {
			refCount, exists := refCounts[resolution]
			if !exists {
				// No reference info, skip (shouldn't happen with new system)
				logger.WarnWithContext(ctx, "No reference count info for resolution",
					zap.String("image_id", imageID),
					zap.String("resolution", resolution))
				continue
			}

			logger.InfoWithContext(ctx, "Resolution reference count updated",
				zap.String("image_id", imageID),
				zap.String("resolution", resolution),
				zap.Int64("remaining_refs", refCount.Count),
				zap.Bool("should_delete", refCount.ShouldDelete))

			if refCount.ShouldDelete {
				// No more references - delete physical file
				storageKey := metadata.GetActualStorageKey(resolution)

				// Check if async deletion is enabled
				if s.config.Deletion.AsyncMode && s.deletionQueue != nil && s.intentMgr != nil {
					// Async deletion via queue
					if err := s.enqueueDeletion(ctx, imageID, resolution, storageKey, metadata.Hash.String()); err != nil {
						logger.ErrorWithContext(ctx, "Failed to enqueue deletion",
							zap.String("image_id", imageID),
							zap.String("resolution", resolution),
							zap.Error(err))
						// Fallback to sync deletion
						if err := s.storage.Delete(ctx, storageKey); err != nil {
							logger.WarnWithContext(ctx, "Failed to delete resolution (sync fallback)",
								zap.String("resolution", resolution),
								zap.Error(err))
						}
					} else {
						asyncDeletionIntents = append(asyncDeletionIntents, resolution)
					}
				} else {
					// Synchronous deletion
					if err := s.storage.Delete(ctx, storageKey); err != nil {
						logger.WarnWithContext(ctx, "Failed to delete resolution",
							zap.String("resolution", resolution),
							zap.Error(err))
					} else {
						logger.InfoWithContext(ctx, "Resolution deleted",
							zap.String("resolution", resolution))
					}
				}

				deletedResolutions = append(deletedResolutions, resolution)
			} else {
				// Still referenced by other images
				sharedResolutions = append(sharedResolutions, resolution)
				logger.InfoWithContext(ctx, "Resolution kept (still referenced)",
					zap.String("image_id", imageID),
					zap.String("resolution", resolution),
					zap.Int64("remaining_refs", refCount.Count))
			}
		}

		// Check if deduplication info is now orphaned
		dedupInfo, err := s.dedupRepo.GetDeduplicationInfo(ctx, metadata.Hash)
		if err == nil && dedupInfo.IsOrphaned() {
			logger.InfoWithContext(ctx, "Deduplication info is orphaned, cleaning up",
				zap.String("hash", metadata.Hash.String()))

			// Delete folder
			folderPrefix := fmt.Sprintf("images/%s", dedupInfo.MasterImageID)
			if err := s.storage.DeleteFolder(ctx, folderPrefix); err != nil {
				logger.WarnWithContext(ctx, "Failed to delete image folder",
					zap.String("folder", folderPrefix),
					zap.Error(err))
			}

			// Delete deduplication info
			if err := s.dedupRepo.DeleteDeduplicationInfo(ctx, metadata.Hash); err != nil {
				logger.WarnWithContext(ctx, "Failed to delete deduplication info",
					zap.String("hash", metadata.Hash.String()),
					zap.Error(err))
			}
		}
	} else {
		// Non-deduplicated image - delete all files directly
		logger.InfoWithContext(ctx, "Deleting non-deduplicated image",
			zap.String("image_id", imageID))

		for _, resolution := range allResolutions {
			storageKey := metadata.GetStorageKey(resolution)

			if s.config.Deletion.AsyncMode && s.deletionQueue != nil && s.intentMgr != nil {
				// Async deletion
				if err := s.enqueueDeletion(ctx, imageID, resolution, storageKey, ""); err != nil {
					logger.ErrorWithContext(ctx, "Failed to enqueue deletion",
						zap.String("resolution", resolution),
						zap.Error(err))
					// Fallback to sync
					if err := s.storage.Delete(ctx, storageKey); err != nil {
						logger.WarnWithContext(ctx, "Fallback sync deletion failed", zap.String("resolution", resolution), zap.Error(err))
					}
				} else {
					asyncDeletionIntents = append(asyncDeletionIntents, resolution)
				}
			} else {
				// Sync deletion
				if err := s.storage.Delete(ctx, storageKey); err != nil {
					logger.WarnWithContext(ctx, "Failed to delete resolution",
						zap.String("resolution", resolution),
						zap.Error(err))
				}
			}

			deletedResolutions = append(deletedResolutions, resolution)
		}

		// Delete folder
		folderPrefix := fmt.Sprintf("images/%s", imageID)
		if err := s.storage.DeleteFolder(ctx, folderPrefix); err != nil {
			logger.WarnWithContext(ctx, "Failed to delete image folder",
				zap.String("folder", folderPrefix),
				zap.Error(err))
		}
	}

	// Delete metadata from repository
	if err := s.repo.Delete(ctx, imageID); err != nil {
		return models.StorageError{
			Operation: "delete_metadata",
			Backend:   "Repository",
			Reason:    err.Error(),
		}
	}

	logger.InfoWithContext(ctx, "Image deletion completed",
		zap.String("image_id", imageID),
		zap.Strings("deleted_resolutions", deletedResolutions),
		zap.Strings("shared_resolutions", sharedResolutions),
		zap.Strings("async_deletions", asyncDeletionIntents),
		zap.Bool("async_mode", s.config.Deletion.AsyncMode))

	return nil
}

// enqueueDeletion creates a deletion intent and enqueues a deletion message
func (s *ImageServiceImpl) enqueueDeletion(ctx context.Context, imageID, resolution, storageKey, hash string) error {
	// Create deletion intent
	intentID := uuid.New().String()
	intent := &queue.DeletionIntent{
		IntentID:   intentID,
		ImageID:    imageID,
		Resolution: resolution,
		StorageKey: storageKey,
		Hash:       hash,
		Status:     "pending",
		CreatedAt:  time.Now(),
	}

	if err := s.intentMgr.CreateIntent(ctx, intent); err != nil {
		return fmt.Errorf("failed to create intent: %w", err)
	}

	// Enqueue deletion message
	msg := &queue.DeletionMessage{
		IntentID:   intentID,
		ImageID:    imageID,
		Resolution: resolution,
		StorageKey: storageKey,
		Hash:       hash,
		Priority:   "normal",
	}

	messageID, err := s.deletionQueue.Enqueue(ctx, msg)
	if err != nil {
		// Cleanup intent on enqueue failure
		if delErr := s.intentMgr.DeleteIntent(ctx, intentID); delErr != nil {
			logger.WarnWithContext(ctx, "Failed to cleanup intent after enqueue failure",
				zap.String("intent_id", intentID),
				zap.Error(delErr))
		}
		return fmt.Errorf("failed to enqueue message: %w", err)
	}

	// Update intent with message ID
	intent.MessageID = messageID
	if err := s.intentMgr.UpdateStatus(ctx, intentID, "pending", "", ""); err != nil {
		logger.WarnWithContext(ctx, "Failed to update intent status",
			zap.String("intent_id", intentID),
			zap.Error(err))
	}

	logger.InfoWithContext(ctx, "Deletion enqueued",
		zap.String("intent_id", intentID),
		zap.String("message_id", messageID),
		zap.String("image_id", imageID),
		zap.String("resolution", resolution))

	return nil
}

// DeleteResolution removes a specific resolution from an image (except original)
// Uses atomic reference counting and async deletion queue
func (s *ImageServiceImpl) DeleteResolution(ctx context.Context, imageID, resolution string) error {
	logger.InfoWithContext(ctx, "Deleting resolution",
		zap.String("image_id", imageID),
		zap.String("resolution", resolution))

	// Validate that it's not the original
	if resolution == "original" {
		return models.ValidationError{
			Field:   "resolution",
			Message: "Cannot delete the original resolution",
		}
	}

	// Get metadata
	metadata, err := s.GetMetadata(ctx, imageID)
	if err != nil {
		return err
	}

	// Check if resolution exists
	if !metadata.HasResolution(resolution) {
		return models.NotFoundError{
			Resource: "resolution",
			ID:       fmt.Sprintf("%s/%s", imageID, resolution),
		}
	}

	var physicallyDeleted bool
	var remainingRefs int64

	// Handle deduplication with atomic reference counting
	if metadata.Hash.Value != "" {
		// Atomically decrement reference for this resolution
		refCounts, err := s.dedupRepo.DecrementResolutionRefs(ctx, metadata.Hash, imageID, []string{resolution})
		if err != nil {
			logger.ErrorWithContext(ctx, "Failed to decrement resolution reference",
				zap.String("image_id", imageID),
				zap.String("resolution", resolution),
				zap.Error(err))
			return fmt.Errorf("failed to decrement resolution reference: %w", err)
		}

		refCount, exists := refCounts[resolution]
		if !exists {
			logger.WarnWithContext(ctx, "No reference count returned for resolution",
				zap.String("image_id", imageID),
				zap.String("resolution", resolution))
			refCount = &repository.ResolutionRefCount{Count: 0, ShouldDelete: false}
		}

		remainingRefs = refCount.Count

		logger.InfoWithContext(ctx, "Resolution reference decremented",
			zap.String("image_id", imageID),
			zap.String("resolution", resolution),
			zap.Int64("remaining_refs", remainingRefs),
			zap.Bool("should_delete", refCount.ShouldDelete))

		// Delete physical file if no more references
		if refCount.ShouldDelete {
			storageKey := metadata.GetActualStorageKey(resolution)

			// Check if async deletion is enabled
			if s.config.Deletion.AsyncMode && s.deletionQueue != nil && s.intentMgr != nil {
				// Async deletion via queue
				if err := s.enqueueDeletion(ctx, imageID, resolution, storageKey, metadata.Hash.String()); err != nil {
					logger.ErrorWithContext(ctx, "Failed to enqueue deletion",
						zap.String("image_id", imageID),
						zap.String("resolution", resolution),
						zap.Error(err))
					// Fallback to sync deletion
					if err := s.storage.Delete(ctx, storageKey); err != nil {
						logger.WarnWithContext(ctx, "Failed to delete resolution (sync fallback)",
							zap.String("resolution", resolution),
							zap.Error(err))
					} else {
						physicallyDeleted = true
					}
				} else {
					// Queued for deletion (not immediate)
					physicallyDeleted = false
					logger.InfoWithContext(ctx, "Resolution deletion queued",
						zap.String("resolution", resolution))
				}
			} else {
				// Synchronous deletion
				if err := s.storage.Delete(ctx, storageKey); err != nil {
					logger.WarnWithContext(ctx, "Failed to delete resolution",
						zap.String("resolution", resolution),
						zap.Error(err))
				} else {
					physicallyDeleted = true
					logger.InfoWithContext(ctx, "Resolution physically deleted",
						zap.String("resolution", resolution))
				}
			}
		} else {
			logger.InfoWithContext(ctx, "Resolution kept (still referenced by other images)",
				zap.String("image_id", imageID),
				zap.String("resolution", resolution),
				zap.Int64("remaining_refs", remainingRefs))
		}
	} else {
		// Non-deduplicated image - delete directly
		storageKey := metadata.GetStorageKey(resolution)

		if s.config.Deletion.AsyncMode && s.deletionQueue != nil && s.intentMgr != nil {
			// Async deletion
			if err := s.enqueueDeletion(ctx, imageID, resolution, storageKey, ""); err != nil {
				logger.ErrorWithContext(ctx, "Failed to enqueue deletion",
					zap.String("resolution", resolution),
					zap.Error(err))
				// Fallback to sync
				if err := s.storage.Delete(ctx, storageKey); err != nil {
					logger.WarnWithContext(ctx, "Failed to delete resolution",
						zap.String("resolution", resolution),
						zap.Error(err))
				} else {
					physicallyDeleted = true
				}
			}
		} else {
			// Sync deletion
			if err := s.storage.Delete(ctx, storageKey); err != nil {
				logger.WarnWithContext(ctx, "Failed to delete resolution",
					zap.String("resolution", resolution),
					zap.Error(err))
			} else {
				physicallyDeleted = true
			}
		}
	}

	// Remove resolution from metadata
	newResolutions := []string{}
	for _, res := range metadata.Resolutions {
		if res != resolution {
			newResolutions = append(newResolutions, res)
		}
	}
	metadata.Resolutions = newResolutions
	metadata.UpdatedAt = time.Now()

	// Update metadata in repository
	if err := s.repo.Update(ctx, metadata); err != nil {
		return models.StorageError{
			Operation: "update_metadata",
			Backend:   "Repository",
			Reason:    err.Error(),
		}
	}

	logger.InfoWithContext(ctx, "Resolution deletion completed",
		zap.String("image_id", imageID),
		zap.String("resolution", resolution),
		zap.Bool("physically_deleted", physicallyDeleted),
		zap.Int64("remaining_references", remainingRefs),
		zap.Bool("async_mode", s.config.Deletion.AsyncMode))

	return nil
}

// ListImages retrieves paginated list of images
func (s *ImageServiceImpl) ListImages(ctx context.Context, offset, limit int) ([]*models.ImageMetadata, int, error) {
	logger.DebugWithContext(ctx, "Listing images",
		zap.Int("offset", offset),
		zap.Int("limit", limit))

	if limit <= 0 || limit > 100 {
		limit = 50 // Default limit
	}

	images, err := s.repo.List(ctx, offset, limit)
	if err != nil {
		return nil, 0, models.StorageError{
			Operation: "list_images",
			Backend:   "Redis",
			Reason:    err.Error(),
		}
	}

	// Get total count (this could be cached for better performance)
	// For now, return -1 to indicate total is unknown
	total := -1

	return images, total, nil
}

// GeneratePresignedURL generates a pre-signed URL for direct access to storage
func (s *ImageServiceImpl) GeneratePresignedURL(ctx context.Context, storageKey string, duration time.Duration) (string, error) {
	logger.DebugWithContext(ctx, "Generating presigned URL",
		zap.String("storage_key", storageKey),
		zap.Duration("duration", duration))

	presignedURL, err := s.storage.GeneratePresignedURL(ctx, storageKey, duration)
	if err != nil {
		logger.ErrorWithContext(ctx, "Failed to generate presigned URL",
			zap.String("storage_key", storageKey),
			zap.Error(err))
		return "", models.StorageError{
			Operation: "generate_presigned_url",
			Backend:   "S3",
			Reason:    err.Error(),
		}
	}

	logger.InfoWithContext(ctx, "Presigned URL generated successfully",
		zap.String("storage_key", storageKey),
		zap.Duration("duration", duration))

	return presignedURL, nil
}

// Helper methods

// generateUniqueImageID generates a UUID and ensures it doesn't already exist in the repository
func (s *ImageServiceImpl) generateUniqueImageID(ctx context.Context) (string, error) {
	const maxAttempts = 10 // Prevent infinite loops in case of system issues

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		// Generate a new UUID
		candidateID := uuid.New().String()

		logger.DebugWithContext(ctx, "Generating UUID for image upload",
			zap.String("candidate_id", candidateID),
			zap.Int("attempt", attempt))

		// Check if this UUID already exists in the repository
		exists, err := s.repo.Exists(ctx, candidateID)
		if err != nil {
			logger.ErrorWithContext(ctx, "Failed to check UUID existence in repository",
				zap.String("candidate_id", candidateID),
				zap.Int("attempt", attempt),
				zap.Error(err))
			return "", fmt.Errorf("failed to check UUID existence: %w", err)
		}

		if !exists {
			// UUID is unique, we can use it
			logger.InfoWithContext(ctx, "Generated unique UUID for image upload",
				zap.String("image_id", candidateID),
				zap.Int("attempts_required", attempt))
			return candidateID, nil
		}

		// UUID collision detected, log it and try again
		logger.WarnWithContext(ctx, "UUID collision detected, regenerating",
			zap.String("colliding_id", candidateID),
			zap.Int("attempt", attempt),
			zap.Int("max_attempts", maxAttempts))
	}

	// If we've reached this point, we've exceeded max attempts
	logger.ErrorWithContext(ctx, "Failed to generate unique UUID after maximum attempts",
		zap.Int("max_attempts", maxAttempts))
	return "", fmt.Errorf("failed to generate unique UUID after %d attempts", maxAttempts)
}

// validateUploadInput validates the upload input
func (s *ImageServiceImpl) validateUploadInput(input UploadInput) error {
	if input.Filename == "" {
		return models.ValidationError{
			Field:   "filename",
			Message: "Filename is required",
		}
	}

	if len(input.Data) == 0 {
		return models.ValidationError{
			Field:   "data",
			Message: "Image data is required",
		}
	}

	if input.Size != int64(len(input.Data)) {
		return models.ValidationError{
			Field:   "size",
			Message: "Size mismatch with actual data length",
		}
	}

	// Validate requested resolutions - support comma-separated values
	validatedResolutions := []string{}
	for _, resolution := range input.Resolutions {
		// Handle comma-separated resolutions in a single field
		resolutions := strings.Split(resolution, ",")
		for _, res := range resolutions {
			res = strings.TrimSpace(res) // Remove whitespace
			if res == "" {
				continue // Skip empty strings
			}
			if rc, err := models.ParseResolution(res); err != nil {
				return models.ValidationError{
					Field:   "resolutions",
					Message: fmt.Sprintf("Invalid resolution format '%s': %s", res, err.Error()),
				}
			} else {
				// Enforce configured maximums for requested resolutions
				if rc.Width > s.config.Image.MaxWidth || rc.Height > s.config.Image.MaxHeight {
					return models.ValidationError{
						Field:   "resolutions",
						Message: fmt.Sprintf("Requested resolution '%s' exceeds maximum configured %dx%d", res, s.config.Image.MaxWidth, s.config.Image.MaxHeight),
					}
				}
			}
			validatedResolutions = append(validatedResolutions, res)
		}
	}
	// Update input with parsed resolutions
	input.Resolutions = validatedResolutions

	return nil
}

// processResolution processes a single resolution
func (s *ImageServiceImpl) processResolution(ctx context.Context, imageID, resolutionName string, originalData []byte, mimeType string) error {
	return s.processResolutionWithMetadata(ctx, imageID, resolutionName, originalData, mimeType, nil)
}

// processResolutionWithMetadata processes a single resolution with metadata context
func (s *ImageServiceImpl) processResolutionWithMetadata(ctx context.Context, imageID, resolutionName string, originalData []byte, mimeType string, metadata *models.ImageMetadata) error {
	// Determine the storage image ID (use shared ID if deduplicated)
	storageImageID := imageID
	if metadata != nil && metadata.IsDeduped && metadata.SharedImageID != "" {
		storageImageID = metadata.SharedImageID
	}
	// Parse resolution configuration
	resolutionConfig, err := models.ParseResolution(resolutionName)
	if err != nil {
		return models.ValidationError{
			Field:   "resolution",
			Message: err.Error(),
		}
	}

	// Convert MIME type to format string for processor
	format := ""
	switch mimeType {
	case "image/jpeg":
		format = "jpeg"
	case "image/png":
		format = "png"
	case "image/gif":
		format = "gif"
	case "image/webp":
		format = "webp"
	default:
		format = "jpeg" // fallback to JPEG
	}

	// Configure resize parameters
	resizeConfig := ResizeConfig{
		Width:           resolutionConfig.Width,
		Height:          resolutionConfig.Height,
		Quality:         s.config.Image.Quality,
		Format:          format,
		Mode:            ResizeMode(s.config.Image.ResizeMode),
		BackgroundColor: s.config.Canvas.BackgroundColor,
	}

	// Process the image
	processedData, err := s.processor.ProcessImage(originalData, resizeConfig)
	if err != nil {
		return models.ProcessingError{
			Operation: "resize",
			Reason:    err.Error(),
		}
	}

	// Upload processed image using dimensions-only storage key (no aliases)
	// This ensures no duplicate files are stored and uses shared storage for deduplicated images
	// Use actual dimensions from parsed config instead of resolution name
	dimensions := fmt.Sprintf("%dx%d", resolutionConfig.Width, resolutionConfig.Height)
	// Validate that dimensions string is safe for filesystem path
	if !models.IsSafeDimensionString(dimensions) {
		return models.ValidationError{
			Field:   "resolution",
			Message: fmt.Sprintf("Bad resolution dimensions: %q", dimensions),
		}
	}
	storageKey := fmt.Sprintf("images/%s/%s.%s", storageImageID, dimensions, models.GetExtensionFromMimeType(mimeType))
	if err := s.storage.Upload(ctx, storageKey, bytes.NewReader(processedData), int64(len(processedData)), mimeType); err != nil {
		return models.StorageError{
			Operation: "upload_processed",
			Backend:   "S3",
			Reason:    err.Error(),
		}
	}

	logger.DebugWithContext(ctx, "Resolution processed successfully",
		zap.String("image_id", imageID),
		zap.String("resolution", resolutionName),
		zap.String("storage_key", storageKey),
		zap.Int("processed_size", len(processedData)))

	return nil
}

// ...existing code...

// isSafePathComponent validates that a string is safe to use as a path component (filename segment)
func isSafePathComponent(component string) bool {
	if component == "" ||
		strings.Contains(component, "/") ||
		strings.Contains(component, "\\") ||
		strings.Contains(component, "..") ||
		strings.HasPrefix(component, ".") {
		return false
	}
	return true
}

// cleanupUploadedImages cleans up images if upload fails
func (s *ImageServiceImpl) cleanupUploadedImages(ctx context.Context, imageID string, resolutions []string) {
	logger.WarnWithContext(ctx, "Cleaning up uploaded images due to failure",
		zap.String("image_id", imageID),
		zap.Strings("resolutions", resolutions))

	for _, resolution := range resolutions {
		if !isSafePathComponent(resolution) {
			logger.ErrorWithContext(ctx, "Unsafe resolution name detected during cleanup, skipping",
				zap.String("image_id", imageID),
				zap.String("resolution", resolution))
			continue
		}
		storageKey := fmt.Sprintf("images/%s/%s.jpg", imageID, resolution) // Simplified
		if err := s.storage.Delete(ctx, storageKey); err != nil {
			logger.ErrorWithContext(ctx, "Failed to cleanup uploaded image",
				zap.String("image_id", imageID),
				zap.String("resolution", resolution),
				zap.String("storage_key", storageKey),
				zap.Error(err))
		}
	}
}

// verifyDuplicateByBytes performs byte-to-byte comparison to verify if images are truly identical
// This is the second stage of deduplication verification to handle hash collisions
func (s *ImageServiceImpl) verifyDuplicateByBytes(ctx context.Context, existingImageID string, newImageData []byte) (bool, error) {
	logger.DebugWithContext(ctx, "Performing byte-to-byte duplicate verification",
		zap.String("existing_image_id", existingImageID),
		zap.Int("new_image_size", len(newImageData)))

	// Download the existing original image
	existingStream, _, err := s.GetImageStream(ctx, existingImageID, "original")
	if err != nil {
		return false, fmt.Errorf("failed to download existing image for comparison: %w", err)
	}
	defer func() {
		if err := existingStream.Close(); err != nil {
			logger.WarnWithContext(ctx, "Failed to close existing stream", zap.String("error", err.Error()))
		}
	}()

	// Read existing image data
	existingData, err := io.ReadAll(existingStream)
	if err != nil {
		return false, fmt.Errorf("failed to read existing image data: %w", err)
	}

	// Compare byte-by-byte
	isDuplicate := models.CompareBytesByBytes(existingData, newImageData)

	logger.DebugWithContext(ctx, "Byte-to-byte comparison completed",
		zap.String("existing_image_id", existingImageID),
		zap.Int("existing_size", len(existingData)),
		zap.Int("new_size", len(newImageData)),
		zap.Bool("is_duplicate", isDuplicate))

	return isDuplicate, nil
}
