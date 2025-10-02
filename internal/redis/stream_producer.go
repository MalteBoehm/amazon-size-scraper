package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// StreamProducer handles writing events to Redis streams
type StreamProducer struct {
	client     *redis.Client
	streamName string
}

// NewStreamProducer creates a new Redis stream producer
func NewStreamProducer(client *redis.Client, streamName string) *StreamProducer {
	return &StreamProducer{
		client:     client,
		streamName: streamName,
	}
}

// ProduceEnrichmentRequest produces a product enrichment request event
func (p *StreamProducer) ProduceEnrichmentRequest(ctx context.Context, asin, region string, priority string) (string, error) {
	requestID := uuid.New().String()

	// Create the event data
	data := &ProductEnrichmentRequestedData{
		ASIN:       asin,
		Region:     region,
		RequestID:  requestID,
		Priority:   priority,
		RetryCount: 0,
	}

	// Create CloudEvent
	event, err := NewProductEnrichmentRequestedEvent("batch_enricher", data)
	if err != nil {
		return "", fmt.Errorf("failed to create event: %w", err)
	}

	// Marshal event to JSON
	eventJSON, err := json.Marshal(event)
	if err != nil {
		return "", fmt.Errorf("failed to marshal event: %w", err)
	}

	// Add to Redis stream
	args := &redis.XAddArgs{
		Stream: p.streamName,
		Values: map[string]interface{}{
			"data":       string(eventJSON), // Nested event format for consumers
			"event":      string(eventJSON), // Kept for backward compatibility
			"type":       event.Type,
			"source":     event.Source,
			"subject":    event.Subject,
			"request_id": requestID,
			"timestamp":  time.Now().Unix(),
		},
	}

	messageID, err := p.client.XAdd(ctx, args).Result()
	if err != nil {
		return "", fmt.Errorf("failed to add event to stream: %w", err)
	}

	return messageID, nil
}

// ProduceBatchEnrichmentRequests produces multiple enrichment requests as a batch
func (p *StreamProducer) ProduceBatchEnrichmentRequests(ctx context.Context, asins []string, region string, priority string) ([]string, error) {
	var messageIDs []string

	// Use pipeline for batch operations
	pipe := p.client.Pipeline()

	for _, asin := range asins {
		requestID := uuid.New().String()

		data := &ProductEnrichmentRequestedData{
			ASIN:       asin,
			Region:     region,
			RequestID:  requestID,
			Priority:   priority,
			RetryCount: 0,
		}

		event, err := NewProductEnrichmentRequestedEvent("batch_enricher", data)
		if err != nil {
			return nil, fmt.Errorf("failed to create event for ASIN %s: %w", asin, err)
		}

		eventJSON, err := json.Marshal(event)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal event for ASIN %s: %w", asin, err)
		}

		args := &redis.XAddArgs{
			Stream: p.streamName,
			Values: map[string]interface{}{
				"data":       string(eventJSON), // Nested event format for consumers
				"event":      string(eventJSON), // Backward compatibility
				"type":       event.Type,
				"source":     event.Source,
				"subject":    event.Subject,
				"request_id": requestID,
				"timestamp":  time.Now().Unix(),
			},
		}

		pipe.XAdd(ctx, args)
	}

	// Execute pipeline
	cmds, err := pipe.Exec(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to execute pipeline: %w", err)
	}

	// Extract message IDs
	for _, cmd := range cmds {
		if xAddCmd, ok := cmd.(*redis.StringCmd); ok {
			messageID, err := xAddCmd.Result()
			if err == nil {
				messageIDs = append(messageIDs, messageID)
			}
		}
	}

	return messageIDs, nil
}

// ProduceEnrichedEvent produces a product enriched event
func (p *StreamProducer) ProduceEnrichedEvent(ctx context.Context, data *ProductEnrichedData) (string, error) {
	event, err := NewProductEnrichedEvent("pa_api_worker", data)
	if err != nil {
		return "", fmt.Errorf("failed to create enriched event: %w", err)
	}

	eventJSON, err := json.Marshal(event)
	if err != nil {
		return "", fmt.Errorf("failed to marshal enriched event: %w", err)
	}

	args := &redis.XAddArgs{
		Stream: p.streamName,
		Values: map[string]interface{}{
			"data":       string(eventJSON), // Nested event format for consumers
			"event":      string(eventJSON), // Backward compatibility
			"type":       event.Type,
			"source":     event.Source,
			"subject":    event.Subject,
			"request_id": data.RequestID,
			"timestamp":  time.Now().Unix(),
		},
	}

	messageID, err := p.client.XAdd(ctx, args).Result()
	if err != nil {
		return "", fmt.Errorf("failed to add enriched event to stream: %w", err)
	}

	return messageID, nil
}

// ProduceFailedEvent produces a product enrichment failed event
func (p *StreamProducer) ProduceFailedEvent(ctx context.Context, data *ProductEnrichmentFailedData) (string, error) {
	event, err := NewProductEnrichmentFailedEvent("pa_api_worker", data)
	if err != nil {
		return "", fmt.Errorf("failed to create failed event: %w", err)
	}

	eventJSON, err := json.Marshal(event)
	if err != nil {
		return "", fmt.Errorf("failed to marshal failed event: %w", err)
	}

	args := &redis.XAddArgs{
		Stream: p.streamName,
		Values: map[string]interface{}{
			"data":       string(eventJSON), // Nested event format for consumers
			"event":      string(eventJSON), // Backward compatibility
			"type":       event.Type,
			"source":     event.Source,
			"subject":    event.Subject,
			"request_id": data.RequestID,
			"timestamp":  time.Now().Unix(),
			"retryable":  data.Retryable,
		},
	}

	messageID, err := p.client.XAdd(ctx, args).Result()
	if err != nil {
		return "", fmt.Errorf("failed to add failed event to stream: %w", err)
	}

	return messageID, nil
}

// CreateConsumerGroup creates a consumer group for the stream
func (p *StreamProducer) CreateConsumerGroup(ctx context.Context, groupName string) error {
	// Try to create the consumer group, starting from the beginning of the stream
	err := p.client.XGroupCreateMkStream(ctx, p.streamName, groupName, "0").Err()
	if err != nil {
		// Check if the group already exists
		if err.Error() == "BUSYGROUP Consumer Group name already exists" {
			return nil // Group already exists, which is fine
		}
		return fmt.Errorf("failed to create consumer group: %w", err)
	}
	return nil
}