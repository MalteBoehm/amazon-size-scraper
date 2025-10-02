package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/signal"
	"syscall"

	"github.com/maltedev/amazon-size-scraper/internal/browser"
	"github.com/maltedev/amazon-size-scraper/internal/config"
	"github.com/maltedev/amazon-size-scraper/internal/storage"
	"github.com/maltedev/amazon-size-scraper/pkg/logger"
	"github.com/playwright-community/playwright-go"
	"log/slog"
)

func main() {
	var (
		searchURL   = flag.String("url", "", "Amazon search URL")
		storageFile = flag.String("storage", "products-fixed.json", "Storage file")
		maxPages    = flag.Int("pages", 5, "Max pages to crawl")
		headless    = flag.Bool("headless", false, "Run headless")
	)
	flag.Parse()

	if *searchURL == "" {
		fmt.Println("Please provide a search URL with -url")
		os.Exit(1)
	}

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	logger := logger.New(cfg.Logging.Level, cfg.Logging.Format)
	logger.Info("Starting Fixed Crawler")

	// Fix URL encoding
	fixedURL := fixURLEncoding(*searchURL)
	logger.Info("URL fixed", "original", *searchURL, "fixed", fixedURL)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		logger.Info("Shutdown signal received")
		cancel()
	}()

	collectProducts(ctx, logger, cfg, fixedURL, *maxPages, *headless, *storageFile)
}

func fixURLEncoding(rawURL string) string {
	// Parse the URL
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL // Return original if parsing fails
	}

	// Decode the query string
	decodedQuery, err := url.QueryUnescape(u.RawQuery)
	if err != nil {
		return rawURL
	}

	// Reconstruct URL with decoded query
	u.RawQuery = decodedQuery
	return u.String()
}

func collectProducts(ctx context.Context, logger *slog.Logger, cfg *config.Config, startURL string, maxPages int, headless bool, storageFile string) {
	browserOpts := &browser.Options{
		Headless:       headless,
		Timeout:        cfg.Browser.Timeout,
		ViewportWidth:  1920,
		ViewportHeight: 1080,
		AcceptLanguage: "de-DE,de;q=0.9,en;q=0.8",
		TimezoneID:     "Europe/Berlin",
		Locale:         "de-DE",
		UserAgent:      "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
	}

	b, err := browser.New(browserOpts)
	if err != nil {
		logger.Error("Failed to initialize browser", "error", err)
		return
	}
	defer b.Close()

	linkStorage, err := storage.NewLinkStorage(storageFile)
	if err != nil {
		logger.Error("Failed to init storage", "error", err)
		return
	}

	page, err := b.NewPage()
	if err != nil {
		logger.Error("Failed to create page", "error", err)
		return
	}
	defer page.Close()

	currentURL := startURL
	totalProducts := 0

	for pageNum := 1; pageNum <= maxPages && currentURL != ""; pageNum++ {
		logger.Info("Crawling page", "page", pageNum, "url", currentURL)

		// Navigate
		if _, err := page.Goto(currentURL, playwright.PageGotoOptions{
			WaitUntil: playwright.WaitUntilStateNetworkidle,
			Timeout:   playwright.Float(30000),
		}); err != nil {
			logger.Error("Failed to navigate", "error", err)
			break
		}

		// Wait a bit
		page.WaitForTimeout(3000)

		// Check title
		title, _ := page.Title()
		logger.Info("Page loaded", "title", title)

		if title == "Tut uns Leid!" {
			logger.Error("Error page detected")
			break
		}

		// Extract products
		products, err := extractProducts(page, logger)
		if err != nil {
			logger.Error("Failed to extract products", "error", err)
			break
		}

		logger.Info("Found products", "count", len(products), "page", pageNum)
		totalProducts += len(products)

		// Save to storage
		if err := linkStorage.AddBatch(products); err != nil {
			logger.Error("Failed to save products", "error", err)
		}

		// Find next page
		nextURL := findNextPage(page)
		if nextURL == "" {
			logger.Info("No more pages")
			break
		}

		currentURL = nextURL
		page.WaitForTimeout(3000) // Rate limit
	}

	stats := linkStorage.GetStats()
	logger.Info("Collection completed", "total_products", totalProducts, "stats", stats)
}

func extractProducts(page playwright.Page, logger *slog.Logger) ([]*storage.ProductLink, error) {
	var links []*storage.ProductLink

	// Wait for products
	page.WaitForSelector("[data-component-type='s-search-result']", playwright.PageWaitForSelectorOptions{
		Timeout: playwright.Float(10000),
	})

	products, err := page.Locator("[data-component-type='s-search-result']").All()
	if err != nil {
		return nil, err
	}

	for _, product := range products {
		asin, _ := product.GetAttribute("data-asin")
		if asin == "" {
			continue
		}

		// Title
		titleElem := product.Locator("h2 a span").First()
		title, _ := titleElem.TextContent()

		// Price
		priceElem := product.Locator(".a-price-whole").First()
		price, _ := priceElem.TextContent()

		// URL
		linkElem := product.Locator("h2 a").First()
		href, _ := linkElem.GetAttribute("href")
		productURL := ""
		if href != "" {
			if href[0] == '/' {
				productURL = "https://www.amazon.de" + href
			} else {
				productURL = href
			}
		}

		link := &storage.ProductLink{
			ASIN:  asin,
			Title: title,
			URL:   productURL,
			Price: price,
		}

		links = append(links, link)
	}

	return links, nil
}

func findNextPage(page playwright.Page) string {
	// Look for "Weiter" button
	nextButton := page.Locator("a:has-text('Weiter')").First()
	
	if count, _ := nextButton.Count(); count > 0 {
		href, _ := nextButton.GetAttribute("href")
		if href != "" {
			if href[0] == '/' {
				return "https://www.amazon.de" + href
			}
			// Fix encoding for next page URL too
			return fixURLEncoding(href)
		}
	}

	return ""
}