package scraper

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/maltedev/amazon-size-scraper/internal/browser"
	"github.com/maltedev/amazon-size-scraper/internal/config"
	"github.com/maltedev/amazon-size-scraper/internal/database"
	"github.com/maltedev/amazon-size-scraper/internal/models"
	"github.com/maltedev/amazon-size-scraper/internal/parser"
	"github.com/playwright-community/playwright-go"
)

type ProductScraper struct {
	browser   *browser.Browser
	db        *database.DB
	logger    *slog.Logger
	rateLimit time.Duration
	config    *config.Config
}

func NewProductScraper(b *browser.Browser, db *database.DB, cfg *config.Config) *ProductScraper {
	return &ProductScraper{
		browser:   b,
		db:        db,
		logger:    slog.Default().With("component", "product_scraper"),
		rateLimit: 5 * time.Second,
		config:    cfg,
	}
}

// ScrapeSingleProduct scrapes a single product from a mock product object (debug mode)
func (ps *ProductScraper) ScrapeSingleProduct(ctx context.Context, product *database.Product) error {
	return ps.ScrapeProduct(ctx, product.ASIN)
}

// ScrapeProduct scrapes size data from a single product
func (ps *ProductScraper) ScrapeProduct(ctx context.Context, asin string) error {
	ps.logger.Info("scraping product", "asin", asin)

	// Get product from database (skip in debug mode)
	var product *database.Product
	if ps.db != nil {
		var err error
		product, err = ps.db.GetProduct(ctx, asin)
		if err != nil {
			return fmt.Errorf("failed to get product: %w", err)
		}
		if product == nil {
			return fmt.Errorf("product not found: %s", asin)
		}

		// Skip if already completed
		if product.Status == database.StatusCompleted {
			ps.logger.Info("product already scraped", "asin", asin)
			return nil
		}
	}

	page, err := ps.browser.NewPage()
	if err != nil {
		return fmt.Errorf("failed to create page: %w", err)
	}
	defer page.Close()

	// Navigate to product page
	var url string
	if product != nil {
		url = product.URL
	} else {
		// In debug mode, construct URL from ASIN
		url = fmt.Sprintf("https://www.amazon.de/dp/%s", asin)
	}

	if err := ps.browser.NavigateWithRetry(page, url, 3); err != nil {
		ps.updateProductError(ctx, asin, fmt.Sprintf("Navigation failed: %v", err))
		return fmt.Errorf("failed to navigate: %w", err)
	}

	// Save HTML for debugging if configured (product page)
	if ps.config != nil && ps.config.Logging.LogHTML {
		if html, err := page.Content(); err == nil {
			if err := saveHTML(ctx, ps.logger, ps.config.Logging.HTMLDir, asin, "product_page", html); err != nil {
				ps.logger.Warn("failed to save product page HTML", "error", err, "asin", asin)
			}
		}
	}

	// Add human-like behavior
	ps.browser.HumanizeInteraction(page)

	// Extract material composition in debug mode (before clicking size table button)
	var materialInfo *models.MaterialInfo
	// Check both config and environment variable for debug mode
	debugMode := ps.config.Logging.DebugMode || os.Getenv("DEBUG_MODE") == "true"
	if debugMode {
		ps.logger.Debug("extracting material composition", "asin", asin)
		// Get page content for material extraction
		html, err := page.Content()
		if err == nil {
			// Parse product page for material information
			productParser := parser.NewAmazonParser()
			parsedProduct, err := productParser.ParseProductPage(html, asin)
			if err == nil {
				materialInfo = parsedProduct.MaterialInfo
				if materialInfo != nil && len(materialInfo.Materials) > 0 {
					ps.logger.Info("material composition extracted", "asin", asin, "materials", len(materialInfo.Materials), "source", materialInfo.Source)
				} else {
					ps.logger.Debug("no material composition found", "asin", asin)
				}
			} else {
				ps.logger.Error("failed to parse product for material extraction", "asin", asin, "error", err)
			}
		} else {
			ps.logger.Error("failed to get page content for material extraction", "asin", asin, "error", err)
		}
	}

	// Look for size table button
	sizeTable, err := ps.extractSizeTable(page)
	if err != nil {
		ps.logger.Warn("no size table found", "asin", asin, "error", err)
		ps.updateProductError(ctx, asin, "No size table found")
		return nil // Not an error, just no size data
	}

	// Extract dimensions from size table
	ps.logger.Debug("size table contents", "sizes", sizeTable.Sizes, "measurements", sizeTable.Measurements)

	// Validate size table for realistic measurements
	isValid, invalidReasons := database.ValidateSizeTableWithDetails(sizeTable)

	if !isValid {
		// Log detailed reasons for skipping
		ps.logger.Info("skipping product - invalid size table",
			"asin", asin,
			"reasons", invalidReasons)

		// Determine the primary reason for the error message
		errorMsg := "Invalid size table"
		if len(invalidReasons) > 0 {
			// Check for specific unrealistic measurement patterns
			hasUnrealistic := false
			for _, reason := range invalidReasons {
				if strings.Contains(reason, "unrealistic_") {
					hasUnrealistic = true
					// Extract the problematic measurement for logging
					ps.logger.Warn("unrealistic measurement detected",
						"asin", asin,
						"issue", reason)
				}
			}

			if hasUnrealistic {
				errorMsg = "Unrealistic measurements in size table"
			} else if contains(invalidReasons, "chest_measurements") {
				errorMsg = "No chest measurement in size table"
			} else if contains(invalidReasons, "length_measurements") {
				errorMsg = "No length measurement in size table"
			}
		}

		ps.updateProductError(ctx, asin, errorMsg)
		return nil
	}

	// Material composition already extracted above before size table button click

	// Update product in database or log to console in debug mode
	if ps.config.Logging.DebugMode {
		title := "Unknown Product"
		if product != nil {
			title = product.Title
		}
		ps.logProductResult(asin, title, sizeTable, materialInfo)
	} else {
		if err := ps.db.UpdateProductSizes(ctx, asin, sizeTable); err != nil {
			return fmt.Errorf("failed to update product sizes: %w", err)
		}
	}

	ps.logger.Info("successfully scraped product", "asin", asin,
		"sizeCount", len(sizeTable.Sizes))

	// Color extraction has been moved to PA-API batch enricher for better reliability
	ps.logger.Debug("color extraction via scraper removed - use PA-API batch enricher", "asin", asin)

	// Rate limiting
	time.Sleep(ps.rateLimit)

	return nil
}

// extractSizeTable finds and extracts the size table from the product page
func (ps *ProductScraper) extractSizeTable(page playwright.Page) (*database.SizeTable, error) {
	// Find and click size table button/link using Playwright locators (no JS evaluate)
	candidates := []string{
		`role=button[name="Größentabelle"]`,
		`text=Größentabelle`,
		`role=link[name="Größentabelle"]`,
		`role=button[name="Größentabelle anzeigen"]`,
		`text=Größentabelle anzeigen`,
		`role=button[name="Size chart"]`,
		`text=Size chart`,
		`role=button[name="Size guide"]`,
		`text=Size guide`,
	}
	var clicked bool
	for _, sel := range candidates {
		btn := page.Locator(sel).First()
		if count, _ := btn.Count(); count > 0 {
			ps.logger.Info("found size table trigger", "selector", sel)
			if err := btn.Click(); err == nil {
				clicked = true
				break
			}
		}
	}
	if !clicked {
		return nil, fmt.Errorf("size table button not found")
	}

	ps.logger.Info("clicked size table button")

	// Wait for modal/popup to appear and table to be present
	table := page.Locator(`.a-popover-content table, .a-modal-content table, [id*="popover"] table`).First()
	if err := table.WaitFor(); err != nil {
		return nil, fmt.Errorf("size table not found in modal")
	}

	// Save full HTML for debugging if configured (size table modal)
	if ps.config != nil && ps.config.Logging.LogHTML {
		url := page.URL()
		asin := extractASINFromURL(url)
		if asin == "" {
			asin = "UNKNOWN"
		}
		if html, err := page.Content(); err == nil {
			if err := saveHTML(context.Background(), ps.logger, ps.config.Logging.HTMLDir, asin, "size_table", html); err != nil {
				ps.logger.Warn("failed to save size table HTML", "error", err, "asin", asin)
			}
		}
	}

	// Extract table data using JavaScript
	tableData, err := page.Evaluate(`() => {
		const tables = document.querySelectorAll('.a-popover-content table, .a-modal-content table, [id*="popover"] table');
		if (tables.length === 0) {
			return null;
		}
		
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

	if err != nil {
		return nil, fmt.Errorf("failed to extract table data: %w", err)
	}

	if tableData == nil {
		return nil, fmt.Errorf("size table not found in modal")
	}

	ps.logger.Info("extracted table data")

	// Parse the JavaScript data into our structure
	ps.logger.Debug("raw table data", "data", tableData)
	return ps.parseJSTableData(tableData)
}

// parseJSTableData parses the JavaScript table data into our structure
func (ps *ProductScraper) parseJSTableData(data interface{}) (*database.SizeTable, error) {
	tableMap, ok := data.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid table data format")
	}

	headers, ok := tableMap["headers"].([]interface{})
	if !ok || len(headers) == 0 {
		return nil, fmt.Errorf("no headers found")
	}

	rows, ok := tableMap["rows"].([]interface{})
	if !ok || len(rows) == 0 {
		return nil, fmt.Errorf("no data rows found")
	}

	sizeTable := &database.SizeTable{
		Measurements: make(map[string]map[string]float64),
		Unit:         "cm",
	}

	// Log the structure for debugging
	ps.logger.Debug("table structure", "headers", headers, "rowCount", len(rows))

	// Amazon tables often have sizes in first column and measurements in first row
	// Let's check if first row contains size labels (S, M, L, etc.)
	firstRow := rows[0].([]interface{})
	hasSizeLabels := false

	for _, cell := range firstRow {
		cellStr := fmt.Sprintf("%v", cell)
		if isSizeLabel(cellStr) {
			hasSizeLabels = true
			break
		}
	}

	ps.logger.Debug("layout detection", "hasSizeLabels", hasSizeLabels, "firstRow", firstRow)

	if hasSizeLabels {
		// Horizontal layout: sizes in first column of each row
		ps.logger.Debug("using horizontal layout - sizes in first column")

		// Map header labels to measurement types
		measurementLabels := make([]string, 0)
		for i := 0; i < len(headers); i++ {
			label := ps.normalizeLabel(fmt.Sprintf("%v", headers[i]))
			measurementLabels = append(measurementLabels, label)
		}

		// Process all data rows
		for i := 0; i < len(rows); i++ {
			rowData := rows[i].([]interface{})
			if len(rowData) < 1 {
				continue
			}

			// First cell should be size
			size := fmt.Sprintf("%v", rowData[0])
			size = strings.TrimSpace(size)

			if !isSizeLabel(size) {
				continue
			}

			// Add size to list
			sizeTable.Sizes = append(sizeTable.Sizes, size)
			sizeTable.Measurements[size] = make(map[string]float64)

			// Extract values for each measurement
			for j := 1; j < len(rowData) && j < len(measurementLabels); j++ {
				label := measurementLabels[j]
				valueStr := fmt.Sprintf("%v", rowData[j])
				value := ps.parseValue(valueStr)
				if value > 0 {
					sizeTable.Measurements[size][label] = value
				}
			}
		}
	} else {
		// Vertical layout: sizes in header row, measurements below
		ps.logger.Debug("using vertical layout")
		// Find where sizes start in header
		sizeColStart := 1
		for i, h := range headers {
			header := fmt.Sprintf("%v", h)
			if isSizeLabel(header) {
				sizeColStart = i
				break
			}
		}

		// Extract sizes from headers
		for i := sizeColStart; i < len(headers); i++ {
			size := fmt.Sprintf("%v", headers[i])
			size = strings.TrimSpace(size)
			if size != "" && isSizeLabel(size) {
				sizeTable.Sizes = append(sizeTable.Sizes, size)
				sizeTable.Measurements[size] = make(map[string]float64)
			}
		}

		// Process data rows
		for _, row := range rows {
			rowData, ok := row.([]interface{})
			if !ok || len(rowData) < 2 {
				continue
			}

			// Get measurement label
			label := ps.normalizeLabel(fmt.Sprintf("%v", rowData[0]))

			// Extract values for each size
			for i := sizeColStart; i < len(rowData) && i-sizeColStart < len(sizeTable.Sizes); i++ {
				size := sizeTable.Sizes[i-sizeColStart]
				valueStr := fmt.Sprintf("%v", rowData[i])
				value := ps.parseValue(valueStr)
				if value > 0 {
					sizeTable.Measurements[size][label] = value
				}
			}
		}
	}

	return sizeTable, nil
}

// isSizeLabel checks if a string is a size label
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

// parseTable extracts data from the HTML table (keeping for compatibility)
func (ps *ProductScraper) parseTable(table playwright.Locator) (*database.SizeTable, error) {
	sizeTable := &database.SizeTable{
		Measurements: make(map[string]map[string]float64),
		Unit:         "cm", // Default to cm
	}

	// Get headers (sizes)
	headers := table.Locator("thead th, tbody tr:first-child td")
	headerCount, _ := headers.Count()

	for i := 1; i < headerCount; i++ { // Skip first column (row labels)
		headerText, err := headers.Nth(i).TextContent()
		if err == nil && headerText != "" {
			size := strings.TrimSpace(headerText)
			sizeTable.Sizes = append(sizeTable.Sizes, size)
			sizeTable.Measurements[size] = make(map[string]float64)
		}
	}

	// Get rows
	rows := table.Locator("tbody tr")
	rowCount, _ := rows.Count()

	for i := 0; i < rowCount; i++ {
		row := rows.Nth(i)
		cells := row.Locator("td")
		cellCount, _ := cells.Count()

		if cellCount > 1 {
			// Get row label
			labelText, err := cells.Nth(0).TextContent()
			if err != nil || labelText == "" {
				continue
			}

			label := ps.normalizeLabel(strings.TrimSpace(labelText))

			// Get values for each size
			for j := 1; j < cellCount && j-1 < len(sizeTable.Sizes); j++ {
				valueText, err := cells.Nth(j).TextContent()
				if err == nil && valueText != "" {
					value := ps.parseValue(valueText)
					if value > 0 {
						size := sizeTable.Sizes[j-1]
						sizeTable.Measurements[size][label] = value
					}
				}
			}
		}
	}

	return sizeTable, nil
}

// normalizeLabel normalizes measurement labels to standard names
func (ps *ProductScraper) normalizeLabel(label string) string {
	label = strings.ToLower(label)

	// Map German labels to standard names
	mappings := map[string]string{
		"länge":       "length",
		"breite":      "width",
		"brustumfang": "chest",
		"brust":       "chest",
		"schulter":    "shoulder",
		"ärmel":       "sleeve",
		"höhe":        "height",
		"taille":      "waist",
		"hüfte":       "hip",
	}

	for german, english := range mappings {
		if strings.Contains(label, german) {
			return english
		}
	}

	// Handle special cases
	if strings.Contains(label, "länge") || strings.Contains(label, "laenge") {
		return "length"
	}

	return label
}

// parseValue extracts numeric value from text
func (ps *ProductScraper) parseValue(text string) float64 {
	// Handle ranges (e.g., "84 - 94") by taking the average
	if strings.Contains(text, "-") {
		parts := strings.Split(text, "-")
		if len(parts) == 2 {
			val1 := ps.parseValue(parts[0])
			val2 := ps.parseValue(parts[1])
			if val1 > 0 && val2 > 0 {
				return (val1 + val2) / 2
			}
		}
	}

	// Remove all non-numeric characters except comma and dot
	re := regexp.MustCompile(`[^\d,.]`)
	cleaned := re.ReplaceAllString(text, "")

	// Replace German decimal comma with dot
	cleaned = strings.Replace(cleaned, ",", ".", 1)

	// Parse the value
	value, err := strconv.ParseFloat(cleaned, 64)
	if err != nil {
		return 0
	}

	return value
}

// updateProductError updates the product status with an error
func (ps *ProductScraper) updateProductError(ctx context.Context, asin, errorMsg string) {
	if ps.config.Logging.DebugMode {
		ps.logger.Error("product error in debug mode", "asin", asin, "error", errorMsg)
	} else {
		if err := ps.db.UpdateProductStatus(ctx, asin, database.StatusFailed, errorMsg); err != nil {
			ps.logger.Error("failed to update product error status", "asin", asin, "error", err)
		}
	}
}

// logProductResult logs product data to console in debug mode
func (ps *ProductScraper) logProductResult(asin, title string, sizeTable *database.SizeTable, materialInfo *models.MaterialInfo) {
	fmt.Printf("\n=== PRODUCT RESULT ===\n")
	fmt.Printf("ASIN: %s\n", asin)
	fmt.Printf("Title: %s\n", title)

	// Display material composition if available
	if materialInfo != nil && len(materialInfo.Materials) > 0 {
		fmt.Printf("Material Composition:\n")
		for material, percentage := range materialInfo.Materials {
			if percentage > 0 {
				fmt.Printf("  - %s: %.1f%%\n", material, percentage)
			} else {
				fmt.Printf("  - %s: present\n", material)
			}
		}
		fmt.Printf("  Source: %s (confidence: %.2f)\n", materialInfo.Source, materialInfo.Confidence)
		fmt.Printf("  Raw Text: %s\n", materialInfo.RawText)
	} else {
		fmt.Printf("Material Composition: <not detected>\n")
	}

	fmt.Printf("Sizes (%d):\n", len(sizeTable.Sizes))
	for _, size := range sizeTable.Sizes {
		fmt.Printf("  - %s\n", size)
	}
	fmt.Printf("Measurements:\n")
	for size, measurements := range sizeTable.Measurements {
		fmt.Printf("  %s:\n", size)
		for measurement, value := range measurements {
			fmt.Printf("    %s: %.1f cm\n", measurement, value)
		}
	}
	fmt.Printf("====================\n\n")
}

// contains checks if a slice contains a string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// ScrapeAllPending scrapes all pending products
func (ps *ProductScraper) ScrapeAllPending(ctx context.Context, limit int) error {
	// Add detailed logging to understand why no products are found
	ps.logger.Info("starting ScrapeAllPending", "limit", limit)

	// First, let's check the database connection and table status
	if ps.db != nil {
		// Check total product counts by status
		counts, err := ps.db.CountProductsByStatus(ctx)
		if err != nil {
			ps.logger.Error("failed to get product counts", "error", err)
		} else {
			ps.logger.Info("product status counts",
				"pending", counts[database.StatusPending],
				"completed", counts[database.StatusCompleted],
				"failed", counts[database.StatusFailed])
		}

		// Check if there are any products at all using a simple query
		var totalCount int
		row := ps.db.QueryRow(ctx, "SELECT COUNT(*) FROM product")
		err = row.Scan(&totalCount)
		if err != nil {
			ps.logger.Error("failed to get total product count", "error", err)
		} else {
			ps.logger.Info("total products in database", "count", totalCount)
		}
	} else {
		ps.logger.Warn("database connection is nil")
	}

	for {
		// Get pending products
		products, err := ps.db.GetPendingProducts(ctx, limit)
		if err != nil {
			return fmt.Errorf("failed to get pending products: %w", err)
		}

		if len(products) == 0 {
			ps.logger.Info("no pending products found", "attempted_limit", limit)
			break
		}

		ps.logger.Info("found pending products", "count", len(products))

		// Scrape each product
		for _, product := range products {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
				if err := ps.ScrapeProduct(ctx, product.ASIN); err != nil {
					ps.logger.Error("failed to scrape product", "asin", product.ASIN, "error", err)
					// Continue with next product
				}
			}
		}
	}

	return nil
}
