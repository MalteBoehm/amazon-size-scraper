//go:build examples

package main

import (
	"context"
	"log"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/maltedev/amazon-size-scraper/internal/browser"
	"github.com/maltedev/amazon-size-scraper/internal/metrics"
	"github.com/maltedev/amazon-size-scraper/internal/scraper"
	"github.com/prometheus/client_golang/prometheus"
)

func main() {
	// Initialize logger
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	// Create custom registry for metrics
	registry := prometheus.NewRegistry()

	// Initialize metrics
	extractionMetrics := metrics.NewExtractionMetricsWithRegistry(registry)
	scraperMetrics := metrics.NewScraperMetricsWithRegistry(registry)

	// Initialize browser
	browserOpts := browser.Options{
		Headless: true,
		Timeout:  30 * time.Second,
	}
	b, err := browser.NewBrowser(browserOpts)
	if err != nil {
		log.Fatal("failed to create browser:", err)
	}
	defer b.Close()

	// Create instrumented product extractor
	extractor := scraper.NewInstrumentedProductExtractor(b, logger, extractionMetrics)

	// Start metrics HTTP server
	go func() {
		http.Handle("/metrics", metrics.HandlerFor(registry))
		logger.Info("Starting metrics server on :2112")
		if err := http.ListenAndServe(":2112", nil); err != nil {
			logger.Error("metrics server failed", "error", err)
		}
	}()

	// Example: Extract a product with metrics
	ctx := context.Background()
	asin := "B08N5WRWNW"
	url := "https://www.amazon.de/dp/" + asin

	// Record page load metrics
	pageLoadStart := time.Now()
	product, err := extractor.ExtractCompleteProduct(ctx, asin, url)
	pageLoadDuration := time.Since(pageLoadStart)

	if err != nil {
		scraperMetrics.RecordPageLoad(url, false, pageLoadDuration)
		logger.Error("extraction failed", "error", err)
		return
	}

	scraperMetrics.RecordPageLoad(url, true, pageLoadDuration)

	// Log extraction results
	logger.Info("Product extracted successfully",
		"asin", product.ASIN,
		"title", product.Title,
		"gender", product.Gender,
		"isPrime", product.IsPrimeEligible,
		"inStock", product.InStock,
		"hasSize_table", product.SizeTable != nil,
	)

	// Example: Record individual field extraction metrics
	fieldStart := time.Now()
	success := product.Gender != ""
	extractionMetrics.RecordFieldExtraction("gender", success, time.Since(fieldStart))

	// Example: Record fallback usage
	if product.Material == "" {
		extractionMetrics.RecordFallbackUsage("material", 1)
	}

	// Metrics are now available at http://localhost:2112/metrics
	logger.Info("Metrics available at http://localhost:2112/metrics")

	// Keep the program running to serve metrics
	select {}
}
