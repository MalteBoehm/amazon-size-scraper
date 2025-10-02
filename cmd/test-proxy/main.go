package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"time"

	"github.com/maltedev/amazon-size-scraper/internal/browser"
	"github.com/maltedev/amazon-size-scraper/internal/config"
	"github.com/maltedev/amazon-size-scraper/internal/parser"
	"github.com/maltedev/amazon-size-scraper/internal/proxy"
	"github.com/maltedev/amazon-size-scraper/internal/scraper"
)

func main() {
	var (
		proxyListFile = flag.String("proxy-list", "proxy_list", "Path to proxy list file")
		asin          = flag.String("asin", "B08N5WRWNW", "ASIN to test scraping")
		poolSize      = flag.Int("pool-size", 3, "Browser pool size")
		maxRequests   = flag.Int("max-requests", 5, "Max requests per browser")
	)
	flag.Parse()

	// Setup logging
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	logger.Info("starting proxy rotation test", "asin", *asin)

	// Load config
	cfg, err := config.Load()
	if err != nil {
		logger.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	// Initialize proxy manager
	proxyManager, err := proxy.NewProxyManager(*proxyListFile, logger)
	if err != nil {
		logger.Error("failed to initialize proxy manager", "error", err)
		os.Exit(1)
	}

	logger.Info("proxy manager initialized",
		"total_proxies", proxyManager.GetProxyCount(),
		"healthy_proxies", proxyManager.GetHealthyProxyCount())

	// Create browser pool with proxy rotation
	browserOpts := browser.DefaultOptions()
	browserOpts.Headless = true

	poolConfig := &browser.PoolConfig{
		PoolSize:    *poolSize,
		MaxRequests: *maxRequests,
	}

	browserPool, err := browser.NewBrowserPool(proxyManager, browserOpts, poolConfig, logger)
	if err != nil {
		logger.Error("failed to create browser pool", "error", err)
		os.Exit(1)
	}
	defer browserPool.Close()

	// Create scraper with browser pool
	parser := parser.NewAmazonParser()
	amazonScraper := scraper.NewAmazonScraper(browserPool, parser, logger, cfg)
	defer amazonScraper.Close()

	// Test scraping with proxy rotation
	ctx := context.Background()
	for i := 0; i < 10; i++ {
		logger.Info("scraping attempt", "iteration", i+1)

		product, err := amazonScraper.ScrapeByASIN(ctx, *asin)
		if err != nil {
			logger.Error("scraping failed", "iteration", i+1, "error", err)
		} else {
			logger.Info("scraping successful", 
				"iteration", i+1,
				"title", product.Title,
				"price", product.Price.Amount)
		}

		// Show pool statistics
		stats := amazonScraper.GetPoolStats()
		logger.Info("pool statistics", "stats", stats)

		// Wait between requests
		time.Sleep(2 * time.Second)
	}

	logger.Info("test completed")
}