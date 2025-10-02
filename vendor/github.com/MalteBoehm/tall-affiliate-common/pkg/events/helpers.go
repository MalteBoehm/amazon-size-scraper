package events

import (
	"encoding/json"
	"fmt"
)

// ParsePayload is a helper function to parse event payload into a specific type
func ParsePayload(payload interface{}, target interface{}) error {
	// If payload is already a map or struct, marshal it first
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	// Unmarshal into the target type
	if err := json.Unmarshal(jsonData, target); err != nil {
		return fmt.Errorf("failed to unmarshal payload: %w", err)
	}

	return nil
}

// MustParsePayload is like ParsePayload but panics on error
func MustParsePayload(payload interface{}, target interface{}) {
	if err := ParsePayload(payload, target); err != nil {
		panic(err)
	}
}