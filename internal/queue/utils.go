package queue

import (
	"encoding/json"

	"github.com/google/uuid"
)

// generateID generates a unique ID for messages and intents
func generateID() string {
	return uuid.New().String()
}

// serializeMessage converts a DeletionMessage to JSON
func serializeMessage(msg *DeletionMessage) ([]byte, error) {
	return json.Marshal(msg)
}

// deserializeMessage converts JSON to a DeletionMessage
func deserializeMessage(data []byte) (*DeletionMessage, error) {
	var msg DeletionMessage
	err := json.Unmarshal(data, &msg)
	return &msg, err
}

// isPriorityHigh returns true if priority is high
func isPriorityHigh(priority string) bool {
	return priority == "high"
}
