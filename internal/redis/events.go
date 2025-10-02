package redis

import (
	"time"
)

// Simple event structures to replace external dependency

type CloudEvent struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`
	Source    string    `json:"source"`
	Subject   string    `json:"subject"`
	Data      any       `json:"data"`
	Timestamp time.Time `json:"timestamp"`
}

type ProductEnrichmentRequestedData struct {
	ASIN       string `json:"asin"`
	Region     string `json:"region"`
	RequestID  string `json:"request_id"`
	Priority   string `json:"priority"`
	RetryCount int    `json:"retry_count"`
}

type ProductEnrichedData struct {
	RequestID string `json:"request_id"`
	ASIN      string `json:"asin"`
	Region    string `json:"region"`
	// Add other fields as needed
}

type ProductEnrichmentFailedData struct {
	RequestID string `json:"request_id"`
	ASIN      string `json:"asin"`
	Region    string `json:"region"`
	Retryable bool   `json:"retryable"`
	Error     string `json:"error"`
}

// Event creators
func NewProductEnrichmentRequestedEvent(source string, data *ProductEnrichmentRequestedData) (*CloudEvent, error) {
	return &CloudEvent{
		ID:        data.RequestID,
		Type:      "product.enrichment.requested",
		Source:    source,
		Subject:   data.ASIN,
		Data:      data,
		Timestamp: time.Now(),
	}, nil
}

func NewProductEnrichedEvent(source string, data *ProductEnrichedData) (*CloudEvent, error) {
	return &CloudEvent{
		ID:        data.RequestID,
		Type:      "product.enriched",
		Source:    source,
		Subject:   data.ASIN,
		Data:      data,
		Timestamp: time.Now(),
	}, nil
}

func NewProductEnrichmentFailedEvent(source string, data *ProductEnrichmentFailedData) (*CloudEvent, error) {
	return &CloudEvent{
		ID:        data.RequestID,
		Type:      "product.enrichment.failed",
		Source:    source,
		Subject:   data.ASIN,
		Data:      data,
		Timestamp: time.Now(),
	}, nil
}