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

type Service struct {
	browser *browser.Browser
	db      *database.DB
	logger  *slog.Logger
}

func NewService(browser *browser.Browser, db *database.DB, logger *slog.Logger) *Service {
	return &Service{
		browser: browser,
		db:      db,
		logger:  logger.With("component", "scraper"),
	}
}

// GetBrowser returns the browser instance
func (s *Service) GetBrowser() *browser.Browser {
	return s.browser
}

// Dimensions represents extracted product dimensions
type Dimensions struct {
	Found     bool
	SizeTable *database.SizeTable
}

// ExtractSizeChart extracts size chart dimensions from a product page
func (s *Service) ExtractSizeChart(ctx context.Context, asin, url string) (*Dimensions, error) {
	// Construct URL if only ASIN is provided
	if url == "" && asin != "" {
		url = fmt.Sprintf("https://www.amazon.de/dp/%s", asin)
	}

	s.logger.Info("extracting size chart", "asin", asin, "url", url)

	page, err := s.browser.NewPage()
	if err != nil {
		return nil, fmt.Errorf("failed to create page: %w", err)
	}
	defer page.Close()

	// Navigate to product page
	if err := s.browser.NavigateWithRetry(page, url, 3); err != nil {
		return nil, fmt.Errorf("failed to navigate: %w", err)
	}

	// Add human-like behavior
	s.browser.HumanizeInteraction(page)

	// Look for and click size table button
	clicked, err := page.Evaluate(`() => {
		// Try multiple selectors for size table
		const selectors = [
			'a:has-text("Größentabelle")',
			'a[href*="size-chart"]',
			'a[href*="size_chart"]',
			'span:has-text("Größentabelle")',
			'button:has-text("Größentabelle")',
			'[data-action*="size-chart"]',
			'[class*="size-chart"]'
		];
		
		// Also try with text content
		const elements = document.querySelectorAll('a, span, button');
		for (let el of elements) {
			const text = el.textContent || '';
			if (text.includes('Größentabelle') || text.includes('Size Chart') || text.includes('Größenratgeber')) {
				console.log('Found size element:', el.tagName, text);
				el.scrollIntoView();
				el.click();
				return true;
			}
		}
		
		// Fallback: try clicking any element with size-related text
		const allElements = document.querySelectorAll('*');
		for (let el of allElements) {
			if (el.onclick || el.href) {
				const text = el.textContent || '';
				if (text === 'Größentabelle' || text === 'Size Chart') {
					el.scrollIntoView();
					el.click();
					return true;
				}
			}
		}
		
		return false;
	}`)

	if err != nil || !clicked.(bool) {
		s.logger.Warn("size table button not found", "asin", asin)
		return &Dimensions{Found: false}, nil
	}

	// Wait for modal to appear
	time.Sleep(3 * time.Second)

	// Extract table data
	tableData, err := page.Evaluate(`() => {
		const tables = document.querySelectorAll('.a-popover-content table, .a-modal-content table, [id*="popover"] table');
		if (tables.length === 0) return null;
		
		const table = tables[0];
		const data = {
			headers: [],
			rows: []
		};
		
		// Get all rows
		for (let i = 0; i < table.rows.length; i++) {
			const row = table.rows[i];
			const rowData = [];
			for (let j = 0; j < row.cells.length; j++) {
				rowData.push(row.cells[j].textContent.trim());
			}
			
			if (i === 0) {
				data.headers = rowData;
			} else {
				data.rows.push(rowData);
			}
		}
		
		return data;
	}`)

	if err != nil || tableData == nil {
		s.logger.Warn("failed to extract table data", "asin", asin, "error", err)
		return &Dimensions{Found: false}, nil
	}

	// Parse the complete size table
	sizeTable := s.parseFullSizeTable(tableData)

	dimensions := &Dimensions{
		Found:     true,
		SizeTable: sizeTable,
	}

	s.logger.Info("extracted dimensions", 
		"asin", asin,
		"hasSizeTable", sizeTable != nil,
		"sizeCount", func() int {
			if sizeTable != nil {
				return len(sizeTable.Sizes)
			}
			return 0
		}(),
	)

	return dimensions, nil
}

// UNUSED - extractSizeTableWithXPath extracts size table data using XPath selectors
func (s *Service) extractSizeTableWithXPath(page playwright.Page) (*database.SizeTable, error) {
	// Find size table in popover/modal
	tables := page.Locator("//div[contains(@class, 'a-popover-content') or contains(@class, 'a-modal-content') or contains(@id, 'popover')]//table")
	
	count, err := tables.Count()
	if err != nil || count == 0 {
		return nil, fmt.Errorf("no size table found")
	}

	// Get the first table
	table := tables.First()

	// Find headers with XPath
	headers := table.Locator("//th")
	headerCount, _ := headers.Count()
	
	var chestIndex, lengthIndex int = -1, -1
	var sizeIndex int = 0 // Usually first column
	
	// Find column indices for "Brustumfang" and "Länge"
	for i := 0; i < headerCount; i++ {
		headerText, _ := headers.Nth(i).TextContent()
		headerLower := strings.ToLower(headerText)
		
		if strings.Contains(headerLower, "brustumfang") {
			chestIndex = i
		} else if strings.Contains(headerLower, "länge") && !strings.Contains(headerLower, "armlänge") {
			lengthIndex = i
		} else if strings.Contains(headerLower, "größe") || strings.Contains(headerLower, "size") {
			sizeIndex = i
		}
	}

	if chestIndex == -1 || lengthIndex == -1 {
		return nil, fmt.Errorf("required columns not found")
	}

	sizeTable := &database.SizeTable{
		Sizes:        []string{},
		Measurements: make(map[string]map[string]float64),
		Unit:         "cm",
	}

	// Get all rows with size data
	rows := table.Locator("//tr[position()>1]") // Skip header row
	rowCount, _ := rows.Count()

	for i := 0; i < rowCount; i++ {
		row := rows.Nth(i)
		cells := row.Locator("//th | //td")
		cellCount, _ := cells.Count()

		if cellCount > max(chestIndex, lengthIndex) {
			// Get size label
			sizeText, _ := cells.Nth(sizeIndex).TextContent()
			sizeText = strings.TrimSpace(sizeText)

			if isSizeLabel(sizeText) {
				sizeTable.Sizes = append(sizeTable.Sizes, sizeText)
				sizeTable.Measurements[sizeText] = make(map[string]float64)

				// Get chest measurement
				if chestText, _ := cells.Nth(chestIndex).TextContent(); chestText != "" {
					if val := parseValue(chestText); val > 0 {
						sizeTable.Measurements[sizeText]["chest"] = val
					}
				}

				// Get length measurement
				if lengthText, _ := cells.Nth(lengthIndex).TextContent(); lengthText != "" {
					if val := parseValue(lengthText); val > 0 {
						sizeTable.Measurements[sizeText]["length"] = val
					}
				}
			}
		}
	}

	if len(sizeTable.Sizes) == 0 {
		return nil, fmt.Errorf("no valid sizes found")
	}

	return sizeTable, nil
}

// max returns the larger of two integers
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// parseFullSizeTable parses the JavaScript table data into a complete size table
func (s *Service) parseFullSizeTable(data interface{}) *database.SizeTable {
	sizeTable := &database.SizeTable{
		Sizes:        []string{},
		Measurements: make(map[string]map[string]float64),
		Unit:         "cm",
	}

	tableMap, ok := data.(map[string]interface{})
	if !ok {
		return nil
	}

	headers, ok := tableMap["headers"].([]interface{})
	if !ok || len(headers) == 0 {
		return nil
	}

	rows, ok := tableMap["rows"].([]interface{})
	if !ok || len(rows) == 0 {
		return nil
	}

	// Determine table structure
	// Option 1: Sizes in first column, measurements in rows
	// Option 2: Sizes in header row, measurements in columns
	
	// Check if first row contains size labels
	firstRowHasSizes := false
	if len(headers) > 1 {
		for i := 1; i < len(headers); i++ {
			headerStr := fmt.Sprintf("%v", headers[i])
			if isSizeLabel(headerStr) {
				firstRowHasSizes = true
				break
			}
		}
	}

	if firstRowHasSizes {
		// Sizes are in the header row
		// Extract sizes from headers (skip first column which is usually the measurement type)
		for i := 1; i < len(headers); i++ {
			sizeStr := strings.TrimSpace(fmt.Sprintf("%v", headers[i]))
			if isSizeLabel(sizeStr) {
				sizeTable.Sizes = append(sizeTable.Sizes, sizeStr)
				sizeTable.Measurements[sizeStr] = make(map[string]float64)
			}
		}

		// Extract measurements from rows
		for _, row := range rows {
			rowData, ok := row.([]interface{})
			if !ok || len(rowData) < 2 {
				continue
			}

			measurementType := strings.ToLower(fmt.Sprintf("%v", rowData[0]))
			measurementKey := ""

			// Map German/English measurement names
			if strings.Contains(measurementType, "brust") || strings.Contains(measurementType, "chest") {
				measurementKey = "chest"
			} else if strings.Contains(measurementType, "länge") || strings.Contains(measurementType, "length") {
				measurementKey = "length"
			} else if strings.Contains(measurementType, "schulter") || strings.Contains(measurementType, "shoulder") {
				measurementKey = "shoulder"
			} else if strings.Contains(measurementType, "ärmel") || strings.Contains(measurementType, "sleeve") {
				measurementKey = "sleeve"
			}

			if measurementKey != "" {
				// Extract values for each size
				for i := 1; i < len(rowData) && i-1 < len(sizeTable.Sizes); i++ {
					size := sizeTable.Sizes[i-1]
					valueStr := fmt.Sprintf("%v", rowData[i])
					if val := parseValue(valueStr); val > 0 {
						sizeTable.Measurements[size][measurementKey] = val
					}
				}
			}
		}
	} else {
		// Sizes are in the first column of each row
		// Extract measurements from headers (skip first column)
		measurementTypes := []string{}
		for i := 1; i < len(headers); i++ {
			headerStr := strings.ToLower(fmt.Sprintf("%v", headers[i]))
			measurementKey := ""

			if strings.Contains(headerStr, "brust") || strings.Contains(headerStr, "chest") {
				measurementKey = "chest"
			} else if strings.Contains(headerStr, "länge") || strings.Contains(headerStr, "length") {
				measurementKey = "length"
			} else if strings.Contains(headerStr, "schulter") || strings.Contains(headerStr, "shoulder") {
				measurementKey = "shoulder"
			} else if strings.Contains(headerStr, "ärmel") || strings.Contains(headerStr, "sleeve") {
				measurementKey = "sleeve"
			}

			measurementTypes = append(measurementTypes, measurementKey)
		}

		// Extract sizes and values from rows
		for _, row := range rows {
			rowData, ok := row.([]interface{})
			if !ok || len(rowData) < 2 {
				continue
			}

			sizeStr := strings.TrimSpace(fmt.Sprintf("%v", rowData[0]))
			if isSizeLabel(sizeStr) {
				sizeTable.Sizes = append(sizeTable.Sizes, sizeStr)
				sizeTable.Measurements[sizeStr] = make(map[string]float64)

				// Extract measurements for this size
				for i := 1; i < len(rowData) && i-1 < len(measurementTypes); i++ {
					if measurementTypes[i-1] != "" {
						valueStr := fmt.Sprintf("%v", rowData[i])
						if val := parseValue(valueStr); val > 0 {
							sizeTable.Measurements[sizeStr][measurementTypes[i-1]] = val
						}
					}
				}
			}
		}
	}

	// Return nil if no valid data found
	if len(sizeTable.Sizes) == 0 {
		return nil
	}

	return sizeTable
}

// ReviewData represents extracted review information
type ReviewData struct {
	Reviews       []ReviewInfo
	AverageRating float64
	TotalReviews  int
}

type ReviewInfo struct {
	Rating         int
	Title          string
	Text           string
	VerifiedBuyer  bool
	Date           string
	MentionsSize   bool
	MentionsLength bool
}

// ExtractReviews extracts product reviews from Amazon
func (s *Service) ExtractReviews(ctx context.Context, asin, url string) (*ReviewData, error) {
	// Construct URL if only ASIN is provided
	if url == "" && asin != "" {
		url = fmt.Sprintf("https://www.amazon.de/dp/%s", asin)
	}

	s.logger.Info("extracting reviews", "asin", asin, "url", url)

	page, err := s.browser.NewPage()
	if err != nil {
		return nil, fmt.Errorf("failed to create page: %w", err)
	}
	defer page.Close()

	// Navigate to product page
	if err := s.browser.NavigateWithRetry(page, url, 3); err != nil {
		return nil, fmt.Errorf("failed to navigate: %w", err)
	}

	// Click on reviews section
	reviewsLink := page.Locator(`a[data-hook="see-all-reviews-link-foot"]`).First()
	if count, _ := reviewsLink.Count(); count > 0 {
		reviewsLink.Click()
		time.Sleep(2 * time.Second)
	}

	// Extract review data
	reviewData, err := page.Evaluate(`() => {
		const reviews = [];
		const reviewElements = document.querySelectorAll('[data-hook="review"]');
		
		reviewElements.forEach(review => {
			const rating = review.querySelector('[data-hook="review-star-rating"]');
			const title = review.querySelector('[data-hook="review-title"]');
			const text = review.querySelector('[data-hook="review-body"]');
			const verified = review.querySelector('[data-hook="avp-badge"]');
			const date = review.querySelector('[data-hook="review-date"]');
			
			if (rating && text) {
				const reviewText = text.textContent.trim().toLowerCase();
				reviews.push({
					rating: parseInt(rating.textContent.match(/\d/)?.[0] || '0'),
					title: title ? title.textContent.trim() : '',
					text: text.textContent.trim(),
					verified_buyer: !!verified,
					date: date ? date.textContent.trim() : '',
					mentions_size: reviewText.includes('größe') || reviewText.includes('size'),
					mentions_length: reviewText.includes('länge') || reviewText.includes('length')
				});
			}
		});
		
		// Get summary data
		const avgRating = document.querySelector('[data-hook="rating-out-of-text"]');
		const totalReviews = document.querySelector('[data-hook="cr-filter-info-review-rating-count"]');
		
		return {
			reviews: reviews.slice(0, 10), // Limit to 10 reviews
			average_rating: avgRating ? parseFloat(avgRating.textContent.match(/[\d,]+/)?.[0].replace(',', '.') || '0') : 0,
			total_reviews: totalReviews ? parseInt(totalReviews.textContent.match(/\d+/)?.[0] || '0') : 0
		};
	}`)

	if err != nil {
		return nil, fmt.Errorf("failed to extract reviews: %w", err)
	}

	// Convert to ReviewData
	result := &ReviewData{
		Reviews: make([]ReviewInfo, 0),
	}

	if reviewMap, ok := reviewData.(map[string]interface{}); ok {
		if reviews, ok := reviewMap["reviews"].([]interface{}); ok {
			for _, r := range reviews {
				if review, ok := r.(map[string]interface{}); ok {
					result.Reviews = append(result.Reviews, ReviewInfo{
						Rating:         int(review["rating"].(float64)),
						Title:          review["title"].(string),
						Text:           review["text"].(string),
						VerifiedBuyer:  review["verified_buyer"].(bool),
						Date:           review["date"].(string),
						MentionsSize:   review["mentions_size"].(bool),
						MentionsLength: review["mentions_length"].(bool),
					})
				}
			}
		}
		
		result.AverageRating = reviewMap["average_rating"].(float64)
		result.TotalReviews = int(reviewMap["total_reviews"].(float64))
	}

	s.logger.Info("extracted reviews", 
		"asin", asin,
		"count", len(result.Reviews),
		"avg_rating", result.AverageRating,
		"total", result.TotalReviews,
	)

	return result, nil
}

// Helper functions
func isSizeLabel(s string) bool {
	s = strings.ToUpper(strings.TrimSpace(s))
	sizeLabels := []string{"XS", "S", "M", "L", "XL", "XXL", "XXXL", "3XL", "4XL", "5XL", "6XL"}
	for _, label := range sizeLabels {
		if s == label {
			return true
		}
	}
	return false
}

func parseValue(text string) float64 {
	// Handle ranges (e.g., "84 - 94") by taking the maximum
	if strings.Contains(text, "-") {
		parts := strings.Split(text, "-")
		if len(parts) == 2 {
			val1 := parseValue(parts[0])
			val2 := parseValue(parts[1])
			if val2 > 0 {
				return val2 // Take the maximum
			}
			return val1
		}
	}

	// Extract numeric value
	var numStr string
	for _, r := range text {
		if (r >= '0' && r <= '9') || r == '.' || r == ',' {
			if r == ',' {
				numStr += "."
			} else {
				numStr += string(r)
			}
		}
	}

	var result float64
	fmt.Sscanf(numStr, "%f", &result)
	return result
}