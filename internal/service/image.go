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
	"resizr/internal/repository"
	"resizr/internal/storage"
	"resizr/pkg/logger"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// ImageServiceImpl implements the ImageService interface
type ImageServiceImpl struct {
	repo      repository.ImageRepository
	dedupRepo repository.DeduplicationRepository
	storage   storage.ImageStorage
	processor ProcessorService
	config    *config.Config
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
	}
}

// ProcessUpload handles the complete image upload workflow
func (s *ImageServiceImpl) ProcessUpload(ctx context.Context, input UploadInput) (*UploadResult, error) {
	logger.InfoWithContext(ctx, "Starting image upload processing",
		zap.String("filename", input.Filename),
		zap.Int64("size", input.Size),
		zap.Strings("requested_resolutions", input.Resolutions))

	// Generate unique ID for the image
	imageID := uuid.New().String()

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
				// Add reference for original resolution (all images have original)
				existingDedupInfo.AddResolutionReference("original", imageID)

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
		// Add reference for original resolution
		dedupInfo.AddResolutionReference("original", imageID)

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

		if shouldProcess {
			if err := s.processResolutionWithMetadata(ctx, imageID, resolutionName, input.Data, mimeType, metadata); err != nil {
				logger.ErrorWithContext(ctx, "Failed to process resolution",
					zap.String("image_id", imageID),
					zap.String("resolution", resolutionName),
					zap.Error(err))
				// Continue with other resolutions instead of failing completely
				continue
			}
		}

		metadata.AddResolution(resolutionName)
		processedResolutions = append(processedResolutions, resolutionName)

		// Add resolution reference for deduplication tracking
		if metadata.IsDeduped {
			dedupInfo, err := s.dedupRepo.GetDeduplicationInfo(ctx, metadata.Hash)
			if err == nil {
				dedupInfo.AddResolutionReference(resolutionName, imageID)
				if updateErr := s.dedupRepo.UpdateDeduplicationInfo(ctx, dedupInfo); updateErr != nil {
					logger.WarnWithContext(ctx, "Failed to update resolution reference",
						zap.String("image_id", imageID),
						zap.String("resolution", resolutionName),
						zap.Error(updateErr))
				}
			}
		} else {
			// For non-deduplicated images, also track resolution references
			dedupInfo, err := s.dedupRepo.GetDeduplicationInfo(ctx, metadata.Hash)
			if err == nil {
				dedupInfo.AddResolutionReference(resolutionName, imageID)
				if updateErr := s.dedupRepo.UpdateDeduplicationInfo(ctx, dedupInfo); updateErr != nil {
					logger.WarnWithContext(ctx, "Failed to update resolution reference",
						zap.String("image_id", imageID),
						zap.String("resolution", resolutionName),
						zap.Error(updateErr))
				}
			}
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
func (s *ImageServiceImpl) DeleteImage(ctx context.Context, imageID string) error {
	logger.InfoWithContext(ctx, "Deleting image",
		zap.String("image_id", imageID))

	// Get metadata to know what to delete
	metadata, err := s.GetMetadata(ctx, imageID)
	if err != nil {
		return err
	}

	// Handle deduplication cleanup
	if metadata.Hash.Value != "" {
		dedupInfo, err := s.dedupRepo.GetDeduplicationInfo(ctx, metadata.Hash)
		if err == nil {
			// Ensure ResolutionRefs is initialized (for backward compatibility)
			if dedupInfo.ResolutionRefs == nil {
				dedupInfo.ResolutionRefs = make(map[string]*models.ResolutionReference)
				logger.WarnWithContext(ctx, "Found deduplication info without resolution references, rebuilding resolution tracking",
					zap.String("image_id", imageID),
					zap.String("hash", metadata.Hash.String()))

				// Rebuild resolution references for all existing images
				if rebuildErr := s.rebuildResolutionReferences(ctx, dedupInfo); rebuildErr != nil {
					logger.WarnWithContext(ctx, "Failed to rebuild resolution references, proceeding with manual cleanup",
						zap.String("image_id", imageID),
						zap.Error(rebuildErr))
				}
			}

			// Track which files should be deleted
			resolutionsToDelete := make(map[string]bool)

			// Remove references for all resolutions this image uses
			allResolutions := append([]string{"original"}, metadata.Resolutions...)

			for _, resolution := range allResolutions {
				// Remove this image's reference
				dedupInfo.RemoveResolutionReference(resolution, imageID)

				// Check if this resolution should be physically deleted
				shouldDeletePhysicalFile := dedupInfo.GetResolutionReferenceCount(resolution) == 0

				// Double-check by manually verifying remaining images (for robustness)
				if shouldDeletePhysicalFile && len(dedupInfo.ReferencingIDs) > 0 {
					for _, otherImageID := range dedupInfo.ReferencingIDs {
						if otherImageID != imageID {
							otherMetadata, err := s.GetMetadata(ctx, otherImageID)
							if err == nil {
								if resolution == "original" || otherMetadata.HasResolution(resolution) {
									shouldDeletePhysicalFile = false
									logger.InfoWithContext(ctx, "Resolution still used by other image",
										zap.String("image_id", imageID),
										zap.String("resolution", resolution),
										zap.String("other_image", otherImageID))
									// Re-add the resolution reference if it was missing
									if dedupInfo.GetResolutionReferenceCount(resolution) == 0 {
										dedupInfo.AddResolutionReference(resolution, otherImageID)
									}
									break
								}
							}
						}
					}
				}

				// Additional check: Only mark for deletion if the file actually exists in storage
				// This prevents trying to delete files that were never created for this image
				if shouldDeletePhysicalFile {
					storageKey := metadata.GetActualStorageKey(resolution)
					exists, err := s.storage.Exists(ctx, storageKey)
					if err != nil {
						logger.WarnWithContext(ctx, "Failed to check if resolution exists in storage",
							zap.String("image_id", imageID),
							zap.String("resolution", resolution),
							zap.String("storage_key", storageKey),
							zap.Error(err))
						// If we can't check existence, be conservative and don't delete
						shouldDeletePhysicalFile = false
					} else if !exists {
						logger.InfoWithContext(ctx, "Resolution file doesn't exist in storage, skipping deletion",
							zap.String("image_id", imageID),
							zap.String("resolution", resolution),
							zap.String("storage_key", storageKey))
						shouldDeletePhysicalFile = false
					}
				}

				if shouldDeletePhysicalFile {
					resolutionsToDelete[resolution] = true
					logger.InfoWithContext(ctx, "Resolution marked for deletion",
						zap.String("image_id", imageID),
						zap.String("resolution", resolution))
				}
			}

			// Remove general image reference
			dedupInfo.RemoveReference(imageID)

			// Delete physical files before updating deduplication info
			for resolution := range resolutionsToDelete {
				storageKey := metadata.GetActualStorageKey(resolution)
				if err := s.storage.Delete(ctx, storageKey); err != nil {
					logger.WarnWithContext(ctx, "Failed to delete resolution from storage",
						zap.String("image_id", imageID),
						zap.String("resolution", resolution),
						zap.String("storage_key", storageKey),
						zap.Error(err))
				} else {
					logger.InfoWithContext(ctx, "Physical resolution file deleted",
						zap.String("image_id", imageID),
						zap.String("resolution", resolution),
						zap.String("storage_key", storageKey))
				}
			}

			// Update or delete deduplication info
			if dedupInfo.IsOrphaned() {
				// No images reference this content anymore, perform final cleanup
				logger.InfoWithContext(ctx, "Deduplication info is orphaned, performing final cleanup",
					zap.String("image_id", imageID),
					zap.String("hash", metadata.Hash.String()),
					zap.String("master_id", dedupInfo.MasterImageID))

				// Attempt to delete any remaining files that might exist
				// (this handles cases where files exist but references were lost)
				allPossibleResolutions := []string{"original", "thumbnail"}
				// Add any custom resolutions that might exist
				for resolution := range dedupInfo.ResolutionRefs {
					allPossibleResolutions = append(allPossibleResolutions, resolution)
				}
				// Add resolutions from the deleted image metadata
				allPossibleResolutions = append(allPossibleResolutions, metadata.Resolutions...)

				// Remove duplicates
				resolutionSet := make(map[string]bool)
				for _, res := range allPossibleResolutions {
					resolutionSet[res] = true
				}

				for resolution := range resolutionSet {
					storageKey := metadata.GetActualStorageKey(resolution)
					if err := s.storage.Delete(ctx, storageKey); err != nil {
						logger.DebugWithContext(ctx, "Failed to clean up resolution (likely doesn't exist)",
							zap.String("resolution", resolution),
							zap.String("storage_key", storageKey),
							zap.Error(err))
					} else {
						logger.InfoWithContext(ctx, "Final cleanup: deleted remaining resolution file",
							zap.String("resolution", resolution),
							zap.String("storage_key", storageKey))
					}
				}

				// Now delete the entire folder for the master image
				folderPrefix := fmt.Sprintf("images/%s", dedupInfo.MasterImageID)
				if err := s.storage.DeleteFolder(ctx, folderPrefix); err != nil {
					logger.WarnWithContext(ctx, "Failed to delete image folder (but individual files were cleaned up)",
						zap.String("image_id", imageID),
						zap.String("master_id", dedupInfo.MasterImageID),
						zap.String("folder", folderPrefix),
						zap.Error(err))
				} else {
					logger.InfoWithContext(ctx, "Image folder deleted successfully",
						zap.String("image_id", imageID),
						zap.String("master_id", dedupInfo.MasterImageID),
						zap.String("folder", folderPrefix))
				}

				if err := s.dedupRepo.DeleteDeduplicationInfo(ctx, metadata.Hash); err != nil {
					logger.WarnWithContext(ctx, "Failed to delete deduplication info",
						zap.String("image_id", imageID),
						zap.String("hash", metadata.Hash.String()),
						zap.Error(err))
				} else {
					logger.InfoWithContext(ctx, "Deduplication info deleted successfully",
						zap.String("image_id", imageID),
						zap.String("hash", metadata.Hash.String()))
				}
			} else {
				// Update deduplication info with removed references
				if err := s.dedupRepo.UpdateDeduplicationInfo(ctx, dedupInfo); err != nil {
					logger.WarnWithContext(ctx, "Failed to update deduplication info",
						zap.String("image_id", imageID),
						zap.String("hash", metadata.Hash.String()),
						zap.Error(err))
				} else {
					logger.InfoWithContext(ctx, "Deduplication info updated with removed references",
						zap.String("image_id", imageID),
						zap.Int("remaining_references", dedupInfo.ReferenceCount))
				}
			}
		} else {
			logger.WarnWithContext(ctx, "Failed to get deduplication info during deletion, performing standalone cleanup",
				zap.String("image_id", imageID),
				zap.String("hash", metadata.Hash.String()),
				zap.Error(err))

			// If we can't get deduplication info, delete the files anyway since this might be the last image
			allResolutions := append([]string{"original"}, metadata.Resolutions...)
			for _, resolution := range allResolutions {
				storageKey := metadata.GetStorageKey(resolution)
				if deleteErr := s.storage.Delete(ctx, storageKey); deleteErr != nil {
					logger.DebugWithContext(ctx, "Failed to delete resolution during standalone cleanup",
						zap.String("resolution", resolution),
						zap.String("storage_key", storageKey),
						zap.Error(deleteErr))
				} else {
					logger.InfoWithContext(ctx, "Standalone cleanup: deleted resolution file",
						zap.String("resolution", resolution),
						zap.String("storage_key", storageKey))
				}
			}

			// Try to delete the folder as well
			folderPrefix := fmt.Sprintf("images/%s", imageID)
			if err := s.storage.DeleteFolder(ctx, folderPrefix); err != nil {
				logger.WarnWithContext(ctx, "Standalone cleanup: failed to delete image folder",
					zap.String("image_id", imageID),
					zap.String("folder", folderPrefix),
					zap.Error(err))
			} else {
				logger.InfoWithContext(ctx, "Standalone cleanup: image folder deleted successfully",
					zap.String("image_id", imageID),
					zap.String("folder", folderPrefix))
			}
		}
	} else {
		// Handle images without hash (non-deduplicated images)
		logger.InfoWithContext(ctx, "Deleting non-deduplicated image files",
			zap.String("image_id", imageID))

		allResolutions := append([]string{"original"}, metadata.Resolutions...)
		for _, resolution := range allResolutions {
			storageKey := metadata.GetStorageKey(resolution)
			if deleteErr := s.storage.Delete(ctx, storageKey); deleteErr != nil {
				logger.WarnWithContext(ctx, "Failed to delete resolution file",
					zap.String("resolution", resolution),
					zap.String("storage_key", storageKey),
					zap.Error(deleteErr))
			} else {
				logger.InfoWithContext(ctx, "Deleted resolution file",
					zap.String("resolution", resolution),
					zap.String("storage_key", storageKey))
			}
		}

		// Delete the entire folder for this non-deduplicated image
		folderPrefix := fmt.Sprintf("images/%s", imageID)
		if err := s.storage.DeleteFolder(ctx, folderPrefix); err != nil {
			logger.WarnWithContext(ctx, "Failed to delete image folder (but individual files were cleaned up)",
				zap.String("image_id", imageID),
				zap.String("folder", folderPrefix),
				zap.Error(err))
		} else {
			logger.InfoWithContext(ctx, "Image folder deleted successfully",
				zap.String("image_id", imageID),
				zap.String("folder", folderPrefix))
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

	logger.InfoWithContext(ctx, "Image deleted successfully",
		zap.String("image_id", imageID),
		zap.Bool("was_deduplicated", metadata.IsDeduped))

	return nil
}

// DeleteResolution removes a specific resolution from an image (except original)
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

	// Check if other images are using this resolution (works for both deduplicated and non-deduplicated)
	shouldDeletePhysicalFile := true

	// Get deduplication info to check per-resolution references
	dedupInfo, err := s.dedupRepo.GetDeduplicationInfo(ctx, metadata.Hash)
	if err == nil {
		// Ensure ResolutionRefs is initialized (for backward compatibility)
		if dedupInfo.ResolutionRefs == nil {
			dedupInfo.ResolutionRefs = make(map[string]*models.ResolutionReference)
			logger.WarnWithContext(ctx, "Found deduplication info without resolution references, rebuilding",
				zap.String("image_id", imageID),
				zap.String("resolution", resolution),
				zap.String("hash", metadata.Hash.String()))

			// Rebuild resolution references for all existing images
			if rebuildErr := s.rebuildResolutionReferences(ctx, dedupInfo); rebuildErr != nil {
				logger.WarnWithContext(ctx, "Failed to rebuild resolution references",
					zap.String("image_id", imageID),
					zap.Error(rebuildErr))
			}
		}

		// Remove this image's reference to the resolution
		dedupInfo.RemoveResolutionReference(resolution, imageID)

		// Check if any other images still reference this resolution
		if dedupInfo.GetResolutionReferenceCount(resolution) > 0 {
			shouldDeletePhysicalFile = false
			logger.InfoWithContext(ctx, "Resolution is still used by other images, keeping physical file",
				zap.String("image_id", imageID),
				zap.String("resolution", resolution),
				zap.Int("remaining_refs", dedupInfo.GetResolutionReferenceCount(resolution)))
		} else {
			// Double-check by manually verifying remaining images (for robustness)
			for _, otherImageID := range dedupInfo.ReferencingIDs {
				if otherImageID != imageID {
					otherMetadata, err := s.GetMetadata(ctx, otherImageID)
					if err == nil && otherMetadata.HasResolution(resolution) {
						shouldDeletePhysicalFile = false
						logger.InfoWithContext(ctx, "Resolution still used by other image (fallback check)",
							zap.String("image_id", imageID),
							zap.String("resolution", resolution),
							zap.String("other_image", otherImageID))
						// Re-add the resolution reference if it was missing
						dedupInfo.AddResolutionReference(resolution, otherImageID)
						break
					}
				}
			}
		}

		// Update deduplication info
		if updateErr := s.dedupRepo.UpdateDeduplicationInfo(ctx, dedupInfo); updateErr != nil {
			logger.WarnWithContext(ctx, "Failed to update resolution reference",
				zap.String("image_id", imageID),
				zap.String("resolution", resolution),
				zap.Error(updateErr))
		}
	} else {
		// If we can't get deduplication info, be conservative and check manually
		logger.WarnWithContext(ctx, "Failed to get deduplication info, performing manual check",
			zap.String("image_id", imageID),
			zap.String("resolution", resolution),
			zap.Error(err))

		// For non-deduplicated images or fallback, we can safely delete the resolution
		// since each image has its own files
		if !metadata.IsDeduped {
			shouldDeletePhysicalFile = true
		} else {
			// For deduplicated images without dedup info, be conservative
			shouldDeletePhysicalFile = false
			logger.WarnWithContext(ctx, "Deduplicated image without dedup info, conservatively keeping resolution",
				zap.String("image_id", imageID),
				zap.String("resolution", resolution))
		}
	}

	// Delete physical file if no other images need it
	if shouldDeletePhysicalFile {
		storageKey := metadata.GetActualStorageKey(resolution)

		// Check if file actually exists before trying to delete
		exists, existsErr := s.storage.Exists(ctx, storageKey)
		if existsErr != nil {
			logger.WarnWithContext(ctx, "Failed to check if resolution exists in storage",
				zap.String("image_id", imageID),
				zap.String("resolution", resolution),
				zap.String("storage_key", storageKey),
				zap.Error(existsErr))
			// Continue without deletion if we can't check existence
		} else if !exists {
			logger.InfoWithContext(ctx, "Resolution file doesn't exist in storage, skipping deletion",
				zap.String("image_id", imageID),
				zap.String("resolution", resolution),
				zap.String("storage_key", storageKey))
		} else {
			// File exists, proceed with deletion
			if err := s.storage.Delete(ctx, storageKey); err != nil {
				logger.WarnWithContext(ctx, "Failed to delete resolution from storage",
					zap.String("image_id", imageID),
					zap.String("resolution", resolution),
					zap.String("storage_key", storageKey),
					zap.Error(err))
				// Continue with metadata update even if storage deletion fails
			} else {
				logger.InfoWithContext(ctx, "Physical resolution file deleted",
					zap.String("image_id", imageID),
					zap.String("resolution", resolution),
					zap.String("storage_key", storageKey))
			}
		}
	} else {
		logger.InfoWithContext(ctx, "Resolution removed virtually (physical file kept for other images)",
			zap.String("image_id", imageID),
			zap.String("resolution", resolution))
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

	logger.InfoWithContext(ctx, "Resolution deleted successfully",
		zap.String("image_id", imageID),
		zap.String("resolution", resolution),
		zap.Bool("physical_file_deleted", shouldDeletePhysicalFile))

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

	// Configure resize parameters
	resizeConfig := ResizeConfig{
		Width:           resolutionConfig.Width,
		Height:          resolutionConfig.Height,
		Quality:         s.config.Image.Quality,
		Format:          mimeType,
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
	dimensions := models.ExtractDimensions(resolutionName)
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

// cleanupUploadedImages cleans up images if upload fails
func (s *ImageServiceImpl) cleanupUploadedImages(ctx context.Context, imageID string, resolutions []string) {
	logger.WarnWithContext(ctx, "Cleaning up uploaded images due to failure",
		zap.String("image_id", imageID),
		zap.Strings("resolutions", resolutions))

	for _, resolution := range resolutions {
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

// rebuildResolutionReferences rebuilds resolution references for backward compatibility
func (s *ImageServiceImpl) rebuildResolutionReferences(ctx context.Context, dedupInfo *models.DeduplicationInfo) error {
	logger.InfoWithContext(ctx, "Rebuilding resolution references",
		zap.String("hash", dedupInfo.Hash.String()),
		zap.Strings("referencing_ids", dedupInfo.ReferencingIDs))

	for _, imageID := range dedupInfo.ReferencingIDs {
		metadata, err := s.GetMetadata(ctx, imageID)
		if err != nil {
			logger.WarnWithContext(ctx, "Failed to get metadata for image during resolution rebuild",
				zap.String("image_id", imageID),
				zap.Error(err))
			continue
		}

		// Add reference for original resolution (all images have original)
		dedupInfo.AddResolutionReference("original", imageID)

		// Add references for all custom resolutions
		for _, resolution := range metadata.Resolutions {
			dedupInfo.AddResolutionReference(resolution, imageID)
		}
	}

	logger.InfoWithContext(ctx, "Resolution references rebuilt successfully",
		zap.String("hash", dedupInfo.Hash.String()),
		zap.Int("resolution_count", len(dedupInfo.ResolutionRefs)))

	return nil
}
