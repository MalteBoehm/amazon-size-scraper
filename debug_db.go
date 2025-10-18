package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func main() {
	// Database connection
	dbURL := fmt.Sprintf("postgres://postgres:%s@localhost:%s/tall_affiliate?sslmode=disable",
		getEnv("DB_PASSWORD", "postgres"),
		getEnv("DB_PORT", "5433"),
	)

	db, err := sql.Open("pgx", dbURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}

	fmt.Println("Connected to database")

	// Check current product statuses
	fmt.Println("\n=== Current Product Statuses ===")
	rows, err := db.Query(`
		SELECT status, COUNT(*) as count
		FROM products
		GROUP BY status
		ORDER BY status
	`)
	if err != nil {
		log.Fatalf("Failed to query product statuses: %v", err)
	}
	defer rows.Close()

	var totalProducts int
	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			log.Printf("Failed to scan row: %v", err)
			continue
		}
		fmt.Printf("Status: %s, Count: %d\n", status, count)
		totalProducts += count
	}
	fmt.Printf("Total products: %d\n", totalProducts)

	// Check products with size tables
	fmt.Println("\n=== Products with Size Tables ===")
	sizeTableRows, err := db.Query(`
		SELECT
			CASE WHEN size_table IS NOT NULL THEN 'has_size_table' ELSE 'no_size_table' END as size_status,
			COUNT(*) as count
		FROM products
		GROUP BY size_status
	`)
	if err != nil {
		log.Fatalf("Failed to query size tables: %v", err)
	}
	defer sizeTableRows.Close()

	for sizeTableRows.Next() {
		var sizeStatus string
		var count int
		if err := sizeTableRows.Scan(&sizeStatus, &count); err != nil {
			log.Printf("Failed to scan size table row: %v", err)
			continue
		}
		fmt.Printf("Size Status: %s, Count: %d\n", sizeStatus, count)
	}

	// Show sample products
	fmt.Println("\n=== Sample Products ===")
	sampleRows, err := db.Query(`
		SELECT asin, title, status,
			CASE WHEN size_table IS NOT NULL THEN true ELSE false END as has_size_table,
			created_at, updated_at
		FROM products
		ORDER BY created_at DESC
		LIMIT 5
	`)
	if err != nil {
		log.Fatalf("Failed to query sample products: %v", err)
	}
	defer sampleRows.Close()

	for sampleRows.Next() {
		var asin, title, status string
		var hasSizeTable bool
		var createdAt, updatedAt sql.NullTime
		if err := sampleRows.Scan(&asin, &title, &status, &hasSizeTable, &createdAt, &updatedAt); err != nil {
			log.Printf("Failed to scan sample row: %v", err)
			continue
		}
		fmt.Printf("ASIN: %s\n", asin)
		fmt.Printf("  Title: %s\n", title)
		fmt.Printf("  Status: %s\n", status)
		fmt.Printf("  Has Size Table: %t\n", hasSizeTable)
		fmt.Printf("  Created: %s\n", formatTime(createdAt))
		fmt.Printf("  Updated: %s\n", formatTime(updatedAt))
		fmt.Println()
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func formatTime(t sql.NullTime) string {
	if t.Valid {
		return t.Time.Format("2006-01-02 15:04:05")
	}
	return "NULL"
}