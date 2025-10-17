package database

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// RedisClient interface for Redis operations (for testing)
type RedisClient interface {
	XAdd(ctx context.Context, args *redis.XAddArgs) *redis.StringCmd
	Close() error
}

// OutboxRepo interface for outbox operations (for testing)
type OutboxRepo interface {
	GetPending(ctx context.Context, limit int) ([]*OutboxEvent, error)
	MarkProcessed(ctx context.Context, id uuid.UUID) error
	MarkFailed(ctx context.Context, id uuid.UUID, err error) error
}

// Relay processes events from the outbox table to Redis streams
type Relay struct {
	db        *DB
	redis     RedisClient
	outbox    OutboxRepo
	logger    *slog.Logger
	interval  time.Duration
	batchSize int
}

// RelayConfig contains configuration for the relay
type RelayConfig struct {
	PollInterval time.Duration
	BatchSize    int
}

// NewRelay creates a new relay instance
func NewRelay(db *DB, redisClient *redis.Client, logger *slog.Logger, config RelayConfig) *Relay {
	if config.PollInterval == 0 {
		config.PollInterval = 5 * time.Second
	}
	if config.BatchSize == 0 {
		config.BatchSize = 100
	}

	return &Relay{
		db:        db,
		redis:     redisClient,
		outbox:    NewOutboxRepository(db),
		logger:    logger.With("component", "relay"),
		interval:  config.PollInterval,
		batchSize: config.BatchSize,
	}
}

// Start begins processing events from the outbox
func (r *Relay) Start(ctx context.Context) error {
	r.logger.Info("starting relay", 
		"interval", r.interval, 
		"batch_size", r.batchSize)

	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()

	// Process immediately on start
	if err := r.processEvents(ctx); err != nil {
		r.logger.Error("failed to process events on startup", "error", err)
	}

	for {
		select {
		case <-ctx.Done():
			r.logger.Info("relay stopped")
			return ctx.Err()
		case <-ticker.C:
			if err := r.processEvents(ctx); err != nil {
				r.logger.Error("failed to process events", "error", err)
				// Continue running even on error
			}
		}
	}
}

// processEvents fetches and processes a batch of events
func (r *Relay) processEvents(ctx context.Context) error {
	events, err := r.outbox.GetPending(ctx, r.batchSize)
	if err != nil {
		return fmt.Errorf("failed to get pending events: %w", err)
	}

	if len(events) == 0 {
		return nil
	}

	r.logger.Debug("processing events", "count", len(events))

	for _, event := range events {
		if err := r.processEvent(ctx, event); err != nil {
			r.logger.Error("failed to process event", 
				"event_id", event.ID,
				"aggregate_id", event.AggregateID,
				"error", err)
			// Continue with other events
		}
	}

	return nil
}

// processEvent processes a single event
func (r *Relay) processEvent(ctx context.Context, event *OutboxEvent) error {
	// Publish to Redis
	if err := r.publishToRedis(ctx, event); err != nil {
		// Mark as failed
		if markErr := r.outbox.MarkFailed(ctx, event.ID, err); markErr != nil {
			r.logger.Error("failed to mark event as failed", 
				"event_id", event.ID,
				"error", markErr)
		}
		return err
	}

	// Mark as processed
	if err := r.outbox.MarkProcessed(ctx, event.ID); err != nil {
		r.logger.Error("failed to mark event as processed", 
			"event_id", event.ID,
			"error", err)
		return err
	}

	r.logger.Info("event processed successfully",
		"event_id", event.ID,
		"event_type", event.EventType,
		"aggregate_id", event.AggregateID,
		"target_stream", event.TargetStream)

	return nil
}

// publishToRedis publishes an event to Redis stream
func (r *Relay) publishToRedis(ctx context.Context, event *OutboxEvent) error {
	// Parse the event payload
	var payload map[string]interface{}
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return fmt.Errorf("failed to unmarshal payload: %w", err)
	}

	// Create the stream data structure expected by consumers
	streamData := map[string]interface{}{
		"id":             event.ID.String(),
		"type":           event.EventType,
		"aggregate_type": event.AggregateType,
		"aggregate_id":   event.AggregateID,
		"timestamp":      event.CreatedAt.Format(time.RFC3339),
		"payload":        payload,
		"metadata": map[string]interface{}{
			"source":        "amazon-scraper",
			"outbox_id":     event.ID.String(),
			"retry_count":   event.RetryCount,
			"target_stream": event.TargetStream,
		},
	}

	// Marshal to JSON for the data field
	dataJSON, err := json.Marshal(streamData)
	if err != nil {
		return fmt.Errorf("failed to marshal stream data: %w", err)
	}

	// Publish to Redis stream
	args := &redis.XAddArgs{
		Stream: event.TargetStream,
		Values: map[string]interface{}{
			"data":          string(dataJSON),
			"type":          event.EventType,
			"timestamp":     fmt.Sprintf("%d", event.CreatedAt.UnixNano()),
			"original_id":   event.ID.String(),
			"aggregate_id":  event.AggregateID,
			"aggregate_type": event.AggregateType,
			"event_type":    event.EventType,
		},
	}

	if _, err := r.redis.XAdd(ctx, args).Result(); err != nil {
		return fmt.Errorf("failed to publish to redis: %w", err)
	}

	return nil
}

// GetPendingCount returns the number of pending events in the outbox
func (r *Relay) GetPendingCount(ctx context.Context) (int64, error) {
	var count int64
	query := `
		SELECT COUNT(*) 
		FROM outbox_event 
		WHERE status IN ($1, $2)`
	
	err := r.db.pool.QueryRow(ctx, query, OutboxStatusPending, OutboxStatusFailed).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to get pending count: %w", err)
	}

	return count, nil
}

// GetDeadLetterCount returns the number of events in dead letter
func (r *Relay) GetDeadLetterCount(ctx context.Context) (int64, error) {
	var count int64
	query := `
		SELECT COUNT(*) 
		FROM outbox_event 
		WHERE status = $1`
	
	err := r.db.pool.QueryRow(ctx, query, OutboxStatusDeadLetter).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to get dead letter count: %w", err)
	}

	return count, nil
}