package database

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"sync"
	"time"

	_ "github.com/lib/pq"
)

// ConnectionPoolManager manages database connection pools with monitoring and health checks
type ConnectionPoolManager struct {
	config         PoolConfig
	db             *sql.DB
	logger         *slog.Logger
	healthChecker  *HealthChecker
	metrics        *PoolMetrics
	mu             sync.RWMutex
	isHealthy      bool
	lastHealthCheck time.Time
}

// PoolConfig defines connection pool configuration
type PoolConfig struct {
	// Basic connection settings
	Host            string
	Port            int
	User            string
	Password        string
	Database        string
	SSLMode         string

	// Pool sizing
	MaxOpenConns    int
	MaxIdleConns    int
	MinOpenConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
	ConnMaxAge      time.Duration

	// Health check settings
	HealthCheckInterval time.Duration
	HealthCheckTimeout  time.Duration
	MaxHealthFailures   int

	// Connection retry settings
	ConnectRetryCount   int
	ConnectRetryDelay   time.Duration
	ConnectTimeout      time.Duration

	// Query timeout settings
	DefaultQueryTimeout time.Duration
	MaxQueryTimeout     time.Duration
	SlowQueryThreshold  time.Duration
}

// DefaultPoolConfig returns sensible defaults for connection pool
func DefaultPoolConfig() PoolConfig {
	return PoolConfig{
		MaxOpenConns:       25,
		MaxIdleConns:       10,
		MinOpenConns:       5,
		ConnMaxLifetime:    1 * time.Hour,
		ConnMaxIdleTime:    30 * time.Minute,
		ConnMaxAge:         2 * time.Hour,

		HealthCheckInterval: 30 * time.Second,
		HealthCheckTimeout:  5 * time.Second,
		MaxHealthFailures:   3,

		ConnectRetryCount:   5,
		ConnectRetryDelay:   1 * time.Second,
		ConnectTimeout:      30 * time.Second,

		DefaultQueryTimeout: 30 * time.Second,
		MaxQueryTimeout:     5 * time.Minute,
		SlowQueryThreshold:  1 * time.Second,
	}
}

// PoolMetrics tracks connection pool metrics
type PoolMetrics struct {
	mu                    sync.RWMutex
	OpenConnections       int
	InUseConnections      int
	IdleConnections       int
	TotalConnections      int64
	TotalQueries          int64
	SlowQueries           int64
	FailedQueries         int64
	FailedConnections     int64
	LastHealthCheck       time.Time
	HealthCheckFailures   int
	AverageQueryTime      time.Duration
	PeakOpenConnections   int
	PeakInUseConnections  int
}

// HealthChecker performs database health checks
type HealthChecker struct {
	interval time.Duration
	timeout  time.Duration
	maxFails int
	failures int
	stopCh   chan struct{}
}

// NewConnectionPoolManager creates a new connection pool manager
func NewConnectionPoolManager(config PoolConfig, logger *slog.Logger) (*ConnectionPoolManager, error) {
	// Apply defaults for missing values
	if config.MaxOpenConns == 0 {
		config.MaxOpenConns = DefaultPoolConfig().MaxOpenConns
	}
	if config.MaxIdleConns == 0 {
		config.MaxIdleConns = DefaultPoolConfig().MaxIdleConns
	}
	if config.ConnMaxLifetime == 0 {
		config.ConnMaxLifetime = DefaultPoolConfig().ConnMaxLifetime
	}
	if config.ConnMaxIdleTime == 0 {
		config.ConnMaxIdleTime = DefaultPoolConfig().ConnMaxIdleTime
	}
	if config.HealthCheckInterval == 0 {
		config.HealthCheckInterval = DefaultPoolConfig().HealthCheckInterval
	}
	if config.HealthCheckTimeout == 0 {
		config.HealthCheckTimeout = DefaultPoolConfig().HealthCheckTimeout
	}
	if config.MaxHealthFailures == 0 {
		config.MaxHealthFailures = DefaultPoolConfig().MaxHealthFailures
	}
	if config.ConnectRetryCount == 0 {
		config.ConnectRetryCount = DefaultPoolConfig().ConnectRetryCount
	}
	if config.ConnectRetryDelay == 0 {
		config.ConnectRetryDelay = DefaultPoolConfig().ConnectRetryDelay
	}
	if config.ConnectTimeout == 0 {
		config.ConnectTimeout = DefaultPoolConfig().ConnectTimeout
	}

	cpm := &ConnectionPoolManager{
		config: config,
		logger: logger,
		metrics: &PoolMetrics{},
		isHealthy: false,
	}

	// Create database connection with retry logic
	db, err := cpm.connectWithRetry(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	cpm.db = db
	cpm.isHealthy = true
	cpm.lastHealthCheck = time.Now()

	// Configure connection pool
	cpm.configurePool()

	// Start health checker
	cpm.startHealthChecker()

	// Start metrics collector
	go cpm.collectMetrics()

	return cpm, nil
}

// connectWithRetry attempts to connect to the database with retry logic
func (cpm *ConnectionPoolManager) connectWithRetry(ctx context.Context) (*sql.DB, error) {
	var lastErr error

	for attempt := 0; attempt < cpm.config.ConnectRetryCount; attempt++ {
		if attempt > 0 {
			cpm.logger.Warn("Retrying database connection",
				"attempt", attempt+1,
				"max_attempts", cpm.config.ConnectRetryCount,
				"delay", cpm.config.ConnectRetryDelay,
				"last_error", lastErr,
			)

			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(cpm.config.ConnectRetryDelay):
			}
		}

		db, err := cpm.createConnection(ctx)
		if err == nil {
			return db, nil
		}

		lastErr = err
		cpm.metrics.mu.Lock()
		cpm.metrics.FailedConnections++
		cpm.metrics.mu.Unlock()
	}

	return nil, fmt.Errorf("failed to connect after %d attempts: %w", cpm.config.ConnectRetryCount, lastErr)
}

// createConnection creates a new database connection
func (cpm *ConnectionPoolManager) createConnection(ctx context.Context) (*sql.DB, error) {
	// Use configured SSL mode, default to disable for local development
	sslMode := cpm.config.SSLMode
	if sslMode == "" {
		sslMode = "disable"
	}

	connStr := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s connect_timeout=%d",
		cpm.config.Host, cpm.config.Port, cpm.config.User, cpm.config.Password,
		cpm.config.Database, sslMode, int(cpm.config.ConnectTimeout.Seconds()),
	)

	// Add additional connection parameters for better reliability
	connStr += "&application_name=amazon-size-scraper"
	connStr += "&binary_parameters=yes"
	connStr += "&default_transaction_isolation=read_committed"
	connStr += "&timezone=UTC"

	// Log connection attempt (masking password)
	maskedConnStr := fmt.Sprintf(
		"host=%s port=%d user=%s password=**** dbname=%s sslmode=%s connect_timeout=%d",
		cpm.config.Host, cpm.config.Port, cpm.config.User,
		cpm.config.Database, sslMode, int(cpm.config.ConnectTimeout.Seconds()),
	)

	cpm.logger.Info("Attempting database connection",
		"dsn", maskedConnStr,
		"timeout", cpm.config.ConnectTimeout,
	)

	// Create connection with timeout
	connectCtx, cancel := context.WithTimeout(ctx, cpm.config.ConnectTimeout)
	defer cancel()

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open database connection: %w", err)
	}

	// Test connection with timeout
	if err := db.PingContext(connectCtx); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	cpm.logger.Info("Successfully connected to database",
		"host", cpm.config.Host,
		"database", cpm.config.Database,
	)

	return db, nil
}

// configurePool configures the database connection pool
func (cpm *ConnectionPoolManager) configurePool() {
	cpm.db.SetMaxOpenConns(cpm.config.MaxOpenConns)
	cpm.db.SetMaxIdleConns(cpm.config.MaxIdleConns)
	cpm.db.SetConnMaxLifetime(cpm.config.ConnMaxLifetime)
	cpm.db.SetConnMaxIdleTime(cpm.config.ConnMaxIdleTime)

	cpm.logger.Info("Configured connection pool",
		"max_open_conns", cpm.config.MaxOpenConns,
		"max_idle_conns", cpm.config.MaxIdleConns,
		"conn_max_lifetime", cpm.config.ConnMaxLifetime,
		"conn_max_idle_time", cpm.config.ConnMaxIdleTime,
	)
}

// startHealthChecker starts the background health checker
func (cpm *ConnectionPoolManager) startHealthChecker() {
	cpm.healthChecker = &HealthChecker{
		interval: cpm.config.HealthCheckInterval,
		timeout:  cpm.config.HealthCheckTimeout,
		maxFails: cpm.config.MaxHealthFailures,
		stopCh:   make(chan struct{}),
	}

	go func() {
		ticker := time.NewTicker(cpm.healthChecker.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				cpm.performHealthCheck()
			case <-cpm.healthChecker.stopCh:
				return
			}
		}
	}()

	cpm.logger.Info("Started health checker",
		"interval", cpm.config.HealthCheckInterval,
		"timeout", cpm.config.HealthCheckTimeout,
	)
}

// performHealthCheck performs a database health check
func (cpm *ConnectionPoolManager) performHealthCheck() {
	ctx, cancel := context.WithTimeout(context.Background(), cpm.config.HealthCheckTimeout)
	defer cancel()

	start := time.Now()
	err := cpm.db.PingContext(ctx)
	duration := time.Since(start)

	cpm.metrics.mu.Lock()
	cpm.metrics.LastHealthCheck = time.Now()

	if err != nil {
		cpm.healthChecker.failures++
		cpm.metrics.HealthCheckFailures++
		cpm.mu.Lock()
		cpm.isHealthy = false
		cpm.mu.Unlock()

		cpm.logger.Error("Health check failed",
			"error", err,
			"duration", duration,
			"failures", cpm.healthChecker.failures,
			"max_failures", cpm.healthChecker.maxFails,
		)

		// Attempt reconnection if we've exceeded max failures
		if cpm.healthChecker.failures >= cpm.healthChecker.maxFails {
			cpm.logger.Warn("Attempting to reconnect due to health check failures")
			if reconnectErr := cpm.reconnect(); reconnectErr != nil {
				cpm.logger.Error("Failed to reconnect", "error", reconnectErr)
			}
		}
	} else {
		cpm.healthChecker.failures = 0
		cpm.mu.Lock()
		cpm.isHealthy = true
		cpm.lastHealthCheck = time.Now()
		cpm.mu.Unlock()

		cpm.logger.Debug("Health check passed", "duration", duration)
	}
	cpm.metrics.mu.Unlock()
}

// reconnect attempts to reconnect to the database
func (cpm *ConnectionPoolManager) reconnect() error {
	cpm.mu.Lock()
	defer cpm.mu.Unlock()

	// Close existing connection
	if cpm.db != nil {
		cpm.db.Close()
	}

	// Attempt new connection
	ctx, cancel := context.WithTimeout(context.Background(), cpm.config.ConnectTimeout)
	defer cancel()

	db, err := cpm.connectWithRetry(ctx)
	if err != nil {
		return fmt.Errorf("failed to reconnect: %w", err)
	}

	cpm.db = db
	cpm.configurePool()
	cpm.healthChecker.failures = 0
	cpm.isHealthy = true
	cpm.lastHealthCheck = time.Now()

	cpm.logger.Info("Successfully reconnected to database")
	return nil
}

// collectMetrics periodically collects connection pool metrics
func (cpm *ConnectionPoolManager) collectMetrics() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		stats := cpm.db.Stats()

		cpm.metrics.mu.Lock()
		cpm.metrics.OpenConnections = stats.OpenConnections
		cpm.metrics.InUseConnections = stats.InUse
		cpm.metrics.IdleConnections = stats.Idle

		// Track peak usage
		if stats.OpenConnections > cpm.metrics.PeakOpenConnections {
			cpm.metrics.PeakOpenConnections = stats.OpenConnections
		}
		if stats.InUse > cpm.metrics.PeakInUseConnections {
			cpm.metrics.PeakInUseConnections = stats.InUse
		}

		cpm.metrics.mu.Unlock()

		// Log if we're approaching pool limits
		if stats.OpenConnections >= cpm.config.MaxOpenConns*90/100 {
			cpm.logger.Warn("Connection pool approaching limit",
				"open_connections", stats.OpenConnections,
				"max_connections", cpm.config.MaxOpenConns,
				"utilization_percent", stats.OpenConnections*100/cpm.config.MaxOpenConns,
			)
		}
	}
}

// GetDB returns the database connection
func (cpm *ConnectionPoolManager) GetDB() *DB {
	return &DB{pool: &sqlDBAdapter{db: cpm.db}}
}

// IsHealthy returns the current health status
func (cpm *ConnectionPoolManager) IsHealthy() bool {
	cpm.mu.RLock()
	defer cpm.mu.RUnlock()
	return cpm.isHealthy
}

// GetMetrics returns current pool metrics
func (cpm *ConnectionPoolManager) GetMetrics() PoolMetrics {
	cpm.metrics.mu.RLock()
	defer cpm.metrics.mu.RUnlock()
	return *cpm.metrics
}

// Close closes the connection pool and stops background processes
func (cpm *ConnectionPoolManager) Close() error {
	// Stop health checker
	if cpm.healthChecker != nil {
		close(cpm.healthChecker.stopCh)
	}

	// Close database connection
	if cpm.db != nil {
		return cpm.db.Close()
	}

	return nil
}

// QueryWithContext executes a query with context and timeout
func (cpm *ConnectionPoolManager) QueryWithContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	start := time.Now()

	// Apply query timeout if not already set
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, cpm.config.DefaultQueryTimeout)
		defer cancel()
	}

	rows, err := cpm.db.QueryContext(ctx, query, args...)
	duration := time.Since(start)

	// Update metrics
	cpm.metrics.mu.Lock()
	cpm.metrics.TotalQueries++
	if duration > cpm.config.SlowQueryThreshold {
		cpm.metrics.SlowQueries++
		cpm.logger.Warn("Slow query detected",
			"query", query,
			"duration", duration,
			"threshold", cpm.config.SlowQueryThreshold,
		)
	}
	if err != nil {
		cpm.metrics.FailedQueries++
	}
	cpm.metrics.mu.Unlock()

	return rows, err
}

// TransactionOptions options for transaction execution
type TransactionOptions struct {
	IsolationLevel sql.IsolationLevel
	ReadOnly       bool
	Timeout        time.Duration
	Retryable      bool
}

// ExecuteInTransaction executes a function within a transaction with enhanced safety
func (cpm *ConnectionPoolManager) ExecuteInTransaction(
	ctx context.Context,
	fn func(*sql.Tx) error,
	opts TransactionOptions,
) error {
	// Apply timeout if not already set
	if _, hasDeadline := ctx.Deadline(); !hasDeadline && opts.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, opts.Timeout)
		defer cancel()
	}

	// Check health before starting transaction
	if !cpm.IsHealthy() {
		return fmt.Errorf("database is not healthy")
	}

	tx, err := cpm.db.BeginTx(ctx, &sql.TxOptions{
		Isolation: opts.IsolationLevel,
		ReadOnly:  opts.ReadOnly,
	})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	defer func() {
		if err != nil {
			if rbErr := tx.Rollback(); rbErr != nil {
				cpm.logger.Error("Transaction rollback failed",
					"rollback_error", rbErr,
					"original_error", err,
				)
			}
		}
	}()

	// Execute user function
	if err = fn(tx); err != nil {
		return err
	}

	// Commit transaction
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}