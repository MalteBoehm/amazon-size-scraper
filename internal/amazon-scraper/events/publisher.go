package events

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/maltedev/amazon-size-scraper/internal/database"
)

// EventType represents the type of event
type EventType string

const (
	// EventTypeNewProductDetected is published when a new product is found
	EventTypeNewProductDetected EventType = "NEW_PRODUCT_DETECTED"
)

// NewProductDetectedPayload represents the payload for NEW_PRODUCT_DETECTED event
type NewProductDetectedPayload struct {
	EventID        string                 `json:"event_id"`
	EventType      string                 `json:"event_type"`
	Timestamp      time.Time              `json:"timestamp"`
	ASIN           string                 `json:"asin"`
	Title          string                 `json:"title"`
	Brand          string                 `json:"brand,omitempty"`
	DetailPageURL  string                 `json:"detail_page_url"`
	Category       string                 `json:"category,omitempty"`
	Price          *Price                 `json:"price,omitempty"`
	Rating         *float64               `json:"rating,omitempty"`
	ReviewCount    *int                   `json:"review_count,omitempty"`
	Images         []string               `json:"images,omitempty"`
	Features       []string               `json:"features,omitempty"`
	AvailableSizes []string               `json:"available_sizes,omitempty"`
	SizeTable      *database.SizeTable    `json:"size_table,omitempty"`
	Source         string                 `json:"source"` // "scraper" instead of "pa-api"
}

// EnhancedNewProductDetectedPayload is an alias for backward compatibility
type EnhancedNewProductDetectedPayload = NewProductDetectedPayload

// Price represents product pricing information
type Price struct {
	Amount   float64 `json:"amount"`
	Currency string  `json:"currency"`
}

// HasValidSizeTable checks if the payload has a valid size table with length and width
func (p *NewProductDetectedPayload) HasValidSizeTable() bool {
	return database.ValidateSizeTable(p.SizeTable)
}

// Publisher handles event publishing using transactional outbox pattern
type Publisher struct {
	db     *database.DB
	outbox *database.OutboxRepository
	logger *slog.Logger
}

// NewPublisher creates a new event publisher with database connection
func NewPublisher(db *database.DB, logger *slog.Logger) *Publisher {
	return &Publisher{
		db:     db,
		outbox: database.NewOutboxRepository(db),
		logger: logger.With("component", "event_publisher"),
	}
}

// PublishNewProductDetected publishes a NEW_PRODUCT_DETECTED event using transactional outbox
func (p *Publisher) PublishNewProductDetected(ctx context.Context, payload *NewProductDetectedPayload) error {
	// Set event metadata
	if payload.EventID == "" {
		payload.EventID = uuid.New().String()
	}
	if payload.EventType == "" {
		payload.EventType = string(EventTypeNewProductDetected)
	}
	if payload.Timestamp.IsZero() {
		payload.Timestamp = time.Now()
	}
	if payload.Source == "" {
		payload.Source = "scraper"
	}

	// Convert to JSON
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	// Create outbox event
	outboxEvent := &database.OutboxEvent{
		AggregateType: "product",
		AggregateID:   payload.ASIN,
		EventType:     string(EventTypeNewProductDetected),
		Payload:       data,
		TargetStream:  "stream:product_lifecycle",
	}

	// Use transaction to ensure atomicity
	err = p.db.Transaction(ctx, func(tx pgx.Tx) error {
		if err := p.outbox.InsertWithTx(ctx, tx, outboxEvent); err != nil {
			return fmt.Errorf("failed to insert outbox event: %w", err)
		}

		// Additional transactional operations can be added here
		// For example, updating a products table, etc.

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to publish event: %w", err)
	}

	p.logger.Info("event published to outbox",
		"type", payload.EventType,
		"event_id", payload.EventID,
		"asin", payload.ASIN,
		"outbox_id", outboxEvent.ID,
	)

	return nil
}

// PublishEnhancedNewProductDetected is an alias for PublishNewProductDetected for backward compatibility
func (p *Publisher) PublishEnhancedNewProductDetected(ctx context.Context, payload *EnhancedNewProductDetectedPayload) error {
	return p.PublishNewProductDetected(ctx, payload)
}