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
	"github.com/maltedev/amazon-size-scraper/internal/config"
	"github.com/maltedev/amazon-size-scraper/internal/parser"
	"github.com/maltedev/amazon-size-scraper/internal/scraper"
	"github.com/maltedev/amazon-size-scraper/pkg/logger"

	"github.com/MalteBoehm/tall-affiliate-common/pkg/database"
	"github.com/MalteBoehm/tall-affiliate-common/pkg/models"
)

func main() {
	var (
		configFile   = flag.String("config", "configs/config.yaml", "Path to config file")
		dbURL        = flag.String("db", "", "Database URL (overrides config)")
		pollInterval = flag.Duration("poll-interval", 10*time.Second, "Job polling interval")
		concurrent   = flag.Int("concurrent", 1, "Number of concurrent scrapers")
	)
	flag.Parse()

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Override database URL if provided
	if *dbURL != "" {
		cfg.Database.URL = *dbURL
	}

	// Setup logger
	logger := logger.New(cfg.Logging.Level, cfg.Logging.Format)
	logger.Info("Starting job-based scraper",
		"config", *configFile,
		"poll_interval", *pollInterval,
		"concurrent", *concurrent)

	// Setup context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		logger.Info("Received shutdown signal")
		cancel()
	}()

	// Connect to database
	db, err := database.NewConnectionFromURL(ctx, cfg.Database.URL)
	if err != nil {
		logger.Error("Failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	// Create job repository
	jobRepo := database.NewJobRepository(db)

	// Initialize browser pool
	browserOpts := browser.Options{
		Headless: cfg.Browser.Headless,
		Timeout:  cfg.Browser.Timeout,
	}

	// Start concurrent scrapers
	for i := 0; i < *concurrent; i++ {
		go func(workerID int) {
			if err := runScraper(ctx, workerID, jobRepo, browserOpts, *pollInterval, logger); err != nil {
				logger.Error("Scraper worker failed", "worker_id", workerID, "error", err)
			}
		}(i)
	}

	// Wait for shutdown
	<-ctx.Done()
	logger.Info("Shutting down job scraper")
}

func runScraper(ctx context.Context, workerID int, jobRepo *database.JobRepository,
	browserOpts browser.Options, pollInterval time.Duration, logger *slog.Logger) error {

	workerLogger := logger.With("worker_id", workerID)
	workerLogger.Info("Starting scraper worker")

	// Initialize browser
	b, err := browser.New(&browserOpts)
	if err != nil {
		return fmt.Errorf("failed to initialize browser: %w", err)
	}
	defer b.Close()

	// Initialize parser
	p := parser.NewAmazonParser()

	// Initialize search scraper
	searchScraper := scraper.NewSearchScraper(b, p, workerLogger)

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			workerLogger.Info("Worker shutting down")
			return nil
		case <-ticker.C:
			if err := processNextJob(ctx, jobRepo, searchScraper, p, workerLogger); err != nil {
				workerLogger.Error("Failed to process job", "error", err)
			}
		}
	}
}

func processNextJob(ctx context.Context, jobRepo *database.JobRepository,
	searchScraper *scraper.SearchScraper, parser *parser.AmazonParser, logger *slog.Logger) error {

	// Get pending job with FOR UPDATE SKIP LOCKED
	job, err := jobRepo.GetPendingJob(ctx)
	if err != nil {
		return fmt.Errorf("failed to get pending job: %w", err)
	}

	if job == nil {
		// No pending jobs
		return nil
	}

	logger.Info("Processing job",
		"job_id", job.ID,
		"search_query", job.SearchQuery,
		"max_pages", job.MaxPages)

	// Mark job as running
	if err := jobRepo.MarkJobAsRunning(ctx, job.ID); err != nil {
		return fmt.Errorf("failed to mark job as running: %w", err)
	}

	// Process the job
	if err := processJob(ctx, job, jobRepo, searchScraper, parser, logger); err != nil {
		// Mark job as failed
		if markErr := jobRepo.MarkJobAsFailed(ctx, job.ID, err.Error()); markErr != nil {
			logger.Error("Failed to mark job as failed", "job_id", job.ID, "error", markErr)
		}
		return fmt.Errorf("job processing failed: %w", err)
	}

	// Mark job as completed
	if err := jobRepo.MarkJobAsCompleted(ctx, job.ID); err != nil {
		return fmt.Errorf("failed to mark job as completed: %w", err)
	}

	logger.Info("Job completed successfully", "job_id", job.ID)
	return nil
}

func processJob(ctx context.Context, job *models.ScraperJob, jobRepo *database.JobRepository,
	searchScraper *scraper.SearchScraper, parser *parser.AmazonParser, logger *slog.Logger) error {

	// Build Amazon search URL
	searchURL := fmt.Sprintf("https://www.amazon.de/s?k=%s", job.SearchQuery)
	if job.Category != "" && job.Category != "test-category" {
		// Add category filter if specified
		searchURL += fmt.Sprintf("&rh=n:%s", job.Category)
	}

	logger.Info("Starting search scraping",
		"search_url", searchURL,
		"max_pages", job.MaxPages)

	var totalProductsFound int
	var totalProductsComplete int

	// Scrape search results page by page
	for page := 1; page <= job.MaxPages; page++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		pageURL := searchURL
		if page > 1 {
			pageURL += fmt.Sprintf("&page=%d", page)
		}

		logger.Info("Scraping page", "page", page, "url", pageURL)

		// Scrape search results
		results, err := searchScraper.ScrapeSearchResults(ctx, pageURL)
		if err != nil {
			logger.Error("Failed to scrape search page", "page", page, "error", err)
			continue // Continue with next page
		}

		logger.Info("Found products on page", "page", page, "count", len(results))
		totalProductsFound += len(results)

		// Process each product result
		for _, result := range results {
			// Here you would typically:
			// 1. Check if product already exists in database
			// 2. Create new product record if needed
			// 3. Trigger product enrichment/scraping

			logger.Debug("Found product",
				"asin", result.ASIN,
				"title", result.Title,
				"price", result.Price)

			totalProductsComplete++
		}

		// Update job progress
		if err := jobRepo.UpdateJobProgress(ctx, job.ID, page, totalProductsFound, totalProductsComplete); err != nil {
			logger.Error("Failed to update job progress", "error", err)
		}

		// Rate limiting between pages
		time.Sleep(2 * time.Second)
	}

	logger.Info("Job processing completed",
		"job_id", job.ID,
		"pages_scraped", job.MaxPages,
		"products_found", totalProductsFound,
		"products_complete", totalProductsComplete)

	return nil
}
