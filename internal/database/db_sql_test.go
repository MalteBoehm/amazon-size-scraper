package database

import (
	"context"
	"testing"
	"time"
)

func TestNewSqlDB(t *testing.T) {
	// Skip if no database is available
	t.Skip("Skipping database connection test - requires running PostgreSQL")

	ctx := context.Background()
	cfg := Config{
		Host:         "localhost",
		Port:         5433,
		User:         "postgres",
		Password:     "postgres",
		Database:     "tall_affiliate",
		MaxConns:     25,
		MinConns:     5,
		MaxConnLife:  5 * time.Minute,
		MaxConnIdle:  1 * time.Minute,
	}

	db, err := NewSqlDB(ctx, cfg)
	if err != nil {
		t.Fatalf("Failed to create database connection: %v", err)
	}
	defer db.Close()

	// Test ping
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var result int
	err = db.pool.QueryRow(ctx, "SELECT 1").Scan(&result)
	if err != nil {
		t.Fatalf("Failed to execute test query: %v", err)
	}

	if result != 1 {
		t.Errorf("Expected 1, got %d", result)
	}
}