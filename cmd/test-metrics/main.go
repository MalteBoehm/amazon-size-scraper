package main

import (
	"fmt"
	"io"
	"log"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/maltedev/amazon-size-scraper/internal/metrics"
	"github.com/prometheus/client_golang/prometheus"
)

func main() {
	// Initialize logger
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	// Create custom registry for metrics
	registry := prometheus.NewRegistry()
	
	// Initialize metrics
	extractionMetrics := metrics.NewExtractionMetricsWithRegistry(registry)
	scraperMetrics := metrics.NewScraperMetricsWithRegistry(registry)

	// Start metrics HTTP server
	mux := http.NewServeMux()
	mux.Handle("/metrics", metrics.HandlerFor(registry))
	
	server := &http.Server{
		Addr:    ":2112",
		Handler: mux,
	}
	
	go func() {
		logger.Info("Starting metrics server", "port", 2112)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("metrics server failed", "error", err)
		}
	}()

	// Give the server time to start
	time.Sleep(100 * time.Millisecond)

	// Simulate some metric recordings
	logger.Info("Recording test metrics...")
	
	// Record various field extractions
	fields := []string{"gender", "material", "colors", "prime_eligibility", "stock_status"}
	for _, field := range fields {
		// Simulate some successful extractions
		for i := 0; i < 8; i++ {
			extractionMetrics.RecordFieldExtraction(field, true, time.Duration(50+i*10)*time.Millisecond)
		}
		// Simulate some failed extractions
		for i := 0; i < 2; i++ {
			extractionMetrics.RecordFieldExtraction(field, false, time.Duration(100+i*20)*time.Millisecond)
		}
	}

	// Record fallback usage
	extractionMetrics.RecordFallbackUsage("material", 1)
	extractionMetrics.RecordFallbackUsage("material", 2)
	extractionMetrics.RecordFallbackUsage("colors", 1)

	// Record product extractions
	extractionMetrics.RecordProductExtraction("complete", 5*time.Second)
	extractionMetrics.RecordProductExtraction("complete", 4*time.Second)
	extractionMetrics.RecordProductExtraction("partial", 3*time.Second)
	extractionMetrics.RecordProductExtraction("failed", 2*time.Second)

	// Record size table validations
	for i := 0; i < 7; i++ {
		extractionMetrics.RecordSizeTableValidation(true)
	}
	for i := 0; i < 3; i++ {
		extractionMetrics.RecordSizeTableValidation(false)
	}

	// Record scraper metrics
	scraperMetrics.RecordPageLoad("https://amazon.de/dp/test1", true, 2*time.Second)
	scraperMetrics.RecordPageLoad("https://amazon.de/dp/test2", true, 3*time.Second)
	scraperMetrics.RecordPageLoad("https://amazon.de/dp/test3", false, 5*time.Second)
	
	scraperMetrics.RecordCaptchaDetection()
	scraperMetrics.RecordCaptchaDetection()
	
	scraperMetrics.RecordRateLimitDelay(5 * time.Second)
	scraperMetrics.RecordRateLimitDelay(10 * time.Second)
	scraperMetrics.RecordRateLimitDelay(15 * time.Second)

	// Test the metrics endpoint
	logger.Info("Testing metrics endpoint...")
	
	resp, err := http.Get("http://localhost:2112/metrics")
	if err != nil {
		log.Fatal("Failed to fetch metrics:", err)
	}
	defer resp.Body.Close()

	logger.Info("Metrics endpoint response", "status", resp.Status)
	
	// Read and display first 2000 bytes of metrics
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatal("Failed to read response:", err)
	}

	fmt.Println("\n=== METRICS OUTPUT (first 2000 chars) ===")
	if len(body) > 2000 {
		fmt.Println(string(body[:2000]) + "...\n[truncated]")
	} else {
		fmt.Println(string(body))
	}
	
	// Print some specific metric examples
	fmt.Println("\n=== EXAMPLE METRICS ===")
	bodyStr := string(body)
	
	// Find and print field extraction metrics
	printMetricExample(bodyStr, "scraper_field_extraction_total{field=\"gender\",status=\"success\"}")
	printMetricExample(bodyStr, "scraper_field_extraction_total{field=\"gender\",status=\"failure\"}")
	printMetricExample(bodyStr, "scraper_product_extraction_total{status=\"complete\"}")
	printMetricExample(bodyStr, "scraper_captcha_detected_total")
	printMetricExample(bodyStr, "scraper_size_table_validation_total{status=\"valid\"}")

	logger.Info("Metrics test completed successfully!")
	logger.Info("You can access the full metrics at: http://localhost:2112/metrics")
	
	// Keep server running for manual testing
	logger.Info("Server will keep running. Press Ctrl+C to stop.")
	select {}
}

func printMetricExample(metrics, metricName string) {
	// Simple search for the metric line
	lines := strings.Split(metrics, "\n")
	for _, line := range lines {
		if strings.Contains(line, metricName) && !strings.HasPrefix(line, "#") {
			fmt.Printf("%s\n", line)
			return
		}
	}
}