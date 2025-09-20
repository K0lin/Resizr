package storage

import (
	"context"
	"testing"
	"time"

	"resizr/internal/config"

	"github.com/stretchr/testify/assert"
)

func TestS3Storage_DeleteFolder(t *testing.T) {
	tests := []struct {
		name        string
		prefix      string
		expectError bool
		description string
	}{
		{
			name:        "valid_folder_prefix",
			prefix:      "images/test-image-id",
			expectError: false,
			description: "Should attempt to delete folder with valid prefix",
		},
		{
			name:        "empty_prefix",
			prefix:      "",
			expectError: true,
			description: "Should return error for empty prefix",
		},
		{
			name:        "root_folder",
			prefix:      "images",
			expectError: false,
			description: "Should handle root folder deletion",
		},
		{
			name:        "nested_folder",
			prefix:      "images/subfolder/deep/nested",
			expectError: false,
			description: "Should handle nested folder deletion",
		},
	}

	// Note: These tests would require actual S3 connection or mocking
	// For now, we test the basic structure and error handling
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a mock S3 storage (would need proper mocking in real implementation)
			cfg := &config.S3Config{
				Endpoint:  "http://localhost:9000",
				Bucket:    "test-bucket",
				Region:    "us-east-1",
				AccessKey: "minioadmin",
				SecretKey: "minioadmin",
				UseSSL:    false,
			}

			storage, err := NewS3Storage(cfg)
			if err != nil {
				t.Skip("S3 storage not available for testing")
			}

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			err = storage.DeleteFolder(ctx, tt.prefix)

			if tt.expectError {
				assert.Error(t, err, tt.description)
			} else {
				// In a real test, we'd mock the S3 client or use a test S3 instance
				// For now, we just ensure the method doesn't panic
				assert.NotPanics(t, func() {
					_ = storage.DeleteFolder(ctx, tt.prefix)
				}, tt.description)
			}
		})
	}
}

func TestS3Storage_DeleteFolder_EdgeCases(t *testing.T) {
	t.Run("context_cancellation", func(t *testing.T) {
		cfg := &config.S3Config{
			Endpoint:  "http://localhost:9000",
			Bucket:    "test-bucket",
			Region:    "us-east-1",
			AccessKey: "minioadmin",
			SecretKey: "minioadmin",
			UseSSL:    false,
		}

		storage, err := NewS3Storage(cfg)
		if err != nil {
			t.Skip("S3 storage not available for testing")
		}

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		err = storage.DeleteFolder(ctx, "images/test")
		assert.Error(t, err, "Should handle context cancellation")
	})

	t.Run("special_characters_in_prefix", func(t *testing.T) {
		cfg := &config.S3Config{
			Endpoint:  "http://localhost:9000",
			Bucket:    "test-bucket",
			Region:    "us-east-1",
			AccessKey: "minioadmin",
			SecretKey: "minioadmin",
			UseSSL:    false,
		}

		storage, err := NewS3Storage(cfg)
		if err != nil {
			t.Skip("S3 storage not available for testing")
		}

		ctx := context.Background()

		// Test with special characters
		prefixes := []string{
			"images/test-with-dashes",
			"images/test_with_underscores",
			"images/test.with.dots",
			"images/test with spaces",
			"images/test@with#special$chars",
		}

		for _, prefix := range prefixes {
			assert.NotPanics(t, func() {
				_ = storage.DeleteFolder(ctx, prefix)
			}, "Should handle special characters in prefix: %s", prefix)
		}
	})
}
