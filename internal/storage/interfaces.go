package storage

import (
	"context"
	"io"
	"time"
)

// ImageStorage defines the interface for image file operations
type ImageStorage interface {
	// Upload uploads a file to storage
	Upload(ctx context.Context, key string, reader io.Reader, size int64, contentType string) error

	// Download downloads a file from storage as a stream
	Download(ctx context.Context, key string) (io.ReadCloser, error)

	// Delete removes a file from storage
	Delete(ctx context.Context, key string) error

	// DeleteFolder removes all files in a folder recursively
	DeleteFolder(ctx context.Context, prefix string) error

	// Exists checks if a file exists in storage
	Exists(ctx context.Context, key string) (bool, error)

	// GetMetadata retrieves file metadata (size, last modified, etc.)
	GetMetadata(ctx context.Context, key string) (*FileMetadata, error)

	// GeneratePresignedURL generates a pre-signed URL for direct access
	GeneratePresignedURL(ctx context.Context, key string, expiration time.Duration) (string, error)

	// ListObjects lists objects with a given prefix
	ListObjects(ctx context.Context, prefix string, maxKeys int) ([]ObjectInfo, error)

	// CopyObject copies an object to a new location
	CopyObject(ctx context.Context, sourceKey, destKey string) error

	// GetURL returns the public URL for an object (if bucket is public)
	GetURL(key string) string

	// Health checks storage service health
	Health(ctx context.Context) error
}

// FileMetadata represents metadata about a stored file
type FileMetadata struct {
	Key          string            `json:"key"`
	Size         int64             `json:"size"`
	ContentType  string            `json:"content_type"`
	ETag         string            `json:"etag"`
	LastModified time.Time         `json:"last_modified"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

// ObjectInfo represents information about a stored object
type ObjectInfo struct {
	Key          string    `json:"key"`
	Size         int64     `json:"size"`
	LastModified time.Time `json:"last_modified"`
	ETag         string    `json:"etag"`
	ContentType  string    `json:"content_type"`
}

// UploadOptions provides options for upload operations
type UploadOptions struct {
	ContentType          string            `json:"content_type"`
	ContentEncoding      string            `json:"content_encoding,omitempty"`
	CacheControl         string            `json:"cache_control,omitempty"`
	Metadata             map[string]string `json:"metadata,omitempty"`
	ServerSideEncryption bool              `json:"server_side_encryption"`
}

// DownloadOptions provides options for download operations
type DownloadOptions struct {
	Range     *ByteRange `json:"range,omitempty"`      // For partial content
	VersionID string     `json:"version_id,omitempty"` // For versioned objects
}

// ByteRange represents a byte range for partial downloads
type ByteRange struct {
	Start int64 `json:"start"`
	End   int64 `json:"end"`
}

// StorageStats represents storage usage statistics
type StorageStats struct {
	TotalObjects   int64     `json:"total_objects"`
	TotalSize      int64     `json:"total_size_bytes"`
	UsedSpace      int64     `json:"used_space_bytes"`
	AvailableSpace int64     `json:"available_space_bytes,omitempty"`
	LastSync       time.Time `json:"last_sync"`
}

// MultipartUpload interface for large file uploads
type MultipartUpload interface {
	// InitiateMultipartUpload starts a multipart upload
	InitiateMultipartUpload(ctx context.Context, key string, options *UploadOptions) (string, error)

	// UploadPart uploads a part of a multipart upload
	UploadPart(ctx context.Context, uploadID string, partNumber int, reader io.Reader) (string, error)

	// CompleteMultipartUpload completes a multipart upload
	CompleteMultipartUpload(ctx context.Context, uploadID string, parts []CompletedPart) error

	// AbortMultipartUpload aborts a multipart upload
	AbortMultipartUpload(ctx context.Context, uploadID string) error
}

// CompletedPart represents a completed part in multipart upload
type CompletedPart struct {
	PartNumber int    `json:"part_number"`
	ETag       string `json:"etag"`
}

// StorageConfig represents storage configuration
type StorageConfig struct {
	Provider       string `json:"provider"` // "s3", "gcs", "azure", etc.
	Region         string `json:"region"`
	Bucket         string `json:"bucket"`
	Endpoint       string `json:"endpoint"`
	AccessKey      string `json:"access_key"`
	SecretKey      string `json:"secret_key"`
	UseSSL         bool   `json:"use_ssl"`
	ForcePathStyle bool   `json:"force_path_style"` // For MinIO compatibility
}

// BatchUploadOperation represents a batch upload operation
type BatchUploadOperation struct {
	Key         string         `json:"key"`
	Reader      io.Reader      `json:"-"`
	Size        int64          `json:"size"`
	ContentType string         `json:"content_type"`
	Options     *UploadOptions `json:"options,omitempty"`
}

// BatchDeleteOperation represents a batch delete operation
type BatchDeleteOperation struct {
	Key string `json:"key"`
}

// BatchStorage interface for batch operations
type BatchStorage interface {
	// BatchUpload uploads multiple files in a single operation
	BatchUpload(ctx context.Context, operations []BatchUploadOperation) ([]BatchResult, error)

	// BatchDelete deletes multiple files in a single operation
	BatchDelete(ctx context.Context, operations []BatchDeleteOperation) ([]BatchResult, error)
}

// BatchResult represents the result of a batch operation
type BatchResult struct {
	Key     string `json:"key"`
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

// StorageProvider represents different storage providers
type StorageProvider string

const (
	_ProviderS3    StorageProvider = "s3"
	_ProviderGCS   StorageProvider = "gcs"
	_ProviderAzure StorageProvider = "azure"
	_ProviderMinIO StorageProvider = "minio"
	_ProviderLocal StorageProvider = "local"
)
