package service

import (
	"context"
	"runtime"
	"time"

	"resizr/internal/repository"
	"resizr/internal/storage"
	"resizr/pkg/logger"

	"go.uber.org/zap"
)

// HealthServiceImpl implements the HealthService interface
type HealthServiceImpl struct {
	repo      repository.ImageRepository
	storage   storage.ImageStorage
	startTime time.Time
	version   string
}

// NewHealthService creates a new health service
func NewHealthService(
	repo repository.ImageRepository,
	storage storage.ImageStorage,
	version string,
) HealthService {
	return &HealthServiceImpl{
		repo:      repo,
		storage:   storage,
		startTime: time.Now(),
		version:   version,
	}
}

// CheckHealth performs comprehensive health check
func (s *HealthServiceImpl) CheckHealth(ctx context.Context) (*HealthStatus, error) {
	logger.DebugWithContext(ctx, "Starting health check")

	services := make(map[string]string)

	// Check Redis/Repository health
	if err := s.repo.Health(ctx); err != nil {
		logger.WarnWithContext(ctx, "Redis health check failed",
			zap.Error(err))
		services["redis"] = "unhealthy: " + err.Error()
	} else {
		services["redis"] = "connected"
	}

	// Check S3/Storage health
	if err := s.storage.Health(ctx); err != nil {
		logger.WarnWithContext(ctx, "S3 health check failed",
			zap.Error(err))
		services["s3"] = "unhealthy: " + err.Error()
	} else {
		services["s3"] = "connected"
	}

	// Add application info
	services["application"] = "healthy"

	uptime := time.Since(s.startTime).Milliseconds()

	status := &HealthStatus{
		Services: services,
		Uptime:   uptime,
		Version:  s.version,
	}

	logger.InfoWithContext(ctx, "Health check completed",
		zap.Any("services", services),
		zap.Int64("uptime", uptime))

	return status, nil
}

// GetMetrics retrieves system metrics
func (s *HealthServiceImpl) GetMetrics(ctx context.Context) (map[string]interface{}, error) {
	logger.DebugWithContext(ctx, "Collecting system metrics")

	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	metrics := map[string]interface{}{
		"system": map[string]interface{}{
			"uptime_milliseconds": time.Since(s.startTime).Milliseconds(),
			"version":             s.version,
			"go_version":          runtime.Version(),
			"goroutines":          runtime.NumGoroutine(),
			"cpu_count":           runtime.NumCPU(),
		},
		"memory": map[string]interface{}{
			"alloc_bytes":       memStats.Alloc,
			"total_alloc_bytes": memStats.TotalAlloc,
			"sys_bytes":         memStats.Sys,
			"heap_alloc_bytes":  memStats.HeapAlloc,
			"heap_sys_bytes":    memStats.HeapSys,
			"heap_objects":      memStats.HeapObjects,
			"gc_runs":           memStats.NumGC,
			"gc_pause_ns":       memStats.PauseNs[(memStats.NumGC+255)%256],
		},
		"timestamp": time.Now().Unix(),
	}

	// Try to get repository stats
	if repoStats, err := s.repo.GetStats(ctx); err == nil && repoStats != nil {
		metrics["repository"] = map[string]interface{}{
			"total_images": repoStats.TotalImages,
			"cache_hits":   repoStats.CacheHits,
			"cache_misses": repoStats.CacheMisses,
		}
	}

	logger.DebugWithContext(ctx, "System metrics collected",
		zap.Int("goroutines", runtime.NumGoroutine()),
		zap.Uint64("heap_alloc_mb", memStats.HeapAlloc/1024/1024))

	return metrics, nil
}

// RepositoryStats represents repository statistics
type RepositoryStats struct {
	TotalImages int64 `json:"total_images"`
	CacheHits   int64 `json:"cache_hits"`
	CacheMisses int64 `json:"cache_misses"`
}
