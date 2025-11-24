package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/lib/pq" // PostgreSQL driver
)

// Config represents database configuration
type Config struct {
	Host         string
	Port         int
	User         string
	Password     string
	Database     string
	SSLMode      string
	MaxConns     int32
	MinConns     int32
	MaxConnLife  time.Duration
	MaxConnIdle  time.Duration
}

// SqlDB represents a database connection using database/sql
type SqlDB struct {
	db *sql.DB
}

// NewSqlDB creates a new database connection using database/sql with lib/pq
func NewSqlDB(ctx context.Context, cfg Config) (*DB, error) {
	// Create connection string matching api-gateway format
	// Use configured SSL mode, default to disable for local development
	sslMode := cfg.SSLMode
	if sslMode == "" {
		sslMode = "disable"
	}
	connStr := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s application_name=amazon-size-scraper search_path=public",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.Database, sslMode,
	)



	// Open database connection
	sqlDB, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open database connection: %w", err)
	}

	// Configure connection pool
	if cfg.MaxConns > 0 {
		sqlDB.SetMaxOpenConns(int(cfg.MaxConns))
	} else {
		sqlDB.SetMaxOpenConns(25) // Default from api-gateway
	}
	
	if cfg.MinConns > 0 {
		sqlDB.SetMaxIdleConns(int(cfg.MinConns))
	} else {
		sqlDB.SetMaxIdleConns(5) // Default from api-gateway
	}
	
	// Set connection lifetime to avoid stale connections
	// This ensures connections are refreshed before they can expire
	if cfg.MaxConnLife > 0 {
		sqlDB.SetConnMaxLifetime(cfg.MaxConnLife)
	} else {
		sqlDB.SetConnMaxLifetime(10 * time.Minute) // Increased to reduce reconnection frequency
	}
	
	if cfg.MaxConnIdle > 0 {
		sqlDB.SetConnMaxIdleTime(cfg.MaxConnIdle)
	} else {
		sqlDB.SetConnMaxIdleTime(2 * time.Minute) // Increased to keep connections alive longer
	}

	// Test connection with longer timeout
	pingCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	
	if err := sqlDB.PingContext(pingCtx); err != nil {
		sqlDB.Close()
		fmt.Printf("[DB] Connection failed. Error details: %v\n", err)
		fmt.Printf("[DB] Connection parameters - Host: %s, Port: %d, User: %s, DB: %s\n", 
			cfg.Host, cfg.Port, cfg.User, cfg.Database)
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	fmt.Printf("[DB] Successfully connected to database\n")

	// Create wrapper that implements the original DB interface
	return &DB{
		pool: &sqlDBAdapter{db: sqlDB},
	}, nil
}

// sqlDBAdapter adapts sql.DB to match the pgxpool.Pool interface methods we need
type sqlDBAdapter struct {
	db *sql.DB
}

// Close closes the database connection
func (a *sqlDBAdapter) Close() {
	a.db.Close()
}

// QueryRow executes a query that is expected to return at most one row
func (a *sqlDBAdapter) QueryRow(ctx context.Context, sql string, args ...interface{}) Row {
	return &sqlRowAdapter{row: a.db.QueryRowContext(ctx, sql, args...)}
}

// Query executes a query that returns rows
func (a *sqlDBAdapter) Query(ctx context.Context, sql string, args ...interface{}) (Rows, error) {
	rows, err := a.db.QueryContext(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	return &sqlRowsAdapter{rows: rows}, nil
}

// Exec executes a query without returning any rows
func (a *sqlDBAdapter) Exec(ctx context.Context, sql string, args ...interface{}) (CommandTag, error) {
	result, err := a.db.ExecContext(ctx, sql, args...)
	if err != nil {
		return &sqlCommandTag{}, err
	}
	return &sqlCommandTag{result: result}, nil
}

// Begin starts a transaction
func (a *sqlDBAdapter) Begin(ctx context.Context) (Tx, error) {
	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	return &sqlTxAdapter{tx: tx}, nil
}

// Row interface adapter
type Row interface {
	Scan(dest ...interface{}) error
}

type sqlRowAdapter struct {
	row *sql.Row
}

func (r *sqlRowAdapter) Scan(dest ...interface{}) error {
	return r.row.Scan(dest...)
}

// Rows interface adapter
type Rows interface {
	Next() bool
	Scan(dest ...interface{}) error
	Close()
	Err() error
}

type sqlRowsAdapter struct {
	rows *sql.Rows
}

func (r *sqlRowsAdapter) Next() bool {
	return r.rows.Next()
}

func (r *sqlRowsAdapter) Scan(dest ...interface{}) error {
	return r.rows.Scan(dest...)
}

func (r *sqlRowsAdapter) Close() {
	r.rows.Close()
}

func (r *sqlRowsAdapter) Err() error {
	return r.rows.Err()
}

// CommandTag interface adapter
type CommandTag interface {
	RowsAffected() int64
}

type sqlCommandTag struct {
	result sql.Result
}

func (c *sqlCommandTag) RowsAffected() int64 {
	n, _ := c.result.RowsAffected()
	return n
}

// Tx interface adapter
type Tx interface {
	Exec(ctx context.Context, sql string, args ...interface{}) (CommandTag, error)
	QueryRow(ctx context.Context, sql string, args ...interface{}) Row
	Query(ctx context.Context, sql string, args ...interface{}) (Rows, error)
	Commit(ctx context.Context) error
	Rollback(ctx context.Context) error
}

type sqlTxAdapter struct {
	tx *sql.Tx
}

func (t *sqlTxAdapter) Exec(ctx context.Context, sql string, args ...interface{}) (CommandTag, error) {
	result, err := t.tx.ExecContext(ctx, sql, args...)
	if err != nil {
		return &sqlCommandTag{}, err
	}
	return &sqlCommandTag{result: result}, nil
}

func (t *sqlTxAdapter) QueryRow(ctx context.Context, sql string, args ...interface{}) Row {
	return &sqlRowAdapter{row: t.tx.QueryRowContext(ctx, sql, args...)}
}

func (t *sqlTxAdapter) Query(ctx context.Context, sql string, args ...interface{}) (Rows, error) {
	rows, err := t.tx.QueryContext(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	return &sqlRowsAdapter{rows: rows}, nil
}

func (t *sqlTxAdapter) Commit(ctx context.Context) error {
	return t.tx.Commit()
}

func (t *sqlTxAdapter) Rollback(ctx context.Context) error {
	return t.tx.Rollback()
}

// Pool interface to match what the DB struct expects
type Pool interface {
	Close()
	QueryRow(ctx context.Context, sql string, args ...interface{}) Row
	Query(ctx context.Context, sql string, args ...interface{}) (Rows, error)
	Exec(ctx context.Context, sql string, args ...interface{}) (CommandTag, error)
	Begin(ctx context.Context) (Tx, error)
}

// Update the DB struct to use the interface
type DB struct {
	pool Pool
}

// New creates a new database connection (redirects to NewSqlDB for compatibility)
func New(ctx context.Context, cfg Config) (*DB, error) {
	return NewSqlDB(ctx, cfg)
}

// Close closes the database connection
func (db *DB) Close() {
	db.pool.Close()
}

// Ping validates the database connection
func (db *DB) Ping(ctx context.Context) error {
	// Try a simple query to validate the connection
	var result int
	row := db.pool.QueryRow(ctx, "SELECT 1")
	return row.Scan(&result)
}

// Transaction executes a function within a database transaction
func (db *DB) Transaction(ctx context.Context, fn func(Tx) error) error {
	tx, err := db.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	defer func() {
		if err != nil {
			if rbErr := tx.Rollback(ctx); rbErr != nil {
				err = fmt.Errorf("tx rollback failed: %v (original error: %w)", rbErr, err)
			}
		}
	}()

	if err = fn(tx); err != nil {
		return err
	}

	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// Exec executes a query without returning any rows
func (db *DB) Exec(ctx context.Context, sql string, args ...interface{}) (CommandTag, error) {
	return db.pool.Exec(ctx, sql, args...)
}

// Query executes a query that returns rows
func (db *DB) Query(ctx context.Context, sql string, args ...interface{}) (Rows, error) {
	return db.pool.Query(ctx, sql, args...)
}

// QueryRow executes a query that is expected to return at most one row
func (db *DB) QueryRow(ctx context.Context, sql string, args ...interface{}) Row {
	return db.pool.QueryRow(ctx, sql, args...)
}

// Pool returns the underlying pool interface
func (db *DB) Pool() Pool {
	return db.pool
}

// WithTx is a transaction helper
func (db *DB) WithTx(ctx context.Context, fn func(Tx) error) error {
	return db.Transaction(ctx, fn)
}

