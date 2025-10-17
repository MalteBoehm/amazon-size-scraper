package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/maltedev/amazon-size-scraper/internal/browser"
	"github.com/maltedev/amazon-size-scraper/internal/config"
	"github.com/maltedev/amazon-size-scraper/internal/parser"
	"github.com/maltedev/amazon-size-scraper/internal/scraper"
	"github.com/maltedev/amazon-size-scraper/internal/storage"
	"github.com/maltedev/amazon-size-scraper/pkg/logger"
	"github.com/playwright-community/playwright-go"
	"log/slog"
)

func main() {
	var (
		mode       = flag.String("mode", "collect", "Mode: collect or process")
		searchURL  = flag.String("url", "", "Amazon search/category URL (for collect mode)")
		storageFile = flag.String("storage", "products.json", "Storage file for product links")
		maxPages   = flag.Int("pages", 10, "Maximum pages to crawl (0 = unlimited)")
		headless   = flag.Bool("headless", true, "Run browser in headless mode")
		concurrent = flag.Int("concurrent", 1, "Number of concurrent scrapers (for process mode)")
	)
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	logger := logger.New(cfg.Logging.Level, cfg.Logging.Format)
	logger.Info("Starting Amazon Crawler", "mode", *mode)

	// Load or create storage
	linkStorage, err := storage.NewLinkStorage(*storageFile)
	if err != nil {
		logger.Error("Failed to initialize storage", "error", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		logger.Info("Shutdown signal received")
		cancel()
	}()

	switch *mode {
	case "collect":
		if *searchURL == "" {
			fmt.Println("Please provide a search/category URL with -url for collect mode")
			flag.Usage()
			os.Exit(1)
		}
		collectLinks(ctx, logger, cfg, *searchURL, *maxPages, *headless, linkStorage)
	
	case "process":
		processLinks(ctx, logger, cfg, *concurrent, *headless, linkStorage)
	
	default:
		fmt.Printf("Unknown mode: %s\n", *mode)
		flag.Usage()
		os.Exit(1)
	}
}

func collectLinks(ctx context.Context, logger *slog.Logger, cfg *config.Config, startURL string, maxPages int, headless bool, storage *storage.LinkStorage) {
	browserOpts := &browser.Options{
		Headless:       headless,
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
		return
	}
	defer b.Close()

	page, err := b.NewPage()
	if err != nil {
		logger.Error("Failed to create page", "error", err)
		return
	}
	defer page.Close()

	currentURL := startURL
	pageCount := 0
	totalProducts := 0

	for {
		if maxPages > 0 && pageCount >= maxPages {
			logger.Info("Reached max pages limit", "pages", pageCount)
			break
		}

		pageCount++
		logger.Info("Crawling page", "page", pageCount, "url", currentURL)

		// Navigate to page
		if err := b.NavigateWithRetry(page, currentURL, 3); err != nil {
			logger.Error("Failed to navigate", "error", err, "url", currentURL)
			break
		}

		// Wait for products to load
		logger.Info("Waiting for page to load...")
		
		// Take screenshot for debugging
		screenshotPath := fmt.Sprintf("page-%d.png", pageCount)
		if _, err := page.Screenshot(playwright.PageScreenshotOptions{
			Path: &screenshotPath,
		}); err == nil {
			logger.Info("Screenshot saved", "file", screenshotPath)
		}
		
		// Check page title
		title, _ := page.Title()
		logger.Info("Page title", "title", title)
		
		// Check for various product selectors
		selectors := []string{
			"[data-component-type='s-search-result']",
			"div[data-asin]",
			"[data-index]",
			".s-result-item",
			".s-main-slot .s-result-item",
		}
		
		foundSelector := ""
		for _, selector := range selectors {
			count, _ := page.Locator(selector).Count()
			logger.Info("Checking selector", "selector", selector, "count", count)
			if count > 0 {
				foundSelector = selector
				break
			}
		}
		
		if foundSelector == "" {
			// Check for captcha
			if captchaCount, _ := page.Locator("#captchacharacters").Count(); captchaCount > 0 {
				logger.Error("CAPTCHA detected! Manual intervention required")
				time.Sleep(30 * time.Second) // Give time to solve manually
			}
		}
		
		time.Sleep(3 * time.Second)

		// Extract product links
		products := extractProductLinks(page, logger)
		
		if len(products) == 0 {
			logger.Warn("No products found on page", "page", pageCount)
			// Try alternative selectors
			products = extractAlternativeProducts(page, logger)
		}

		logger.Info("Found products on page", "count", len(products), "page", pageCount)
		totalProducts += len(products)

		// Save to storage
		if err := storage.AddBatch(products); err != nil {
			logger.Error("Failed to save products", "error", err)
		}

		// Print summary
		for _, p := range products {
			fmt.Printf("✓ %s - %s\n", p.ASIN, p.Title)
		}

		// Check for next page
		nextURL := findNextPageURL(page, logger)
		if nextURL == "" {
			logger.Info("No more pages found")
			break
		}

		currentURL = nextURL
		
		// Rate limit between pages
		logger.Info("Waiting before next page...")
		time.Sleep(3 * time.Second)
	}

	// Print final stats
	stats := storage.GetStats()
	logger.Info("Collection completed", 
		"total_pages", pageCount,
		"total_products", totalProducts,
		"storage_stats", stats)
}

func extractProductLinks(page playwright.Page, logger *slog.Logger) []*storage.ProductLink {
	var links []*storage.ProductLink

	// Try multiple selectors for products
	productSelectors := []string{
		"[data-component-type='s-search-result']",
		"div[data-asin]:not([data-asin=''])",
		"[data-index]",
		".s-result-item[data-asin]",
	}
	
	var products []playwright.Locator
	for _, selector := range productSelectors {
		found, err := page.Locator(selector).All()
		if err == nil && len(found) > 0 {
			logger.Info("Using product selector", "selector", selector, "count", len(found))
			products = found
			break
		}
	}
	
	if len(products) == 0 {
		logger.Error("No products found with any selector")
		return links
	}

	for _, product := range products {
		// Extract ASIN
		asin, _ := product.GetAttribute("data-asin")
		if asin == "" {
			continue
		}

		// Extract title
		var title string
		titleSelectors := []string{
			"h2 a span",
			"h2 span",
			".s-title-instructions-style span",
			".a-size-base-plus",
		}
		
		for _, selector := range titleSelectors {
			if elem := product.Locator(selector).First(); elem != nil {
				if t, err := elem.TextContent(); err == nil && t != "" {
					title = strings.TrimSpace(t)
					break
				}
			}
		}

		// Extract URL
		var url string
		if linkElem := product.Locator("h2 a").First(); linkElem != nil {
			if href, err := linkElem.GetAttribute("href"); err == nil && href != "" {
				if strings.HasPrefix(href, "/") {
					url = "https://www.amazon.de" + href
				} else {
					url = href
				}
			}
		}

		// Extract price
		var price string
		priceSelectors := []string{
			".a-price-whole",
			".a-price span",
			".a-price",
		}
		
		for _, selector := range priceSelectors {
			if elem := product.Locator(selector).First(); elem != nil {
				if p, err := elem.TextContent(); err == nil && p != "" {
					price = strings.TrimSpace(p)
					break
				}
			}
		}

		link := &storage.ProductLink{
			ASIN:  asin,
			Title: title,
			URL:   url,
			Price: price,
		}

		links = append(links, link)
	}

	return links
}

func extractAlternativeProducts(page playwright.Page, logger *slog.Logger) []*storage.ProductLink {
	var links []*storage.ProductLink
	
	// Try alternative product container selectors
	selectors := []string{
		"[data-asin]:not([data-asin=''])",
		".s-result-item[data-asin]",
		".sg-col-inner [data-asin]",
	}
	
	for _, selector := range selectors {
		products, err := page.Locator(selector).All()
		if err != nil || len(products) == 0 {
			continue
		}
		
		logger.Info("Found products with alternative selector", "selector", selector, "count", len(products))
		
		for _, product := range products {
			asin, _ := product.GetAttribute("data-asin")
			if asin == "" {
				continue
			}
			
			// Try to extract title from various locations
			title := ""
			titleSelectors := []string{
				"h2",
				".a-link-normal",
				"[data-cy='title-recipe']",
			}
			
			for _, ts := range titleSelectors {
				if elem := product.Locator(ts).First(); elem != nil {
					if t, err := elem.TextContent(); err == nil && t != "" {
						title = strings.TrimSpace(t)
						break
					}
				}
			}
			
			link := &storage.ProductLink{
				ASIN:  asin,
				Title: title,
				URL:   fmt.Sprintf("https://www.amazon.de/dp/%s", asin),
			}
			
			links = append(links, link)
		}
		
		if len(links) > 0 {
			break
		}
	}
	
	return links
}

func findNextPageURL(page playwright.Page, logger *slog.Logger) string {
	// Multiple strategies to find next page
	nextSelectors := []string{
		".s-pagination-next:not(.s-pagination-disabled)",
		"a.s-pagination-item.s-pagination-next",
		"li.a-last a",
		"span.s-pagination-strip a:has-text('Weiter')",
		"a:has-text('Weiter')",
	}

	for _, selector := range nextSelectors {
		elem := page.Locator(selector).First()
		if count, _ := elem.Count(); count > 0 {
			if href, err := elem.GetAttribute("href"); err == nil && href != "" {
				logger.Info("Found next page", "selector", selector)
				if strings.HasPrefix(href, "/") {
					return "https://www.amazon.de" + href
				}
				return href
			}
		}
	}

	return ""
}

func processLinks(ctx context.Context, logger *slog.Logger, cfg *config.Config, concurrent int, headless bool, storage *storage.LinkStorage) {
	// Show current stats
	stats := storage.GetStats()
	logger.Info("Processing links", "stats", stats)

	pending := storage.GetPending()
	if len(pending) == 0 {
		logger.Info("No pending links to process")
		return
	}

	logger.Info("Links to process", "count", len(pending))

	browserOpts := &browser.Options{
		Headless:       headless,
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
		return
	}
	defer b.Close()

	p := parser.NewAmazonParser()
	s := scraper.NewAmazonScraper(b, p, logger)

	// Process each link
	for i, link := range pending {
		select {
		case <-ctx.Done():
			logger.Info("Context cancelled, stopping processing")
			return
		default:
		}

		logger.Info("Processing product", 
			"progress", fmt.Sprintf("%d/%d", i+1, len(pending)),
			"asin", link.ASIN,
			"title", link.Title)

		// Update status to processing
		storage.UpdateStatus(link.ASIN, "processing", "")

		// Scrape the product
		product, err := s.ScrapeByASIN(ctx, link.ASIN)
		if err != nil {
			logger.Error("Failed to scrape product", "asin", link.ASIN, "error", err)
			storage.UpdateStatus(link.ASIN, "failed", err.Error())
			continue
		}

		// Check if we got dimensions
		if product.Dimensions.IsValid() {
			logger.Info("✓ Found dimensions", 
				"asin", link.ASIN,
				"dimensions", fmt.Sprintf("%.1fx%.1fx%.1f %s", 
					product.Dimensions.Length,
					product.Dimensions.Width,
					product.Dimensions.Height,
					product.Dimensions.Unit))
			
			// TODO: Save to database or export
			storage.UpdateStatus(link.ASIN, "completed", "")
		} else {
			logger.Warn("✗ No dimensions found", "asin", link.ASIN)
			storage.UpdateStatus(link.ASIN, "completed", "no dimensions")
		}

		// Rate limiting
		time.Sleep(cfg.Scraper.RateLimitMin)
	}

	// Final stats
	finalStats := storage.GetStats()
	logger.Info("Processing completed", "stats", finalStats)
}