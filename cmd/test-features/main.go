package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"

	"github.com/maltedev/amazon-size-scraper/internal/browser"
	"github.com/maltedev/amazon-size-scraper/internal/config"
	"github.com/maltedev/amazon-size-scraper/internal/parser"
	"github.com/maltedev/amazon-size-scraper/internal/scraper"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run main.go <ASIN>")
		fmt.Println("Example: go run main.go B08N5WRWNW")
		os.Exit(1)
	}

	asin := os.Args[1]

	// Setup logging
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	logger.Info("testing feature extraction", "asin", asin)

	// Load config
	cfg, err := config.Load()
	if err != nil {
		logger.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	// Create browser
	browserOpts := browser.DefaultOptions()
	browserOpts.Headless = true

	b, err := browser.New(browserOpts)
	if err != nil {
		logger.Error("failed to create browser", "error", err)
		os.Exit(1)
	}
	defer b.Close()

	// Create scraper
	parser := parser.NewAmazonParser()
	amazonScraper := scraper.NewAmazonScraperLegacy(b, parser, logger, cfg)
	defer amazonScraper.Close()

	// Test scraping
	ctx := context.Background()
	product, err := amazonScraper.ScrapeByASIN(ctx, asin)
	if err != nil {
		logger.Error("scraping failed", "error", err)
		os.Exit(1)
	}

	// Pretty print the results
	productJSON, _ := json.MarshalIndent(product, "", "  ")
	fmt.Println(string(productJSON))

	// Highlight the new features
	fmt.Printf("\n=== NEW EXTRACTED DATA ===\n")
	fmt.Printf("Features (%d items):\n", len(product.Features))
	for i, feature := range product.Features {
		fmt.Printf("  %d. %s\n", i+1, feature)
	}

	fmt.Printf("\nProduct Groups (%d items):\n", len(product.ProductGroups))
	for i, group := range product.ProductGroups {
		fmt.Printf("  %d. %s\n", i+1, group)
	}
}