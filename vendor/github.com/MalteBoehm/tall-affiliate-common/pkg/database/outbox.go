package database

import (
	"encoding/json"
	"time"
	"github.com/google/uuid"
)

// OutboxEvent represents an event to be published
type OutboxEvent struct {
	ID            string
	AggregateType string
	AggregateID   string
	EventType     string
	Payload       json.RawMessage
	StreamName    string
	CreatedAt     time.Time
	ProcessedAt   *time.Time
}

// NewOutboxEvent creates a new outbox event
func NewOutboxEvent(aggregateType, aggregateID, eventType string, payload interface{}, streamName string) (*OutboxEvent, error) {
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	return &OutboxEvent{
		ID:            uuid.New().String(),
		AggregateType: aggregateType,
		AggregateID:   aggregateID,
		EventType:     eventType,
		Payload:       payloadJSON,
		StreamName:    streamName,
		CreatedAt:     time.Now(),
	}, nil
}