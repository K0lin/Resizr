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
