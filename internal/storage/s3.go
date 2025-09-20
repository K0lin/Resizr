package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"resizr/internal/config"
	"resizr/pkg/logger"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"go.uber.org/zap"
)

// S3Storage implements ImageStorage interface for AWS S3 and S3-compatible storage
type S3Storage struct {
	client     *s3.Client
	uploader   *manager.Uploader
	downloader *manager.Downloader
	config     *config.S3Config
	bucket     string
}

// NewS3Storage creates a new S3 storage instance
func NewS3Storage(cfg *config.S3Config) (ImageStorage, error) {
	logger.Info("Initializing S3 storage",
		zap.String("endpoint", cfg.Endpoint),
		zap.String("region", cfg.Region),
		zap.String("bucket", cfg.Bucket),
		zap.Bool("use_ssl", cfg.UseSSL))

	// Create AWS config
	awsConfig, err := createAWSConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create AWS config: %w", err)
	}

	// Create S3 client
	client := s3.NewFromConfig(awsConfig, func(o *s3.Options) {
		if cfg.Endpoint != "https://s3.amazonaws.com" {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
			o.UsePathStyle = true // Required for MinIO and custom endpoints
		}
	})

	// Create upload/download managers
	uploader := manager.NewUploader(client, func(u *manager.Uploader) {
		u.PartSize = 10 * 1024 * 1024 // 10MB parts for multipart uploads
		u.Concurrency = 3             // 3 concurrent uploads
	})

	downloader := manager.NewDownloader(client, func(d *manager.Downloader) {
		d.PartSize = 10 * 1024 * 1024 // 10MB parts for concurrent downloads
		d.Concurrency = 3             // 3 concurrent downloads
	})

	storage := &S3Storage{
		client:     client,
		uploader:   uploader,
		downloader: downloader,
		config:     cfg,
		bucket:     cfg.Bucket,
	}

	// Test connection
	if err := storage.Health(context.Background()); err != nil {
		return nil, fmt.Errorf("S3 health check failed: %w", err)
	}

	logger.Info("S3 storage initialized successfully")
	return storage, nil
}

// Upload uploads a file to S3
func (s *S3Storage) Upload(ctx context.Context, key string, reader io.Reader, size int64, contentType string) error {
	logger.DebugWithContext(ctx, "Uploading file to S3",
		zap.String("key", key),
		zap.Int64("size", size),
		zap.String("content_type", contentType))

	// Prepare upload input
	uploadInput := &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(key),
		Body:        reader,
		ContentType: aws.String(contentType),
	}

	// Set content length if known
	if size > 0 {
		uploadInput.ContentLength = aws.Int64(size)
	}

	// Set cache control headers for images
	if strings.HasPrefix(contentType, "image/") {
		uploadInput.CacheControl = aws.String("public, max-age=31536000, immutable") // 1 year
	}

	// Use uploader for large files (handles multipart automatically)
	if size > 10*1024*1024 { // > 10MB
		_, err := s.uploader.Upload(ctx, &s3.PutObjectInput{
			Bucket:        uploadInput.Bucket,
			Key:           uploadInput.Key,
			Body:          uploadInput.Body,
			ContentType:   uploadInput.ContentType,
			ContentLength: uploadInput.ContentLength,
			CacheControl:  uploadInput.CacheControl,
		})
		if err != nil {
			logger.ErrorWithContext(ctx, "Failed to upload large file to S3",
				zap.String("key", key),
				zap.Int64("size", size),
				zap.Error(err))
			return fmt.Errorf("failed to upload file: %w", err)
		}
	} else {
		// Use regular PutObject for smaller files
		_, err := s.client.PutObject(ctx, uploadInput)
		if err != nil {
			logger.ErrorWithContext(ctx, "Failed to upload file to S3",
				zap.String("key", key),
				zap.Int64("size", size),
				zap.Error(err))
			return fmt.Errorf("failed to upload file: %w", err)
		}
	}

	logger.DebugWithContext(ctx, "File uploaded to S3 successfully",
		zap.String("key", key),
		zap.Int64("size", size))

	return nil
}

// Download downloads a file from S3 as a stream
func (s *S3Storage) Download(ctx context.Context, key string) (io.ReadCloser, error) {
	logger.DebugWithContext(ctx, "Downloading file from S3",
		zap.String("key", key))

	// Get object from S3
	result, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		logger.ErrorWithContext(ctx, "Failed to download file from S3",
			zap.String("key", key),
			zap.Error(err))

		// Convert S3 errors to our error types
		if isNotFoundError(err) {
			return nil, fmt.Errorf("file not found: %s", key)
		}
		return nil, fmt.Errorf("failed to download file: %w", err)
	}

	logger.DebugWithContext(ctx, "File downloaded from S3 successfully",
		zap.String("key", key))

	return result.Body, nil
}

// Delete removes a file from S3
func (s *S3Storage) Delete(ctx context.Context, key string) error {
	logger.DebugWithContext(ctx, "Deleting file from S3",
		zap.String("key", key))

	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		logger.ErrorWithContext(ctx, "Failed to delete file from S3",
			zap.String("key", key),
			zap.Error(err))
		return fmt.Errorf("failed to delete file: %w", err)
	}

	logger.DebugWithContext(ctx, "File deleted from S3 successfully",
		zap.String("key", key))

	return nil
}

// DeleteFolder removes all files in a folder recursively using custom S3 API
func (s *S3Storage) DeleteFolder(ctx context.Context, prefix string) error {
	logger.DebugWithContext(ctx, "Deleting folder from S3",
		zap.String("prefix", prefix))

	// Ensure prefix ends with / for folder deletion
	if !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}

	// Extract base endpoint from S3 endpoint
	baseEndpoint := strings.TrimSuffix(s.config.Endpoint, "/")
	if strings.HasSuffix(baseEndpoint, "/s3") {
		baseEndpoint = strings.TrimSuffix(baseEndpoint, "/s3")
	}

	// Build custom API URL for recursive folder deletion
	// Format: https://s3.site/api/v1/buckets/{bucket}/objects?prefix={prefix}&recursive=true
	deleteURL := fmt.Sprintf("%s/api/v1/buckets/%s/objects", baseEndpoint, s.bucket)

	// Add query parameters
	params := url.Values{}
	params.Add("prefix", prefix)
	params.Add("all_versions", "false")
	params.Add("bypass", "false")
	params.Add("recursive", "true")

	fullURL := fmt.Sprintf("%s?%s", deleteURL, params.Encode())

	logger.DebugWithContext(ctx, "Making DELETE request to custom S3 API",
		zap.String("url", fullURL))

	// Create HTTP DELETE request
	req, err := http.NewRequestWithContext(ctx, "DELETE", fullURL, nil)
	if err != nil {
		logger.ErrorWithContext(ctx, "Failed to create DELETE request",
			zap.String("prefix", prefix),
			zap.Error(err))
		return fmt.Errorf("failed to create delete request: %w", err)
	}

	// Add authentication headers if needed
	// For MinIO Console API, we might need different auth
	req.Header.Set("Content-Type", "application/json")

	// Execute the request
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		logger.ErrorWithContext(ctx, "Failed to execute folder delete request",
			zap.String("prefix", prefix),
			zap.Error(err))
		return fmt.Errorf("failed to delete folder: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			// Log the error but don't return it as it's a cleanup operation
			logger.Warn("Failed to close response body", zap.Error(err))
		}
	}()

	// Read response body for debugging
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		logger.ErrorWithContext(ctx, "Folder delete request failed",
			zap.String("prefix", prefix),
			zap.Int("status_code", resp.StatusCode),
			zap.String("response", string(body)))
		return fmt.Errorf("folder delete failed with status %d: %s", resp.StatusCode, string(body))
	}

	logger.InfoWithContext(ctx, "Folder deleted from S3 successfully",
		zap.String("prefix", prefix),
		zap.Int("status_code", resp.StatusCode))

	return nil
}

// Exists checks if a file exists in S3
func (s *S3Storage) Exists(ctx context.Context, key string) (bool, error) {
	_, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		if isNotFoundError(err) {
			return false, nil
		}

		// If we get a 403 Forbidden error, it likely means we don't have HeadObject permissions
		// but the file might still exist. We should assume it exists to avoid breaking deduplication.
		if strings.Contains(err.Error(), "403") || strings.Contains(err.Error(), "Forbidden") {
			logger.WarnWithContext(ctx, "HeadObject permission denied, assuming file exists for deduplication",
				zap.String("key", key),
				zap.Error(err))
			return true, nil
		}

		return false, fmt.Errorf("failed to check file existence: %w", err)
	}

	return true, nil
}

// GetMetadata retrieves file metadata
func (s *S3Storage) GetMetadata(ctx context.Context, key string) (*FileMetadata, error) {
	logger.DebugWithContext(ctx, "Getting file metadata from S3",
		zap.String("key", key))

	result, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		if isNotFoundError(err) {
			return nil, fmt.Errorf("file not found: %s", key)
		}
		return nil, fmt.Errorf("failed to get metadata: %w", err)
	}

	metadata := &FileMetadata{
		Key:          key,
		Size:         aws.ToInt64(result.ContentLength),
		ContentType:  aws.ToString(result.ContentType),
		ETag:         aws.ToString(result.ETag),
		LastModified: aws.ToTime(result.LastModified),
		Metadata:     make(map[string]string),
	}

	// Copy user metadata
	for k, v := range result.Metadata {
		metadata.Metadata[k] = v
	}

	return metadata, nil
}

// GeneratePresignedURL generates a pre-signed URL
func (s *S3Storage) GeneratePresignedURL(ctx context.Context, key string, expiration time.Duration) (string, error) {
	logger.DebugWithContext(ctx, "Generating pre-signed URL",
		zap.String("key", key),
		zap.Duration("expiration", expiration))

	// Create pre-sign client
	presignClient := s3.NewPresignClient(s.client)

	// Generate pre-signed GET request
	presignResult, err := presignClient.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	}, func(opts *s3.PresignOptions) {
		opts.Expires = expiration
	})
	if err != nil {
		logger.ErrorWithContext(ctx, "Failed to generate pre-signed URL",
			zap.String("key", key),
			zap.Error(err))
		return "", fmt.Errorf("failed to generate pre-signed URL: %w", err)
	}

	logger.DebugWithContext(ctx, "Pre-signed URL generated successfully",
		zap.String("key", key),
		zap.Duration("expiration", expiration))

	return presignResult.URL, nil
}

// ListObjects lists objects with a given prefix
func (s *S3Storage) ListObjects(ctx context.Context, prefix string, maxKeys int) ([]ObjectInfo, error) {
	logger.DebugWithContext(ctx, "Listing objects from S3",
		zap.String("prefix", prefix),
		zap.Int("max_keys", maxKeys))

	input := &s3.ListObjectsV2Input{
		Bucket: aws.String(s.bucket),
		Prefix: aws.String(prefix),
	}

	if maxKeys > 0 {
		input.MaxKeys = aws.Int32(int32(maxKeys))
	}

	result, err := s.client.ListObjectsV2(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to list objects: %w", err)
	}

	objects := make([]ObjectInfo, len(result.Contents))
	for i, obj := range result.Contents {
		objects[i] = ObjectInfo{
			Key:          aws.ToString(obj.Key),
			Size:         aws.ToInt64(obj.Size),
			LastModified: aws.ToTime(obj.LastModified),
			ETag:         aws.ToString(obj.ETag),
		}
	}

	logger.DebugWithContext(ctx, "Objects listed successfully",
		zap.String("prefix", prefix),
		zap.Int("count", len(objects)))

	return objects, nil
}

// CopyObject copies an object to a new location
func (s *S3Storage) CopyObject(ctx context.Context, sourceKey, destKey string) error {
	logger.DebugWithContext(ctx, "Copying object in S3",
		zap.String("source_key", sourceKey),
		zap.String("dest_key", destKey))

	copySource := fmt.Sprintf("%s/%s", s.bucket, sourceKey)

	_, err := s.client.CopyObject(ctx, &s3.CopyObjectInput{
		Bucket:     aws.String(s.bucket),
		CopySource: aws.String(copySource),
		Key:        aws.String(destKey),
	})
	if err != nil {
		logger.ErrorWithContext(ctx, "Failed to copy object",
			zap.String("source_key", sourceKey),
			zap.String("dest_key", destKey),
			zap.Error(err))
		return fmt.Errorf("failed to copy object: %w", err)
	}

	logger.DebugWithContext(ctx, "Object copied successfully",
		zap.String("source_key", sourceKey),
		zap.String("dest_key", destKey))

	return nil
}

// GetURL returns the public URL for an object
func (s *S3Storage) GetURL(key string) string {
	if s.config.UseSSL {
		if s.config.Endpoint == "https://s3.amazonaws.com" {
			return fmt.Sprintf("https://%s.s3.amazonaws.com/%s", s.bucket, key)
		}
		return fmt.Sprintf("%s/%s/%s", s.config.Endpoint, s.bucket, key)
	}

	return fmt.Sprintf("http://%s/%s/%s",
		strings.TrimPrefix(s.config.Endpoint, "http://"), s.bucket, key)
}

// Health checks storage service health
func (s *S3Storage) Health(ctx context.Context) error {
	// Check if we can list bucket (basic connectivity test)
	_, err := s.client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket:  aws.String(s.bucket),
		MaxKeys: aws.Int32(1),
	})
	if err != nil {
		return fmt.Errorf("S3 health check failed: %w", err)
	}

	// Test write permissions with a health check object
	healthKey := fmt.Sprintf("health-check/%d", time.Now().Unix())

	// Try to put a small test object
	_, err = s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(healthKey),
		Body:        strings.NewReader("health-check"),
		ContentType: aws.String("text/plain"),
	})
	if err != nil {
		return fmt.Errorf("S3 write test failed: %w", err)
	}

	// Clean up test object
	_, err = s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(healthKey),
	})
	if err != nil {
		logger.WarnWithContext(ctx, "Failed to cleanup health check object",
			zap.String("key", healthKey),
			zap.Error(err))
		// Not a critical error for health check
	}

	return nil
}

// Helper functions

// createAWSConfig creates AWS configuration
func createAWSConfig(cfg *config.S3Config) (aws.Config, error) {
	// Static credentials provider
	credProvider := credentials.NewStaticCredentialsProvider(
		cfg.AccessKey,
		cfg.SecretKey,
		"", // session token not needed for static credentials
	)

	// Load config with credentials
	awsConfig, err := awsconfig.LoadDefaultConfig(context.Background(),
		awsconfig.WithCredentialsProvider(credProvider),
		awsconfig.WithRegion(cfg.Region),
	)
	if err != nil {
		return aws.Config{}, err
	}

	return awsConfig, nil
}

// isNotFoundError checks if the error is a "not found" error
func isNotFoundError(err error) bool {
	if err == nil {
		return false
	}

	// Check for NoSuchKey error
	var noSuchKey *types.NoSuchKey
	if errors.As(err, &noSuchKey) {
		return true
	}

	// Check for NotFound error
	var notFound *types.NotFound
	if errors.As(err, &notFound) {
		return true
	}

	// Check for HTTP 404 in error message
	return strings.Contains(err.Error(), "404") ||
		strings.Contains(err.Error(), "NoSuchKey") ||
		strings.Contains(err.Error(), "Not Found")
}

// BatchDelete implements batch delete operations
func (s *S3Storage) BatchDelete(ctx context.Context, operations []BatchDeleteOperation) ([]BatchResult, error) {
	if len(operations) == 0 {
		return []BatchResult{}, nil
	}

	logger.DebugWithContext(ctx, "Batch deleting objects from S3",
		zap.Int("count", len(operations)))

	// Prepare delete objects
	var objectIdentifiers []types.ObjectIdentifier
	for _, op := range operations {
		objectIdentifiers = append(objectIdentifiers, types.ObjectIdentifier{
			Key: aws.String(op.Key),
		})
	}

	// Execute batch delete
	deleteInput := &s3.DeleteObjectsInput{
		Bucket: aws.String(s.bucket),
		Delete: &types.Delete{
			Objects: objectIdentifiers,
			Quiet:   aws.Bool(false), // We want to know which failed
		},
	}

	result, err := s.client.DeleteObjects(ctx, deleteInput)
	if err != nil {
		logger.ErrorWithContext(ctx, "Batch delete failed",
			zap.Error(err))
		return nil, fmt.Errorf("batch delete failed: %w", err)
	}

	// Process results
	results := make([]BatchResult, len(operations))

	// Mark successful deletions
	deletedKeys := make(map[string]bool)
	for _, deleted := range result.Deleted {
		deletedKeys[aws.ToString(deleted.Key)] = true
	}

	// Mark failed deletions
	errorKeys := make(map[string]string)
	for _, deleteError := range result.Errors {
		key := aws.ToString(deleteError.Key)
		message := aws.ToString(deleteError.Message)
		errorKeys[key] = message
	}

	// Build results
	for i, op := range operations {
		if errorMsg, hasError := errorKeys[op.Key]; hasError {
			results[i] = BatchResult{
				Key:     op.Key,
				Success: false,
				Error:   errorMsg,
			}
		} else if deletedKeys[op.Key] {
			results[i] = BatchResult{
				Key:     op.Key,
				Success: true,
			}
		} else {
			// Should not happen, but handle just in case
			results[i] = BatchResult{
				Key:     op.Key,
				Success: false,
				Error:   "unknown error",
			}
		}
	}

	logger.DebugWithContext(ctx, "Batch delete completed",
		zap.Int("total", len(operations)),
		zap.Int("successful", len(result.Deleted)),
		zap.Int("failed", len(result.Errors)))

	return results, nil
}
