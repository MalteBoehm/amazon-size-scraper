package scraper

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"strconv"
	"strings"

	"github.com/maltedev/amazon-size-scraper/internal/browser"
	"github.com/maltedev/amazon-size-scraper/internal/database"
	"github.com/playwright-community/playwright-go"
)

// CompleteProduct represents a product with all extracted data
type CompleteProduct struct {
	ASIN           string                 `json:"asin"`
	Title          string                 `json:"title"`
	Brand          string                 `json:"brand"`
	DetailPageURL  string                 `json:"detail_page_url"`
	Category       string                 `json:"category"`
	ImageURLs      []string               `json:"image_urls"`
	Features       []string               `json:"features"`
	CurrentPrice   *float64               `json:"current_price"`
	Currency       string                 `json:"currency"`
	Rating         *float64               `json:"rating"`
	ReviewCount    *int                   `json:"review_count"`
	AvailableSizes []string               `json:"available_sizes"`
	SizeTable      *database.SizeTable    `json:"size_table"`
}

// ProductExtractor handles comprehensive product data extraction
type ProductExtractor struct {
	browser *browser.Browser
	logger  *slog.Logger
}

// NewProductExtractor creates a new product extractor
func NewProductExtractor(browser *browser.Browser, logger *slog.Logger) *ProductExtractor {
	return &ProductExtractor{
		browser: browser,
		logger:  logger.With("component", "product_extractor"),
	}
}

// ExtractCompleteProduct extracts all product data including size table
func (pe *ProductExtractor) ExtractCompleteProduct(ctx context.Context, asin, url string) (*CompleteProduct, error) {
	if url == "" && asin != "" {
		url = fmt.Sprintf("https://www.amazon.de/dp/%s", asin)
	}

	pe.logger.Info("extracting complete product data", "asin", asin, "url", url)

	page, err := pe.browser.NewPage()
	if err != nil {
		return nil, fmt.Errorf("failed to create page: %w", err)
	}
	defer page.Close()

	// Navigate to product page
	if err := pe.browser.NavigateWithRetry(page, url, 3); err != nil {
		return nil, fmt.Errorf("failed to navigate: %w", err)
	}

	// Add human-like behavior
	pe.browser.HumanizeInteraction(page)

	// Extract all product data
	product := &CompleteProduct{
		ASIN:          asin,
		DetailPageURL: url,
	}

	// Extract basic info
	if err := pe.extractBasicInfo(page, product); err != nil {
		pe.logger.Warn("failed to extract basic info", "error", err)
	}

	// Extract images
	if err := pe.extractImages(page, product); err != nil {
		pe.logger.Warn("failed to extract images", "error", err)
	}

	// Extract features
	if err := pe.extractFeatures(page, product); err != nil {
		pe.logger.Warn("failed to extract features", "error", err)
	}

	// Extract price
	if err := pe.extractPrice(page, product); err != nil {
		pe.logger.Warn("failed to extract price", "error", err)
	}

	// Extract ratings
	if err := pe.extractRatings(page, product); err != nil {
		pe.logger.Warn("failed to extract ratings", "error", err)
	}

	// Extract available sizes
	if err := pe.extractAvailableSizes(page, product); err != nil {
		pe.logger.Warn("failed to extract sizes", "error", err)
	}

	// Extract size table - this is critical
	sizeTable, err := pe.extractSizeTable(page, asin)
	if err != nil {
		pe.logger.Warn("failed to extract size table", "error", err)
		return nil, fmt.Errorf("no size table found")
	}

	// Validate size table has length and chest
	if !database.ValidateSizeTable(sizeTable) {
		pe.logger.Warn("size table missing length/chest", "asin", asin)
		return nil, fmt.Errorf("size table missing length or chest measurements")
	}

	product.SizeTable = sizeTable

	pe.logger.Info("extracted complete product data",
		"asin", asin,
		"hasImages", len(product.ImageURLs) > 0,
		"hasFeatures", len(product.Features) > 0,
		"hasSizeTable", product.SizeTable != nil,
		"sizeCount", len(product.SizeTable.Sizes),
	)

	return product, nil
}

func (pe *ProductExtractor) extractBasicInfo(page playwright.Page, product *CompleteProduct) error {
	// Extract title
	titleEl, err := page.QuerySelector("#productTitle")
	if err == nil && titleEl != nil {
		title, _ := titleEl.TextContent()
		product.Title = strings.TrimSpace(title)
	}

	// Extract brand
	brandSelectors := []string{
		"a#bylineInfo",
		"span.a-size-base.po-break-word",
		"div.a-section.a-spacing-none span.a-size-base",
	}
	for _, selector := range brandSelectors {
		brandEl, err := page.QuerySelector(selector)
		if err == nil && brandEl != nil {
			brand, _ := brandEl.TextContent()
			brand = strings.TrimSpace(brand)
			brand = strings.TrimPrefix(brand, "Marke: ")
			brand = strings.TrimPrefix(brand, "Brand: ")
			product.Brand = brand
			break
		}
	}

	// Extract category from breadcrumbs
	breadcrumbs, err := page.QuerySelectorAll("div#wayfinding-breadcrumbs_feature_div a")
	if err == nil && len(breadcrumbs) > 0 {
		// Get the last non-product breadcrumb
		for i := len(breadcrumbs) - 1; i >= 0; i-- {
			text, _ := breadcrumbs[i].TextContent()
			text = strings.TrimSpace(text)
			if text != "" && text != product.Title {
				product.Category = text
				break
			}
		}
	}

	return nil
}

func (pe *ProductExtractor) extractImages(page playwright.Page, product *CompleteProduct) error {
	// Extract main image and thumbnails
	imageURLs := []string{}

	// Try to get images from the image block
	thumbs, err := page.QuerySelectorAll("div#altImages img")
	if err == nil {
		for _, thumb := range thumbs {
			src, _ := thumb.GetAttribute("src")
			if src != "" {
				// Convert thumbnail to full size image
				fullSizeURL := strings.Replace(src, "_AC_US40_", "_AC_SL1500_", 1)
				fullSizeURL = strings.Replace(fullSizeURL, "_AC_SR38,50_", "_AC_SL1500_", 1)
				imageURLs = append(imageURLs, fullSizeURL)
			}
		}
	}

	// Fallback to main image
	if len(imageURLs) == 0 {
		mainImg, err := page.QuerySelector("#landingImage")
		if err == nil && mainImg != nil {
			src, _ := mainImg.GetAttribute("src")
			if src != "" {
				imageURLs = append(imageURLs, src)
			}
		}
	}

	product.ImageURLs = imageURLs
	return nil
}

func (pe *ProductExtractor) extractFeatures(page playwright.Page, product *CompleteProduct) error {
	features := []string{}

	// Extract from feature bullets
	bullets, err := page.QuerySelectorAll("div#feature-bullets span.a-list-item")
	if err == nil {
		for _, bullet := range bullets {
			text, _ := bullet.TextContent()
			text = strings.TrimSpace(text)
			if text != "" && !strings.Contains(text, "Weitere Informationen") {
				features = append(features, text)
			}
		}
	}

	product.Features = features
	return nil
}

func (pe *ProductExtractor) extractPrice(page playwright.Page, product *CompleteProduct) error {
	priceSelectors := []string{
		"span.a-price-whole",
		"span#priceblock_dealprice",
		"span#priceblock_ourprice",
		"span.a-price.a-text-price.a-size-medium.apexPriceToPay",
		"span.a-price-range",
	}

	for _, selector := range priceSelectors {
		priceEl, err := page.QuerySelector(selector)
		if err == nil && priceEl != nil {
			priceText, _ := priceEl.TextContent()
			price := pe.parsePrice(priceText)
			if price > 0 {
				product.CurrentPrice = &price
				product.Currency = "EUR"
				break
			}
		}
	}

	return nil
}

func (pe *ProductExtractor) extractRatings(page playwright.Page, product *CompleteProduct) error {
	// Extract rating
	ratingEl, err := page.QuerySelector("span.a-icon-alt")
	if err == nil && ratingEl != nil {
		ratingText, _ := ratingEl.TextContent()
		rating := pe.parseRating(ratingText)
		if rating > 0 {
			product.Rating = &rating
		}
	}

	// Extract review count
	reviewEl, err := page.QuerySelector("#acrCustomerReviewText")
	if err == nil && reviewEl != nil {
		reviewText, _ := reviewEl.TextContent()
		count := pe.parseReviewCount(reviewText)
		if count > 0 {
			product.ReviewCount = &count
		}
	}

	return nil
}

func (pe *ProductExtractor) extractAvailableSizes(page playwright.Page, product *CompleteProduct) error {
	sizes := []string{}

	// Try different size selection methods
	sizeOptions, err := page.QuerySelectorAll("select#native_dropdown_selected_size_name option")
	if err == nil && len(sizeOptions) > 0 {
		for _, option := range sizeOptions {
			size, _ := option.TextContent()
			size = strings.TrimSpace(size)
			if size != "" && size != "Größe auswählen" {
				sizes = append(sizes, size)
			}
		}
	} else {
		// Try button-based size selector
		sizeButtons, err := page.QuerySelectorAll("div#variation_size_name span.a-button-text")
		if err == nil {
			for _, button := range sizeButtons {
				size, _ := button.TextContent()
				size = strings.TrimSpace(size)
				if size != "" {
					sizes = append(sizes, size)
				}
			}
		}
	}

	product.AvailableSizes = sizes
	return nil
}

func (pe *ProductExtractor) extractSizeTable(page playwright.Page, asin string) (*database.SizeTable, error) {
	// Use the existing ExtractSizeChart method from Service
	service := &Service{
		browser: pe.browser,
		logger:  pe.logger,
	}

	dimensions, err := service.ExtractSizeChart(context.Background(), asin, "")
	if err != nil {
		return nil, err
	}

	if !dimensions.Found || dimensions.SizeTable == nil {
		return nil, fmt.Errorf("no size table found")
	}

	return dimensions.SizeTable, nil
}

func (pe *ProductExtractor) parsePrice(text string) float64 {
	// Remove currency symbols and spaces
	text = strings.Replace(text, "€", "", -1)
	text = strings.Replace(text, "EUR", "", -1)
	text = strings.TrimSpace(text)

	// Handle German decimal format (1.234,56)
	text = strings.Replace(text, ".", "", -1)  // Remove thousand separators
	text = strings.Replace(text, ",", ".", -1) // Convert comma to dot

	// Extract numeric value
	re := regexp.MustCompile(`\d+\.?\d*`)
	match := re.FindString(text)
	if match != "" {
		price, err := strconv.ParseFloat(match, 64)
		if err == nil {
			return price
		}
	}

	return 0
}

func (pe *ProductExtractor) parseRating(text string) float64 {
	// Extract rating from text like "4,5 von 5 Sternen"
	re := regexp.MustCompile(`(\d+[,.]?\d*)\s*von\s*5`)
	match := re.FindStringSubmatch(text)
	if len(match) > 1 {
		rating := strings.Replace(match[1], ",", ".", 1)
		if val, err := strconv.ParseFloat(rating, 64); err == nil {
			return val
		}
	}
	return 0
}

func (pe *ProductExtractor) parseReviewCount(text string) int {
	// Extract number from text like "1.234 Bewertungen"
	text = strings.Replace(text, ".", "", -1) // Remove thousand separators
	re := regexp.MustCompile(`(\d+)`)
	match := re.FindString(text)
	if match != "" {
		count, err := strconv.Atoi(match)
		if err == nil {
			return count
		}
	}
	return 0
}

// ConvertToLifecycleProduct converts CompleteProduct to database ProductLifecycle
func (pe *ProductExtractor) ConvertToLifecycleProduct(cp *CompleteProduct) (*database.ProductLifecycle, error) {
	p := &database.ProductLifecycle{
		ASIN:          cp.ASIN,
		Title:         cp.Title,
		Brand:         cp.Brand,
		DetailPageURL: cp.DetailPageURL,
		Category:      cp.Category,
		CurrentPrice:  cp.CurrentPrice,
		Currency:      cp.Currency,
		Rating:        cp.Rating,
		ReviewCount:   cp.ReviewCount,
		Status:        "SCRAPED",
	}

	// Convert arrays to JSON
	if len(cp.ImageURLs) > 0 {
		data, _ := json.Marshal(cp.ImageURLs)
		p.ImageURLs = json.RawMessage(data)
	}

	if len(cp.Features) > 0 {
		data, _ := json.Marshal(cp.Features)
		p.Features = json.RawMessage(data)
	}

	if len(cp.AvailableSizes) > 0 {
		data, _ := json.Marshal(cp.AvailableSizes)
		p.AvailableSizes = json.RawMessage(data)
	}

	if cp.SizeTable != nil {
		data, _ := json.Marshal(cp.SizeTable)
		p.SizeTable = json.RawMessage(data)
	}

	return p, nil
}