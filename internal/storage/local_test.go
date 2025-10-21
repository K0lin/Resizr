package storage

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewLocalStorage(t *testing.T) {
	dir, err := ioutil.TempDir("", "local_storage_test")
	assert.NoError(t, err)
	defer os.RemoveAll(dir)

	storage, err := NewLocalStorage(dir)
	assert.NoError(t, err)
	assert.NotNil(t, storage)
}

func TestLocalStorage_UploadDownload(t *testing.T) {
	dir, err := ioutil.TempDir("", "local_storage_test")
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

	data, err := ioutil.ReadAll(reader)
	assert.NoError(t, err)
	assert.Equal(t, content, string(data))
}

func TestLocalStorage_Exists(t *testing.T) {
	dir, err := ioutil.TempDir("", "local_storage_test")
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
	dir, err := ioutil.TempDir("", "local_storage_test")
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
	dir, err := ioutil.TempDir("", "local_storage_test")
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
	dir, err := ioutil.TempDir("", "local_storage_test")
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
	dir, err := ioutil.TempDir("", "local_storage_test")
	assert.NoError(t, err)
	defer os.RemoveAll(dir)

	storage, err := NewLocalStorage(dir)
	assert.NoError(t, err)

	err = storage.Health(context.Background())
	assert.NoError(t, err)
}
