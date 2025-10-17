package scraper

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/playwright-community/playwright-go"
	"github.com/maltedev/amazon-size-scraper/internal/browser"
	"github.com/maltedev/amazon-size-scraper/internal/database"
)

type SearchCrawler struct {
	browser    *browser.Browser
	db         *database.DB
	logger     *slog.Logger
	rateLimit  time.Duration
}

type ProductListing struct {
	ASIN     string
	Title    string
	URL      string
	Brand    string
	Category string
}

func NewSearchCrawler(b *browser.Browser, db *database.DB) *SearchCrawler {
	return &SearchCrawler{
		browser:   b,
		db:        db,
		logger:    slog.Default().With("component", "search_crawler"),
		rateLimit: 5 * time.Second,
	}
}

// CrawlSearch crawls all products from a search URL
func (sc *SearchCrawler) CrawlSearch(ctx context.Context, searchURL string) error {
	sc.logger.Info("starting search crawl", "url", searchURL)
	
	page, err := sc.browser.NewPage()
	if err != nil {
		return fmt.Errorf("failed to create page: %w", err)
	}
	defer page.Close()
	
	// First navigate to Amazon.de to handle bot check
	sc.logger.Info("navigating to Amazon.de first")
	if err := sc.browser.NavigateWithRetry(page, "https://www.amazon.de", 3); err != nil {
		sc.logger.Warn("failed to navigate to homepage", "error", err)
	}
	
	// Now navigate to search page
	if err := sc.browser.NavigateWithRetry(page, searchURL, 3); err != nil {
		return fmt.Errorf("failed to navigate to search: %w", err)
	}
	
	// Add human-like behavior
	sc.browser.HumanizeInteraction(page)
	
	pageNum := 1
	totalProducts := 0
	
	for {
		sc.logger.Info("processing search page", "page", pageNum)
		
		// Extract products from current page
		sc.logger.Debug("calling extractProductsFromPage")
		products, err := sc.extractProductsFromPage(page)
		if err != nil {
			sc.logger.Error("product extraction failed", "error", err, "page", pageNum)
			return fmt.Errorf("failed to extract products from page %d: %w", pageNum, err)
		}
		
		sc.logger.Info("found products on page", "page", pageNum, "count", len(products))
		
		// Save products to database
		for _, product := range products {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
				if err := sc.saveProduct(ctx, product); err != nil {
					sc.logger.Error("failed to save product", "asin", product.ASIN, "error", err)
					// Continue with other products
				}
			}
		}
		
		totalProducts += len(products)
		
		// Check for next page
		hasNext, err := sc.goToNextPage(page)
		if err != nil {
			return fmt.Errorf("failed to navigate to next page: %w", err)
		}
		
		if !hasNext {
			sc.logger.Info("no more pages found")
			break
		}
		
		pageNum++
		
		// Rate limiting
		time.Sleep(sc.rateLimit)
	}
	
	sc.logger.Info("search crawl completed", "total_products", totalProducts, "pages", pageNum)
	return nil
}

// extractProductsFromPage extracts all products from the current page
func (sc *SearchCrawler) extractProductsFromPage(page playwright.Page) ([]*ProductListing, error) {
	sc.logger.Debug("waiting for product selector")
	
	// Wait for products to load
	_, err := page.WaitForSelector(`[data-component-type="s-search-result"]`, playwright.PageWaitForSelectorOptions{
		Timeout: playwright.Float(10000),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to wait for products: %w", err)
	}
	
	sc.logger.Debug("finding product elements")
	
	// Find all product containers
	productElements := page.Locator(`[data-component-type="s-search-result"]`)
	count, err := productElements.Count()
	if err != nil {
		return nil, fmt.Errorf("failed to count products: %w", err)
	}
	
	sc.logger.Debug("found product elements", "count", count)
	
	var products []*ProductListing
	
	for i := 0; i < count; i++ {
		productEl := productElements.Nth(i)
		
		// Extract ASIN
		asin, err := productEl.GetAttribute("data-asin")
		if err != nil || asin == "" {
			continue
		}
		
		product := &ProductListing{
			ASIN: asin,
			URL:  fmt.Sprintf("https://www.amazon.de/dp/%s", asin),
		}
		
		// Extract title
		titleEl := productEl.Locator("h2 a span").First()
		if titleEl != nil {
			title, err := titleEl.TextContent()
			if err == nil {
				product.Title = strings.TrimSpace(title)
			}
		}
		
		// Extract brand if available
		brandEl := productEl.Locator(`[class*="s-line-clamp"] .s-size-override-12`).First()
		if brandEl != nil {
			brand, err := brandEl.TextContent()
			if err == nil && brand != "" {
				product.Brand = strings.TrimSpace(brand)
			}
		}
		
		// Set category from search (we're looking for t-shirts)
		product.Category = "T-Shirt"
		
		products = append(products, product)
	}
	
	return products, nil
}

// goToNextPage attempts to navigate to the next page
func (sc *SearchCrawler) goToNextPage(page playwright.Page) (bool, error) {
	// Look for "Weiter" button
	nextButtonSelectors := []string{
		`a.s-pagination-next`,
		`a:has-text("Weiter")`,
		`.s-pagination-next`,
	}
	
	for _, selector := range nextButtonSelectors {
		nextButton := page.Locator(selector).First()
		
		// Check if button exists and is not disabled
		count, err := nextButton.Count()
		if err != nil || count == 0 {
			continue
		}
		
		// Check if disabled
		isDisabled, err := nextButton.GetAttribute("aria-disabled")
		if err == nil && isDisabled == "true" {
			sc.logger.Info("next button is disabled")
			return false, nil
		}
		
		sc.logger.Info("clicking next button", "selector", selector)
		
		// Click next button
		if err := nextButton.Click(); err != nil {
			sc.logger.Error("failed to click next button", "error", err)
			continue
		}
		
		// Wait for navigation
		time.Sleep(3 * time.Second)
		
		// Verify we moved to a new page
		if _, err := page.WaitForSelector(`[data-component-type="s-search-result"]`, playwright.PageWaitForSelectorOptions{
			Timeout: playwright.Float(10000),
		}); err != nil {
			sc.logger.Warn("failed to wait for products on next page", "error", err)
		}
		
		return true, nil
	}
	
	sc.logger.Info("no next button found")
	return false, nil
}

// saveProduct saves a product to the database
func (sc *SearchCrawler) saveProduct(ctx context.Context, product *ProductListing) error {
	dbProduct := &database.Product{
		ASIN:     product.ASIN,
		Title:    product.Title,
		URL:      product.URL,
		Status:   database.StatusPending,
	}
	
	if product.Brand != "" {
		dbProduct.Brand.String = product.Brand
		dbProduct.Brand.Valid = true
	}
	
	if product.Category != "" {
		dbProduct.Category.String = product.Category
		dbProduct.Category.Valid = true
	}
	
	return sc.db.InsertProduct(ctx, dbProduct)
}