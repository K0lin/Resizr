package service

import (
	"context"
	"runtime"
	"sync"
	"time"

	"resizr/internal/config"
	"resizr/internal/models"
	"resizr/internal/repository"
	"resizr/internal/storage"
	"resizr/pkg/logger"

	"go.uber.org/zap"
)

// StatisticsCache holds cached statistics with timestamp
type StatisticsCache struct {
	data      *models.ResizrStatistics
	timestamp time.Time
	mu        sync.RWMutex
}

// StatisticsServiceImpl implements the StatisticsService interface
type StatisticsServiceImpl struct {
	imageRepo         repository.ImageRepository
	deduplicationRepo repository.DeduplicationRepository
	storage           storage.ImageStorage
	config            *config.Config
	startTime         time.Time
	cache             *StatisticsCache
}

// NewStatisticsService creates a new statistics service
func NewStatisticsService(
	imageRepo repository.ImageRepository,
	deduplicationRepo repository.DeduplicationRepository,
	storage storage.ImageStorage,
	config *config.Config,
) models.StatisticsService {
	return &StatisticsServiceImpl{
		imageRepo:         imageRepo,
		deduplicationRepo: deduplicationRepo,
		storage:           storage,
		config:            config,
		startTime:         time.Now(),
		cache:             &StatisticsCache{},
	}
}

// GetComprehensiveStatistics returns complete system statistics
func (s *StatisticsServiceImpl) GetComprehensiveStatistics(options *models.StatisticsOptions) (*models.ResizrStatistics, error) {
	// Check cache first if enabled
	if s.config.Statistics.CacheEnabled {
		if cached := s.getCachedStatistics(); cached != nil {
			logger.Debug("Returning cached statistics")
			return cached, nil
		}
	}

	logger.Info("Generating comprehensive ResizR statistics")

	stats := s.generateStatistics(options)

	// Cache the results if caching is enabled
	if s.config.Statistics.CacheEnabled {
		s.setCachedStatistics(stats)
	}

	logger.Info("ResizR statistics generated successfully",
		zap.Int64("total_images", stats.Images.TotalImages),
		zap.Int64("storage_used", stats.Storage.TotalStorageUsed),
		zap.Int64("deduped_images", stats.Deduplication.DedupedImages),
		zap.Float64("dedup_rate", stats.Deduplication.DeduplicationRate))

	return stats, nil
}

// GetImageStatistics returns only image-related statistics
func (s *StatisticsServiceImpl) GetImageStatistics() (*models.ImageStatistics, error) {
	ctx := context.Background()

	// Try to get detailed stats from repository
	if imageStats, err := s.imageRepo.GetImageStatistics(ctx); err == nil && imageStats != nil {
		return imageStats, nil
	}

	// Fallback to basic repository stats
	repoStats, err := s.imageRepo.GetStats(ctx)
	if err != nil {
		return nil, err
	}

	// Calculate time-based statistics
	now := time.Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	weekStart := todayStart.AddDate(0, 0, -7)
	monthStart := todayStart.AddDate(0, -1, 0)

	imagesToday, _ := s.imageRepo.GetImagesByTimeRange(ctx, todayStart, now)
	imagesWeek, _ := s.imageRepo.GetImagesByTimeRange(ctx, weekStart, now)
	imagesMonth, _ := s.imageRepo.GetImagesByTimeRange(ctx, monthStart, now)

	// Get format statistics
	formatCounts, _ := s.imageRepo.GetImageCountByFormat(ctx)
	if formatCounts == nil {
		formatCounts = make(map[string]int64)
	}

	// Get resolution statistics
	resolutionStats, _ := s.imageRepo.GetResolutionStatistics(ctx)
	if resolutionStats == nil {
		resolutionStats = []models.ResolutionStat{}
	}

	// Calculate total resolutions
	var totalResolutions int64
	resolutionCounts := make(map[string]int64)
	for _, stat := range resolutionStats {
		totalResolutions += stat.Count
		resolutionCounts[stat.Resolution] = stat.Count
	}

	return &models.ImageStatistics{
		TotalImages:        repoStats.TotalImages,
		ImagesByFormat:     formatCounts,
		ResolutionCounts:   resolutionCounts,
		ImagesCreatedToday: imagesToday,
		ImagesCreatedWeek:  imagesWeek,
		ImagesCreatedMonth: imagesMonth,
		TotalResolutions:   totalResolutions,
		TopResolutions:     resolutionStats,
	}, nil
}

// GetStorageStatistics returns only storage-related statistics
func (s *StatisticsServiceImpl) GetStorageStatistics() (*models.StorageStatistics, error) {
	ctx := context.Background()

	// Try to get detailed stats from repository
	if storageStats, err := s.imageRepo.GetStorageStatistics(ctx); err == nil && storageStats != nil {
		return storageStats, nil
	}

	// Fallback to calculating from repository stats
	repoStats, err := s.imageRepo.GetStats(ctx)
	if err != nil {
		return nil, err
	}

	// Get storage usage by resolution
	storageByResolution, _ := s.imageRepo.GetStorageUsageByResolution(ctx)
	if storageByResolution == nil {
		storageByResolution = make(map[string]int64)
	}

	// Calculate original vs processed storage
	var originalSize, processedSize int64
	for resolution, size := range storageByResolution {
		if resolution == "original" {
			originalSize += size
		} else {
			processedSize += size
		}
	}

	totalStorage := repoStats.StorageUsed
	if totalStorage == 0 {
		totalStorage = originalSize + processedSize
	}

	// Calculate compression ratio
	var compressionRatio float64 = 1.0
	if originalSize > 0 && processedSize > 0 {
		compressionRatio = float64(processedSize) / float64(originalSize)
	}

	return &models.StorageStatistics{
		TotalStorageUsed:        totalStorage,
		OriginalImagesSize:      originalSize,
		ProcessedImagesSize:     processedSize,
		StorageByResolution:     storageByResolution,
		AverageCompressionRatio: compressionRatio,
	}, nil
}

// GetDeduplicationStatistics returns only deduplication-related statistics
func (s *StatisticsServiceImpl) GetDeduplicationStatistics() (*models.DeduplicationStatistics, error) {
	ctx := context.Background()

	// Try to get detailed stats from repository
	if dedupStats, err := s.deduplicationRepo.GetDeduplicationStatistics(ctx); err == nil && dedupStats != nil {
		return dedupStats, nil
	}

	// Fallback to calculating from basic data
	duplicateCount, _ := s.deduplicationRepo.GetDuplicateCount(ctx)
	uniqueCount, _ := s.deduplicationRepo.GetUniqueHashCount(ctx)

	// Calculate deduplication rate
	totalImages := duplicateCount + uniqueCount
	var dedupRate float64
	if totalImages > 0 {
		dedupRate = float64(duplicateCount) / float64(totalImages) * 100
	}

	// Get hash statistics
	hashStats, _ := s.deduplicationRepo.GetHashStatistics(ctx)
	var mostReferenced models.HashStat
	var avgReferences int64
	if len(hashStats) > 0 {
		mostReferenced = hashStats[0] // Assume sorted by reference count
		var totalReferences int64
		for _, stat := range hashStats {
			if stat.ReferenceCount > mostReferenced.ReferenceCount {
				mostReferenced = stat
			}
			totalReferences += stat.ReferenceCount
		}
		avgReferences = totalReferences / int64(len(hashStats))
	}

	return &models.DeduplicationStatistics{
		TotalDuplicatesFound:     duplicateCount,
		DedupedImages:            duplicateCount,
		UniqueImages:             uniqueCount,
		DeduplicationRate:        dedupRate,
		AverageReferencesPerHash: avgReferences,
	}, nil
}

// getSystemStatistics returns system-level statistics
func (s *StatisticsServiceImpl) getSystemStatistics() models.SystemStatistics {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	uptime := time.Since(s.startTime)

	return models.SystemStatistics{
		UptimeSeconds:   int64(uptime.Seconds()),
		GoVersion:       runtime.Version(),
		Version:         "4.0.0", // Should come from config or build info
		CPUCount:        runtime.NumCPU(),
		MemoryAllocated: memStats.Alloc,
		MemoryTotal:     memStats.TotalAlloc,
		MemorySystem:    memStats.Sys,
		GoroutineCount:  runtime.NumGoroutine(),
		GCCycles:        memStats.NumGC,
		LastGCPause:     time.Duration(memStats.PauseNs[(memStats.NumGC+255)%256]),
	}
}

// RefreshStatistics forces a refresh of cached statistics
func (s *StatisticsServiceImpl) RefreshStatistics() error {
	logger.Info("Refreshing ResizR statistics cache")

	if s.config.Statistics.CacheEnabled {
		// Clear the cache to force regeneration
		s.cache.mu.Lock()
		s.cache.data = nil
		s.cache.timestamp = time.Time{}
		s.cache.mu.Unlock()

		logger.Info("Statistics cache cleared, will regenerate on next request")
	}

	return nil
}

// getCachedStatistics returns cached statistics if valid and not expired
func (s *StatisticsServiceImpl) getCachedStatistics() *models.ResizrStatistics {
	s.cache.mu.RLock()
	defer s.cache.mu.RUnlock()

	if s.cache.data == nil {
		return nil
	}

	// Check if cache is expired
	if time.Since(s.cache.timestamp) > s.config.Statistics.CacheTTL {
		logger.Debug("Statistics cache expired",
			zap.Duration("age", time.Since(s.cache.timestamp)),
			zap.Duration("ttl", s.config.Statistics.CacheTTL))
		return nil
	}

	logger.Debug("Statistics cache hit",
		zap.Duration("age", time.Since(s.cache.timestamp)),
		zap.Duration("ttl", s.config.Statistics.CacheTTL))

	return s.cache.data
}

// setCachedStatistics stores statistics in cache
func (s *StatisticsServiceImpl) setCachedStatistics(stats *models.ResizrStatistics) {
	s.cache.mu.Lock()
	defer s.cache.mu.Unlock()

	s.cache.data = stats
	s.cache.timestamp = time.Now()

	logger.Debug("Statistics cached",
		zap.Duration("ttl", s.config.Statistics.CacheTTL))
}

// generateStatistics generates fresh statistics (bypasses cache)
func (s *StatisticsServiceImpl) generateStatistics(options *models.StatisticsOptions) *models.ResizrStatistics {
	stats := &models.ResizrStatistics{
		Timestamp: time.Now(),
	}

	// Get image statistics
	if imageStats, err := s.GetImageStatistics(); err != nil {
		logger.Error("Failed to get image statistics", zap.Error(err))
		// Don't return error, continue with partial stats
	} else {
		stats.Images = *imageStats
	}

	// Get storage statistics
	if storageStats, err := s.GetStorageStatistics(); err != nil {
		logger.Error("Failed to get storage statistics", zap.Error(err))
	} else {
		stats.Storage = *storageStats
	}

	// Get deduplication statistics
	if dedupStats, err := s.GetDeduplicationStatistics(); err != nil {
		logger.Error("Failed to get deduplication statistics", zap.Error(err))
	} else {
		stats.Deduplication = *dedupStats
	}

	// Get system statistics
	if options == nil || options.IncludeSystemMetrics {
		stats.System = s.getSystemStatistics()
	}

	return stats
}
