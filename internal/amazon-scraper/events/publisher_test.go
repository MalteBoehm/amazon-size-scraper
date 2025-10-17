package events

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockDB is a mock for database operations
type MockDB struct {
	mock.Mock
}

func (m *MockDB) BeginTx(ctx context.Context, opts pgx.TxOptions) (pgx.Tx, error) {
	args := m.Called(ctx, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(pgx.Tx), args.Error(1)
}

// MockTx is a mock for database transaction
type MockTx struct {
	mock.Mock
}

func (m *MockTx) Begin(ctx context.Context) (pgx.Tx, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(pgx.Tx), args.Error(1)
}

func (m *MockTx) Commit(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockTx) Rollback(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockTx) Exec(ctx context.Context, sql string, arguments ...interface{}) (pgconn.CommandTag, error) {
	args := m.Called(ctx, sql, arguments)
	return pgconn.CommandTag{}, args.Error(0)
}

func (m *MockTx) Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error) {
	mockArgs := m.Called(ctx, sql, args)
	if mockArgs.Get(0) == nil {
		return nil, mockArgs.Error(1)
	}
	return mockArgs.Get(0).(pgx.Rows), mockArgs.Error(1)
}

func (m *MockTx) QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row {
	mockArgs := m.Called(ctx, sql, args)
	return mockArgs.Get(0).(pgx.Row)
}

func (m *MockTx) SendBatch(ctx context.Context, b *pgx.Batch) pgx.BatchResults {
	args := m.Called(ctx, b)
	return args.Get(0).(pgx.BatchResults)
}

func (m *MockTx) LargeObjects() pgx.LargeObjects {
	args := m.Called()
	return args.Get(0).(pgx.LargeObjects)
}

func (m *MockTx) Prepare(ctx context.Context, name, sql string) (*pgconn.StatementDescription, error) {
	args := m.Called(ctx, name, sql)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*pgconn.StatementDescription), args.Error(1)
}

func (m *MockTx) CopyFrom(ctx context.Context, tableName pgx.Identifier, columnNames []string, rowSrc pgx.CopyFromSource) (int64, error) {
	args := m.Called(ctx, tableName, columnNames, rowSrc)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockTx) Conn() *pgx.Conn {
	args := m.Called()
	return args.Get(0).(*pgx.Conn)
}

// MockOutboxRepository is a mock for OutboxRepository
type MockOutboxRepository struct {
	mock.Mock
}

func (m *MockOutboxRepository) InsertWithTx(ctx context.Context, tx pgx.Tx, event *OutboxEvent) error {
	args := m.Called(ctx, tx, event)
	return args.Error(0)
}

func TestPublisher_PublishNewProductDetected(t *testing.T) {
	ctx := context.Background()

	t.Run("successfully publish to outbox", func(t *testing.T) {
		mockDB := new(MockDB)
		mockTx := new(MockTx)
		mockOutbox := new(MockOutboxRepository)
		
		publisher := &Publisher{
			db:     mockDB,
			outbox: mockOutbox,
			logger: slog.Default(),
		}

		payload := &NewProductDetectedPayload{
			ASIN:          "B001TEST",
			Title:         "Test Product",
			DetailPageURL: "https://www.amazon.de/dp/B001TEST",
			Source:        "scraper",
		}

		// Mock transaction
		mockDB.On("BeginTx", ctx, pgx.TxOptions{}).Return(mockTx, nil)
		mockTx.On("Commit", ctx).Return(nil)

		// Mock outbox insert
		mockOutbox.On("InsertWithTx", ctx, mockTx, mock.MatchedBy(func(event *OutboxEvent) bool {
			// Verify event structure
			assert.Equal(t, "product", event.AggregateType)
			assert.Equal(t, "B001TEST", event.AggregateID)
			assert.Equal(t, "NEW_PRODUCT_DETECTED", event.EventType)
			assert.Equal(t, "stream:product_lifecycle", event.TargetStream)
			
			// Verify payload
			var p NewProductDetectedPayload
			err := json.Unmarshal(event.Payload, &p)
			assert.NoError(t, err)
			assert.Equal(t, "B001TEST", p.ASIN)
			assert.Equal(t, "Test Product", p.Title)
			assert.NotEmpty(t, p.EventID)
			assert.Equal(t, "NEW_PRODUCT_DETECTED", p.EventType)
			assert.Equal(t, "scraper", p.Source)
			
			return true
		})).Return(nil)

		err := publisher.PublishNewProductDetected(ctx, payload)
		require.NoError(t, err)

		mockDB.AssertExpectations(t)
		mockTx.AssertExpectations(t)
		mockOutbox.AssertExpectations(t)
	})

	t.Run("rollback on outbox insert failure", func(t *testing.T) {
		mockDB := new(MockDB)
		mockTx := new(MockTx)
		mockOutbox := new(MockOutboxRepository)
		
		publisher := &Publisher{
			db:     mockDB,
			outbox: mockOutbox,
			logger: slog.Default(),
		}

		payload := &NewProductDetectedPayload{
			ASIN:  "B001TEST",
			Title: "Test Product",
		}

		// Mock transaction
		mockDB.On("BeginTx", ctx, pgx.TxOptions{}).Return(mockTx, nil)
		mockTx.On("Rollback", ctx).Return(nil)

		// Mock outbox insert failure
		mockOutbox.On("InsertWithTx", ctx, mockTx, mock.Anything).
			Return(assert.AnError)

		err := publisher.PublishNewProductDetected(ctx, payload)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to insert outbox event")

		mockDB.AssertExpectations(t)
		mockTx.AssertExpectations(t)
		mockOutbox.AssertExpectations(t)
	})

	t.Run("set default values", func(t *testing.T) {
		mockDB := new(MockDB)
		mockTx := new(MockTx)
		mockOutbox := new(MockOutboxRepository)
		
		publisher := &Publisher{
			db:     mockDB,
			outbox: mockOutbox,
			logger: slog.Default(),
		}

		// Minimal payload
		payload := &NewProductDetectedPayload{
			ASIN: "B001TEST",
		}

		mockDB.On("BeginTx", ctx, pgx.TxOptions{}).Return(mockTx, nil)
		mockTx.On("Commit", ctx).Return(nil)

		mockOutbox.On("InsertWithTx", ctx, mockTx, mock.MatchedBy(func(event *OutboxEvent) bool {
			// Verify defaults are set
			var p NewProductDetectedPayload
			json.Unmarshal(event.Payload, &p)
			
			assert.NotEmpty(t, p.EventID)
			assert.Equal(t, "NEW_PRODUCT_DETECTED", p.EventType)
			assert.Equal(t, "scraper", p.Source)
			assert.False(t, p.Timestamp.IsZero())
			
			return true
		})).Return(nil)

		err := publisher.PublishNewProductDetected(ctx, payload)
		require.NoError(t, err)

		mockDB.AssertExpectations(t)
		mockTx.AssertExpectations(t)
		mockOutbox.AssertExpectations(t)
	})

	t.Run("handle transaction begin failure", func(t *testing.T) {
		mockDB := new(MockDB)
		mockOutbox := new(MockOutboxRepository)
		
		publisher := &Publisher{
			db:     mockDB,
			outbox: mockOutbox,
			logger: slog.Default(),
		}

		payload := &NewProductDetectedPayload{
			ASIN: "B001TEST",
		}

		// Mock transaction begin failure
		mockDB.On("BeginTx", ctx, pgx.TxOptions{}).Return(nil, assert.AnError)

		err := publisher.PublishNewProductDetected(ctx, payload)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to begin transaction")

		mockDB.AssertExpectations(t)
	})
}

// commandTag implementation for testing
type commandTag struct{}

func (c commandTag) String() string {
	return "INSERT 0 1"
}

func (c commandTag) RowsAffected() int64 {
	return 1
}