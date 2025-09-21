package models

import "time"

// StatisticsService defines the interface for statistics operations
type StatisticsService interface {
	GetComprehensiveStatistics(options *StatisticsOptions) (*ResizrStatistics, error)
	GetImageStatistics() (*ImageStatistics, error)
	GetStorageStatistics() (*StorageStatistics, error)
	GetDeduplicationStatistics() (*DeduplicationStatistics, error)
	RefreshStatistics() error
}

// StatisticsOptions represents options for statistics retrieval
type StatisticsOptions struct {
	IncludeDetailedBreakdown  bool       `json:"include_detailed_breakdown"`
	IncludePerformanceMetrics bool       `json:"include_performance_metrics"`
	IncludeSystemMetrics      bool       `json:"include_system_metrics"`
	TimeRange                 *TimeRange `json:"time_range,omitempty"`
}

// ResizrStatistics represents comprehensive system statistics
type ResizrStatistics struct {
	Images        ImageStatistics         `json:"images"`
	Storage       StorageStatistics       `json:"storage"`
	Deduplication DeduplicationStatistics `json:"deduplication"`
	System        SystemStatistics        `json:"system"`
	Timestamp     time.Time               `json:"timestamp"`
}

// ImageStatistics represents statistics about images
type ImageStatistics struct {
	TotalImages        int64            `json:"total_images"`
	ImagesByFormat     map[string]int64 `json:"images_by_format"`
	ResolutionCounts   map[string]int64 `json:"resolution_counts"`
	ImagesCreatedToday int64            `json:"images_created_today"`
	ImagesCreatedWeek  int64            `json:"images_created_week"`
	ImagesCreatedMonth int64            `json:"images_created_month"`
	TotalResolutions   int64            `json:"total_resolutions"`
	TopResolutions     []ResolutionStat `json:"top_resolutions"`
}

// StorageStatistics represents storage usage statistics
type StorageStatistics struct {
	TotalStorageUsed        int64            `json:"total_storage_used_bytes"`
	OriginalImagesSize      int64            `json:"original_images_size_bytes"`
	ProcessedImagesSize     int64            `json:"processed_images_size_bytes"`
	StorageByResolution     map[string]int64 `json:"storage_by_resolution_bytes"`
	AverageCompressionRatio float64          `json:"average_compression_ratio"`
}

// DeduplicationStatistics represents deduplication statistics
type DeduplicationStatistics struct {
	TotalDuplicatesFound     int64   `json:"total_duplicates_found"`
	DedupedImages            int64   `json:"deduped_images"`
	UniqueImages             int64   `json:"unique_images"`
	DeduplicationRate        float64 `json:"deduplication_rate_percent"`
	AverageReferencesPerHash int64   `json:"average_references_per_hash"`
}

// SystemStatistics represents system-level statistics
type SystemStatistics struct {
	UptimeSeconds   int64         `json:"uptime_seconds"`
	GoVersion       string        `json:"go_version"`
	Version         string        `json:"version"`
	CPUCount        int           `json:"cpu_count"`
	MemoryAllocated uint64        `json:"memory_allocated_bytes"`
	MemoryTotal     uint64        `json:"memory_total_bytes"`
	MemorySystem    uint64        `json:"memory_system_bytes"`
	GoroutineCount  int           `json:"goroutine_count"`
	GCCycles        uint32        `json:"gc_cycles"`
	LastGCPause     time.Duration `json:"last_gc_pause_ns"`
}

// ResolutionStat represents statistics for a specific resolution
type ResolutionStat struct {
	Resolution string `json:"resolution"`
	Count      int64  `json:"count"`
}

// HashStat represents statistics for a hash
type HashStat struct {
	Hash           string `json:"hash"`
	ReferenceCount int64  `json:"reference_count"`
	StorageKey     string `json:"storage_key"`
	TotalSizeBytes int64  `json:"total_size_bytes"`
}

// TimeRange represents a time range for filtering statistics
type TimeRange struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}
