package storage

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"resizr/pkg/logger"

	"go.uber.org/zap"
)

// LocalStorage implements ImageStorage for local filesystem
type LocalStorage struct {
	directory string
}

// NewLocalStorage creates a new LocalStorage instance
func NewLocalStorage(directory string) (ImageStorage, error) {
	logger.Info("Initializing local storage", zap.String("directory", directory))
	if err := os.MkdirAll(directory, 0755); err != nil {
		return nil, fmt.Errorf("failed to create storage directory: %w", err)
	}
	return &LocalStorage{directory: directory}, nil
}

// Upload uploads a file to the local filesystem
func (s *LocalStorage) Upload(ctx context.Context, key string, reader io.Reader, size int64, contentType string) error {
	path := filepath.Join(s.directory, key)

	// Validate that the resulting path is within the storage directory
	absRoot, err := filepath.Abs(s.directory)
	if err != nil {
		return fmt.Errorf("failed to resolve storage directory: %w", err)
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("failed to resolve target path: %w", err)
	}
	if absPath == absRoot || !strings.HasPrefix(absPath+string(os.PathSeparator), absRoot+string(os.PathSeparator)) {
		return fmt.Errorf("invalid path: %q escapes from storage root", key)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	_, err = io.Copy(file, reader)
	if err != nil {
		return fmt.Errorf("failed to write to file: %w", err)
	}

	return nil
}

// Download downloads a file from the local filesystem
func (s *LocalStorage) Download(ctx context.Context, key string) (io.ReadCloser, error) {
	path := filepath.Join(s.directory, key)
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("file not found: %s", key)
		}
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	return file, nil
}

// Delete removes a file from the local filesystem
func (s *LocalStorage) Delete(ctx context.Context, key string) error {
	path := filepath.Join(s.directory, key)

	// Validate that the resulting path is within the storage directory,
	// and not the storage directory itself
	absRoot, err := filepath.Abs(s.directory)
	if err != nil {
		return fmt.Errorf("failed to resolve storage directory: %w", err)
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("failed to resolve target file path: %w", err)
	}
	if absPath == absRoot || !strings.HasPrefix(absPath+string(os.PathSeparator), absRoot+string(os.PathSeparator)) {
		return fmt.Errorf("invalid file key: %q escapes from storage root", key)
	}

	err = os.Remove(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // Already deleted
		}
		return fmt.Errorf("failed to delete file: %w", err)
	}
	return nil
}

// DeleteFolder removes a folder from the local filesystem
func (s *LocalStorage) DeleteFolder(ctx context.Context, prefix string) error {
	path := filepath.Join(s.directory, prefix)

	// Validate that the resulting path is strictly within the storage directory and not the directory itself
	absRoot, err := filepath.Abs(s.directory)
	if err != nil {
		return fmt.Errorf("failed to resolve storage directory: %w", err)
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("failed to resolve target path: %w", err)
	}
	rel, err := filepath.Rel(absRoot, absPath)
	if err != nil {
		return fmt.Errorf("failed to compute relative path: %w", err)
	}
	if rel == "." || strings.HasPrefix(rel, "..") {
		return fmt.Errorf("invalid path: %q escapes from storage root", prefix)
	}

	err = os.RemoveAll(absPath)
	if err != nil {
		return fmt.Errorf("failed to delete folder: %w", err)
	}
	return nil
}

// Exists checks if a file exists in the local filesystem
func (s *LocalStorage) Exists(ctx context.Context, key string) (bool, error) {
	path := filepath.Join(s.directory, key)
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

// GetMetadata retrieves file metadata from the local filesystem
func (s *LocalStorage) GetMetadata(ctx context.Context, key string) (*FileMetadata, error) {
	path := filepath.Join(s.directory, key)
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("file not found: %s", key)
		}
		return nil, fmt.Errorf("failed to get file metadata: %w", err)
	}

	return &FileMetadata{
		Key:          key,
		Size:         info.Size(),
		LastModified: info.ModTime(),
	}, nil
}

// GeneratePresignedURL is not applicable for local storage
func (s *LocalStorage) GeneratePresignedURL(ctx context.Context, key string, expiration time.Duration) (string, error) {
	return "", fmt.Errorf("GeneratePresignedURL is not implemented for local storage")
}

// ListObjects lists objects in the local filesystem
func (s *LocalStorage) ListObjects(ctx context.Context, prefix string, maxKeys int) ([]ObjectInfo, error) {
	var objects []ObjectInfo
	err := filepath.Walk(s.directory, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			relPath, err := filepath.Rel(s.directory, path)
			if err != nil {
				return err
			}
			if strings.HasPrefix(relPath, prefix) {
				objects = append(objects, ObjectInfo{
					Key:          relPath,
					Size:         info.Size(),
					LastModified: info.ModTime(),
				})
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list objects: %w", err)
	}
	return objects, nil
}

// CopyObject copies an object in the local filesystem
func (s *LocalStorage) CopyObject(ctx context.Context, sourceKey, destKey string) error {
	sourcePath := filepath.Join(s.directory, sourceKey)
	destPath := filepath.Join(s.directory, destKey)

	sourceFile, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer sourceFile.Close()

	// Ensure destination directory exists
	destDir := filepath.Dir(destPath)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	destFile, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	if err != nil {
		return fmt.Errorf("failed to copy file: %w", err)
	}
	return nil
}

// GetURL returns the local file path
func (s *LocalStorage) GetURL(key string) string {
	return filepath.Join(s.directory, key)
}

// Health checks the health of the local storage
func (s *LocalStorage) Health(ctx context.Context) error {
	_, err := os.Stat(s.directory)
	if err != nil {
		return fmt.Errorf("local storage directory not found: %w", err)
	}
	return nil
}
