package worker

import (
	"context"
	"testing"
	"time"

	"resizr/internal/queue"

	"github.com/dgraph-io/badger/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Redis tests removed - will be added later when Redis is available

func TestBadgerIntentManager_CreateAndGet(t *testing.T) {
	tmpDir := t.TempDir()

	opts := badger.DefaultOptions(tmpDir).WithLogger(nil)
	db, err := badger.Open(opts)
	require.NoError(t, err)
	defer db.Close()

	manager := NewBadgerIntentManager(db)
	ctx := context.Background()

	intent := &queue.DeletionIntent{
		IntentID:   "test-intent-1",
		ImageID:    "image-1",
		Resolution: "thumbnail",
		StorageKey: "images/image-1/thumbnail.jpg",
		Hash:       "test-hash",
		Status:     "pending",
		CreatedAt:  time.Now(),
	}

	// Create intent
	err = manager.CreateIntent(ctx, intent)
	require.NoError(t, err)

	// Get intent
	retrieved, err := manager.GetIntent(ctx, "test-intent-1")
	require.NoError(t, err)
	assert.Equal(t, intent.IntentID, retrieved.IntentID)
	assert.Equal(t, intent.ImageID, retrieved.ImageID)
	assert.Equal(t, intent.Status, retrieved.Status)

	// Clean up
	err = manager.DeleteIntent(ctx, "test-intent-1")
	assert.NoError(t, err)
}

func TestBadgerIntentManager_UpdateStatus(t *testing.T) {
	tmpDir := t.TempDir()

	opts := badger.DefaultOptions(tmpDir).WithLogger(nil)
	db, err := badger.Open(opts)
	require.NoError(t, err)
	defer db.Close()

	manager := NewBadgerIntentManager(db)
	ctx := context.Background()

	intent := &queue.DeletionIntent{
		IntentID:   "test-intent-2",
		ImageID:    "image-1",
		Resolution: "thumbnail",
		StorageKey: "images/image-1/thumbnail.jpg",
		Hash:       "test-hash",
		Status:     "pending",
		CreatedAt:  time.Now(),
	}

	err = manager.CreateIntent(ctx, intent)
	require.NoError(t, err)

	// Update to processing
	err = manager.UpdateStatus(ctx, "test-intent-2", "processing", "worker-1", "")
	assert.NoError(t, err)

	// Verify
	retrieved, err := manager.GetIntent(ctx, "test-intent-2")
	require.NoError(t, err)
	assert.Equal(t, "processing", retrieved.Status)
	assert.Equal(t, "worker-1", retrieved.WorkerID)

	// Update to failed
	err = manager.UpdateStatus(ctx, "test-intent-2", "failed", "worker-1", "connection timeout")
	assert.NoError(t, err)

	retrieved, err = manager.GetIntent(ctx, "test-intent-2")
	require.NoError(t, err)
	assert.Equal(t, "failed", retrieved.Status)
	assert.Equal(t, "connection timeout", retrieved.Error)

	// Clean up
	err = manager.DeleteIntent(ctx, "test-intent-2")
	assert.NoError(t, err)
}

func TestBadgerIntentManager_ListPendingIntents(t *testing.T) {
	tmpDir := t.TempDir()

	opts := badger.DefaultOptions(tmpDir).WithLogger(nil)
	db, err := badger.Open(opts)
	require.NoError(t, err)
	defer db.Close()

	manager := NewBadgerIntentManager(db)
	ctx := context.Background()

	// Create intents with different statuses
	statuses := []string{"pending", "processing", "completed", "pending"}
	for i, status := range statuses {
		intent := &queue.DeletionIntent{
			IntentID:   "test-intent-" + string(rune('a'+i)),
			ImageID:    "image-1",
			Resolution: "thumbnail",
			StorageKey: "images/image-1/thumbnail.jpg",
			Hash:       "test-hash",
			Status:     status,
			CreatedAt:  time.Now(),
		}
		err := manager.CreateIntent(ctx, intent)
		require.NoError(t, err)
	}

	// List pending - should return only pending intents
	pending, err := manager.ListPendingIntents(ctx, 10)
	require.NoError(t, err)
	assert.Equal(t, 2, len(pending)) // Two pending intents

	// Clean up
	for i := range statuses {
		intentID := "test-intent-" + string(rune('a'+i))
		_ = manager.DeleteIntent(ctx, intentID)
	}
}
