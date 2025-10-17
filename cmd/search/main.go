package main

import (
	"context"
	"encoding/csv"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/maltedev/amazon-size-scraper/internal/browser"
	"github.com/maltedev/amazon-size-scraper/internal/config"
	"github.com/maltedev/amazon-size-scraper/internal/parser"
	"github.com/maltedev/amazon-size-scraper/internal/scraper"
	"github.com/maltedev/amazon-size-scraper/pkg/logger"
)

func main() {
	var (
		searchURL   = flag.String("url", "", "Amazon search URL")
		outputFile  = flag.String("output", "", "Output CSV file (optional)")
		maxPages    = flag.Int("pages", 1, "Maximum number of pages to scrape")
		headless    = flag.Bool("headless", true, "Run browser in headless mode")
		scrapeItems = flag.Bool("scrape", false, "Also scrape individual product pages for dimensions")
	)
	flag.Parse()

	if *searchURL == "" {
		fmt.Println("Please provide a search URL with -url")
		flag.Usage()
		os.Exit(1)
	}

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	logger := logger.New(cfg.Logging.Level, cfg.Logging.Format)
	logger.Info("Starting Amazon Search Scraper")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		logger.Info("Shutdown signal received")
		cancel()
	}()

	browserOpts := &browser.Options{
		Headless:       *headless && cfg.Browser.Headless,
		Timeout:        cfg.Browser.Timeout,
		ViewportWidth:  cfg.Browser.ViewportWidth,
		ViewportHeight: cfg.Browser.ViewportHeight,
		AcceptLanguage: cfg.Browser.AcceptLanguage,
		TimezoneID:     cfg.Browser.TimezoneID,
		Locale:         cfg.Browser.Locale,
	}

	if len(cfg.Scraper.UserAgents) > 0 {
		browserOpts.UserAgent = cfg.Scraper.UserAgents[0]
	}

	b, err := browser.New(browserOpts)
	if err != nil {
		logger.Error("Failed to initialize browser", "error", err)
		os.Exit(1)
	}
	defer b.Close()

	p := parser.NewAmazonParser()
	searchScraper := scraper.NewSearchScraper(b, p, logger)
	productScraper := scraper.NewAmazonScraper(b, p, logger)

	var allResults []scraper.SearchResult
	currentURL := *searchURL
	
	for page := 1; page <= *maxPages && currentURL != ""; page++ {
		logger.Info("Scraping page", "page", page, "url", currentURL)
		
		results, err := searchScraper.ScrapeSearchResults(ctx, currentURL)
		if err != nil {
			logger.Error("Failed to scrape search results", "error", err, "page", page)
			break
		}
		
		logger.Info("Found products on page", "count", len(results), "page", page)
		allResults = append(allResults, results...)
		
		// Print results
		for _, result := range results {
			fmt.Printf("ASIN: %s\n", result.ASIN)
			fmt.Printf("Title: %s\n", result.Title)
			fmt.Printf("Price: %s\n", result.Price)
			fmt.Printf("URL: %s\n", result.URL)
			if result.HasTable {
				fmt.Println("➜ Might have size table!")
			}
			
			// Optionally scrape the product page for dimensions
			if *scrapeItems && result.ASIN != "" {
				fmt.Println("  Scraping product details...")
				product, err := productScraper.ScrapeByASIN(ctx, result.ASIN)
				if err != nil {
					logger.Error("Failed to scrape product", "asin", result.ASIN, "error", err)
				} else if product.Dimensions.IsValid() {
					fmt.Printf("  ✓ Dimensions: %.1f x %.1f x %.1f %s\n", 
						product.Dimensions.Length,
						product.Dimensions.Width,
						product.Dimensions.Height,
						product.Dimensions.Unit)
				} else {
					fmt.Println("  ✗ No dimensions found")
				}
			}
			fmt.Println("---")
		}
		
		if page < *maxPages {
			// Try to get next page URL
			newPage, err := b.NewPage()
			if err != nil {
				logger.Error("Failed to create page for navigation", "error", err)
				break
			}
			
			if err := b.NavigateWithRetry(newPage, currentURL, 3); err != nil {
				logger.Error("Failed to navigate for next page", "error", err)
				newPage.Close()
				break
			}
			
			nextURL, err := searchScraper.GetNextPageURL(newPage)
			newPage.Close()
			
			if err != nil || nextURL == "" {
				logger.Info("No more pages available")
				break
			}
			
			currentURL = nextURL
		}
	}
	
	logger.Info("Total products found", "count", len(allResults))
	
	// Save to CSV if requested
	if *outputFile != "" {
		if err := saveToCSV(allResults, *outputFile); err != nil {
			logger.Error("Failed to save CSV", "error", err)
		} else {
			logger.Info("Results saved to CSV", "file", *outputFile)
		}
	}
}

func saveToCSV(results []scraper.SearchResult, filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()
	
	writer := csv.NewWriter(file)
	defer writer.Flush()
	
	// Write header
	if err := writer.Write([]string{"ASIN", "Title", "URL", "Price", "HasSizeInfo"}); err != nil {
		return err
	}
	
	// Write data
	for _, result := range results {
		record := []string{
			result.ASIN,
			result.Title,
			result.URL,
			result.Price,
			fmt.Sprintf("%v", result.HasTable),
		}
		if err := writer.Write(record); err != nil {
			return err
		}
	}
	
	return nil
}