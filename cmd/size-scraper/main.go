package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/maltedev/amazon-size-scraper/internal/browser"
	"github.com/maltedev/amazon-size-scraper/internal/database"
	"github.com/maltedev/amazon-size-scraper/internal/scraper"
)

func main() {
	// Command line flags
	var (
		searchURL   = flag.String("search", "", "Amazon search URL to crawl")
		dbHost      = flag.String("db-host", getEnv("DB_HOST", "localhost"), "Database host")
		dbPort      = flag.Int("db-port", getEnvInt("DB_PORT", 5432), "Database port")
		dbUser      = flag.String("db-user", getEnv("DB_USER", "postgres"), "Database user")
		dbPassword  = flag.String("db-password", getEnv("DB_PASSWORD", ""), "Database password")
		dbName      = flag.String("db-name", getEnv("DB_NAME", "amazon_scraper"), "Database name")
		headless    = flag.Bool("headless", getEnvBool("HEADLESS", true), "Run browser in headless mode")
		concurrent  = flag.Int("concurrent", getEnvInt("CONCURRENT_SCRAPERS", 1), "Number of concurrent product scrapers")
		scrapeOnly  = flag.Bool("scrape-only", false, "Only scrape products, don't crawl search results")
	)
	flag.Parse()
	
	// Setup logging
	logLevel := slog.LevelInfo
	if os.Getenv("LOG_LEVEL") == "debug" {
		logLevel = slog.LevelDebug
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevel,
	}))
	slog.SetDefault(logger)
	
	// Context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	// Handle shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		logger.Info("received shutdown signal")
		cancel()
	}()
	
	// Database connection
	dbConfig := database.Config{
		Host:        *dbHost,
		Port:        *dbPort,
		User:        *dbUser,
		Password:    *dbPassword,
		Database:    *dbName,
		MaxConns:    int32(*concurrent * 2),
		MinConns:    1,
		MaxConnLife: 5 * time.Minute,
		MaxConnIdle: 1 * time.Minute,
	}
	
	db, err := database.New(ctx, dbConfig)
	if err != nil {
		logger.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer db.Close()
	
	logger.Info("connected to database")
	
	// Browser setup
	browserOpts := browser.DefaultOptions()
	browserOpts.Headless = *headless
	
	// Phase 1: Search crawling (if URL provided and not scrape-only)
	if *searchURL != "" && !*scrapeOnly {
		logger.Info("starting search crawl phase", "url", *searchURL)
		
		b, err := browser.New(browserOpts)
		if err != nil {
			logger.Error("failed to create browser", "error", err)
			os.Exit(1)
		}
		
		searchCrawler := scraper.NewSearchCrawler(b, db)
		if err := searchCrawler.CrawlSearch(ctx, *searchURL); err != nil {
			logger.Error("search crawl failed", "error", err)
			b.Close()
			os.Exit(1)
		}
		
		b.Close()
		logger.Info("search crawl completed")
		
		// Get stats
		counts, _ := db.CountProductsByStatus(ctx)
		logger.Info("product statistics", 
			"pending", counts[database.StatusPending],
			"completed", counts[database.StatusCompleted],
			"failed", counts[database.StatusFailed])
	}
	
	// Phase 2: Product scraping
	logger.Info("starting product scraping phase", "concurrent", *concurrent)
	
	// Create multiple browsers for concurrent scraping
	scrapers := make([]*scraper.ProductScraper, *concurrent)
	browsers := make([]*browser.Browser, *concurrent)
	
	for i := 0; i < *concurrent; i++ {
		b, err := browser.New(browserOpts)
		if err != nil {
			logger.Error("failed to create browser", "index", i, "error", err)
			// Clean up already created browsers
			for j := 0; j < i; j++ {
				browsers[j].Close()
			}
			os.Exit(1)
		}
		browsers[i] = b
		scrapers[i] = scraper.NewProductScraper(b, db)
	}
	
	// Start concurrent scrapers
	errChan := make(chan error, *concurrent)
	for i, s := range scrapers {
		go func(index int, scraper *scraper.ProductScraper) {
			logger.Info("starting scraper", "index", index)
			if err := scraper.ScrapeAllPending(ctx, 10); err != nil {
				errChan <- fmt.Errorf("scraper %d failed: %w", index, err)
			} else {
				errChan <- nil
			}
		}(i, s)
	}
	
	// Wait for all scrapers to complete
	var scrapeErrors []error
	for i := 0; i < *concurrent; i++ {
		if err := <-errChan; err != nil {
			scrapeErrors = append(scrapeErrors, err)
		}
	}
	
	// Clean up browsers
	for _, b := range browsers {
		b.Close()
	}
	
	if len(scrapeErrors) > 0 {
		logger.Error("some scrapers failed", "errors", scrapeErrors)
	}
	
	// Final statistics
	counts, _ := db.CountProductsByStatus(ctx)
	logger.Info("scraping completed", 
		"pending", counts[database.StatusPending],
		"completed", counts[database.StatusCompleted],
		"failed", counts[database.StatusFailed])
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		var i int
		fmt.Sscanf(value, "%d", &i)
		return i
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		return value == "true" || value == "1"
	}
	return defaultValue
}