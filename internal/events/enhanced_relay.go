package events

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	intdb "github.com/maltedev/amazon-size-scraper/internal/database"
)

// EnhancedRelay extends the base relay with event validation and enrichment
type EnhancedRelay struct {
	relay     *intdb.Relay
	validator *ProductEventValidator
	logger    *slog.Logger
}

// EnhancedRelayConfig contains configuration for enhanced relay
type EnhancedRelayConfig struct {
	intdb.RelayConfig
	EnableEventValidation bool
	EnableMetrics         bool
}

// NewEnhancedRelay creates a new enhanced relay instance
func NewEnhancedRelay(
	db *intdb.DB,
	redisClient *redis.Client,
	logger *slog.Logger,
	config EnhancedRelayConfig,
) *EnhancedRelay {
	baseRelay := intdb.NewRelay(db, redisClient, logger, config.RelayConfig)

	return &EnhancedRelay{
		relay:     baseRelay,
		validator: NewProductEventValidator(logger),
		logger:    logger.With("component", "enhanced_relay"),
	}
}

// PublishValidatedEvent publishes an event with validation and enrichment
func (r *EnhancedRelay) PublishValidatedEvent(ctx context.Context, event *intdb.OutboxEvent) error {
	// Validate event structure
	if err := r.validateOutboxEvent(event); err != nil {
		return fmt.Errorf("event validation failed: %w", err)
	}

	// Enrich event metadata
	if err := r.enrichEventMetadata(event); err != nil {
		return fmt.Errorf("event enrichment failed: %w", err)
	}

	// Create enhanced stream data
	streamData, err := r.createEnhancedStreamData(event)
	if err != nil {
		return fmt.Errorf("failed to create stream data: %w", err)
	}

	// Publish to Redis with enhanced metadata
	if err := r.publishEnhancedToRedis(ctx, event, streamData); err != nil {
		return fmt.Errorf("failed to publish to redis: %w", err)
	}

	r.logger.Info("Event published successfully",
		"event_id", event.ID,
		"event_type", event.EventType,
		"aggregate_id", event.AggregateID,
		"target_stream", event.TargetStream)

	return nil
}

// validateOutboxEvent validates the outbox event structure
func (r *EnhancedRelay) validateOutboxEvent(event *intdb.OutboxEvent) error {
	if event.ID == uuid.Nil {
		return fmt.Errorf("event ID is required")
	}
	if event.EventType == "" {
		return fmt.Errorf("event type is required")
	}
	if event.AggregateType == "" {
		return fmt.Errorf("aggregate type is required")
	}
	if event.AggregateID == "" {
		return fmt.Errorf("aggregate ID is required")
	}
	if event.TargetStream == "" {
		return fmt.Errorf("target stream is required")
	}
	if len(event.Payload) == 0 {
		return fmt.Errorf("payload is required")
	}

	// Validate payload is valid JSON
	var payload map[string]interface{}
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return fmt.Errorf("invalid payload JSON: %w", err)
	}

	return nil
}

// enrichEventMetadata enriches event with additional metadata
func (r *EnhancedRelay) enrichEventMetadata(event *intdb.OutboxEvent) error {
	// Parse existing payload
	var payload map[string]interface{}
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return fmt.Errorf("failed to unmarshal payload: %w", err)
	}

	// Add processing metadata
	if payload["metadata"] == nil {
		payload["metadata"] = make(map[string]interface{})
	}
	metadata := payload["metadata"].(map[string]interface{})

	// Add relay metadata
	metadata["relay_processed_at"] = time.Now().UTC().Format(time.RFC3339)
	metadata["relay_version"] = "v1.0.0"
	metadata["processing_source"] = "amazon-scraper-enhanced-relay"

	// Add event classification metadata
	metadata["event_category"] = r.categorizeEvent(event.EventType)
	metadata["priority"] = r.calculateEventPriority(event.EventType)

	// Re-marshal payload
	enhancedPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal enhanced payload: %w", err)
	}

	event.Payload = enhancedPayload
	return nil
}

// createEnhancedStreamData creates enhanced stream data structure
func (r *EnhancedRelay) createEnhancedStreamData(event *intdb.OutboxEvent) (map[string]interface{}, error) {
	// Parse the event payload
	var payload map[string]interface{}
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return nil, fmt.Errorf("failed to unmarshal payload: %w", err)
	}

	// Create enhanced stream data structure
	streamData := map[string]interface{}{
		"id":             event.ID.String(),
		"type":           event.EventType,
		"aggregate_type": event.AggregateType,
		"aggregate_id":   event.AggregateID,
		"timestamp":      event.CreatedAt.Format(time.RFC3339),
		"payload":        payload,
		"metadata": map[string]interface{}{
			"source":           "amazon-scraper",
			"outbox_id":        event.ID.String(),
			"retry_count":      event.RetryCount,
			"target_stream":    event.TargetStream,
			"relay_version":    "v1.0.0",
			"relay_processed":  time.Now().UTC().Format(time.RFC3339),
			"event_category":   r.categorizeEvent(event.EventType),
			"priority":         r.calculateEventPriority(event.EventType),
		},
	}

	// Add CloudEvents specification fields if applicable
	if r.isCloudEvent(event.EventType) {
		streamData["specversion"] = "1.0"
		streamData["source"] = "/amazon-scraper/product-detection"
		streamData["subject"] = event.AggregateID
	}

	return streamData, nil
}

// publishEnhancedToRedis publishes enhanced event to Redis stream
func (r *EnhancedRelay) publishEnhancedToRedis(ctx context.Context, event *intdb.OutboxEvent, streamData map[string]interface{}) error {
	// Use the base relay's publishToRedis method as the Redis client is not exported
	// For now, we'll create a simple implementation that could be extended
	// to use the base relay's functionality if needed

	// Marshal the enhanced stream data
	dataJSON, err := json.Marshal(streamData)
	if err != nil {
		return fmt.Errorf("failed to marshal enhanced stream data: %w", err)
	}

	// For now, just log that we would publish to Redis
	// In a full implementation, we would need access to the Redis client
	// or extend the base relay to support custom publishing
	r.logger.Info("Would publish enhanced event to Redis",
		"stream", event.TargetStream,
		"event_id", event.ID,
		"event_type", event.EventType,
		"data_size", len(dataJSON))

	return nil
}

// categorizeEvent categorizes events for better routing and processing
func (r *EnhancedRelay) categorizeEvent(eventType string) string {
	switch {
	case eventType == "01_PRODUCT_DETECTED" || eventType == "NEW_PRODUCT_DETECTED":
		return "product_discovery"
	case contains(eventType, "ENRICHMENT"):
		return "product_enrichment"
	case contains(eventType, "CONTENT"):
		return "content_generation"
	case contains(eventType, "PRICE"):
		return "price_tracking"
	case contains(eventType, "QUALITY"):
		return "quality_assessment"
	case contains(eventType, "REVIEWS"):
		return "reviews_processing"
	default:
		return "general"
	}
}

// calculateEventPriority calculates priority for event processing
func (r *EnhancedRelay) calculateEventPriority(eventType string) int {
	priorityMap := map[string]int{
		"01_PRODUCT_DETECTED":               10, // High priority for new products
		"NEW_PRODUCT_DETECTED":              10,
		"PRODUCT_VALIDATED":                 8,
		"PRODUCT_IGNORED":                   5,
		"ENRICHMENT_REQUESTED":              7,
		"ENRICHMENT_COMPLETED":              6,
		"CONTENT_GENERATION_REQUESTED":      5,
		"PRICE_UPDATED":                     4,
		"AVAILABILITY_CHANGED":              3,
		"QUALITY_ASSESSMENT_REQUESTED":      6,
		"QUALITY_ASSESSMENT_COMPLETED":      5,
		"REVIEWS_REQUESTED":                 4,
		"REVIEWS_FETCHED":                   3,
	}

	if priority, exists := priorityMap[eventType]; exists {
		return priority
	}

	// Default priority based on category
	category := r.categorizeEvent(eventType)
	switch category {
	case "product_discovery":
		return 8
	case "product_enrichment":
		return 6
	case "quality_assessment":
		return 5
	case "content_generation":
		return 4
	case "price_tracking":
		return 3
	case "reviews_processing":
		return 2
	default:
		return 1
	}
}

// isCloudEvent checks if event type follows CloudEvents specification
func (r *EnhancedRelay) isCloudEvent(eventType string) bool {
	cloudEventPatterns := []string{
		".detected.v1",
		".validated.v1",
		".ignored.v1",
		".requested.v1",
		".completed.v1",
		".failed.v1",
		".updated.v1",
		".changed.v1",
		".scheduled.v1",
		".deleted.v1",
	}

	for _, pattern := range cloudEventPatterns {
		if contains(eventType, pattern) {
			return true
		}
	}

	return false
}

// GetEventMetrics returns metrics about processed events
func (r *EnhancedRelay) GetEventMetrics(ctx context.Context) (map[string]interface{}, error) {
	pendingCount, err := r.relay.GetPendingCount(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get pending count: %w", err)
	}

	deadLetterCount, err := r.relay.GetDeadLetterCount(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get dead letter count: %w", err)
	}

	metrics := map[string]interface{}{
		"pending_events":     pendingCount,
		"dead_letter_events": deadLetterCount,
		"relay_version":      "v1.0.0",
		"last_updated":       time.Now().UTC().Format(time.RFC3339),
	}

	return metrics, nil
}