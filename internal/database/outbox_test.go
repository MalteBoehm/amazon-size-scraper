package database

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOutboxRepository_InsertWithTx(t *testing.T) {
	ctx := context.Background()
	db := setupTestDB(t)
	defer db.Close()

	repo := NewOutboxRepository(db)

	t.Run("successful insert with transaction", func(t *testing.T) {
		event := &OutboxEvent{
			AggregateType: "product",
			AggregateID:   "B001TEST",
			EventType:     "NEW_PRODUCT_DETECTED",
			Payload:       json.RawMessage(`{"asin":"B001TEST","title":"Test Product"}`),
			TargetStream:  "stream:product_lifecycle",
		}

		err := db.pool.BeginFunc(ctx, func(tx pgx.Tx) error {
			return repo.InsertWithTx(ctx, tx, event)
		})

		require.NoError(t, err)
		assert.NotEqual(t, uuid.Nil, event.ID)
		assert.Equal(t, "pending", event.Status)
		assert.Equal(t, 0, event.RetryCount)
		assert.False(t, event.CreatedAt.IsZero())
	})

	t.Run("rollback on transaction failure", func(t *testing.T) {
		event := &OutboxEvent{
			AggregateType: "product",
			AggregateID:   "B002TEST",
			EventType:     "NEW_PRODUCT_DETECTED",
			Payload:       json.RawMessage(`{"asin":"B002TEST"}`),
			TargetStream:  "stream:product_lifecycle",
		}

		// Start transaction that will be rolled back
		err := db.pool.BeginFunc(ctx, func(tx pgx.Tx) error {
			if err := repo.InsertWithTx(ctx, tx, event); err != nil {
				return err
			}
			// Force rollback
			return pgx.ErrTxClosed
		})

		assert.Error(t, err)

		// Verify event was not persisted
		events, err := repo.GetPending(ctx, 10)
		require.NoError(t, err)
		for _, e := range events {
			assert.NotEqual(t, "B002TEST", e.AggregateID)
		}
	})

	t.Run("validate required fields", func(t *testing.T) {
		testCases := []struct {
			name  string
			event *OutboxEvent
		}{
			{
				name: "missing aggregate type",
				event: &OutboxEvent{
					AggregateID:  "B003TEST",
					EventType:    "NEW_PRODUCT_DETECTED",
					Payload:      json.RawMessage(`{}`),
					TargetStream: "stream:product_lifecycle",
				},
			},
			{
				name: "missing event type",
				event: &OutboxEvent{
					AggregateType: "product",
					AggregateID:   "B003TEST",
					Payload:       json.RawMessage(`{}`),
					TargetStream:  "stream:product_lifecycle",
				},
			},
			{
				name: "missing payload",
				event: &OutboxEvent{
					AggregateType: "product",
					AggregateID:   "B003TEST",
					EventType:     "NEW_PRODUCT_DETECTED",
					TargetStream:  "stream:product_lifecycle",
				},
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				err := db.pool.BeginFunc(ctx, func(tx pgx.Tx) error {
					return repo.InsertWithTx(ctx, tx, tc.event)
				})
				assert.Error(t, err)
			})
		}
	})
}

func TestOutboxRepository_GetPending(t *testing.T) {
	ctx := context.Background()
	db := setupTestDB(t)
	defer db.Close()

	repo := NewOutboxRepository(db)

	// Insert test events
	now := time.Now()
	events := []*OutboxEvent{
		{
			AggregateType: "product",
			AggregateID:   "B001TEST",
			EventType:     "NEW_PRODUCT_DETECTED",
			Payload:       json.RawMessage(`{"asin":"B001TEST"}`),
			TargetStream:  "stream:product_lifecycle",
			Status:        "pending",
			NextRetryAt:   &now,
		},
		{
			AggregateType: "product",
			AggregateID:   "B002TEST",
			EventType:     "NEW_PRODUCT_DETECTED",
			Payload:       json.RawMessage(`{"asin":"B002TEST"}`),
			TargetStream:  "stream:product_lifecycle",
			Status:        "processed",
			NextRetryAt:   &now,
		},
		{
			AggregateType: "product",
			AggregateID:   "B003TEST",
			EventType:     "NEW_PRODUCT_DETECTED",
			Payload:       json.RawMessage(`{"asin":"B003TEST"}`),
			TargetStream:  "stream:product_lifecycle",
			Status:        "pending",
			NextRetryAt:   &now,
		},
		{
			AggregateType: "product",
			AggregateID:   "B004TEST",
			EventType:     "NEW_PRODUCT_DETECTED",
			Payload:       json.RawMessage(`{"asin":"B004TEST"}`),
			TargetStream:  "stream:product_lifecycle",
			Status:        "failed",
			RetryCount:    2,
			NextRetryAt:   &now,
		},
	}

	for _, event := range events {
		err := db.pool.BeginFunc(ctx, func(tx pgx.Tx) error {
			return repo.InsertWithTx(ctx, tx, event)
		})
		require.NoError(t, err)
	}

	t.Run("get pending events with limit", func(t *testing.T) {
		pending, err := repo.GetPending(ctx, 2)
		require.NoError(t, err)
		assert.Len(t, pending, 2)
		
		// Should get pending and failed (retry) events
		for _, e := range pending {
			assert.Contains(t, []string{"pending", "failed"}, e.Status)
		}
	})

	t.Run("get pending events ordered by created_at", func(t *testing.T) {
		pending, err := repo.GetPending(ctx, 10)
		require.NoError(t, err)
		
		// Verify ordering
		for i := 1; i < len(pending); i++ {
			assert.True(t, pending[i-1].CreatedAt.Before(pending[i].CreatedAt) || 
				pending[i-1].CreatedAt.Equal(pending[i].CreatedAt))
		}
	})

	t.Run("respects next_retry_at", func(t *testing.T) {
		// Update one event to have future retry time
		future := time.Now().Add(1 * time.Hour)
		_, err := db.pool.Exec(ctx, 
			"UPDATE outbox_event SET next_retry_at = $1 WHERE aggregate_id = $2",
			future, "B004TEST")
		require.NoError(t, err)

		pending, err := repo.GetPending(ctx, 10)
		require.NoError(t, err)
		
		// Should not include the event with future retry time
		for _, e := range pending {
			assert.NotEqual(t, "B004TEST", e.AggregateID)
		}
	})
}

func TestOutboxRepository_MarkProcessed(t *testing.T) {
	ctx := context.Background()
	db := setupTestDB(t)
	defer db.Close()

	repo := NewOutboxRepository(db)

	// Insert test event
	event := &OutboxEvent{
		AggregateType: "product",
		AggregateID:   "B001TEST",
		EventType:     "NEW_PRODUCT_DETECTED",
		Payload:       json.RawMessage(`{"asin":"B001TEST"}`),
		TargetStream:  "stream:product_lifecycle",
	}

	err := db.pool.BeginFunc(ctx, func(tx pgx.Tx) error {
		return repo.InsertWithTx(ctx, tx, event)
	})
	require.NoError(t, err)

	t.Run("mark as processed", func(t *testing.T) {
		err := repo.MarkProcessed(ctx, event.ID)
		require.NoError(t, err)

		// Verify status change
		var status string
		var processedAt *time.Time
		err = db.pool.QueryRow(ctx, 
			"SELECT status, processed_at FROM outbox_event WHERE id = $1",
			event.ID).Scan(&status, &processedAt)
		require.NoError(t, err)
		
		assert.Equal(t, "processed", status)
		assert.NotNil(t, processedAt)
		assert.True(t, time.Since(*processedAt) < 1*time.Second)
	})

	t.Run("mark non-existent event", func(t *testing.T) {
		err := repo.MarkProcessed(ctx, uuid.New())
		assert.Error(t, err)
	})
}

func TestOutboxRepository_MarkFailed(t *testing.T) {
	ctx := context.Background()
	db := setupTestDB(t)
	defer db.Close()

	repo := NewOutboxRepository(db)

	t.Run("increment retry count and set backoff", func(t *testing.T) {
		event := &OutboxEvent{
			AggregateType: "product",
			AggregateID:   "B001TEST",
			EventType:     "NEW_PRODUCT_DETECTED",
			Payload:       json.RawMessage(`{"asin":"B001TEST"}`),
			TargetStream:  "stream:product_lifecycle",
		}

		err := db.pool.BeginFunc(ctx, func(tx pgx.Tx) error {
			return repo.InsertWithTx(ctx, tx, event)
		})
		require.NoError(t, err)

		// First failure
		err = repo.MarkFailed(ctx, event.ID, assert.AnError)
		require.NoError(t, err)

		var status string
		var retryCount int
		var errorMsg *string
		var nextRetry *time.Time
		err = db.pool.QueryRow(ctx,
			"SELECT status, retry_count, error_message, next_retry_at FROM outbox_event WHERE id = $1",
			event.ID).Scan(&status, &retryCount, &errorMsg, &nextRetry)
		require.NoError(t, err)

		assert.Equal(t, "failed", status)
		assert.Equal(t, 1, retryCount)
		assert.NotNil(t, errorMsg)
		assert.Contains(t, *errorMsg, "assert.AnError")
		assert.NotNil(t, nextRetry)
		assert.True(t, nextRetry.After(time.Now()))
	})

	t.Run("move to dead letter after max retries", func(t *testing.T) {
		event := &OutboxEvent{
			AggregateType: "product",
			AggregateID:   "B002TEST",
			EventType:     "NEW_PRODUCT_DETECTED",
			Payload:       json.RawMessage(`{"asin":"B002TEST"}`),
			TargetStream:  "stream:product_lifecycle",
			RetryCount:    4, // One below max
		}

		err := db.pool.BeginFunc(ctx, func(tx pgx.Tx) error {
			return repo.InsertWithTx(ctx, tx, event)
		})
		require.NoError(t, err)

		// This should move to dead letter
		err = repo.MarkFailed(ctx, event.ID, assert.AnError)
		require.NoError(t, err)

		var status string
		var retryCount int
		err = db.pool.QueryRow(ctx,
			"SELECT status, retry_count FROM outbox_event WHERE id = $1",
			event.ID).Scan(&status, &retryCount)
		require.NoError(t, err)

		assert.Equal(t, "dead_letter", status)
		assert.Equal(t, 5, retryCount)
	})
}

// setupTestDB creates a test database connection
// In a real implementation, this would use a test container or test database
func setupTestDB(t *testing.T) *DB {
	t.Helper()
	// This is a placeholder - implement based on your test setup
	// For now, we'll skip if no test DB is available
	t.Skip("Test database not configured")
	return nil
}