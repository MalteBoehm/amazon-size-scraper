package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	_ "github.com/lib/pq"
)

// TransactionSafetyTestSuite tests transaction safety and error handling
type TransactionSafetyTestSuite struct {
	suite.Suite
	db           *DB
	tm           *TransactionManager
	poolManager  *ConnectionPoolManager
	monitor      *TransactionMonitor
	deadLetter   *DeadLetterProcessor
	outboxRepo   *OutboxRepository
}

func (suite *TransactionSafetyTestSuite) SetupSuite() {
	// Create test database configuration
	config := database.Config{
		Host:     "localhost",
		Port:     5432,
		User:     "postgres",
		Password: "postgres",
		Database: "amazon_scraper_test",
		SSLMode:  "disable",
		MaxConns: 10,
		MinConns: 2,
	}

	ctx := context.Background()

	// Initialize database connection
	db, err := New(ctx, config)
	require.NoError(suite.T(), err)
	suite.db = db

	// Initialize transaction manager
	logger := &MockLogger{}
	tmConfig := DefaultTransactionConfig()
	suite.tm = NewTransactionManager(db, logger, tmConfig)

	// Initialize connection pool manager
	poolConfig := DefaultPoolConfig()
	poolConfig.Host = config.Host
	poolConfig.Port = config.Port
	poolConfig.User = config.User
	poolConfig.Password = config.Password
	poolConfig.Database = config.Database
	poolConfig.SSLMode = config.SSLMode

	poolManager, err := NewConnectionPoolManager(poolConfig, logger)
	require.NoError(suite.T(), err)
	suite.poolManager = poolManager

	// Initialize transaction monitor
	monitorConfig := DefaultMonitorConfig()
	monitorConfig.Enabled = true
	monitorConfig.CheckInterval = 1 * time.Second
	suite.monitor = NewTransactionMonitor(db, logger, monitorConfig)

	// Initialize outbox repository
	suite.outboxRepo = NewOutboxRepository(db)

	// Initialize dead letter processor
	deadLetterConfig := DefaultDeadLetterConfig()
	deadLetterConfig.ProcessingInterval = 2 * time.Second
	suite.deadLetter = NewDeadLetterProcessor(db, logger, deadLetterConfig)

	// Create test tables
	suite.createTestTables(ctx)
}

func (suite *TransactionSafetyTestSuite) TearDownSuite() {
	ctx := context.Background()

	// Clean up test data
	suite.cleanupTestData(ctx)

	// Close connections
	if suite.monitor != nil {
		suite.monitor.Stop()
	}
	if suite.poolManager != nil {
		suite.poolManager.Close()
	}
	if suite.db != nil {
		suite.db.Close()
	}
}

func (suite *TransactionSafetyTestSuite) SetupTest() {
	ctx := context.Background()
	suite.cleanupTestData(ctx)
}

func (suite *TransactionSafetyTestSuite) createTestTables(ctx context.Context) {
	// Create test product table
	_, err := suite.db.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS test_product (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			asin VARCHAR(20) UNIQUE NOT NULL,
			title VARCHAR(500),
			status VARCHAR(50) DEFAULT 'pending',
			data JSONB,
			created_at TIMESTAMPTZ DEFAULT NOW(),
			updated_at TIMESTAMPTZ DEFAULT NOW()
		)
	`)
	require.NoError(suite.T(), err)

	// Create test outbox table
	_, err = suite.db.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS test_outbox_event (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			aggregate_type VARCHAR(50) NOT NULL,
			aggregate_id VARCHAR(50) NOT NULL,
			event_type VARCHAR(50) NOT NULL,
			payload JSONB NOT NULL,
			status VARCHAR(20) NOT NULL DEFAULT 'pending',
			retry_count INT NOT NULL DEFAULT 0,
			error_message TEXT,
			created_at TIMESTAMPTZ DEFAULT NOW(),
			processed_at TIMESTAMPTZ,
			next_retry_at TIMESTAMPTZ
		)
	`)
	require.NoError(suite.T(), err)

	// Create indexes
	_, err = suite.db.Exec(ctx, `
		CREATE INDEX IF NOT EXISTS idx_test_product_asin ON test_product(asin);
		CREATE INDEX IF NOT EXISTS idx_test_outbox_status ON test_outbox_event(status, next_retry_at);
	`)
	require.NoError(suite.T(), err)
}

func (suite *TransactionSafetyTestSuite) cleanupTestData(ctx context.Context) {
	tables := []string{"test_outbox_event", "test_product"}
	for _, table := range tables {
		_, err := suite.db.Exec(ctx, "TRUNCATE TABLE "+table+" CASCADE")
		if err != nil {
			suite.T().Logf("Failed to truncate table %s: %v", table, err)
		}
	}
}

// TestTransactionManager_BasicTransaction tests basic transaction functionality
func (suite *TransactionSafetyTestSuite) TestTransactionManager_BasicTransaction() {
	ctx := context.Background()

	// Test successful transaction
	err := suite.tm.ExecuteInTransaction(ctx, func(tx Tx) error {
		// Insert a product
		_, err := tx.Exec(ctx, `
			INSERT INTO test_product (asin, title, status)
			VALUES ($1, $2, $3)
		`, "TEST001", "Test Product", "active")
		require.NoError(suite.T(), err)

		// Insert an outbox event
		payload := map[string]interface{}{"asin": "TEST001", "action": "created"}
		payloadJSON, _ := json.Marshal(payload)

		_, err = tx.Exec(ctx, `
			INSERT INTO test_outbox_event (aggregate_type, aggregate_id, event_type, payload)
			VALUES ($1, $2, $3, $4)
		`, "product", "TEST001", "PRODUCT_CREATED", payloadJSON)
		require.NoError(suite.T(), err)

		return nil
	})

	assert.NoError(suite.T(), err)

	// Verify data was committed
	var count int
	err = suite.db.QueryRow(ctx, "SELECT COUNT(*) FROM test_product WHERE asin = $1", "TEST001").Scan(&count)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), 1, count)

	err = suite.db.QueryRow(ctx, "SELECT COUNT(*) FROM test_outbox_event WHERE aggregate_id = $1", "TEST001").Scan(&count)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), 1, count)
}

// TestTransactionManager_TransactionRollback tests transaction rollback on error
func (suite *TransactionSafetyTestSuite) TestTransactionManager_TransactionRollback() {
	ctx := context.Background()

	// Test transaction rollback on error
	err := suite.tm.ExecuteInTransaction(ctx, func(tx Tx) error {
		// Insert a product
		_, err := tx.Exec(ctx, `
			INSERT INTO test_product (asin, title, status)
			VALUES ($1, $2, $3)
		`, "TEST002", "Test Product 2", "active")
		require.NoError(suite.T(), err)

		// Insert an outbox event
		payload := map[string]interface{}{"asin": "TEST002", "action": "created"}
		payloadJSON, _ := json.Marshal(payload)

		_, err = tx.Exec(ctx, `
			INSERT INTO test_outbox_event (aggregate_type, aggregate_id, event_type, payload)
			VALUES ($1, $2, $3, $4)
		`, "product", "TEST002", "PRODUCT_CREATED", payloadJSON)
		require.NoError(suite.T(), err)

		// Simulate an error that should cause rollback
		return assert.AnError
	})

	assert.Error(suite.T(), err)

	// Verify data was rolled back
	var count int
	err = suite.db.QueryRow(ctx, "SELECT COUNT(*) FROM test_product WHERE asin = $1", "TEST002").Scan(&count)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), 0, count)

	err = suite.db.QueryRow(ctx, "SELECT COUNT(*) FROM test_outbox_event WHERE aggregate_id = $1", "TEST002").Scan(&count)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), 0, count)
}

// TestTransactionManager_ConcurrencyConflict tests handling of concurrency conflicts
func (suite *TransactionSafetyTestSuite) TestTransactionManager_ConcurrencyConflict() {
	ctx := context.Background()

	// Insert initial product
	_, err := suite.db.Exec(ctx, `
		INSERT INTO test_product (asin, title, status)
		VALUES ($1, $2, $3)
	`, "TEST003", "Original Product", "pending")
	require.NoError(suite.T(), err)

	// Start two concurrent transactions that will conflict
	errChan1 := make(chan error, 1)
	errChan2 := make(chan error, 1)

	// First transaction
	go func() {
		err := suite.tm.ExecuteInTransaction(ctx, func(tx Tx) error {
			// Add small delay to increase chance of conflict
			time.Sleep(10 * time.Millisecond)

			_, err := tx.Exec(ctx, `
				UPDATE test_product SET status = $1, title = $2 WHERE asin = $3
			`, "processing", "Updated by Tx1", "TEST003")
			return err
		})
		errChan1 <- err
	}()

	// Second transaction
	go func() {
		err := suite.tm.ExecuteInTransaction(ctx, func(tx Tx) error {
			_, err := tx.Exec(ctx, `
				UPDATE test_product SET status = $1, title = $2 WHERE asin = $3
			`, "processing", "Updated by Tx2", "TEST003")
			return err
		})
		errChan2 <- err
	}()

	// Wait for both transactions to complete
	err1 := <-errChan1
	err2 := <-errChan2

	// At least one should succeed
	assert.True(suite.T(), err1 == nil || err2 == nil, "At least one transaction should succeed")

	// Verify final state
	var title, status string
	err = suite.db.QueryRow(ctx, "SELECT title, status FROM test_product WHERE asin = $1", "TEST003").Scan(&title, &status)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), "processing", status)
	assert.True(suite.T(), title == "Updated by Tx1" || title == "Updated by Tx2")
}

// TestTransactionManager_RetryLogic tests transaction retry logic
func (suite *TransactionSafetyTestSuite) TestTransactionManager_RetryLogic() {
	ctx := context.Background()

	retryCount := 0
	maxRetries := 3

	// Test retry logic with transient errors
	err := suite.tm.ExecuteInTransaction(ctx, func(tx Tx) error {
		retryCount++

		// Fail for first few attempts, then succeed
		if retryCount < maxRetries {
			return sql.ErrTxDone // Simulate transient error
		}

		// Insert product on final attempt
		_, err := tx.Exec(ctx, `
			INSERT INTO test_product (asin, title, status)
			VALUES ($1, $2, $3)
		`, "TEST004", "Retry Test Product", "active")
		return err
	})

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), maxRetries, retryCount)

	// Verify product was created
	var count int
	err = suite.db.QueryRow(ctx, "SELECT COUNT(*) FROM test_product WHERE asin = $1", "TEST004").Scan(&count)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), 1, count)
}

// TestConnectionPoolManager_HealthCheck tests connection pool health monitoring
func (suite *TransactionSafetyTestSuite) TestConnectionPoolManager_HealthCheck() {
	// Test initial health
	assert.True(suite.T(), suite.poolManager.IsHealthy())

	// Get metrics
	metrics := suite.poolManager.GetMetrics()
	assert.NotNil(suite.T(), metrics)

	// Test query execution
	ctx := context.Background()
	rows, err := suite.poolManager.QueryWithContext(ctx, "SELECT 1 as test")
	require.NoError(suite.T(), err)
	defer rows.Close()

	assert.True(suite.T(), rows.Next())
	var result int
	err = rows.Scan(&result)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), 1, result)
}

// TestDeadLetterProcessor_BasicProcessing tests dead letter processing
func (suite *TransactionSafetyTestSuite) TestDeadLetterProcessor_BasicProcessing() {
	ctx := context.Background()

	// Create a dead letter event
	event := &OutboxEvent{
		ID:            uuid.New(),
		AggregateType: "test",
		AggregateID:   "TEST001",
		EventType:     "TEST_EVENT",
		Payload:       json.RawMessage(`{"test": true}`),
		TargetStream:  "test:stream",
		Status:        OutboxStatusDeadLetter,
		RetryCount:    5,
		ErrorMessage:  stringPtr("Test error for analysis"),
		CreatedAt:     time.Now(),
	}

	// Insert dead letter event
	_, err := suite.db.Exec(ctx, `
		INSERT INTO test_outbox_event (id, aggregate_type, aggregate_id, event_type, payload, status, retry_count, error_message)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, event.ID, event.AggregateType, event.AggregateID, event.EventType,
		event.Payload, event.Status, event.RetryCount, event.ErrorMessage)
	require.NoError(suite.T(), err)

	// Process a single event manually (not starting the full processor)
	err = suite.deadLetter.processEvent(ctx, event)
	assert.NoError(suite.T(), err)

	// Verify event was analyzed
	var updatedEvent OutboxEvent
	var analysisJSON sql.NullString

	err = suite.db.QueryRow(ctx, `
		SELECT id, status, payload FROM test_outbox_event WHERE id = $1
	`, event.ID).Scan(&updatedEvent.ID, &updatedEvent.Status, &analysisJSON)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), event.ID, updatedEvent.ID)
	assert.True(suite.T(), analysisJSON.Valid)

	// Verify analysis was added to payload
	if analysisJSON.Valid {
		var payload map[string]interface{}
		err = json.Unmarshal([]byte(analysisJSON.String), &payload)
		assert.NoError(suite.T(), err)
		assert.Contains(suite.T(), payload, "cause_type")
		assert.Contains(suite.T(), payload, "is_recoverable")
	}
}

// TestTransactionMonitor_MetricsCollection tests transaction monitoring
func (suite *TransactionSafetyTestSuite) TestTransactionMonitor_MetricsCollection() {
	ctx := context.Background()

	// Start monitor
	err := suite.monitor.Start(ctx)
	require.NoError(suite.T(), err)
	defer suite.monitor.Stop()

	// Record some test transactions
	testRecords := []TransactionRecord{
		{
			ID:        uuid.New(),
			Type:      "test_insert",
			StartTime: time.Now().Add(-100 * time.Millisecond),
			EndTime:   time.Now().Add(-50 * time.Millisecond),
			Duration:  50 * time.Millisecond,
			Status:    "success",
		},
		{
			ID:        uuid.New(),
			Type:      "test_update",
			StartTime: time.Now().Add(-80 * time.Millisecond),
			EndTime:   time.Now().Add(-10 * time.Millisecond),
			Duration:  70 * time.Millisecond,
			Status:    "success",
		},
		{
			ID:        uuid.New(),
			Type:      "test_delete",
			StartTime: time.Now().Add(-60 * time.Millisecond),
			EndTime:   time.Now().Add(-30 * time.Millisecond),
			Duration:  30 * time.Millisecond,
			Status:    "failed",
			Error:     "constraint violation",
		},
	}

	// Record transactions
	for _, record := range testRecords {
		suite.monitor.RecordTransaction(record)
	}

	// Give monitor time to process
	time.Sleep(100 * time.Millisecond)

	// Get metrics
	metrics := suite.monitor.GetMetrics()
	assert.Equal(suite.T(), int64(3), metrics.TotalTransactions)
	assert.Equal(suite.T(), int64(2), metrics.SuccessfulTransactions)
	assert.Equal(suite.T(), int64(1), metrics.FailedTransactions)
	assert.True(suite.T(), metrics.AverageTransactionTime > 0)

	// Check error tracking
	assert.Contains(suite.T(), metrics.ErrorsByType, "constraint violation")
	assert.Equal(suite.T(), int64(1), metrics.ErrorsByType["constraint violation"])

	// Check type-specific metrics
	insertMetrics, exists := metrics.PerformanceByType["test_insert"]
	assert.True(suite.T(), exists)
	assert.Equal(suite.T(), int64(1), insertMetrics.Count)
	assert.Equal(suite.T(), int64(1), insertMetrics.SuccessCount)

	deleteMetrics, exists := metrics.PerformanceByType["test_delete"]
	assert.True(suite.T(), exists)
	assert.Equal(suite.T(), int64(1), deleteMetrics.Count)
	assert.Equal(suite.T(), int64(1), deleteMetrics.FailureCount)
}

// TestProductTransactionService_Integration tests product transaction service integration
func (suite *TransactionSafetyTestSuite) TestProductTransactionService_Integration() {
	ctx := context.Background()

	// Create product transaction service
	pts := NewProductTransactionService(suite.tm, suite.outboxRepo, &MockLogger{})

	// Test data
	product := &ProductLifecycle{
		ASIN:    "TEST005",
		Title:   "Integration Test Product",
		Brand:   "Test Brand",
		Status:  "pending",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Create associated events
	events := []*OutboxEvent{
		{
			ID:            uuid.New(),
			EventType:     "PRODUCT_CREATED",
			Payload:       json.RawMessage(`{"test": "integration"}`),
			TargetStream:  "test:stream",
			Status:        OutboxStatusPending,
		},
		{
			ID:            uuid.New(),
			EventType:     "PRODUCT_VALIDATION_REQUIRED",
			Payload:       json.RawMessage(`{"validation": "required"}`),
			TargetStream:  "test:stream",
			Status:        OutboxStatusPending,
		},
	}

	// Execute product creation with events
	err := pts.CreateProductWithEvents(ctx, product, events)
	assert.NoError(suite.T(), err)

	// Verify product was created
	var count int
	err = suite.db.QueryRow(ctx, "SELECT COUNT(*) FROM product WHERE asin = $1", product.ASIN).Scan(&count)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), 1, count)

	// Verify events were created in outbox
	err = suite.db.QueryRow(ctx, "SELECT COUNT(*) FROM outbox_event WHERE aggregate_id = $1", product.ASIN).Scan(&count)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), len(events), count)
}

// MockLogger implements a simple logger for testing
type MockLogger struct{}

func (m *MockLogger) Debug(msg string, args ...interface{}) {}
func (m *MockLogger) Info(msg string, args ...interface{})  {}
func (m *MockLogger) Warn(msg string, args ...interface{})  {}
func (m *MockLogger) Error(msg string, args ...interface{}) {}

func TestTransactionSafetySuite(t *testing.T) {
	// Skip tests if database is not available
	config := database.Config{
		Host:     "localhost",
		Port:     5432,
		User:     "postgres",
		Password: "postgres",
		Database: "amazon_scraper_test",
		SSLMode:  "disable",
	}

	ctx := context.Background()
	testDB, err := New(ctx, config)
	if err != nil {
		t.Skipf("Database not available for testing: %v", err)
		return
	}
	testDB.Close()

	suite.Run(t, new(TransactionSafetyTestSuite))
}

// Test helpers
func stringPtr(s string) *string {
	return &s
}