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
	// Logging with debug level
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
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

	// Clear table
	db.Pool().Exec(ctx, "TRUNCATE TABLE products")
	logger.Info("Cleared products table")

	// Browser with visible window
	b, err := browser.New(&browser.Options{
		Headless: false,
		Timeout:  60 * time.Second,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer b.Close()

	// Create crawler
	crawler := scraper.NewSearchCrawler(b, db)
	
	// Start crawling
	searchURL := "https://www.amazon.de/s?k=t-shirt+größentabelle+länge&i=fashion"
	logger.Info("Starting crawl", "url", searchURL)
	
	start := time.Now()
	
	// Run crawler
	if err := crawler.CrawlSearch(ctx, searchURL); err != nil {
		logger.Error("Crawl failed", "error", err)
	}
	
	duration := time.Since(start)
	
	// Get counts
	var total int
	db.Pool().QueryRow(ctx, "SELECT COUNT(*) FROM products").Scan(&total)
	
	fmt.Printf("\n=== RESULTS ===\n")
	fmt.Printf("Total products: %d\n", total)
	fmt.Printf("Time taken: %s\n", duration)
	
	// Show sample
	rows, _ := db.Pool().Query(ctx, "SELECT asin, LEFT(title, 50) FROM products LIMIT 5")
	defer rows.Close()
	
	fmt.Println("\nSample products:")
	for rows.Next() {
		var asin, title string
		rows.Scan(&asin, &title)
		fmt.Printf("- %s: %s\n", asin, title)
	}
}