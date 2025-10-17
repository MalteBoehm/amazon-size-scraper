package scraper

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"time"
)

// Product represents a product found on a category page
type Product struct {
	ASIN     string
	Title    string
	URL      string
	Brand    string
	Category string
}

// CategoryCrawler handles crawling of Amazon category/search pages
type CategoryCrawler struct {
	service *Service
	logger  *slog.Logger
}

func NewCategoryCrawler(service *Service, logger *slog.Logger) *CategoryCrawler {
	return &CategoryCrawler{
		service: service,
		logger:  logger.With("component", "category_crawler"),
	}
}

// CrawlPage crawls a single page of search results
func (c *CategoryCrawler) CrawlPage(ctx context.Context, searchURL string, pageNumber int) ([]*Product, bool, error) {
	// Add page parameter if not first page
	if pageNumber > 1 {
		parsedURL, err := url.Parse(searchURL)
		if err != nil {
			return nil, false, fmt.Errorf("failed to parse URL: %w", err)
		}
		
		q := parsedURL.Query()
		q.Set("page", fmt.Sprintf("%d", pageNumber))
		parsedURL.RawQuery = q.Encode()
		searchURL = parsedURL.String()
	}

	c.logger.Info("crawling page", "url", searchURL, "page", pageNumber)

	page, err := c.service.browser.NewPage()
	if err != nil {
		return nil, false, fmt.Errorf("failed to create page: %w", err)
	}
	defer page.Close()

	// First navigate to Amazon.de to handle bot check
	if pageNumber == 1 {
		if err := c.service.browser.NavigateWithRetry(page, "https://www.amazon.de", 1); err != nil {
			c.logger.Warn("failed to navigate to homepage", "error", err)
		}
		time.Sleep(2 * time.Second)
	}

	// Navigate to search page
	if err := c.service.browser.NavigateWithRetry(page, searchURL, 3); err != nil {
		return nil, false, fmt.Errorf("failed to navigate to search page: %w", err)
	}

	// Wait for products to load
	time.Sleep(2 * time.Second)

	// Extract products
	products, err := c.extractProducts(page)
	if err != nil {
		return nil, false, fmt.Errorf("failed to extract products: %w", err)
	}

	// Check if there's a next page
	hasNext, err := c.hasNextPage(page)
	if err != nil {
		c.logger.Warn("failed to check for next page", "error", err)
		hasNext = false
	}

	c.logger.Info("extracted products", "count", len(products), "hasNext", hasNext)

	return products, hasNext, nil
}

// extractProducts extracts product information from the page
func (c *CategoryCrawler) extractProducts(page interface{}) ([]*Product, error) {
	// Import playwright
	pwPage, ok := page.(interface {
		Evaluate(expression string, options ...interface{}) (interface{}, error)
	})
	if !ok {
		return nil, fmt.Errorf("invalid page type")
	}
	
	// Use Evaluate to extract products via JavaScript
	result, err := pwPage.Evaluate(`() => {
		const products = [];
		const elements = document.querySelectorAll('[data-component-type="s-search-result"]');
		
		elements.forEach(el => {
			const asin = el.getAttribute('data-asin');
			if (!asin) return;
			
			const titleEl = el.querySelector('h2 a span');
			const brandEl = el.querySelector('span.s-size-override-12');
			
			products.push({
				asin: asin,
				title: titleEl ? titleEl.textContent.trim() : '',
				brand: brandEl ? brandEl.textContent.trim() : ''
			});
		});
		
		return products;
	}`)
	
	if err != nil {
		return nil, fmt.Errorf("failed to evaluate products: %w", err)
	}

	// Parse the result
	productsData, ok := result.([]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected result type from evaluate")
	}

	var products []*Product
	for _, p := range productsData {
		productMap, ok := p.(map[string]interface{})
		if !ok {
			continue
		}

		asin, _ := productMap["asin"].(string)
		if asin == "" {
			continue
		}

		title, _ := productMap["title"].(string)
		brand, _ := productMap["brand"].(string)

		products = append(products, &Product{
			ASIN:  asin,
			Title: title,
			Brand: brand,
			URL:   fmt.Sprintf("https://www.amazon.de/dp/%s", asin),
		})
	}

	c.logger.Debug("found products", "count", len(products))
	return products, nil
}

// hasNextPage checks if there's a next page button
func (c *CategoryCrawler) hasNextPage(page interface{}) (bool, error) {
	// Type assert to page with Evaluate method
	pwPage, ok := page.(interface {
		Evaluate(expression string, options ...interface{}) (interface{}, error)
	})
	if !ok {
		return false, fmt.Errorf("invalid page type")
	}
	
	// Use Evaluate to check for next page button
	result, err := pwPage.Evaluate(`() => {
		// Check for pagination next button
		const nextButton = document.querySelector('.s-pagination-next:not(.s-pagination-disabled)');
		// Also check for "Weiter" text link
		const weiterLink = document.querySelector('a:contains("Weiter")') || 
			Array.from(document.querySelectorAll('a')).find(a => a.textContent.includes('Weiter'));
		return (nextButton !== null) || (weiterLink !== null);
	}`)
	
	if err != nil {
		return false, err
	}
	
	hasNext, ok := result.(bool)
	if !ok {
		return false, fmt.Errorf("unexpected result type from evaluate")
	}
	
	return hasNext, nil
}