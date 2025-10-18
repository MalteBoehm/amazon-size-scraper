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
	dbURL := fmt.Sprintf("postgres://postgres:%s@%s:%s/tall_affiliate?sslmode=disable",
		getEnv("DB_PASSWORD", "postgres"),
		getEnv("DB_HOST", "49.13.49.90"),
		getEnv("DB_PORT", "5111"),
	)

	db, err := sql.Open("pgx", dbURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}

	fmt.Println("Connected to production database")

	// Check current status
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
	var pendingCount int
	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			log.Printf("Failed to scan row: %v", err)
			continue
		}
		fmt.Printf("Status: %s, Count: %d\n", status, count)
		totalProducts += count
		if status == "pending" {
			pendingCount = count
		}
	}
	fmt.Printf("Total products: %d\n", totalProducts)
	fmt.Printf("Current pending products: %d\n", pendingCount)

	if pendingCount > 0 {
		fmt.Println("Already have pending products - no need to fix")
		return
	}

	// Strategy 1: Find products that are completed but should be re-scraped for size
	fmt.Println("\n=== Strategy 1: Find completed products without size data ===")
	strategy1Rows, err := db.Query(`
		SELECT asin, title, status
		FROM products
		WHERE status = 'completed'
		AND (size_table IS NULL OR size_table = 'null'::jsonb)
		LIMIT 10
	`)
	if err != nil {
		log.Fatalf("Failed to query strategy 1: %v", err)
	}
	defer strategy1Rows.Close()

	var productsToReset []struct {
		ASIN   string
		Title  string
		Status string
	}

	for strategy1Rows.Next() {
		var asin, title, status string
		if err := strategy1Rows.Scan(&asin, &title, &status); err != nil {
			log.Printf("Failed to scan strategy1 row: %v", err)
			continue
		}
		productsToReset = append(productsToReset, struct {
			ASIN   string
			Title  string
			Status string
		}{asin, title, status})
		fmt.Printf("Found candidate: %s - %s\n", asin, title)
	}

	if len(productsToReset) > 0 {
		fmt.Printf("Resetting %d products to pending status\n", len(productsToReset))

		tx, err := db.Begin()
		if err != nil {
			log.Fatalf("Failed to begin transaction: %v", err)
		}

		for _, product := range productsToReset {
			_, err := tx.Exec(`
				UPDATE products
				SET status = 'pending',
				    updated_at = NOW(),
				    scraped_at = NULL
				WHERE asin = $1
			`, product.ASIN)
			if err != nil {
				tx.Rollback()
				log.Fatalf("Failed to update product %s: %v", product.ASIN, err)
			}
			fmt.Printf("Reset %s to pending\n", product.ASIN)
		}

		if err := tx.Commit(); err != nil {
			log.Fatalf("Failed to commit transaction: %v", err)
		}
		fmt.Println("Successfully reset products to pending status")
		return
	}

	// Strategy 2: Reset some completed products for re-processing
	fmt.Println("\n=== Strategy 2: Reset some completed products ===")
	strategy2Rows, err := db.Query(`
		SELECT asin, title
		FROM products
		WHERE status = 'completed'
		ORDER BY updated_at DESC
		LIMIT 10
	`)
	if err != nil {
		log.Fatalf("Failed to query strategy 2: %v", err)
	}
	defer strategy2Rows.Close()

	for strategy2Rows.Next() {
		var asin, title string
		if err := strategy2Rows.Scan(&asin, &title); err != nil {
			log.Printf("Failed to scan strategy2 row: %v", err)
			continue
		}
		productsToReset = append(productsToReset, struct {
			ASIN   string
			Title  string
			Status string
		}{asin, title, "completed"})
		fmt.Printf("Found completed product: %s - %s\n", asin, title)
	}

	if len(productsToReset) > 0 {
		fmt.Printf("Resetting %d completed products to pending status\n", len(productsToReset))

		tx, err := db.Begin()
		if err != nil {
			log.Fatalf("Failed to begin transaction: %v", err)
		}

		for _, product := range productsToReset {
			_, err := tx.Exec(`
				UPDATE products
				SET status = 'pending',
				    updated_at = NOW(),
				    scraped_at = NULL
				WHERE asin = $1
			`, product.ASIN)
			if err != nil {
				tx.Rollback()
				log.Fatalf("Failed to update product %s: %v", product.ASIN, err)
			}
			fmt.Printf("Reset %s to pending\n", product.ASIN)
		}

		if err := tx.Commit(); err != nil {
			log.Fatalf("Failed to commit transaction: %v", err)
		}
		fmt.Println("Successfully reset products to pending status")
		return
	}

	fmt.Println("No products found to reset")
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}