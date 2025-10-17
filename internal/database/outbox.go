package database

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const (
	// OutboxStatusPending indicates the event is waiting to be processed
	OutboxStatusPending = "pending"
	// OutboxStatusProcessed indicates the event was successfully processed
	OutboxStatusProcessed = "processed"
	// OutboxStatusFailed indicates the event processing failed (will be retried)
	OutboxStatusFailed = "failed"
	// OutboxStatusDeadLetter indicates the event failed too many times
	OutboxStatusDeadLetter = "dead_letter"

	// MaxRetryCount is the maximum number of retries before moving to dead letter
	MaxRetryCount = 5
)

// OutboxEvent represents an event in the transactional outbox
type OutboxEvent struct {
	ID            uuid.UUID       `db:"id"`
	AggregateType string          `db:"aggregate_type"`
	AggregateID   string          `db:"aggregate_id"`
	EventType     string          `db:"event_type"`
	Payload       json.RawMessage `db:"payload"`
	TargetStream  string          `db:"target_stream"`
	Status        string          `db:"status"`
	RetryCount    int             `db:"retry_count"`
	ErrorMessage  *string         `db:"error_message"`
	CreatedAt     time.Time       `db:"created_at"`
	ProcessedAt   *time.Time      `db:"processed_at"`
	NextRetryAt   *time.Time      `db:"next_retry_at"`
}

// OutboxRepository handles outbox event persistence
type OutboxRepository struct {
	db *DB
}

// NewOutboxRepository creates a new outbox repository
func NewOutboxRepository(db *DB) *OutboxRepository {
	return &OutboxRepository{db: db}
}

// InsertWithTx inserts an event into the outbox within a transaction
func (r *OutboxRepository) InsertWithTx(ctx context.Context, tx pgx.Tx, event *OutboxEvent) error {
	if event.ID == uuid.Nil {
		event.ID = uuid.New()
	}
	if event.Status == "" {
		event.Status = OutboxStatusPending
	}
	if event.TargetStream == "" {
		event.TargetStream = "stream:product_lifecycle"
	}

	now := time.Now()
	event.CreatedAt = now
	if event.NextRetryAt == nil {
		event.NextRetryAt = &now
	}

	query := `
		INSERT INTO outbox_event (
			id, aggregate_type, aggregate_id, event_type, 
			payload, target_stream, status, retry_count, 
			created_at, next_retry_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10
		)`

	_, err := tx.Exec(ctx, query,
		event.ID, event.AggregateType, event.AggregateID, event.EventType,
		event.Payload, event.TargetStream, event.Status, event.RetryCount,
		event.CreatedAt, event.NextRetryAt,
	)

	if err != nil {
		return fmt.Errorf("failed to insert outbox event: %w", err)
	}

	return nil
}

// GetPending retrieves pending events ready for processing
func (r *OutboxRepository) GetPending(ctx context.Context, limit int) ([]*OutboxEvent, error) {
	query := `
		SELECT 
			id, aggregate_type, aggregate_id, event_type, 
			payload, target_stream, status, retry_count, 
			error_message, created_at, processed_at, next_retry_at
		FROM outbox_event
		WHERE status IN ($1, $2)
			AND next_retry_at <= $3
		ORDER BY created_at ASC
		LIMIT $4`

	rows, err := r.db.pool.Query(ctx, query, 
		OutboxStatusPending, OutboxStatusFailed, 
		time.Now(), limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get pending events: %w", err)
	}
	defer rows.Close()

	var events []*OutboxEvent
	for rows.Next() {
		event := &OutboxEvent{}
		err := rows.Scan(
			&event.ID, &event.AggregateType, &event.AggregateID, &event.EventType,
			&event.Payload, &event.TargetStream, &event.Status, &event.RetryCount,
			&event.ErrorMessage, &event.CreatedAt, &event.ProcessedAt, &event.NextRetryAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan event: %w", err)
		}
		events = append(events, event)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return events, nil
}

// MarkProcessed marks an event as successfully processed
func (r *OutboxRepository) MarkProcessed(ctx context.Context, id uuid.UUID) error {
	query := `
		UPDATE outbox_event 
		SET status = $1, processed_at = $2
		WHERE id = $3`

	result, err := r.db.pool.Exec(ctx, query, OutboxStatusProcessed, time.Now(), id)
	if err != nil {
		return fmt.Errorf("failed to mark event as processed: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("event not found: %s", id)
	}

	return nil
}

// MarkFailed marks an event as failed and schedules retry
func (r *OutboxRepository) MarkFailed(ctx context.Context, id uuid.UUID, processErr error) error {
	// First, get current retry count
	var retryCount int
	err := r.db.pool.QueryRow(ctx, 
		"SELECT retry_count FROM outbox_event WHERE id = $1", id).Scan(&retryCount)
	if err != nil {
		return fmt.Errorf("failed to get retry count: %w", err)
	}

	retryCount++
	errorMsg := processErr.Error()

	// Determine next status and retry time
	status := OutboxStatusFailed
	nextRetryAt := calculateNextRetryTime(retryCount)

	// Move to dead letter if exceeded max retries
	if retryCount >= MaxRetryCount {
		status = OutboxStatusDeadLetter
	}

	query := `
		UPDATE outbox_event 
		SET status = $1, retry_count = $2, error_message = $3, next_retry_at = $4
		WHERE id = $5`

	_, err = r.db.pool.Exec(ctx, query, status, retryCount, errorMsg, nextRetryAt, id)
	if err != nil {
		return fmt.Errorf("failed to mark event as failed: %w", err)
	}

	return nil
}

// calculateNextRetryTime calculates exponential backoff for retries
func calculateNextRetryTime(retryCount int) time.Time {
	// Exponential backoff: 1s, 2s, 4s, 8s, 16s...
	backoffSeconds := 1 << retryCount
	if backoffSeconds > 300 { // Cap at 5 minutes
		backoffSeconds = 300
	}
	return time.Now().Add(time.Duration(backoffSeconds) * time.Second)
}