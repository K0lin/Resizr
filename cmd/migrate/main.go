package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"time"

	"resizr/internal/config"
	"resizr/internal/repository"
	"resizr/pkg/logger"

	"go.uber.org/zap"
)

const (
	AppName    = "Resizr Migration Tool"
	AppVersion = "0.0.1"
)

func main() {
	// Command-line flags
	dryRun := flag.Bool("dry-run", false, "Run in dry-run mode (no changes made)")
	batchSize := flag.Int("batch-size", 100, "Number of images to process per batch")
	flag.Parse()

	if err := run(*dryRun, *batchSize); err != nil {
		log.Fatalf("Migration failed: %v", err)
	}
}

func run(dryRun bool, batchSize int) error {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Initialize logger
	if err := logger.Init(logger.Config{
		Level:  cfg.Logger.Level,
		Format: cfg.Logger.Format,
	}); err != nil {
		return fmt.Errorf("failed to initialize logger: %w", err)
	}

	logger.Info("Starting migration",
		zap.String("app", AppName),
		zap.String("version", AppVersion),
		zap.Bool("dry_run", dryRun),
		zap.Int("batch_size", batchSize),
		zap.String("cache_type", cfg.Cache.Type))

	ctx := context.Background()

	// Initialize repository using factory method
	imageRepo, err := repository.NewImageRepository(cfg)
	if err != nil {
		return fmt.Errorf("failed to initialize repository: %w", err)
	}
	defer imageRepo.Close()

	// Cast to deduplication repository
	dedupRepo, ok := imageRepo.(repository.DeduplicationRepository)
	if !ok {
		return fmt.Errorf("repository does not implement DeduplicationRepository interface")
	}

	logger.Info("Repository initialized successfully",
		zap.String("cache_type", cfg.Cache.Type))

	// Run migration
	stats, err := migrateResolutionReferences(ctx, imageRepo, dedupRepo, dryRun, batchSize)
	if err != nil {
		return fmt.Errorf("migration failed: %w", err)
	}

	// Print summary
	logger.Info("Migration completed successfully",
		zap.Int("total_images", stats.TotalImages),
		zap.Int("total_resolutions", stats.TotalResolutions),
		zap.Int("refs_created", stats.RefsCreated),
		zap.Int("errors", stats.Errors),
		zap.Duration("duration", stats.Duration),
		zap.Bool("dry_run", dryRun))

	if dryRun {
		fmt.Println("\n✓ DRY RUN completed - no changes were made")
	} else {
		fmt.Println("\n✓ Migration completed successfully")
	}

	fmt.Printf("\nStatistics:\n")
	fmt.Printf("  Total images processed: %d\n", stats.TotalImages)
	fmt.Printf("  Total resolutions: %d\n", stats.TotalResolutions)
	fmt.Printf("  Reference entries created: %d\n", stats.RefsCreated)
	fmt.Printf("  Errors encountered: %d\n", stats.Errors)
	fmt.Printf("  Duration: %s\n", stats.Duration)

	return nil
}

type MigrationStats struct {
	TotalImages      int
	TotalResolutions int
	RefsCreated      int
	Errors           int
	Duration         time.Duration
}

func migrateResolutionReferences(
	ctx context.Context,
	imageRepo repository.ImageRepository,
	dedupRepo repository.DeduplicationRepository,
	dryRun bool,
	batchSize int,
) (*MigrationStats, error) {
	startTime := time.Now()
	stats := &MigrationStats{}

	logger.Info("Fetching all images from repository")

	// Get all image IDs
	imageIDs, err := getAllImageIDs(ctx, imageRepo)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch image IDs: %w", err)
	}

	logger.Info("Found images to process",
		zap.Int("count", len(imageIDs)))

	// Process images in batches
	for i := 0; i < len(imageIDs); i += batchSize {
		end := i + batchSize
		if end > len(imageIDs) {
			end = len(imageIDs)
		}

		batch := imageIDs[i:end]
		logger.Info("Processing batch",
			zap.Int("batch_start", i+1),
			zap.Int("batch_end", end),
			zap.Int("total", len(imageIDs)))

		for _, imageID := range batch {
			if err := processSingleImage(ctx, imageID, imageRepo, dedupRepo, dryRun, stats); err != nil {
				logger.Error("Failed to process image",
					zap.String("image_id", imageID),
					zap.Error(err))
				stats.Errors++
				// Continue with next image instead of failing
			}
		}
	}

	stats.Duration = time.Since(startTime)
	return stats, nil
}

func processSingleImage(
	ctx context.Context,
	imageID string,
	imageRepo repository.ImageRepository,
	dedupRepo repository.DeduplicationRepository,
	dryRun bool,
	stats *MigrationStats,
) error {
	// Get image metadata
	metadata, err := imageRepo.Get(ctx, imageID)
	if err != nil {
		return fmt.Errorf("failed to get metadata: %w", err)
	}

	stats.TotalImages++

	// Skip images without hash (legacy non-deduplicated images)
	if metadata.Hash.Value == "" {
		logger.Debug("Skipping image without hash",
			zap.String("image_id", imageID))
		return nil
	}

	// Get all resolutions for this image
	allResolutions := append([]string{"original"}, metadata.Resolutions...)
	stats.TotalResolutions += len(allResolutions)

	logger.Debug("Processing image",
		zap.String("image_id", imageID),
		zap.String("hash", metadata.Hash.String()),
		zap.Strings("resolutions", allResolutions),
		zap.Bool("is_deduped", metadata.IsDeduped))

	// Add reference for each resolution
	for _, resolution := range allResolutions {
		if dryRun {
			logger.Debug("DRY RUN: Would add resolution reference",
				zap.String("image_id", imageID),
				zap.String("hash", metadata.Hash.String()),
				zap.String("resolution", resolution))
			stats.RefsCreated++
		} else {
			// Add atomic reference
			if err := dedupRepo.AddResolutionRef(ctx, metadata.Hash, resolution, imageID); err != nil {
				return fmt.Errorf("failed to add resolution ref for %s: %w", resolution, err)
			}
			stats.RefsCreated++

			logger.Debug("Added resolution reference",
				zap.String("image_id", imageID),
				zap.String("hash", metadata.Hash.String()),
				zap.String("resolution", resolution))
		}
	}

	return nil
}

func getAllImageIDs(ctx context.Context, imageRepo repository.ImageRepository) ([]string, error) {
	const fetchBatchSize = 1000
	var allIDs []string
	offset := 0

	logger.Info("Fetching all image IDs using pagination")

	for {
		// Fetch batch of images
		images, err := imageRepo.List(ctx, offset, fetchBatchSize)
		if err != nil {
			return nil, fmt.Errorf("failed to list images at offset %d: %w", offset, err)
		}

		// No more images
		if len(images) == 0 {
			break
		}

		// Extract IDs
		for _, img := range images {
			allIDs = append(allIDs, img.ID)
		}

		logger.Debug("Fetched batch of image IDs",
			zap.Int("offset", offset),
			zap.Int("count", len(images)),
			zap.Int("total_so_far", len(allIDs)))

		// Check if we got fewer than batch size (last batch)
		if len(images) < fetchBatchSize {
			break
		}

		offset += fetchBatchSize
	}

	return allIDs, nil
}
