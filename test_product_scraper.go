package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"
	"time"

	"github.com/maltedev/amazon-size-scraper/internal/browser"
	"github.com/maltedev/amazon-size-scraper/internal/database"
	"github.com/maltedev/amazon-size-scraper/internal/scraper"
)

func main() {
	// Logging
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	ctx := context.Background()

	// Database
	db, err := database.New(ctx, database.Config{
		Host:     "localhost",
		Port:     5433,
		User:     "postgres",
		Password: "postgres",
		Database: "tall_affiliate",
		MaxConns: 2,
		MinConns: 1,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// Get some pending products
	products, err := db.GetPendingProducts(ctx, 5)
	if err != nil {
		log.Fatal("Failed to get products:", err)
	}

	logger.Info("found pending products", "count", len(products))
	for _, p := range products {
		logger.Info("product", "asin", p.ASIN, "title", p.Title)
	}

	// Browser - visible for debugging
	b, err := browser.New(&browser.Options{
		Headless: false,
		Timeout:  60 * time.Second,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer b.Close()

	// Create product scraper
	productScraper := scraper.NewProductScraper(b, db)

	// Process first few products
	processed := 0
	for _, product := range products {
		logger.Info("scraping product", "asin", product.ASIN)
		
		if err := productScraper.ScrapeProduct(ctx, product.ASIN); err != nil {
			logger.Error("failed to scrape product", "asin", product.ASIN, "error", err)
		} else {
			processed++
		}
		
		// Only do a few for testing
		if processed >= 3 {
			break
		}
	}

	// Check results
	var completed int
	db.Pool().QueryRow(ctx, "SELECT COUNT(*) FROM products WHERE status = 'completed'").Scan(&completed)
	
	logger.Info("scraping completed", "processed", processed, "completed_in_db", completed)

	// Show size data
	rows, _ := db.Pool().Query(ctx, `
		SELECT asin, title, width_cm, length_cm, height_cm 
		FROM products 
		WHERE status = 'completed' AND length_cm > 0
		LIMIT 5
	`)
	defer rows.Close()

	fmt.Println("\nProducts with size data:")
	for rows.Next() {
		var asin, title string
		var width, length, height float64
		rows.Scan(&asin, &title, &width, &length, &height)
		fmt.Printf("- %s: %s\n", asin, title)
		fmt.Printf("  Dimensions: %.1f x %.1f cm\n", width, length)
	}
}