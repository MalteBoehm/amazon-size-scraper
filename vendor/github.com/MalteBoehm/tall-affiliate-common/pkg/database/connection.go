package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/lib/pq" // PostgreSQL driver
)

// Config holds database configuration
type Config struct {
	Host            string
	Port            int
	User            string
	Password        string
	Database        string
	SSLMode         string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
}

// NewConnection creates a new database connection with the given configuration
func NewConnection(config *Config) (*sql.DB, error) {
	if err := validateConfig(config); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	// Build connection string
	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		config.Host, config.Port, config.User, config.Password, config.Database, config.SSLMode)

	// Open database connection
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Configure connection pool
	if config.MaxOpenConns > 0 {
		db.SetMaxOpenConns(config.MaxOpenConns)
	}
	if config.MaxIdleConns > 0 {
		db.SetMaxIdleConns(config.MaxIdleConns)
	}
	if config.ConnMaxLifetime > 0 {
		db.SetConnMaxLifetime(config.ConnMaxLifetime)
	}

	// Test the connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return db, nil
}

// validateConfig validates the database configuration
func validateConfig(config *Config) error {
	if config.Host == "" {
		return fmt.Errorf("host cannot be empty")
	}
	if config.Port <= 0 {
		return fmt.Errorf("port must be positive")
	}
	if config.User == "" {
		return fmt.Errorf("user cannot be empty")
	}
	if config.Database == "" {
		return fmt.Errorf("database cannot be empty")
	}
	if config.SSLMode == "" {
		config.SSLMode = "disable"
	}
	return nil
}

// ExecuteInTransaction executes a function within a database transaction
func ExecuteInTransaction(ctx context.Context, db *sql.DB, fn func(*sql.Tx) error) (err error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	// Ensure rollback on panic
	defer func() {
		if p := recover(); p != nil {
			tx.Rollback()
			err = fmt.Errorf("panic in transaction: %v", p)
		}
	}()

	// Rollback on error
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	// Execute the function
	if err = fn(tx); err != nil {
		return err
	}

	// Commit the transaction
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// WithRetry executes a function with retry logic
func WithRetry(maxRetries int, delay time.Duration, fn func() error) error {
	var err error
	for i := 0; i < maxRetries; i++ {
		if err = fn(); err == nil {
			return nil
		}
		if i < maxRetries-1 {
			time.Sleep(delay)
		}
	}
	return fmt.Errorf("failed after %d retries: %w", maxRetries, err)
}

// IsNoRowsError checks if an error is a no rows error
func IsNoRowsError(err error) bool {
	return err == sql.ErrNoRows
}

// BuildDSN builds a PostgreSQL DSN from configuration
func BuildDSN(config *Config) string {
	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		config.Host, config.Port, config.User, config.Password, config.Database, config.SSLMode)
}