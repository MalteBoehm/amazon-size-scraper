package jobs

import (
	"context"
	"fmt"
	"time"

	"github.com/maltedev/amazon-size-scraper/internal/amazon-scraper/events"
	"github.com/maltedev/amazon-size-scraper/internal/amazon-scraper/scraper"
	"github.com/maltedev/amazon-size-scraper/internal/database"
)

// StartWorker starts the background job worker
func (m *Manager) StartWorker(ctx context.Context) {
	m.logger.Info("job worker started")
	
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			m.logger.Info("job worker stopping")
			return
		case <-ticker.C:
			m.processNextJob(ctx)
		}
	}
}

// processNextJob processes the next pending job
func (m *Manager) processNextJob(ctx context.Context) {
	// Get next pending job
	query := `
		SELECT id, search_query, category, max_pages
		FROM scraper_jobs
		WHERE status = 'pending'
		ORDER BY created_at
		LIMIT 1
		FOR UPDATE SKIP LOCKED
	`

	var jobID, searchQuery, category string
	var maxPages int
	
	err := m.db.QueryRow(ctx, query).Scan(&jobID, &searchQuery, &category, &maxPages)
	if err != nil {
		// No pending jobs
		return
	}

	m.logger.Info("processing job", "id", jobID, "query", searchQuery)

	// Update status to running
	if err := m.updateJobStatus(ctx, jobID, "running", nil); err != nil {
		m.logger.Error("failed to update job status", "error", err)
		return
	}

	// Process the job
	if err := m.processJob(ctx, jobID, searchQuery, category, maxPages); err != nil {
		m.logger.Error("job failed", "id", jobID, "error", err)
		m.updateJobStatus(ctx, jobID, "failed", err)
		return
	}

	// Mark as completed
	if err := m.updateJobStatus(ctx, jobID, "completed", nil); err != nil {
		m.logger.Error("failed to mark job as completed", "error", err)
	}

	m.logger.Info("job completed", "id", jobID)
}

// processJob processes a single job
func (m *Manager) processJob(ctx context.Context, jobID, searchQuery, category string, maxPages int) error {
	// Create category crawler
	crawler := scraper.NewCategoryCrawler(m.scraper, m.logger)
	
	// Construct search URL
	searchURL := fmt.Sprintf("https://www.amazon.de/s?k=%s", searchQuery)
	if category != "" {
		searchURL += fmt.Sprintf("&i=%s", category)
	}

	// Crawl pages
	totalProducts := 0
	for page := 1; page <= maxPages; page++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		m.logger.Info("crawling page", "job", jobID, "page", page)

		// Crawl page and get ASINs
		products, hasNext, err := crawler.CrawlPage(ctx, searchURL, page)
		if err != nil {
			m.logger.Error("failed to crawl page", "page", page, "error", err)
			// Continue with next page even if one fails
			continue
		}

		// Process found products
		for _, product := range products {
			// Extract complete product data including size table
			completeProduct, err := m.extractCompleteProductData(ctx, product)
			if err != nil {
				m.logger.Warn("skipping product - no valid size table", 
					"asin", product.ASIN, 
					"error", err)
				continue
			}
			
			// Save complete product to database
			if err := m.saveCompleteProduct(ctx, jobID, completeProduct, page); err != nil {
				m.logger.Error("failed to save product", "asin", product.ASIN, "error", err)
				continue
			}
			
			// Publish enhanced NEW_PRODUCT_DETECTED event
			if err := m.publishEnhancedProductEvent(ctx, completeProduct); err != nil {
				m.logger.Error("failed to publish event", "asin", product.ASIN, "error", err)
			}
			
			totalProducts++
			
			// Rate limiting between product extractions
			time.Sleep(2 * time.Second)
		}

		// Update progress
		if err := m.updateJobProgress(ctx, jobID, page, totalProducts); err != nil {
			m.logger.Error("failed to update progress", "error", err)
		}

		// Check if there are more pages
		if !hasNext {
			m.logger.Info("no more pages", "job", jobID, "lastPage", page)
			break
		}

		// Rate limiting
		time.Sleep(3 * time.Second)
	}

	m.logger.Info("job processing complete", "job", jobID, "products", totalProducts)
	return nil
}

// saveProduct saves a product to the database
func (m *Manager) saveProduct(ctx context.Context, jobID string, product *scraper.Product, pageNumber int) error {
	// Insert into product table (lifecycle table)
	productQuery := `
		INSERT INTO product (
			id, asin, title, detail_page_url, brand,
			status, created_at, updated_at
		) VALUES (
			gen_random_uuid(), $1, $2, $3, $4,
			'PENDING', NOW(), NOW()
		)
		ON CONFLICT (asin) DO UPDATE SET
			title = EXCLUDED.title,
			detail_page_url = EXCLUDED.detail_page_url,
			brand = EXCLUDED.brand,
			updated_at = NOW()
	`

	_, err := m.db.Exec(ctx, productQuery, 
		product.ASIN, product.Title, product.URL, product.Brand)
	if err != nil {
		return fmt.Errorf("failed to insert product: %w", err)
	}

	// Link to job
	jobProductQuery := `
		INSERT INTO job_products (job_id, asin, page_number)
		VALUES ($1, $2, $3)
		ON CONFLICT (job_id, asin) DO NOTHING
	`

	_, err = m.db.Exec(ctx, jobProductQuery, jobID, product.ASIN, pageNumber)
	if err != nil {
		return fmt.Errorf("failed to link product to job: %w", err)
	}

	return nil
}

// extractCompleteProductData extracts full product data including size table
func (m *Manager) extractCompleteProductData(ctx context.Context, product *scraper.Product) (*scraper.CompleteProduct, error) {
	extractor := scraper.NewProductExtractor(m.scraper.GetBrowser(), m.logger)
	
	completeProduct, err := extractor.ExtractCompleteProduct(ctx, product.ASIN, product.URL)
	if err != nil {
		return nil, err
	}
	
	// Ensure we have a valid size table with length and width
	if completeProduct.SizeTable == nil || !database.ValidateSizeTable(completeProduct.SizeTable) {
		return nil, fmt.Errorf("product does not have valid size table with length and width")
	}
	
	return completeProduct, nil
}

// saveCompleteProduct saves a complete product with all data to the database
func (m *Manager) saveCompleteProduct(ctx context.Context, jobID string, product *scraper.CompleteProduct, pageNumber int) error {
	// Convert to database ProductLifecycle
	extractor := scraper.NewProductExtractor(m.scraper.GetBrowser(), m.logger)
	dbProduct, err := extractor.ConvertToLifecycleProduct(product)
	if err != nil {
		return fmt.Errorf("failed to convert product: %w", err)
	}
	
	// Insert into product table
	if err := m.db.InsertProductLifecycle(ctx, dbProduct); err != nil {
		return fmt.Errorf("failed to insert product: %w", err)
	}
	
	// Link to job
	jobProductQuery := `
		INSERT INTO job_products (job_id, asin, page_number)
		VALUES ($1, $2, $3)
		ON CONFLICT (job_id, asin) DO NOTHING
	`
	
	_, err = m.db.Exec(ctx, jobProductQuery, jobID, product.ASIN, pageNumber)
	if err != nil {
		return fmt.Errorf("failed to link product to job: %w", err)
	}
	
	return nil
}

// publishEnhancedProductEvent publishes a NEW_PRODUCT_DETECTED event with complete data
func (m *Manager) publishEnhancedProductEvent(ctx context.Context, product *scraper.CompleteProduct) error {
	// Create enhanced event payload with all product data
	payload := &events.NewProductDetectedPayload{
		ASIN:           product.ASIN,
		Title:          product.Title,
		Brand:          product.Brand,
		DetailPageURL:  product.DetailPageURL,
		Category:       product.Category,
		Price:          convertPrice(product.CurrentPrice, product.Currency),
		Rating:         product.Rating,
		ReviewCount:    product.ReviewCount,
		Images:         product.ImageURLs,
		Features:       product.Features,
		AvailableSizes: product.AvailableSizes,
		SizeTable:      product.SizeTable,
		Source:         "scraper",
	}
	
	// Publish event
	if err := m.publisher.PublishNewProductDetected(ctx, payload); err != nil {
		return fmt.Errorf("failed to publish event: %w", err)
	}
	
	return nil
}

// convertPrice converts price data to event format
func convertPrice(amount *float64, currency string) *events.Price {
	if amount == nil {
		return nil
	}
	return &events.Price{
		Amount:   *amount,
		Currency: currency,
	}
}

// publishProductEvent publishes a NEW_PRODUCT_DETECTED event
func (m *Manager) publishProductEvent(ctx context.Context, product *scraper.Product) error {
	// Create event payload
	payload := &events.NewProductDetectedPayload{
		ASIN:          product.ASIN,
		Title:         product.Title,
		Brand:         product.Brand,
		DetailPageURL: product.URL,
		// Price, Rating, ReviewCount, Images, Features will be populated by Product Lifecycle Service
		// We only provide basic info from search results
	}
	
	// Publish event
	if err := m.publisher.PublishNewProductDetected(ctx, payload); err != nil {
		return fmt.Errorf("failed to publish event: %w", err)
	}
	
	return nil
}