package queue

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/dgraph-io/badger/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupBadgerQueue(t *testing.T) (*BadgerQueue, func()) {
	// Create temp directory
	tmpDir := t.TempDir()

	// Open Badger DB
	opts := badger.DefaultOptions(tmpDir).WithLogger(nil)
	db, err := badger.Open(opts)
	require.NoError(t, err)

	// Create queue config with unique prefix per test
	cfg := &QueueConfig{
		QueuePrefix:  "test:queue:" + t.Name() + ":",
		MessageTTL:   1 * time.Hour,
		MaxRetries:   3,
		RetryBackoff: 5 * time.Second,
	}

	// Create queue
	queue, err := NewBadgerQueue(db, cfg)
	require.NoError(t, err)

	cleanup := func() {
		queue.Close()
		db.Close()
		os.RemoveAll(tmpDir)
	}

	return queue, cleanup
}

func TestBadgerQueue_EnqueueAndConsume(t *testing.T) {
	queue, cleanup := setupBadgerQueue(t)
	defer cleanup()

	ctx := context.Background()

	// Enqueue a message
	msg := &DeletionMessage{
		IntentID:   "intent-1",
		ImageID:    "image-1",
		Resolution: "thumbnail",
		StorageKey: "images/image-1/thumbnail.jpg",
		Hash:       "test-hash",
		Priority:   "normal",
	}

	messageID, err := queue.Enqueue(ctx, msg)
	require.NoError(t, err)
	assert.NotEmpty(t, messageID)

	// Consume the message
	msgChan, err := queue.Consume(ctx, "test-consumer")
	require.NoError(t, err)

	select {
	case receivedMsg := <-msgChan:
		assert.Equal(t, msg.IntentID, receivedMsg.IntentID)
		assert.Equal(t, msg.ImageID, receivedMsg.ImageID)
		assert.Equal(t, msg.Resolution, receivedMsg.Resolution)
		assert.Equal(t, msg.StorageKey, receivedMsg.StorageKey)

		// Acknowledge the message
		err = queue.Ack(ctx, receivedMsg.MessageID)
		assert.NoError(t, err)

	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for message")
	}
}

func TestBadgerQueue_Nack(t *testing.T) {
	queue, cleanup := setupBadgerQueue(t)
	defer cleanup()

	ctx := context.Background()

	// Enqueue a message
	msg := &DeletionMessage{
		IntentID:   "intent-1",
		ImageID:    "image-1",
		Resolution: "thumbnail",
		StorageKey: "images/image-1/thumbnail.jpg",
		Hash:       "test-hash",
		Priority:   "normal",
	}

	_, err := queue.Enqueue(ctx, msg)
	require.NoError(t, err)

	// Consume the message
	msgChan, err := queue.Consume(ctx, "test-consumer")
	require.NoError(t, err)

	var receivedMsg *DeletionMessage
	select {
	case receivedMsg = <-msgChan:
		assert.NotNil(t, receivedMsg)
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for message")
	}

	// Nack the message (retry after 1 second)
	err = queue.Nack(ctx, receivedMsg.MessageID, 1*time.Second)
	assert.NoError(t, err)

	// Should receive the message again after retry delay
	select {
	case retryMsg := <-msgChan:
		assert.Equal(t, receivedMsg.IntentID, retryMsg.IntentID)
		assert.Equal(t, 1, retryMsg.RetryCount)

		// Ack it this time
		err = queue.Ack(ctx, retryMsg.MessageID)
		assert.NoError(t, err)

	case <-time.After(3 * time.Second):
		t.Fatal("Timeout waiting for retry message")
	}
}

func TestBadgerQueue_MoveToDLQ(t *testing.T) {
	queue, cleanup := setupBadgerQueue(t)
	defer cleanup()

	ctx, cancel := context.WithCancel(context.Background())

	// Enqueue a message
	msg := &DeletionMessage{
		IntentID:   "intent-1",
		ImageID:    "image-1",
		Resolution: "thumbnail",
		StorageKey: "images/image-1/thumbnail.jpg",
		Hash:       "test-hash",
		Priority:   "normal",
		RetryCount: 3,
		LastError:  "max retries exceeded",
	}

	_, err := queue.Enqueue(ctx, msg)
	require.NoError(t, err)

	// Consume the message
	msgChan, err := queue.Consume(ctx, "test-consumer")
	require.NoError(t, err)

	var receivedMsg *DeletionMessage
	select {
	case receivedMsg = <-msgChan:
		assert.NotNil(t, receivedMsg)
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for message")
	}

	// Cancel context to stop the consumer goroutine
	cancel()

	// Give the consumer goroutine time to stop
	time.Sleep(150 * time.Millisecond)

	// Move to DLQ (use new context)
	err = queue.MoveToDLQ(context.Background(), receivedMsg, "max_retries_exceeded")
	assert.NoError(t, err)

	// Verify message is no longer in processing and moved to DLQ
	stats, err := queue.GetStats(context.Background())
	require.NoError(t, err)

	assert.Equal(t, int64(0), stats.QueueSize)  // Main queue should be empty
	assert.Equal(t, int64(0), stats.Processing) // Processing should be empty
	assert.Equal(t, int64(1), stats.DLQSize)    // Should be in DLQ
}

func TestBadgerQueue_GetPending(t *testing.T) {
	queue, cleanup := setupBadgerQueue(t)
	defer cleanup()

	ctx := context.Background()

	// Enqueue multiple messages
	for i := 0; i < 5; i++ {
		msg := &DeletionMessage{
			IntentID:   "intent-" + string(rune('1'+i)),
			ImageID:    "image-1",
			Resolution: "thumbnail",
			StorageKey: "images/image-1/thumbnail.jpg",
			Hash:       "test-hash",
			Priority:   "normal",
		}
		_, err := queue.Enqueue(ctx, msg)
		require.NoError(t, err)
	}

	// Get pending messages
	pending, err := queue.GetPending(ctx, 10)
	require.NoError(t, err)
	assert.Len(t, pending, 5)
}

func TestBadgerQueue_Health(t *testing.T) {
	queue, cleanup := setupBadgerQueue(t)
	defer cleanup()

	ctx := context.Background()

	// Health check should pass
	err := queue.Health(ctx)
	assert.NoError(t, err)
}

func TestBadgerQueue_PriorityOrdering(t *testing.T) {
	queue, cleanup := setupBadgerQueue(t)
	defer cleanup()

	ctx := context.Background()

	// Enqueue normal priority message
	normalMsg := &DeletionMessage{
		IntentID:   "intent-normal",
		ImageID:    "image-1",
		Resolution: "thumbnail",
		StorageKey: "images/image-1/thumbnail.jpg",
		Hash:       "test-hash",
		Priority:   "normal",
	}
	_, err := queue.Enqueue(ctx, normalMsg)
	require.NoError(t, err)

	// Enqueue high priority message
	highMsg := &DeletionMessage{
		IntentID:   "intent-high",
		ImageID:    "image-2",
		Resolution: "thumbnail",
		StorageKey: "images/image-2/thumbnail.jpg",
		Hash:       "test-hash",
		Priority:   "high",
	}
	_, err = queue.Enqueue(ctx, highMsg)
	require.NoError(t, err)

	// Consume - high priority should come first
	msgChan, err := queue.Consume(ctx, "test-consumer")
	require.NoError(t, err)

	select {
	case receivedMsg := <-msgChan:
		assert.Equal(t, "intent-high", receivedMsg.IntentID)
		err = queue.Ack(ctx, receivedMsg.MessageID)
		assert.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for message")
	}
}
