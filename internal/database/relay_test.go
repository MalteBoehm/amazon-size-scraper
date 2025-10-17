package database

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockRedisClient is a mock for Redis client
type MockRedisClient struct {
	mock.Mock
}

func (m *MockRedisClient) XAdd(ctx context.Context, args *redis.XAddArgs) *redis.StringCmd {
	mockArgs := m.Called(ctx, args)
	cmd := redis.NewStringCmd(ctx)
	if mockArgs.Get(0) != nil {
		cmd.SetErr(mockArgs.Error(0))
	} else {
		cmd.SetVal("1234567890-0") // Mock stream ID
	}
	return cmd
}

func (m *MockRedisClient) Close() error {
	args := m.Called()
	return args.Error(0)
}

// MockOutboxRepository is a mock for OutboxRepository
type MockOutboxRepository struct {
	mock.Mock
}

func (m *MockOutboxRepository) GetPending(ctx context.Context, limit int) ([]*OutboxEvent, error) {
	args := m.Called(ctx, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*OutboxEvent), args.Error(1)
}

func (m *MockOutboxRepository) MarkProcessed(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockOutboxRepository) MarkFailed(ctx context.Context, id uuid.UUID, err error) error {
	args := m.Called(ctx, id, err)
	return args.Error(0)
}

func TestRelay_ProcessEvents(t *testing.T) {
	ctx := context.Background()
	logger := slog.Default()

	t.Run("successfully process and publish events", func(t *testing.T) {
		mockRedis := new(MockRedisClient)
		mockOutbox := new(MockOutboxRepository)

		relay := &Relay{
			redis:     mockRedis,
			outbox:    mockOutbox,
			logger:    logger,
			batchSize: 10,
		}

		// Mock events to process
		events := []*OutboxEvent{
			{
				ID:            uuid.New(),
				AggregateType: "product",
				AggregateID:   "B001TEST",
				EventType:     "NEW_PRODUCT_DETECTED",
				Payload:       json.RawMessage(`{"asin":"B001TEST","title":"Test Product"}`),
				TargetStream:  "stream:product_lifecycle",
			},
			{
				ID:            uuid.New(),
				AggregateType: "product",
				AggregateID:   "B002TEST",
				EventType:     "NEW_PRODUCT_DETECTED",
				Payload:       json.RawMessage(`{"asin":"B002TEST","title":"Test Product 2"}`),
				TargetStream:  "stream:product_lifecycle",
			},
		}

		mockOutbox.On("GetPending", ctx, 10).Return(events, nil)

		// Expect Redis XAdd for each event
		for _, event := range events {
			mockRedis.On("XAdd", ctx, mock.MatchedBy(func(args *redis.XAddArgs) bool {
				return args.Stream == event.TargetStream &&
					args.Values["event_type"] == event.EventType &&
					args.Values["aggregate_id"] == event.AggregateID
			})).Return(nil)
			
			mockOutbox.On("MarkProcessed", ctx, event.ID).Return(nil)
		}

		err := relay.processEvents(ctx)
		require.NoError(t, err)

		mockRedis.AssertExpectations(t)
		mockOutbox.AssertExpectations(t)
	})

	t.Run("handle Redis publish failure", func(t *testing.T) {
		mockRedis := new(MockRedisClient)
		mockOutbox := new(MockOutboxRepository)

		relay := &Relay{
			redis:     mockRedis,
			outbox:    mockOutbox,
			logger:    logger,
			batchSize: 10,
		}

		event := &OutboxEvent{
			ID:            uuid.New(),
			AggregateType: "product",
			AggregateID:   "B001TEST",
			EventType:     "NEW_PRODUCT_DETECTED",
			Payload:       json.RawMessage(`{"asin":"B001TEST"}`),
			TargetStream:  "stream:product_lifecycle",
		}

		mockOutbox.On("GetPending", ctx, 10).Return([]*OutboxEvent{event}, nil)
		
		// Simulate Redis error
		redisErr := errors.New("redis connection failed")
		mockRedis.On("XAdd", ctx, mock.Anything).Return(redisErr)
		
		// Should mark as failed
		mockOutbox.On("MarkFailed", ctx, event.ID, mock.MatchedBy(func(err error) bool {
			return err.Error() == "failed to publish to redis: redis connection failed"
		})).Return(nil)

		err := relay.processEvents(ctx)
		assert.NoError(t, err) // processEvents should not fail on individual event errors

		mockRedis.AssertExpectations(t)
		mockOutbox.AssertExpectations(t)
	})

	t.Run("handle empty event batch", func(t *testing.T) {
		mockRedis := new(MockRedisClient)
		mockOutbox := new(MockOutboxRepository)

		relay := &Relay{
			redis:     mockRedis,
			outbox:    mockOutbox,
			logger:    logger,
			batchSize: 10,
		}

		mockOutbox.On("GetPending", ctx, 10).Return([]*OutboxEvent{}, nil)

		err := relay.processEvents(ctx)
		require.NoError(t, err)

		// Should not call Redis at all
		mockRedis.AssertNotCalled(t, "XAdd", mock.Anything, mock.Anything)
		mockOutbox.AssertExpectations(t)
	})

	t.Run("continue processing on individual event failure", func(t *testing.T) {
		mockRedis := new(MockRedisClient)
		mockOutbox := new(MockOutboxRepository)

		relay := &Relay{
			redis:     mockRedis,
			outbox:    mockOutbox,
			logger:    logger,
			batchSize: 10,
		}

		events := []*OutboxEvent{
			{
				ID:            uuid.New(),
				AggregateType: "product",
				AggregateID:   "B001TEST",
				EventType:     "NEW_PRODUCT_DETECTED",
				Payload:       json.RawMessage(`{"asin":"B001TEST"}`),
				TargetStream:  "stream:product_lifecycle",
			},
			{
				ID:            uuid.New(),
				AggregateType: "product",
				AggregateID:   "B002TEST",
				EventType:     "NEW_PRODUCT_DETECTED",
				Payload:       json.RawMessage(`{"asin":"B002TEST"}`),
				TargetStream:  "stream:product_lifecycle",
			},
		}

		mockOutbox.On("GetPending", ctx, 10).Return(events, nil)

		// First event fails
		mockRedis.On("XAdd", ctx, mock.MatchedBy(func(args *redis.XAddArgs) bool {
			return args.Values["aggregate_id"] == "B001TEST"
		})).Return(errors.New("redis error"))
		mockOutbox.On("MarkFailed", ctx, events[0].ID, mock.Anything).Return(nil)

		// Second event succeeds
		mockRedis.On("XAdd", ctx, mock.MatchedBy(func(args *redis.XAddArgs) bool {
			return args.Values["aggregate_id"] == "B002TEST"
		})).Return(nil)
		mockOutbox.On("MarkProcessed", ctx, events[1].ID).Return(nil)

		err := relay.processEvents(ctx)
		require.NoError(t, err)

		mockRedis.AssertExpectations(t)
		mockOutbox.AssertExpectations(t)
	})
}

func TestRelay_PublishToRedis(t *testing.T) {
	ctx := context.Background()
	logger := slog.Default()

	t.Run("correct stream data format", func(t *testing.T) {
		mockRedis := new(MockRedisClient)
		mockOutbox := new(MockOutboxRepository)

		relay := &Relay{
			redis:  mockRedis,
			outbox: mockOutbox,
			logger: logger,
		}

		event := &OutboxEvent{
			ID:            uuid.New(),
			AggregateType: "product",
			AggregateID:   "B001TEST",
			EventType:     "NEW_PRODUCT_DETECTED",
			Payload:       json.RawMessage(`{"asin":"B001TEST","title":"Test Product","price":29.99}`),
			TargetStream:  "stream:product_lifecycle",
			CreatedAt:     time.Now(),
		}

		mockRedis.On("XAdd", ctx, mock.MatchedBy(func(args *redis.XAddArgs) bool {
			// Verify the stream data format
			val, ok := args.Values["data"].(string)
			if !ok {
				return false
			}

			var data map[string]interface{}
			if err := json.Unmarshal([]byte(val), &data); err != nil {
				return false
			}

			// Check required fields
			return data["id"] != nil &&
				data["type"] == "NEW_PRODUCT_DETECTED" &&
				data["aggregate_type"] == "product" &&
				data["aggregate_id"] == "B001TEST" &&
				data["payload"] != nil &&
				data["timestamp"] != nil
		})).Return(nil)

		err := relay.publishToRedis(ctx, event)
		require.NoError(t, err)

		mockRedis.AssertExpectations(t)
	})

	t.Run("include metadata in stream data", func(t *testing.T) {
		mockRedis := new(MockRedisClient)
		mockOutbox := new(MockOutboxRepository)

		relay := &Relay{
			redis:  mockRedis,
			outbox: mockOutbox,
			logger: logger,
		}

		event := &OutboxEvent{
			ID:            uuid.New(),
			AggregateType: "product",
			AggregateID:   "B001TEST",
			EventType:     "NEW_PRODUCT_DETECTED",
			Payload:       json.RawMessage(`{"asin":"B001TEST"}`),
			TargetStream:  "stream:product_lifecycle",
			CreatedAt:     time.Now(),
		}

		mockRedis.On("XAdd", ctx, mock.MatchedBy(func(args *redis.XAddArgs) bool {
			val, ok := args.Values["data"].(string)
			if !ok {
				return false
			}

			var data map[string]interface{}
			if err := json.Unmarshal([]byte(val), &data); err != nil {
				return false
			}

			metadata, ok := data["metadata"].(map[string]interface{})
			if !ok {
				return false
			}

			// Check metadata includes source
			return metadata["source"] == "amazon-scraper"
		})).Return(nil)

		err := relay.publishToRedis(ctx, event)
		require.NoError(t, err)

		mockRedis.AssertExpectations(t)
	})
}

func TestRelay_Start(t *testing.T) {
	logger := slog.Default()

	t.Run("stop on context cancellation", func(t *testing.T) {
		mockRedis := new(MockRedisClient)
		mockOutbox := new(MockOutboxRepository)

		relay := &Relay{
			redis:     mockRedis,
			outbox:    mockOutbox,
			logger:    logger,
			interval:  50 * time.Millisecond,
			batchSize: 10,
		}

		// Return empty events
		mockOutbox.On("GetPending", mock.Anything, 10).Return([]*OutboxEvent{}, nil).Maybe()

		ctx, cancel := context.WithCancel(context.Background())
		
		// Start relay in background
		done := make(chan error)
		go func() {
			done <- relay.Start(ctx)
		}()

		// Let it run for a bit
		time.Sleep(100 * time.Millisecond)
		
		// Cancel context
		cancel()

		// Should exit cleanly
		select {
		case err := <-done:
			assert.ErrorIs(t, err, context.Canceled)
		case <-time.After(1 * time.Second):
			t.Fatal("relay did not stop on context cancellation")
		}
	})
}