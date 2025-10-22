package storage

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewLocalStorage(t *testing.T) {
	dir, err := os.MkdirTemp("", "local_storage_test")
	assert.NoError(t, err)
	defer os.RemoveAll(dir)

	storage, err := NewLocalStorage(dir)
	assert.NoError(t, err)
	assert.NotNil(t, storage)
}

func TestLocalStorage_UploadDownload(t *testing.T) {
	dir, err := os.MkdirTemp("", "local_storage_test")
	assert.NoError(t, err)
	defer os.RemoveAll(dir)

	storage, err := NewLocalStorage(dir)
	assert.NoError(t, err)

	key := "test/image.jpg"
	content := "hello world"
	err = storage.Upload(context.Background(), key, strings.NewReader(content), int64(len(content)), "image/jpeg")
	assert.NoError(t, err)

	reader, err := storage.Download(context.Background(), key)
	assert.NoError(t, err)
	defer reader.Close()

	data, err := io.ReadAll(reader)
	assert.NoError(t, err)
	assert.Equal(t, content, string(data))
}

func TestLocalStorage_Exists(t *testing.T) {
	dir, err := os.MkdirTemp("", "local_storage_test")
	assert.NoError(t, err)
	defer os.RemoveAll(dir)

	storage, err := NewLocalStorage(dir)
	assert.NoError(t, err)

	key := "test/image.jpg"
	content := "hello world"
	err = storage.Upload(context.Background(), key, strings.NewReader(content), int64(len(content)), "image/jpeg")
	assert.NoError(t, err)

	exists, err := storage.Exists(context.Background(), key)
	assert.NoError(t, err)
	assert.True(t, exists)

	exists, err = storage.Exists(context.Background(), "non-existent-key")
	assert.NoError(t, err)
	assert.False(t, exists)
}

func TestLocalStorage_Delete(t *testing.T) {
	dir, err := os.MkdirTemp("", "local_storage_test")
	assert.NoError(t, err)
	defer os.RemoveAll(dir)

	storage, err := NewLocalStorage(dir)
	assert.NoError(t, err)

	key := "test/image.jpg"
	content := "hello world"
	err = storage.Upload(context.Background(), key, strings.NewReader(content), int64(len(content)), "image/jpeg")
	assert.NoError(t, err)

	err = storage.Delete(context.Background(), key)
	assert.NoError(t, err)

	exists, err := storage.Exists(context.Background(), key)
	assert.NoError(t, err)
	assert.False(t, exists)
}

func TestLocalStorage_GetMetadata(t *testing.T) {
	dir, err := os.MkdirTemp("", "local_storage_test")
	assert.NoError(t, err)
	defer os.RemoveAll(dir)

	storage, err := NewLocalStorage(dir)
	assert.NoError(t, err)

	key := "test/image.jpg"
	content := "hello world"
	err = storage.Upload(context.Background(), key, strings.NewReader(content), int64(len(content)), "image/jpeg")
	assert.NoError(t, err)

	metadata, err := storage.GetMetadata(context.Background(), key)
	assert.NoError(t, err)
	assert.NotNil(t, metadata)
	assert.Equal(t, key, metadata.Key)
	assert.Equal(t, int64(len(content)), metadata.Size)
}

func TestLocalStorage_GetURL(t *testing.T) {
	dir, err := os.MkdirTemp("", "local_storage_test")
	assert.NoError(t, err)
	defer os.RemoveAll(dir)

	storage, err := NewLocalStorage(dir)
	assert.NoError(t, err)

	key := "test/image.jpg"
	expectedURL := filepath.Join(dir, key)
	url := storage.GetURL(key)
	assert.Equal(t, expectedURL, url)
}

func TestLocalStorage_Health(t *testing.T) {
	dir, err := os.MkdirTemp("", "local_storage_test")
	assert.NoError(t, err)
	defer os.RemoveAll(dir)

	storage, err := NewLocalStorage(dir)
	assert.NoError(t, err)

	err = storage.Health(context.Background())
	assert.NoError(t, err)
}

func TestLocalStorage_DeleteFolder(t *testing.T) {
	dir, err := os.MkdirTemp("", "local_storage_test")
	assert.NoError(t, err)
	defer os.RemoveAll(dir)

	storage, err := NewLocalStorage(dir)
	assert.NoError(t, err)

	// Create multiple files in a folder
	files := []string{
		"test/folder/file1.jpg",
		"test/folder/file2.jpg",
		"test/folder/subfolder/file3.jpg",
	}

	for _, key := range files {
		err = storage.Upload(context.Background(), key, strings.NewReader("content"), 7, "image/jpeg")
		assert.NoError(t, err)
	}

	// Delete the folder
	err = storage.DeleteFolder(context.Background(), "test/folder")
	assert.NoError(t, err)

	// Verify files are deleted
	for _, key := range files {
		exists, _ := storage.Exists(context.Background(), key)
		assert.False(t, exists)
	}
}

func TestLocalStorage_GeneratePresignedURL(t *testing.T) {
	dir, err := os.MkdirTemp("", "local_storage_test")
	assert.NoError(t, err)
	defer os.RemoveAll(dir)

	storage, err := NewLocalStorage(dir)
	assert.NoError(t, err)

	// Local storage doesn't support presigned URLs
	url, err := storage.GeneratePresignedURL(context.Background(), "test/key", 0)
	assert.Error(t, err)
	assert.Empty(t, url)
	assert.Contains(t, err.Error(), "not implemented")
}

func TestLocalStorage_ListObjects(t *testing.T) {
	dir, err := os.MkdirTemp("", "local_storage_test")
	assert.NoError(t, err)
	defer os.RemoveAll(dir)

	storage, err := NewLocalStorage(dir)
	assert.NoError(t, err)

	// Upload test files
	files := []string{
		"images/img1.jpg",
		"images/img2.jpg",
		"images/subfolder/img3.jpg",
		"other/file.txt",
	}

	for _, key := range files {
		err = storage.Upload(context.Background(), key, strings.NewReader("content"), 7, "image/jpeg")
		assert.NoError(t, err)
	}

	// List objects with prefix (use filepath separator for Windows compatibility)
	prefix := filepath.Join("images")
	objects, err := storage.ListObjects(context.Background(), prefix, 10)
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, len(objects), 3)

	// Verify listed keys - normalize paths for comparison
	foundKeys := make(map[string]bool)
	for _, obj := range objects {
		// Normalize path separators
		normalizedKey := filepath.ToSlash(obj.Key)
		foundKeys[normalizedKey] = true
	}
	assert.True(t, foundKeys["images/img1.jpg"] || foundKeys[filepath.Join("images", "img1.jpg")])
}

func TestLocalStorage_CopyObject(t *testing.T) {
	dir, err := os.MkdirTemp("", "local_storage_test")
	assert.NoError(t, err)
	defer os.RemoveAll(dir)

	storage, err := NewLocalStorage(dir)
	assert.NoError(t, err)

	// Upload source file
	sourceKey := "source/image.jpg"
	content := "test content"
	err = storage.Upload(context.Background(), sourceKey, strings.NewReader(content), int64(len(content)), "image/jpeg")
	assert.NoError(t, err)

	// Copy object
	destKey := "dest/image_copy.jpg"
	err = storage.CopyObject(context.Background(), sourceKey, destKey)
	assert.NoError(t, err)

	// Verify destination exists
	exists, err := storage.Exists(context.Background(), destKey)
	assert.NoError(t, err)
	assert.True(t, exists)

	// Verify content is same
	reader, err := storage.Download(context.Background(), destKey)
	assert.NoError(t, err)
	defer reader.Close()

	data, err := io.ReadAll(reader)
	assert.NoError(t, err)
	assert.Equal(t, content, string(data))
}

func TestLocalStorage_CopyObject_SourceNotExists(t *testing.T) {
	dir, err := os.MkdirTemp("", "local_storage_test")
	assert.NoError(t, err)
	defer os.RemoveAll(dir)

	storage, err := NewLocalStorage(dir)
	assert.NoError(t, err)

	err = storage.CopyObject(context.Background(), "nonexistent", "dest")
	assert.Error(t, err)
}

func TestLocalStorage_Download_NotFound(t *testing.T) {
	dir, err := os.MkdirTemp("", "local_storage_test")
	assert.NoError(t, err)
	defer os.RemoveAll(dir)

	storage, err := NewLocalStorage(dir)
	assert.NoError(t, err)

	_, err = storage.Download(context.Background(), "nonexistent")
	assert.Error(t, err)
}

func TestLocalStorage_GetMetadata_NotFound(t *testing.T) {
	dir, err := os.MkdirTemp("", "local_storage_test")
	assert.NoError(t, err)
	defer os.RemoveAll(dir)

	storage, err := NewLocalStorage(dir)
	assert.NoError(t, err)

	_, err = storage.GetMetadata(context.Background(), "nonexistent")
	assert.Error(t, err)
}

func TestLocalStorage_Delete_NonExistent(t *testing.T) {
	dir, err := os.MkdirTemp("", "local_storage_test")
	assert.NoError(t, err)
	defer os.RemoveAll(dir)

	storage, err := NewLocalStorage(dir)
	assert.NoError(t, err)

	// Deleting non-existent file should not error
	err = storage.Delete(context.Background(), "nonexistent")
	assert.NoError(t, err)
}
