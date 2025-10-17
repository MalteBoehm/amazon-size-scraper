package scraper

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/maltedev/amazon-size-scraper/internal/browser"
	"github.com/maltedev/amazon-size-scraper/internal/parser"
	"github.com/playwright-community/playwright-go"
)

type SearchResult struct {
	ASIN     string
	Title    string
	URL      string
	Price    string
	HasTable bool
}

type SearchScraper struct {
	browser    *browser.Browser
	parser     parser.Parser
	logger     *slog.Logger
	rateLimit  time.Duration
	lastScrape time.Time
}

func NewSearchScraper(b *browser.Browser, p parser.Parser, logger *slog.Logger) *SearchScraper {
	return &SearchScraper{
		browser:   b,
		parser:    p,
		logger:    logger,
		rateLimit: 3 * time.Second,
	}
}

func (s *SearchScraper) ScrapeSearchResults(ctx context.Context, searchURL string) ([]SearchResult, error) {
	s.enforceRateLimit()
	
	s.logger.Info("scraping search results", "url", searchURL)
	
	page, err := s.browser.NewPage()
	if err != nil {
		return nil, fmt.Errorf("failed to create page: %w", err)
	}
	defer page.Close()
	
	if err := s.browser.NavigateWithRetry(page, searchURL, 3); err != nil {
		return nil, fmt.Errorf("failed to navigate: %w", err)
	}
	
	// Wait for search results to load
	page.WaitForSelector("[data-component-type='s-search-result']", playwright.PageWaitForSelectorOptions{
		Timeout: playwright.Float(10000),
	})
	
	time.Sleep(2 * time.Second)
	
	// Extract all product containers
	results := []SearchResult{}
	
	products, err := page.Locator("[data-component-type='s-search-result']").All()
	if err != nil {
		return nil, fmt.Errorf("failed to find products: %w", err)
	}
	
	s.logger.Info("found products", "count", len(products))
	
	for _, product := range products {
		// Extract ASIN
		asin, _ := product.GetAttribute("data-asin")
		if asin == "" {
			continue
		}
		
		// Extract title
		titleElement := product.Locator("h2 a span")
		title, _ := titleElement.TextContent()
		
		// Extract URL
		linkElement := product.Locator("h2 a")
		href, _ := linkElement.GetAttribute("href")
		url := ""
		if href != "" {
			if strings.HasPrefix(href, "/") {
				url = "https://www.amazon.de" + href
			} else {
				url = href
			}
		}
		
		// Extract price
		priceElement := product.Locator(".a-price-whole").First()
		price, _ := priceElement.TextContent()
		
		// Check if title contains size-related keywords
		hasTable := false
		lowerTitle := strings.ToLower(title)
		if strings.Contains(lowerTitle, "größentabelle") || 
		   strings.Contains(lowerTitle, "größe") ||
		   strings.Contains(lowerTitle, "länge") ||
		   strings.Contains(lowerTitle, "breite") {
			hasTable = true
		}
		
		result := SearchResult{
			ASIN:     asin,
			Title:    strings.TrimSpace(title),
			URL:      url,
			Price:    strings.TrimSpace(price),
			HasTable: hasTable,
		}
		
		results = append(results, result)
	}
	
	return results, nil
}

func (s *SearchScraper) ExtractASINsFromSearch(ctx context.Context, searchURL string) ([]string, error) {
	results, err := s.ScrapeSearchResults(ctx, searchURL)
	if err != nil {
		return nil, err
	}
	
	asins := make([]string, 0, len(results))
	for _, result := range results {
		if result.ASIN != "" {
			asins = append(asins, result.ASIN)
		}
	}
	
	return asins, nil
}

func (s *SearchScraper) GetNextPageURL(page playwright.Page) (string, error) {
	// Find next page button
	nextButton := page.Locator(".s-pagination-next:not(.s-pagination-disabled)")
	
	count, err := nextButton.Count()
	if err != nil || count == 0 {
		return "", nil // No next page
	}
	
	href, err := nextButton.GetAttribute("href")
	if err != nil || href == "" {
		return "", nil
	}
	
	if strings.HasPrefix(href, "/") {
		return "https://www.amazon.de" + href, nil
	}
	
	return href, nil
}

func (s *SearchScraper) enforceRateLimit() {
	elapsed := time.Since(s.lastScrape)
	if elapsed < s.rateLimit {
		time.Sleep(s.rateLimit - elapsed)
	}
	s.lastScrape = time.Now()
}