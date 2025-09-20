package models

import (
	"testing"
)

func TestCalculateImageHash(t *testing.T) {
	testData := []byte("test image data")

	hash := CalculateImageHash(testData)

	if hash.Algorithm != "SHA256" {
		t.Errorf("Expected algorithm SHA256, got %s", hash.Algorithm)
	}

	if hash.Value == "" {
		t.Error("Expected non-empty hash value")
	}

	if hash.Size != int64(len(testData)) {
		t.Errorf("Expected size %d, got %d", len(testData), hash.Size)
	}
}

func TestImageHashEquals(t *testing.T) {
	testData := []byte("test image data")

	hash1 := CalculateImageHash(testData)
	hash2 := CalculateImageHash(testData)

	if !hash1.Equals(hash2) {
		t.Error("Expected identical hashes to be equal")
	}

	differentData := []byte("different image data")
	hash3 := CalculateImageHash(differentData)

	if hash1.Equals(hash3) {
		t.Error("Expected different hashes to not be equal")
	}
}

func TestCompareBytesByBytes(t *testing.T) {
	data1 := []byte("identical data")
	data2 := []byte("identical data")
	data3 := []byte("different data")

	if !CompareBytesByBytes(data1, data2) {
		t.Error("Expected identical byte arrays to be equal")
	}

	if CompareBytesByBytes(data1, data3) {
		t.Error("Expected different byte arrays to not be equal")
	}
}

func TestNewDeduplicationInfo(t *testing.T) {
	hash := ImageHash{
		Algorithm: "SHA256",
		Value:     "test-hash",
		Size:      100,
	}

	info := NewDeduplicationInfo(hash, "image-123", "storage/key")

	if info.Hash != hash {
		t.Error("Expected hash to be set correctly")
	}

	if info.MasterImageID != "image-123" {
		t.Errorf("Expected master image ID 'image-123', got %s", info.MasterImageID)
	}

	if info.ReferenceCount != 1 {
		t.Errorf("Expected reference count 1, got %d", info.ReferenceCount)
	}

	if len(info.ReferencingIDs) != 1 || info.ReferencingIDs[0] != "image-123" {
		t.Error("Expected one referencing ID matching master image ID")
	}
}

func TestDeduplicationInfoAddRemoveReference(t *testing.T) {
	hash := ImageHash{Algorithm: "SHA256", Value: "test", Size: 100}
	info := NewDeduplicationInfo(hash, "image-1", "storage/key")

	// Add second reference
	info.AddReference("image-2")

	if info.ReferenceCount != 2 {
		t.Errorf("Expected reference count 2, got %d", info.ReferenceCount)
	}

	if !info.HasReference("image-2") {
		t.Error("Expected image-2 to be referenced")
	}

	// Remove first reference
	info.RemoveReference("image-1")

	if info.ReferenceCount != 1 {
		t.Errorf("Expected reference count 1, got %d", info.ReferenceCount)
	}

	if info.MasterImageID != "image-2" {
		t.Errorf("Expected master image ID to be updated to 'image-2', got %s", info.MasterImageID)
	}

	// Remove last reference
	info.RemoveReference("image-2")

	if !info.IsOrphaned() {
		t.Error("Expected deduplication info to be orphaned")
	}
}

func TestImageHashString(t *testing.T) {
	hash := ImageHash{
		Algorithm: "SHA256",
		Value:     "abcdef123456",
		Size:      1024,
	}

	expected := "SHA256:abcdef123456"
	if hash.String() != expected {
		t.Errorf("Expected %s, got %s", expected, hash.String())
	}
}

func TestImageHashGetHashKey(t *testing.T) {
	hash := ImageHash{
		Algorithm: "SHA256",
		Value:     "abcdef123456",
		Size:      1024,
	}

	expected := "hash:SHA256:abcdef123456"
	if hash.GetHashKey() != expected {
		t.Errorf("Expected %s, got %s", expected, hash.GetHashKey())
	}
}

func TestDeduplicationInfoResolutionReference(t *testing.T) {
	hash := ImageHash{Algorithm: "SHA256", Value: "test", Size: 100}
	info := NewDeduplicationInfo(hash, "image-1", "storage/key")

	// Test AddResolutionReference
	info.AddResolutionReference("800x600", "image-1")
	info.AddResolutionReference("800x600", "image-2")
	info.AddResolutionReference("1024x768", "image-1")

	// Test GetResolutionReferenceCount
	if count := info.GetResolutionReferenceCount("800x600"); count != 2 {
		t.Errorf("Expected resolution reference count 2 for 800x600, got %d", count)
	}

	if count := info.GetResolutionReferenceCount("1024x768"); count != 1 {
		t.Errorf("Expected resolution reference count 1 for 1024x768, got %d", count)
	}

	if count := info.GetResolutionReferenceCount("nonexistent"); count != 0 {
		t.Errorf("Expected resolution reference count 0 for nonexistent, got %d", count)
	}

	// Test HasResolutionReference
	if !info.HasResolutionReference("800x600", "image-1") {
		t.Error("Expected image-1 to have reference to 800x600")
	}

	if !info.HasResolutionReference("800x600", "image-2") {
		t.Error("Expected image-2 to have reference to 800x600")
	}

	if info.HasResolutionReference("800x600", "image-3") {
		t.Error("Expected image-3 to not have reference to 800x600")
	}

	if info.HasResolutionReference("nonexistent", "image-1") {
		t.Error("Expected nonexistent resolution to not be referenced")
	}

	// Test GetUsedResolutions
	usedResolutions := info.GetUsedResolutions()
	expectedResolutions := []string{"800x600", "1024x768"}

	if len(usedResolutions) != len(expectedResolutions) {
		t.Errorf("Expected %d used resolutions, got %d", len(expectedResolutions), len(usedResolutions))
	}

	for _, expected := range expectedResolutions {
		found := false
		for _, actual := range usedResolutions {
			if actual == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected resolution %s to be in used resolutions", expected)
		}
	}

	// Test RemoveResolutionReference
	info.RemoveResolutionReference("800x600", "image-1")
	if count := info.GetResolutionReferenceCount("800x600"); count != 1 {
		t.Errorf("Expected resolution reference count 1 for 800x600 after removal, got %d", count)
	}

	// Remove last reference for a resolution
	info.RemoveResolutionReference("800x600", "image-2")
	if count := info.GetResolutionReferenceCount("800x600"); count != 0 {
		t.Errorf("Expected resolution reference count 0 for 800x600 after removing last reference, got %d", count)
	}

	// Test removing non-existent reference (should not panic)
	info.RemoveResolutionReference("nonexistent", "image-1")
	info.RemoveResolutionReference("1024x768", "nonexistent-image")
}
