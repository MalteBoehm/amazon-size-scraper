package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"regexp"
	"syscall"
	"time"

	"github.com/maltedev/amazon-size-scraper/internal/browser"
	"github.com/maltedev/amazon-size-scraper/internal/config"
	"github.com/maltedev/amazon-size-scraper/internal/database"
	"github.com/maltedev/amazon-size-scraper/internal/proxy"
	"github.com/maltedev/amazon-size-scraper/internal/scraper"
)

func main() {
	// Command line flags
	var (
		searchURL     = flag.String("search", "", "Amazon search URL to crawl")
		productURL    = flag.String("url", "", "Single Amazon product URL to scrape (debug mode)")
		dbHost        = flag.String("db-host", getEnv("DB_HOST", "localhost"), "Database host")
		dbPort        = flag.Int("db-port", getEnvInt("DB_PORT", 5432), "Database port")
		dbUser        = flag.String("db-user", getEnv("DB_USER", "postgres"), "Database user")
		dbPassword    = flag.String("db-password", getEnv("DB_PASSWORD", ""), "Database password")
		dbName        = flag.String("db-name", getEnv("DB_NAME", "amazon_scraper"), "Database name")
		headless      = flag.Bool("headless", getEnvBool("HEADLESS", true), "Run browser in headless mode")
		concurrent    = flag.Int("concurrent", getEnvInt("CONCURRENT_SCRAPERS", 1), "Number of concurrent product scrapers")
		scrapeOnly    = flag.Bool("scrape-only", false, "Only scrape products, don't crawl search results")
        // Default proxy list path aligned with docker-compose and production mounts
        proxyListFile = flag.String("proxy-list", getEnv("PROXY_LIST_FILE", "/app/proxy_data/proxy_list"), "Path to proxy list file")
        // Default: enable proxies unless explicitly disabled
        useProxies    = flag.Bool("use-proxies", getEnvBool("USE_PROXIES", true), "Enable proxy rotation")
		// TODO: Re-enable when browser pool is implemented
		// poolSize      = flag.Int("pool-size", getEnvInt("BROWSER_POOL_SIZE", 3), "Number of browsers in pool")
		// maxRequests   = flag.Int("max-requests", getEnvInt("MAX_REQUESTS_PER_BROWSER", 20), "Max requests before recreating browser")
	)
	flag.Parse()

	// Load config
	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("Failed to load config: %v\n", err)
		os.Exit(1)
	}

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

	// Database connection (skip in debug mode with single URL)
	var db *database.DB
	if *productURL == "" || !cfg.Logging.DebugMode {
		// Handle database name inconsistencies across the system
		// The actual database on the server is named "tall-affiliate" (with hyphen)
		// but different parts of the code use different defaults
		finalDbName := *dbName
		if finalDbName == "amazon_scraper" || finalDbName == "tall_affiliate" {
			finalDbName = "tall-affiliate" // Use the actual database name that exists
			logger.Info("Database name normalized to: tall-affiliate (actual database on server)")
		}

		// Log the final database configuration for debugging
		logger.Info("DATABASE CONFIGURATION DEBUG",
			"original_name", *dbName,
			"final_name", finalDbName,
			"host", *dbHost,
			"port", *dbPort,
			"user", *dbUser,
		)

		dbConfig := database.Config{
			Host:        *dbHost,
			Port:        *dbPort,
			User:        *dbUser,
			Password:    *dbPassword,
			Database:    finalDbName,
			MaxConns:    int32(*concurrent * 2),
			MinConns:    1,
			MaxConnLife: 5 * time.Minute,
			MaxConnIdle: 1 * time.Minute,
		}

		db, err = database.New(ctx, dbConfig)
		if err != nil {
			logger.Error("failed to connect to database", "error", err)
			os.Exit(1)
		}
		defer db.Close()

		logger.Info("connected to database")
	}

	// Proxy setup
	var proxyManager *proxy.ProxyManager
    if *useProxies {
        var err error
        proxyManager, err = proxy.NewProxyManager(*proxyListFile, logger)
        if err != nil {
            logger.Error("failed to initialize proxy manager", "error", err, "file", *proxyListFile)
            logger.Warn("continuing without proxy rotation")
            *useProxies = false
        } else {
            logger.Info("proxy manager initialized",
                "total_proxies", proxyManager.GetProxyCount(),
                "healthy_proxies", proxyManager.GetHealthyProxyCount())
        }
    }

	// Browser setup
	browserOpts := browser.DefaultOptions()
	browserOpts.Headless = *headless

	// Debug mode with single URL
	if *productURL != "" {
		logger.Info("debug mode: scraping single product URL", "url", *productURL)

		// Skip database connection in debug mode
		if !cfg.Logging.DebugMode {
			logger.Error("single URL scraping requires DEBUG_MODE=true")
			os.Exit(1)
		}

		// Create browser
		b, err := browser.New(browserOpts)
		if err != nil {
			logger.Error("failed to create browser", "error", err)
			os.Exit(1)
		}
		defer b.Close()

		// Create scraper with nil database for debug mode
		productScraper := scraper.NewProductScraper(b, nil, cfg)

		// Extract ASIN from URL
		asin := extractASINFromURL(*productURL)
		if asin == "" {
			logger.Error("could not extract ASIN from URL", "url", *productURL)
			os.Exit(1)
		}

		// Create a mock product object for scraping
		mockProduct := &database.Product{
			ASIN:  asin,
			Title: "Product from URL",
			URL:   *productURL,
		}

		// Scrape the product
		if err := productScraper.ScrapeSingleProduct(ctx, mockProduct); err != nil {
			logger.Error("failed to scrape product", "error", err)
			os.Exit(1)
		}

		logger.Info("debug mode scraping completed")
		return
	}

	// Phase 1: Search crawling (if URL provided and not scrape-only)
	if *searchURL != "" && !*scrapeOnly {
		logger.Info("starting search crawl phase", "url", *searchURL)

		var b *browser.Browser
		var err error

		if *useProxies && proxyManager != nil {
			// Use a random proxy for search crawling
			proxy, err := proxyManager.GetRandomProxy()
			if err != nil {
				logger.Error("failed to get proxy for search crawling", "error", err)
				os.Exit(1)
			}

			searchOpts := *browserOpts
			searchOpts.ProxyServer = fmt.Sprintf("http://%s:%s", proxy.Host, proxy.Port)
			searchOpts.ProxyUsername = proxy.Username
			searchOpts.ProxyPassword = proxy.Password

			b, err = browser.New(&searchOpts)
			if err != nil {
				logger.Error("failed to create browser with proxy", "error", err)
				os.Exit(1)
			}

			logger.Info("search crawling with proxy", "proxy_host", proxy.Host)
		} else {
			b, err = browser.New(browserOpts)
			if err != nil {
				logger.Error("failed to create browser", "error", err)
				os.Exit(1)
			}
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
	logger.Info("starting product scraping phase", "concurrent", *concurrent, "use_proxies", *useProxies)

	// Declare variables in outer scope
	var scrapers []*scraper.ProductScraper
	var browsers []*browser.Browser

	if *useProxies && proxyManager != nil {
		// Use browser pools with proxy rotation
		logger.Info("initializing browser pools with proxy rotation")

		// TODO: Use browser pools when pool-based ProductScraper is implemented
		// poolConfig := &browser.PoolConfig{
		// 	PoolSize:    *poolSize,
		// 	MaxRequests: *maxRequests,
		// }

		// For now, create one browser with random proxy per scraper
		// This is a temporary solution until we implement pool-based ProductScraper
		scrapers = make([]*scraper.ProductScraper, *concurrent)
		browsers = make([]*browser.Browser, *concurrent)

		for i := 0; i < *concurrent; i++ {
			// Get a random proxy for each browser
			proxy, err := proxyManager.GetRandomProxy()
			if err != nil {
				logger.Error("failed to get proxy", "index", i, "error", err)
				os.Exit(1)
			}

			// Create browser options with proxy
			proxyOpts := *browserOpts
			proxyOpts.ProxyServer = fmt.Sprintf("http://%s:%s", proxy.Host, proxy.Port)
			proxyOpts.ProxyUsername = proxy.Username
			proxyOpts.ProxyPassword = proxy.Password

			b, err := browser.New(&proxyOpts)
			if err != nil {
				logger.Error("failed to create browser with proxy", "index", i, "proxy", proxy.Host, "error", err)
				// Clean up already created browsers
				for j := 0; j < i; j++ {
					browsers[j].Close()
				}
				os.Exit(1)
			}

			browsers[i] = b
			scrapers[i] = scraper.NewProductScraper(b, db, cfg)

			logger.Info("created scraper with proxy", "index", i, "proxy_host", proxy.Host)
		}
	} else {
		// Legacy mode without proxies
		logger.Info("initializing scrapers without proxy rotation")

		scrapers = make([]*scraper.ProductScraper, *concurrent)
		browsers = make([]*browser.Browser, *concurrent)

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
			scrapers[i] = scraper.NewProductScraper(b, db, cfg)
		}
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

// extractASINFromURL extracts ASIN from Amazon URL
func extractASINFromURL(url string) string {
	// Amazon ASIN pattern: 10 characters alphanumeric
	re := regexp.MustCompile(`/([A-Z0-9]{10})(?:/|[?]|$)`)
	matches := re.FindStringSubmatch(url)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
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
