package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/MalteBoehm/tall-affiliate-common/pkg/constants"
)

// SafeProductRepository provides transaction-safe product operations with comprehensive error handling
type SafeProductRepository struct {
	db                   *DB
	transactionManager   *TransactionManager
	productTxService     *ProductTransactionService
	outboxRepo           *OutboxRepository
	monitor              *TransactionMonitor
	logger               *slog.Logger
	config               SafeRepositoryConfig
}

// SafeRepositoryConfig defines safe repository configuration
type SafeRepositoryConfig struct {
	EnableTransactionSafety   bool
	EnableEventPublishing     bool
	EnableMonitoring          bool
	DefaultTransactionTimeout  time.Duration
	MaxRetryAttempts          int
	RetryBackoffBase          time.Duration
	EnableDeadLetterQueue     bool
}

// DefaultSafeRepositoryConfig returns sensible defaults
func DefaultSafeRepositoryConfig() SafeRepositoryConfig {
	return SafeRepositoryConfig{
		EnableTransactionSafety:  true,
		EnableEventPublishing:    true,
		EnableMonitoring:         true,
		DefaultTransactionTimeout: 30 * time.Second,
		MaxRetryAttempts:         3,
		RetryBackoffBase:         100 * time.Millisecond,
		EnableDeadLetterQueue:    true,
	}
}

// NewSafeProductRepository creates a new safe product repository
func NewSafeProductRepository(
	db *DB,
	logger *slog.Logger,
	transactionManager *TransactionManager,
	monitor *TransactionMonitor,
	config SafeRepositoryConfig,
) *SafeProductRepository {
	outboxRepo := NewOutboxRepository(db)
	productTxService := NewProductTransactionService(transactionManager, outboxRepo, logger)

	return &SafeProductRepository{
		db:                 db,
		transactionManager: transactionManager,
		productTxService:   productTxService,
		outboxRepo:         outboxRepo,
		monitor:            monitor,
		logger:             logger,
		config:             config,
	}
}

// CreateProductWithSafety creates a product with full transaction safety
func (spr *SafeProductRepository) CreateProductWithSafety(
	ctx context.Context,
	product *ProductLifecycle,
) error {
	if spr.config.EnableMonitoring {
		startTime := time.Now()
		defer func() {
			duration := time.Since(startTime)
			spr.monitor.RecordTransaction(TransactionRecord{
				ID:        uuid.New(),
				Type:      "create_product",
				StartTime: startTime,
				EndTime:   time.Now(),
				Duration:  duration,
				Status:    "success",
			})
		}()
	}

	// Prepare events
	events := spr.prepareProductEvents(product, "PRODUCT_CREATED")

	// Use transaction service for atomic creation
	if spr.config.EnableTransactionSafety {
		return spr.productTxService.CreateProductWithEvents(ctx, product, events)
	}

	// Fallback to simple insertion without transaction safety
	return spr.simpleProductInsert(ctx, product)
}

// UpdateProductWithSafety updates a product with full transaction safety
func (spr *SafeProductRepository) UpdateProductWithSafety(
	ctx context.Context,
	product *ProductLifecycle,
) error {
	if spr.config.EnableMonitoring {
		startTime := time.Now()
		defer func() {
			duration := time.Since(startTime)
			spr.monitor.RecordTransaction(TransactionRecord{
				ID:        uuid.New(),
				Type:      "update_product",
				StartTime: startTime,
				EndTime:   time.Now(),
				Duration:  duration,
				Status:    "success",
			})
		}()
	}

	// Prepare events
	events := spr.prepareProductEvents(product, "PRODUCT_UPDATED")

	// Use transaction service for atomic update
	if spr.config.EnableTransactionSafety {
		return spr.productTxService.UpdateProductWithEvents(ctx, product, events)
	}

	// Fallback to simple update without transaction safety
	return spr.simpleProductUpdate(ctx, product)
}

// CreateProductWithSizeTable creates a product and size table atomically
func (spr *SafeProductRepository) CreateProductWithSizeTable(
	ctx context.Context,
	product *ProductLifecycle,
	sizeTable *SizeTable,
) error {
	if spr.config.EnableMonitoring {
		startTime := time.Now()
		recordID := uuid.New()
		defer func() {
			duration := time.Since(startTime)
			status := "success"

			// Update status if there's a panic or error
			if r := recover(); r != nil {
				status = "failed"
			}

			spr.monitor.RecordTransaction(TransactionRecord{
				ID:        recordID,
				Type:      "create_product_with_size_table",
				StartTime: startTime,
				EndTime:   time.Now(),
				Duration:  duration,
				Status:    status,
			})
		}()
	}

	// Validate size table before proceeding
	if sizeTable != nil && !ValidateSizeTable(sizeTable) {
		return fmt.Errorf("invalid size table provided")
	}

	// Attach size table to product
	if sizeTable != nil {
		sizeTableJSON, err := json.Marshal(sizeTable)
		if err != nil {
			return fmt.Errorf("failed to marshal size table: %w", err)
		}
		product.SizeTable = sizeTableJSON
		product.Status = "SCRAPED"
	}

	// Prepare events
	events := []*OutboxEvent{}

	// Product created event
	productEvent := &OutboxEvent{
		ID:            uuid.New(),
		EventType:     "PRODUCT_CREATED",
		Payload:       spr.createProductPayload(product),
		TargetStream:  constants.StreamProductLifecycle,
		Status:        OutboxStatusPending,
	}
	events = append(events, productEvent)

	// Size table scraped event if size table is valid
	if sizeTable != nil && ValidateSizeTable(sizeTable) {
		sizeTableEvent := &OutboxEvent{
			ID:            uuid.New(),
			EventType:     "01B_PRODUCT_SIZE_TABLE_SCRAPED",
			Payload:       spr.createSizeTablePayload(product.ASIN, sizeTable),
			TargetStream:  constants.StreamProductLifecycle,
			Status:        OutboxStatusPending,
		}
		events = append(events, sizeTableEvent)
	}

	// Use transaction service for atomic creation
	if spr.config.EnableTransactionSafety {
		return spr.productTxService.CreateProductWithEvents(ctx, product, events)
	}

	// Fallback to simple operations
	return spr.simpleProductInsert(ctx, product)
}

// IgnoreProductWithReason creates a product ignore event with proper error handling
func (spr *SafeProductRepository) IgnoreProductWithReason(
	ctx context.Context,
	asin string,
	reason string,
	productData map[string]interface{},
) error {
	if spr.config.EnableMonitoring {
		startTime := time.Now()
		defer func() {
			duration := time.Since(startTime)
			spr.monitor.RecordTransaction(TransactionRecord{
				ID:        uuid.New(),
				Type:      "ignore_product",
				StartTime: startTime,
				EndTime:   time.Now(),
				Duration:  duration,
				Status:    "success",
			})
		}()
	}

	// Create ignore event
	event := &OutboxEvent{
		ID:            uuid.New(),
		AggregateType: "product",
		AggregateID:   asin,
		EventType:     "02B_PRODUCT_IGNORED",
		Payload:       spr.createIgnorePayload(asin, reason, productData),
		TargetStream:  constants.StreamProductLifecycle,
		Status:        OutboxStatusPending,
	}

	// Set error message to prevent retries for certain reasons
	if reason == "no_valid_size_table" || reason == "no_size_information" {
		errorMsg := "Product ignored due to missing size table - no retry scheduled"
		event.ErrorMessage = &errorMsg
	}

	// Insert event with transaction safety
	if spr.config.EnableTransactionSafety {
		return spr.transactionManager.ExecuteInTransaction(ctx, func(tx Tx) error {
			return spr.outboxRepo.InsertWithTx(ctx, tx, event)
		})
	}

	// Fallback to simple insertion
	return spr.simpleEventInsert(ctx, event)
}

// GetProductWithRetry retrieves a product with retry logic
func (spr *SafeProductRepository) GetProductWithRetry(
	ctx context.Context,
	asin string,
) (*ProductLifecycle, error) {
	var lastErr error
	var product *ProductLifecycle

	for attempt := 0; attempt < spr.config.MaxRetryAttempts; attempt++ {
		if attempt > 0 {
			// Calculate backoff delay
			delay := time.Duration(1<<uint(attempt-1)) * spr.config.RetryBackoffBase
			if delay > 5*time.Second {
				delay = 5 * time.Second
			}

			spr.logger.Debug("Retrying product fetch",
				"asin", asin,
				"attempt", attempt+1,
				"max_attempts", spr.config.MaxRetryAttempts,
				"delay", delay,
			)

			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}
		}

		product, lastErr = spr.GetProductLifecycleByASIN(ctx, asin)
		if lastErr == nil {
			return product, nil
		}

		// Check if error is retryable
		if !spr.isRetryableError(lastErr) {
			break
		}
	}

	return nil, fmt.Errorf("failed to get product after %d attempts: %w", spr.config.MaxRetryAttempts, lastErr)
}

// BatchCreateProducts creates multiple products with batch transaction safety
func (spr *SafeProductRepository) BatchCreateProducts(
	ctx context.Context,
	products []*ProductLifecycle,
) error {
	if spr.config.EnableMonitoring {
		startTime := time.Now()
		defer func() {
			duration := time.Since(startTime)
			spr.monitor.RecordTransaction(TransactionRecord{
				ID:        uuid.New(),
				Type:      "batch_create_products",
				StartTime: startTime,
				EndTime:   time.Now(),
				Duration:  duration,
				Status:    "success",
			})
		}()
	}

	if !spr.config.EnableTransactionSafety {
		// Fallback to individual inserts
		for _, product := range products {
			if err := spr.simpleProductInsert(ctx, product); err != nil {
				return fmt.Errorf("failed to insert product %s: %w", product.ASIN, err)
			}
		}
		return nil
	}

	// Use transaction for batch operation
	return spr.transactionManager.ExecuteInTransaction(ctx, func(tx Tx) error {
		for _, product := range products {
			// Insert product
			if err := spr.insertProductLifecycleTx(ctx, tx, product); err != nil {
				return fmt.Errorf("failed to insert product %s: %w", product.ASIN, err)
			}

			// Create associated event
			event := &OutboxEvent{
				ID:            uuid.New(),
				AggregateType: "product",
				AggregateID:   product.ASIN,
				EventType:     "PRODUCT_CREATED",
				Payload:       spr.createProductPayload(product),
				TargetStream:  constants.StreamProductLifecycle,
				Status:        OutboxStatusPending,
			}

			if err := spr.outboxRepo.InsertWithTx(ctx, tx, event); err != nil {
				return fmt.Errorf("failed to insert event for product %s: %w", product.ASIN, err)
			}
		}

		spr.logger.Info("Batch created products successfully",
			"count", len(products),
		)

		return nil
	})
}

// GetProductLifecycleByASIN retrieves a product by ASIN
func (spr *SafeProductRepository) GetProductLifecycleByASIN(ctx context.Context, asin string) (*ProductLifecycle, error) {
	query := `
		SELECT
			id, asin, title, brand, detail_page_url,
			image_urls, features, current_price, currency,
			rating, review_count, status, category,
			available_sizes, size_chart_data, created_at, updated_at,
			material_composition, material_full_text, care_instructions,
			colors, material, gender, product_groups, reviews_content,
			is_prime_eligible, in_stock, variation_attributes, model
		FROM product
		WHERE asin = $1`

	var p ProductLifecycle
	var imageURLs, features, availableSizes, sizeTable, materialComposition, careInstructions, colors, productGroups, variationAttributes sql.NullString

	err := spr.db.QueryRow(ctx, query, asin).Scan(
		&p.ID, &p.ASIN, &p.Title, &p.Brand, &p.DetailPageURL,
		&imageURLs, &features, &p.CurrentPrice, &p.Currency,
		&p.Rating, &p.ReviewCount, &p.Status, &p.Category,
		&availableSizes, &sizeTable, &p.CreatedAt, &p.UpdatedAt,
		&materialComposition, &p.MaterialFullText, &careInstructions,
		&colors, &p.Material, &p.Gender, &productGroups, &p.ReviewsContent,
		&p.IsPrimeEligible, &p.InStock, &variationAttributes, &p.Model,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get product lifecycle: %w", err)
	}

	// Handle nullable JSON fields
	if imageURLs.Valid {
		p.ImageURLs = json.RawMessage(imageURLs.String)
	}
	if features.Valid {
		p.Features = json.RawMessage(features.String)
	}
	if availableSizes.Valid {
		p.AvailableSizes = json.RawMessage(availableSizes.String)
	}
	if sizeTable.Valid {
		p.SizeTable = json.RawMessage(sizeTable.String)
	}
	if materialComposition.Valid {
		p.MaterialComposition = json.RawMessage(materialComposition.String)
	}
	if careInstructions.Valid {
		p.CareInstructions = json.RawMessage(careInstructions.String)
	}
	if colors.Valid {
		p.Colors = json.RawMessage(colors.String)
	}
	if productGroups.Valid {
		p.ProductGroups = json.RawMessage(productGroups.String)
	}
	if variationAttributes.Valid {
		p.VariationAttributes = json.RawMessage(variationAttributes.String)
	}

	return &p, nil
}

// Helper methods

func (spr *SafeProductRepository) prepareProductEvents(product *ProductLifecycle, eventType string) []*OutboxEvent {
	event := &OutboxEvent{
		ID:            uuid.New(),
		AggregateType: "product",
		AggregateID:   product.ASIN,
		EventType:     eventType,
		Payload:       spr.createProductPayload(product),
		TargetStream:  constants.StreamProductLifecycle,
		Status:        OutboxStatusPending,
	}

	return []*OutboxEvent{event}
}

func (spr *SafeProductRepository) createProductPayload(product *ProductLifecycle) json.RawMessage {
	payload := map[string]interface{}{
		"asin":               product.ASIN,
		"title":              product.Title,
		"brand":              product.Brand,
		"category":           product.Category,
		"status":             product.Status,
		"created_at":         product.CreatedAt,
		"updated_at":         product.UpdatedAt,
	}

	// Add optional fields if they exist
	if product.CurrentPrice != nil {
		payload["current_price"] = *product.CurrentPrice
	}
	if product.Rating != nil {
		payload["rating"] = *product.Rating
	}
	if product.ReviewCount != nil {
		payload["review_count"] = *product.ReviewCount
	}

	data, _ := json.Marshal(payload)
	return data
}

func (spr *SafeProductRepository) createSizeTablePayload(asin string, sizeTable *SizeTable) json.RawMessage {
	payload := map[string]interface{}{
		"asin":       asin,
		"size_table": sizeTable,
		"valid":      ValidateSizeTable(sizeTable),
	}

	data, _ := json.Marshal(payload)
	return data
}

func (spr *SafeProductRepository) createIgnorePayload(asin string, reason string, productData map[string]interface{}) json.RawMessage {
	payload := map[string]interface{}{
		"asin":        asin,
		"reason":      reason,
		"product_data": productData,
		"ignored_at":  time.Now().UTC().Format(time.RFC3339),
	}

	data, _ := json.Marshal(payload)
	return data
}

func (spr *SafeProductRepository) simpleProductInsert(ctx context.Context, product *ProductLifecycle) error {
	return spr.transactionManager.ExecuteInTransaction(ctx, func(tx Tx) error {
		return spr.insertProductLifecycleTx(ctx, tx, product)
	})
}

func (spr *SafeProductRepository) simpleProductUpdate(ctx context.Context, product *ProductLifecycle) error {
	return spr.transactionManager.ExecuteInTransaction(ctx, func(tx Tx) error {
		return spr.updateProductLifecycleTx(ctx, tx, product)
	})
}

func (spr *SafeProductRepository) simpleEventInsert(ctx context.Context, event *OutboxEvent) error {
	return spr.transactionManager.ExecuteInTransaction(ctx, func(tx Tx) error {
		return spr.outboxRepo.InsertWithTx(ctx, tx, event)
	})
}

func (spr *SafeProductRepository) insertProductLifecycleTx(ctx context.Context, tx Tx, p *ProductLifecycle) error {
	// Implementation copied from product_lifecycle.go but adapted for transaction
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}

	query := `
		INSERT INTO product (
			id, asin, title, brand, detail_page_url, category, status, size_chart_data,
			image_urls, features, current_price, currency, rating, review_count,
			available_sizes, gender, size, color, product_group, reviews_content,
			material_composition, material_full_text, care_instructions, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, NOW(), NOW()
		)
		ON CONFLICT (asin) DO UPDATE SET
			title = EXCLUDED.title,
			brand = EXCLUDED.brand,
			detail_page_url = EXCLUDED.detail_page_url,
			category = EXCLUDED.category,
			status = EXCLUDED.status,
			updated_at = NOW()
		RETURNING created_at, updated_at`

	err := tx.QueryRow(ctx, query,
		p.ID, p.ASIN, p.Title, p.Brand, p.DetailPageURL, p.Category, p.Status, p.SizeTable,
		p.ImageURLs, p.Features, p.CurrentPrice, p.Currency, p.Rating, p.ReviewCount,
		p.AvailableSizes, p.Gender, "", "", "", p.ReviewsContent,
		p.MaterialComposition, p.MaterialFullText, p.CareInstructions,
	).Scan(&p.CreatedAt, &p.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to insert product lifecycle: %w", err)
	}

	return nil
}

func (spr *SafeProductRepository) updateProductLifecycleTx(ctx context.Context, tx Tx, p *ProductLifecycle) error {
	query := `
		UPDATE product SET
			title = $2,
			brand = $3,
			detail_page_url = $4,
			image_urls = $5,
			features = $6,
			current_price = $7,
			currency = $8,
			rating = $9,
			review_count = $10,
			status = $11,
			category = $12,
			available_sizes = $13,
			size_chart_data = $14,
			material_composition = $15,
			material_full_text = $16,
			care_instructions = $17,
			updated_at = NOW()
		WHERE asin = $1`

	result, err := tx.Exec(ctx, query,
		p.ASIN, p.Title, p.Brand, p.DetailPageURL,
		p.ImageURLs, p.Features, p.CurrentPrice, p.Currency,
		p.Rating, p.ReviewCount, p.Status, p.Category,
		p.AvailableSizes, p.SizeTable, p.MaterialComposition, p.MaterialFullText, p.CareInstructions,
	)

	if err != nil {
		return fmt.Errorf("failed to update product: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("product not found: %s", p.ASIN)
	}

	return nil
}

func (spr *SafeProductRepository) isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()

	retryableErrors := []string{
		"connection",
		"timeout",
		"deadlock",
		"connection reset",
		"connection refused",
		"network is unreachable",
		"too many connections",
		"connection pool exhausted",
	}

	for _, retryableErr := range retryableErrors {
		if contains(errStr, retryableErr) {
			return true
		}
	}

	return false
}