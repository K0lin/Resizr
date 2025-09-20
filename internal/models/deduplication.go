package models

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
)

// ImageHash represents a hash of image content for deduplication
type ImageHash struct {
	Algorithm string `json:"algorithm" redis:"algorithm"` // SHA256
	Value     string `json:"value" redis:"value"`         // Hex-encoded hash
	Size      int64  `json:"size" redis:"size"`           // Original file size for quick comparison
}

// ResolutionReference tracks which images use a specific resolution
type ResolutionReference struct {
	Resolution     string   `json:"resolution" redis:"resolution"`           // Resolution name (e.g., "thumbnail", "800x600")
	ReferencingIDs []string `json:"referencing_ids" redis:"referencing_ids"` // Image IDs that use this resolution
	ReferenceCount int      `json:"reference_count" redis:"reference_count"` // Number of images using this resolution
}

// DeduplicationInfo tracks which images share the same content
type DeduplicationInfo struct {
	Hash           ImageHash                       `json:"hash" redis:"hash"`
	MasterImageID  string                          `json:"master_image_id" redis:"master_image_id"` // First image ID with this hash
	ReferenceCount int                             `json:"reference_count" redis:"reference_count"` // Number of images referencing this content
	StorageKey     string                          `json:"storage_key" redis:"storage_key"`         // Actual storage location
	ReferencingIDs []string                        `json:"referencing_ids" redis:"referencing_ids"` // All image IDs using this content
	ResolutionRefs map[string]*ResolutionReference `json:"resolution_refs" redis:"resolution_refs"` // Per-resolution reference tracking
}

// CalculateImageHash calculates SHA-256 hash of image data
func CalculateImageHash(data []byte) ImageHash {
	hasher := sha256.New()
	hasher.Write(data)
	hashBytes := hasher.Sum(nil)

	return ImageHash{
		Algorithm: "SHA256",
		Value:     hex.EncodeToString(hashBytes),
		Size:      int64(len(data)),
	}
}

// CalculateImageHashFromReader calculates SHA-256 hash from io.Reader
func _CalculateImageHashFromReader(reader io.Reader) (ImageHash, []byte, error) {
	// Read all data to calculate hash and return data for further use
	data, err := io.ReadAll(reader)
	if err != nil {
		return ImageHash{}, nil, fmt.Errorf("failed to read data for hashing: %w", err)
	}

	hash := CalculateImageHash(data)
	return hash, data, nil
}

// Equals compares two ImageHash instances
func (ih ImageHash) Equals(other ImageHash) bool {
	return ih.Algorithm == other.Algorithm &&
		ih.Value == other.Value &&
		ih.Size == other.Size
}

// String returns string representation of the hash
func (ih ImageHash) String() string {
	return fmt.Sprintf("%s:%s", ih.Algorithm, ih.Value)
}

// GetHashKey returns the key used to store hash mapping in repository
func (ih ImageHash) GetHashKey() string {
	return fmt.Sprintf("hash:%s:%s", ih.Algorithm, ih.Value)
}

// CompareBytesByBytes performs byte-by-byte comparison of two byte slices
// This is used as second stage verification in case of hash collision
func CompareBytesByBytes(data1, data2 []byte) bool {
	if len(data1) != len(data2) {
		return false
	}
	return bytes.Equal(data1, data2)
}

// NewDeduplicationInfo creates a new DeduplicationInfo for the first occurrence of a hash
func NewDeduplicationInfo(hash ImageHash, masterImageID, storageKey string) *DeduplicationInfo {
	return &DeduplicationInfo{
		Hash:           hash,
		MasterImageID:  masterImageID,
		ReferenceCount: 1,
		StorageKey:     storageKey,
		ReferencingIDs: []string{masterImageID},
		ResolutionRefs: make(map[string]*ResolutionReference),
	}
}

// AddReference adds a new image ID that references this content
func (di *DeduplicationInfo) AddReference(imageID string) {
	// Check if already exists
	for _, id := range di.ReferencingIDs {
		if id == imageID {
			return // Already exists
		}
	}

	di.ReferencingIDs = append(di.ReferencingIDs, imageID)
	di.ReferenceCount = len(di.ReferencingIDs)
}

// RemoveReference removes an image ID reference
func (di *DeduplicationInfo) RemoveReference(imageID string) {
	for i, id := range di.ReferencingIDs {
		if id == imageID {
			// Remove from slice
			di.ReferencingIDs = append(di.ReferencingIDs[:i], di.ReferencingIDs[i+1:]...)
			di.ReferenceCount = len(di.ReferencingIDs)

			// Update master if necessary
			if imageID == di.MasterImageID && len(di.ReferencingIDs) > 0 {
				di.MasterImageID = di.ReferencingIDs[0]
			}
			break
		}
	}
}

// IsOrphaned returns true if no images reference this content
func (di *DeduplicationInfo) IsOrphaned() bool {
	return di.ReferenceCount == 0 || len(di.ReferencingIDs) == 0
}

// HasReference checks if an image ID references this content
func (di *DeduplicationInfo) HasReference(imageID string) bool {
	for _, id := range di.ReferencingIDs {
		if id == imageID {
			return true
		}
	}
	return false
}

// AddResolutionReference adds a resolution reference for a specific image
func (di *DeduplicationInfo) AddResolutionReference(resolution, imageID string) {
	if di.ResolutionRefs == nil {
		di.ResolutionRefs = make(map[string]*ResolutionReference)
	}

	resRef := di.ResolutionRefs[resolution]
	if resRef == nil {
		resRef = &ResolutionReference{
			Resolution:     resolution,
			ReferencingIDs: []string{},
			ReferenceCount: 0,
		}
		di.ResolutionRefs[resolution] = resRef
	}

	// Check if already exists
	for _, id := range resRef.ReferencingIDs {
		if id == imageID {
			return // Already exists
		}
	}

	resRef.ReferencingIDs = append(resRef.ReferencingIDs, imageID)
	resRef.ReferenceCount = len(resRef.ReferencingIDs)
}

// RemoveResolutionReference removes a resolution reference for a specific image
func (di *DeduplicationInfo) RemoveResolutionReference(resolution, imageID string) {
	if di.ResolutionRefs == nil {
		return
	}

	resRef := di.ResolutionRefs[resolution]
	if resRef == nil {
		return
	}

	// Remove image ID from resolution reference
	for i, id := range resRef.ReferencingIDs {
		if id == imageID {
			resRef.ReferencingIDs = append(resRef.ReferencingIDs[:i], resRef.ReferencingIDs[i+1:]...)
			resRef.ReferenceCount = len(resRef.ReferencingIDs)
			break
		}
	}

	// Remove resolution reference if no images use it
	if resRef.ReferenceCount == 0 {
		delete(di.ResolutionRefs, resolution)
	}
}

// GetResolutionReferenceCount returns the number of images using a specific resolution
func (di *DeduplicationInfo) GetResolutionReferenceCount(resolution string) int {
	if di.ResolutionRefs == nil {
		return 0
	}

	resRef := di.ResolutionRefs[resolution]
	if resRef == nil {
		return 0
	}

	return resRef.ReferenceCount
}

// HasResolutionReference checks if a specific image uses a resolution
func (di *DeduplicationInfo) HasResolutionReference(resolution, imageID string) bool {
	if di.ResolutionRefs == nil {
		return false
	}

	resRef := di.ResolutionRefs[resolution]
	if resRef == nil {
		return false
	}

	for _, id := range resRef.ReferencingIDs {
		if id == imageID {
			return true
		}
	}
	return false
}

// GetUsedResolutions returns all resolutions that have at least one reference
func (di *DeduplicationInfo) GetUsedResolutions() []string {
	resolutions := make([]string, 0, len(di.ResolutionRefs))
	for resolution := range di.ResolutionRefs {
		resolutions = append(resolutions, resolution)
	}
	return resolutions
}
