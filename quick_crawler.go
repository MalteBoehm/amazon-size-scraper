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

	// Database connection
	dbConfig := database.Config{
		Host:        "localhost",
		Port:        5433,
		User:        "postgres",
		Password:    "postgres",
		Database:    "tall_affiliate",
		MaxConns:    2,
		MinConns:    1,
		MaxConnLife: 5 * time.Minute,
		MaxConnIdle: 1 * time.Minute,
	}

	db, err := database.New(ctx, dbConfig)
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}
	defer db.Close()

	logger.Info("Connected to database")

	// Browser setup
	browserOpts := browser.DefaultOptions()
	browserOpts.Headless = true // Faster in headless mode

	b, err := browser.New(browserOpts)
	if err != nil {
		log.Fatal("Failed to create browser:", err)
	}
	defer b.Close()

	// Create search crawler
	searchCrawler := scraper.NewSearchCrawler(b, db)
	
	// Crawl just the first page
	searchURL := "https://www.amazon.de/s?k=t-shirt+größentabelle+länge&i=fashion"
	logger.Info("Starting search crawl", "url", searchURL)
	
	if err := searchCrawler.CrawlSearch(ctx, searchURL); err != nil {
		logger.Error("Search crawl failed", "error", err)
	}

	// Get statistics
	counts, err := db.CountProductsByStatus(ctx)
	if err != nil {
		logger.Error("Failed to get counts", "error", err)
	} else {
		total := 0
		for status, count := range counts {
			logger.Info("Product count", "status", status, "count", count)
			total += count
		}
		fmt.Printf("\n✅ Total products collected: %d\n", total)
	}

	// Show some products
	products, err := db.GetPendingProducts(ctx, 5)
	if err == nil && len(products) > 0 {
		fmt.Println("\nSample products:")
		for _, p := range products {
			fmt.Printf("- %s: %s\n", p.ASIN, p.Title)
		}
	}
}