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
	// Setup logging
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	ctx := context.Background()

	// Database
	dbConfig := database.Config{
		Host:        "localhost",
		Port:        5433,
		User:        "postgres",
		Password:    "postgres",
		Database:    "tall_affiliate",
		MaxConns:    2,
		MinConns:    1,
		MaxConnLife: 5 * time.Minute,
	}

	db, err := database.New(ctx, dbConfig)
	if err != nil {
		log.Fatal("DB error:", err)
	}
	defer db.Close()

	// Clear previous data
	db.Pool().Exec(ctx, "TRUNCATE TABLE products")
	logger.Info("Cleared products table")

	// Browser
	browserOpts := browser.DefaultOptions()
	browserOpts.Headless = true

	b, err := browser.New(browserOpts)
	if err != nil {
		log.Fatal("Browser error:", err)
	}
	defer b.Close()

	// Create search crawler
	crawler := scraper.NewSearchCrawler(b, db)
	
	// Modify to only crawl first page
	searchURL := "https://www.amazon.de/s?k=t-shirt+größentabelle+länge&i=fashion"
	
	// Start crawling
	logger.Info("Starting crawl", "url", searchURL)
	
	startTime := time.Now()
	if err := crawler.CrawlSearch(ctx, searchURL); err != nil {
		logger.Error("Crawl failed", "error", err)
	}
	
	duration := time.Since(startTime)
	
	// Get final counts
	var total int
	db.Pool().QueryRow(ctx, "SELECT COUNT(*) FROM products").Scan(&total)
	
	// Show some products
	rows, _ := db.Pool().Query(ctx, "SELECT asin, LEFT(title, 60) as title FROM products LIMIT 10")
	defer rows.Close()
	
	fmt.Println("\n=== RESULTS ===")
	fmt.Printf("Total products collected: %d\n", total)
	fmt.Printf("Time taken: %s\n", duration)
	fmt.Println("\nSample products:")
	
	for rows.Next() {
		var asin, title string
		rows.Scan(&asin, &title)
		fmt.Printf("- %s: %s\n", asin, title)
	}
}