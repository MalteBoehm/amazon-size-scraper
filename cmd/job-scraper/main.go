package main

import (
    "database/sql"
    "context"
    "flag"
    "fmt"
    "log/slog"
    "os"
    "os/signal"
    "strconv"
    "syscall"
    "time"

    "github.com/maltedev/amazon-size-scraper/internal/browser"
    "github.com/maltedev/amazon-size-scraper/internal/config"
    intdb "github.com/maltedev/amazon-size-scraper/internal/database"
    "github.com/maltedev/amazon-size-scraper/internal/parser"
    "github.com/maltedev/amazon-size-scraper/internal/scraper"
    "github.com/maltedev/amazon-size-scraper/pkg/logger"
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

    // Build database config (normalize DB name if needed)
    dbName := cfg.Database.DBName
    if dbName == "amazon_scraper" || dbName == "tall_affiliate" {
        dbName = "tall-affiliate"
    }

    // Allow overriding via DATABASE_URL env only when all fields are empty
    // but internal DB expects discrete fields, so prefer discrete envs
    port := cfg.Database.Port
    if port == 0 {
        // Fallback from env if present
        if v := os.Getenv("DB_PORT"); v != "" {
            if p, err := strconv.Atoi(v); err == nil { port = p }
        }
    }

    dbCfg := intdb.Config{
        Host:        firstNonEmpty(cfg.Database.Host, os.Getenv("DB_HOST"), "localhost"),
        Port:        portIfNonZero(port, 5432),
        User:        firstNonEmpty(cfg.Database.User, os.Getenv("DB_USER"), "postgres"),
        Password:    firstNonEmpty(cfg.Database.Password, os.Getenv("DB_PASSWORD")),
        Database:    dbName,
        SSLMode:     firstNonEmpty(cfg.Database.SSLMode, os.Getenv("DB_SSL_MODE"), "disable"),
        MaxConns:    10,
        MinConns:    1,
        MaxConnLife: 5 * time.Minute,
        MaxConnIdle: 1 * time.Minute,
    }

    db, err := intdb.New(ctx, dbCfg)
    if err != nil {
        logger.Error("Failed to connect to database", "error", err)
        os.Exit(1)
    }
    defer db.Close()

	// Initialize browser pool
	browserOpts := browser.Options{
		Headless: cfg.Browser.Headless,
		Timeout:  cfg.Browser.Timeout,
	}

	// Start concurrent scrapers
	for i := 0; i < *concurrent; i++ {
        go func(workerID int) {
            if err := runScraper(ctx, workerID, db, browserOpts, *pollInterval, logger); err != nil {
				logger.Error("Scraper worker failed", "worker_id", workerID, "error", err)
			}
		}(i)
	}

	// Wait for shutdown
	<-ctx.Done()
	logger.Info("Shutting down job scraper")
}

func runScraper(ctx context.Context, workerID int, db *intdb.DB,
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
            if err := processNextJob(ctx, db, searchScraper, p, workerLogger); err != nil {
				workerLogger.Error("Failed to process job", "error", err)
			}
		}
	}
}

type ScraperJob struct {
    ID          string
    SearchQuery string
    Category    string
    MaxPages    int
}

func processNextJob(ctx context.Context, db *intdb.DB,
    searchScraper *scraper.SearchScraper, parser *parser.AmazonParser, logger *slog.Logger) error {

    // Get and mark next pending job atomically
    job, err := fetchAndMarkNextJob(ctx, db)
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

	// Process the job
    if err := processJob(ctx, job, db, searchScraper, parser, logger); err != nil {
		// Mark job as failed
        if markErr := markJobFailed(ctx, db, job.ID, err.Error()); markErr != nil {
			logger.Error("Failed to mark job as failed", "job_id", job.ID, "error", markErr)
		}
		return fmt.Errorf("job processing failed: %w", err)
	}

	// Mark job as completed
    if err := markJobCompleted(ctx, db, job.ID); err != nil {
		return fmt.Errorf("failed to mark job as completed: %w", err)
	}

	logger.Info("Job completed successfully", "job_id", job.ID)
	return nil
}

func processJob(ctx context.Context, job *ScraperJob, db *intdb.DB,
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

        // Update job progress (best-effort)
        if err := updateJobProgress(ctx, db, job.ID, page, totalProductsFound, totalProductsComplete); err != nil {
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

// fetchAndMarkNextJob selects the next pending job and marks it running atomically
func fetchAndMarkNextJob(ctx context.Context, db *intdb.DB) (*ScraperJob, error) {
    var out *ScraperJob
    err := db.Transaction(ctx, func(tx intdb.Tx) error {
        // Select next pending job with SKIP LOCKED
        row := tx.QueryRow(ctx, `
            SELECT id, search_query, COALESCE(category, ''), COALESCE(max_pages, 1)
            FROM scraper_jobs
            WHERE status = 'pending'
            ORDER BY created_at ASC
            FOR UPDATE SKIP LOCKED
            LIMIT 1`)

        j := ScraperJob{}
        if err := row.Scan(&j.ID, &j.SearchQuery, &j.Category, &j.MaxPages); err != nil {
            if err == sql.ErrNoRows {
                return nil
            }
            return err
        }

        // Mark as running
        if _, err := tx.Exec(ctx, `
            UPDATE scraper_jobs
            SET status = 'running', started_at = CURRENT_TIMESTAMP
            WHERE id = $1`, j.ID); err != nil {
            return err
        }
        out = &j
        return nil
    })
    if err != nil {
        // No rows or actual error
        return nil, err
    }
    return out, nil
}

func markJobCompleted(ctx context.Context, db *intdb.DB, id string) error {
    _, err := db.Exec(ctx, `
        UPDATE scraper_jobs
        SET status = 'completed', completed_at = CURRENT_TIMESTAMP
        WHERE id = $1`, id)
    return err
}

func markJobFailed(ctx context.Context, db *intdb.DB, id, reason string) error {
    // If failure_reason column exists, set it; otherwise just set status
    if _, err := db.Exec(ctx, `
        UPDATE scraper_jobs
        SET status = 'failed', failure_reason = $2
        WHERE id = $1`, id, reason); err != nil {
        // fallback without failure_reason
        _, err2 := db.Exec(ctx, `
            UPDATE scraper_jobs
            SET status = 'failed'
            WHERE id = $1`, id)
        return err2
    }
    return nil
}

func updateJobProgress(ctx context.Context, db *intdb.DB, id string, pages, found, complete int) error {
    // Try with products_complete if present; otherwise fallback to smaller set
    if _, err := db.Exec(ctx, `
        UPDATE scraper_jobs
        SET pages_scraped = $2, products_found = $3, products_complete = $4
        WHERE id = $1`, id, pages, found, complete); err != nil {
        // fallback without products_complete
        _, err2 := db.Exec(ctx, `
            UPDATE scraper_jobs
            SET pages_scraped = $2, products_found = $3
            WHERE id = $1`, id, pages, found)
        return err2
    }
    return nil
}

func firstNonEmpty(vals ...string) string {
    for _, v := range vals {
        if v != "" {
            return v
        }
    }
    return ""
}

func portIfNonZero(p int, def int) int {
    if p > 0 { return p }
    return def
}
