package models

import (
	"testing"
)

// TestResolveToDimensionsWithPlaceholders tests the fix for placeholder resolution deletion bug
func TestResolveToDimensionsWithPlaceholders(t *testing.T) {
	metadata := &ImageMetadata{
		ID:          "test-id",
		Filename:    "test.jpg",
		Resolutions: []string{"thumbnail", "500x642", "100x100:test", "200x200"},
	}

	tests := []struct {
		name       string
		resolution string
		want       string
	}{
		{
			name:       "Plain dimensions without alias",
			resolution: "200x200",
			want:       "200x200",
		},
		{
			name:       "Dimensions with alias (full format)",
			resolution: "100x100:test",
			want:       "100x100", // Should extract dimensions part only
		},
		{
			name:       "Access by alias only",
			resolution: "test",
			want:       "100x100", // Should find "100x100:test" and return dimensions
		},
		{
			name:       "Predefined resolution",
			resolution: "thumbnail",
			want:       "150x150", // Thumbnail resolves to its actual dimensions
		},
		{
			name:       "Another plain dimensions",
			resolution: "500x642",
			want:       "500x642",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := metadata.ResolveToDimensions(tt.resolution)
			if got != tt.want {
				t.Errorf("ResolveToDimensions(%q) = %q, want %q", tt.resolution, got, tt.want)
			}
		})
	}
}

// TestGetStorageKeyWithPlaceholders tests storage key generation for placeholder resolutions
func TestGetStorageKeyWithPlaceholders(t *testing.T) {
	metadata := &ImageMetadata{
		ID:          "test-image-id",
		Filename:    "test.jpg",
		Resolutions: []string{"thumbnail", "500x642", "100x100:test", "200x200"},
	}

	tests := []struct {
		name       string
		resolution string
		wantKey    string
	}{
		{
			name:       "Plain dimensions",
			resolution: "200x200",
			wantKey:    "images/test-image-id/200x200.jpg",
		},
		{
			name:       "Dimensions with alias - should use dimensions only",
			resolution: "100x100:test",
			wantKey:    "images/test-image-id/100x100.jpg", // NOT 100x100:test.jpg
		},
		{
			name:       "Access by alias - should use dimensions",
			resolution: "test",
			wantKey:    "images/test-image-id/100x100.jpg",
		},
		{
			name:       "Thumbnail",
			resolution: "thumbnail",
			wantKey:    "images/test-image-id/150x150.jpg", // Thumbnail stored as actual dimensions
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := metadata.GetStorageKey(tt.resolution)
			if got != tt.wantKey {
				t.Errorf("GetStorageKey(%q) = %q, want %q", tt.resolution, got, tt.wantKey)
			}
		})
	}
}

// TestGetActualStorageKeyWithPlaceholders tests deduplicated storage key generation
func TestGetActualStorageKeyWithPlaceholders(t *testing.T) {
	// Deduplicated image
	dedupMetadata := &ImageMetadata{
		ID:            "image-1",
		Filename:      "test.jpg",
		IsDeduped:     true,
		SharedImageID: "master-image-id",
		Resolutions:   []string{"thumbnail", "100x100:test", "200x200"},
	}

	tests := []struct {
		name       string
		resolution string
		wantKey    string
	}{
		{
			name:       "Deduplicated - plain dimensions",
			resolution: "200x200",
			wantKey:    "images/master-image-id/200x200.jpg",
		},
		{
			name:       "Deduplicated - with alias",
			resolution: "100x100:test",
			wantKey:    "images/master-image-id/100x100.jpg", // Should use dimensions only
		},
		{
			name:       "Deduplicated - access by alias",
			resolution: "test",
			wantKey:    "images/master-image-id/100x100.jpg",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := dedupMetadata.GetActualStorageKey(tt.resolution)
			if got != tt.wantKey {
				t.Errorf("GetActualStorageKey(%q) = %q, want %q", tt.resolution, got, tt.wantKey)
			}
		})
	}
}
