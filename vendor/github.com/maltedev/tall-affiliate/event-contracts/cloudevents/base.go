package cloudevents

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// CloudEvent represents a CloudEvents v1.0 compliant event
type CloudEvent struct {
	// Required fields
	ID          string    `json:"id"`
	Source      string    `json:"source"`
	SpecVersion string    `json:"specversion"`
	Type        string    `json:"type"`
	
	// Optional fields
	Subject         string          `json:"subject,omitempty"`
	Time            time.Time       `json:"time,omitempty"`
	DataContentType string          `json:"datacontenttype,omitempty"`
	Data            json.RawMessage `json:"data,omitempty"`
}

// NewCloudEvent creates a new CloudEvent with defaults
func NewCloudEvent(eventType, source string) *CloudEvent {
	return &CloudEvent{
		ID:              uuid.New().String(),
		Source:          source,
		SpecVersion:     "1.0",
		Type:            eventType,
		Time:            time.Now().UTC(),
		DataContentType: "application/json",
	}
}

// SetData sets the data field with proper JSON marshaling
func (ce *CloudEvent) SetData(data interface{}) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal event data: %w", err)
	}
	ce.Data = jsonData
	return nil
}

// GetData unmarshals the data field into the provided interface
func (ce *CloudEvent) GetData(target interface{}) error {
	if ce.Data == nil {
		return fmt.Errorf("event data is nil")
	}
	if err := json.Unmarshal(ce.Data, target); err != nil {
		return fmt.Errorf("failed to unmarshal event data: %w", err)
	}
	return nil
}

// Validate validates the CloudEvent against spec requirements
func (ce *CloudEvent) Validate() error {
	if ce.ID == "" {
		return fmt.Errorf("id is required")
	}
	if ce.Source == "" {
		return fmt.Errorf("source is required")
	}
	if ce.SpecVersion == "" {
		return fmt.Errorf("specversion is required")
	}
	if ce.Type == "" {
		return fmt.Errorf("type is required")
	}
	return nil
}