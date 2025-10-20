package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
)

// TransactionManager handles complex database transactions with proper error handling
type TransactionManager struct {
	db     *DB
	logger *slog.Logger
	config TransactionConfig
}

// TransactionConfig defines transaction behavior
type TransactionConfig struct {
	MaxRetries          int
	RetryDelay          time.Duration
	MaxRetryDelay       time.Duration
	Timeout             time.Duration
	IsolationLevel      sql.IsolationLevel
	EnableDeadLetter    bool
	MaxDeadLetterRetries int
}

// DefaultTransactionConfig returns sensible defaults
func DefaultTransactionConfig() TransactionConfig {
	return TransactionConfig{
		MaxRetries:           3,
		RetryDelay:           100 * time.Millisecond,
		MaxRetryDelay:        5 * time.Second,
		Timeout:              30 * time.Second,
		IsolationLevel:       sql.LevelReadCommitted,
		EnableDeadLetter:     true,
		MaxDeadLetterRetries: 10,
	}
}

// NewTransactionManager creates a new transaction manager
func NewTransactionManager(db *DB, logger *slog.Logger, config TransactionConfig) *TransactionManager {
	return &TransactionManager{
		db:     db,
		logger: logger,
		config: config,
	}
}

// TransactionOptions defines options for a specific transaction
type TransactionManagerOptions struct {
	Timeout        time.Duration
	IsolationLevel sql.IsolationLevel
	Retryable      bool
	DeadLetterOnFail bool
}

// DefaultTransactionOptions returns default transaction options
func DefaultTransactionOptions() TransactionManagerOptions {
	return TransactionManagerOptions{
		Timeout:           30 * time.Second,
		IsolationLevel:    sql.LevelReadCommitted,
		Retryable:         true,
		DeadLetterOnFail:  true,
	}
}

// TransactionError represents a transaction error with retry information
type TransactionError struct {
	Err          error
	RetryCount   int
	IsRetryable  bool
	IsDeadLetter bool
	Context      map[string]interface{}
}

func (e *TransactionError) Error() string {
	if e.IsDeadLetter {
		return fmt.Sprintf("transaction failed after %d retries, moved to dead letter: %v", e.RetryCount, e.Err)
	}
	return fmt.Sprintf("transaction failed after %d retries: %v", e.RetryCount, e.Err)
}

func (e *TransactionError) Unwrap() error {
	return e.Err
}

// ExecuteInTransaction executes a function within a transaction with retry logic
func (tm *TransactionManager) ExecuteInTransaction(
	ctx context.Context,
	fn func(Tx) error,
	opts ...TransactionManagerOptions,
) error {
	// Apply default options
	options := DefaultTransactionOptions()
	if len(opts) > 0 {
		options = opts[0]
	}

	// Apply config defaults if not specified
	if options.Timeout == 0 {
		options.Timeout = tm.config.Timeout
	}
	if options.IsolationLevel == 0 {
		options.IsolationLevel = tm.config.IsolationLevel
	}

	var lastErr error
	maxRetries := tm.config.MaxRetries
	if !options.Retryable {
		maxRetries = 0
	}

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			// Calculate exponential backoff with jitter
			delay := tm.calculateRetryDelay(attempt)
			tm.logger.Debug("Retrying transaction",
				"attempt", attempt,
				"max_retries", maxRetries,
				"delay", delay,
				"last_error", lastErr,
			)

			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
		}

		// Create timeout context
		timeoutCtx, cancel := context.WithTimeout(ctx, options.Timeout)

		// Execute transaction
		err := tm.executeSingleTransaction(timeoutCtx, fn, options)
		cancel()

		if err == nil {
			// Success
			if attempt > 0 {
				tm.logger.Info("Transaction succeeded after retries",
					"attempt", attempt,
					"max_retries", maxRetries,
				)
			}
			return nil
		}

		lastErr = err

		// Check if error is retryable
		if !tm.isRetryableError(err) || !options.Retryable {
			tm.logger.Error("Non-retryable transaction error",
				"error", err,
				"attempt", attempt,
			)
			break
		}

		tm.logger.Warn("Transaction failed, will retry",
			"error", err,
			"attempt", attempt,
			"max_retries", maxRetries,
		)
	}

	// All retries exhausted
	tm.logger.Error("Transaction failed after all retries",
		"error", lastErr,
		"max_retries", maxRetries,
		"final_attempt", maxRetries,
	)

	// Handle dead letter if enabled
	if options.DeadLetterOnFail && tm.config.EnableDeadLetter {
		if deadLetterErr := tm.handleDeadLetter(ctx, lastErr, maxRetries); deadLetterErr != nil {
			tm.logger.Error("Failed to handle dead letter",
				"original_error", lastErr,
				"dead_letter_error", deadLetterErr,
			)
		}
	}

	return &TransactionError{
		Err:          lastErr,
		RetryCount:   maxRetries,
		IsRetryable:  false,
		IsDeadLetter: options.DeadLetterOnFail && tm.config.EnableDeadLetter,
		Context:      map[string]interface{}{"attempts": maxRetries + 1},
	}
}

// executeSingleTransaction executes a single transaction attempt
func (tm *TransactionManager) executeSingleTransaction(
	ctx context.Context,
	fn func(Tx) error,
	options TransactionManagerOptions,
) error {
	tx, err := tm.db.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	defer func() {
		if err != nil {
			if rbErr := tx.Rollback(ctx); rbErr != nil {
				tm.logger.Error("Transaction rollback failed",
					"rollback_error", rbErr,
					"original_error", err,
				)
			}
		}
	}()

	// Execute user function
	if err = fn(tx); err != nil {
		return err
	}

	// Commit transaction
	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// calculateRetryDelay calculates exponential backoff with jitter
func (tm *TransactionManager) calculateRetryDelay(attempt int) time.Duration {
	// Exponential backoff: base * 2^attempt
	delay := tm.config.RetryDelay * time.Duration(1<<uint(attempt-1))

	// Cap at max delay
	if delay > tm.config.MaxRetryDelay {
		delay = tm.config.MaxRetryDelay
	}

	// Add jitter (±25%)
	jitter := time.Duration(float64(delay) * 0.25 * (2.0*float64(time.Now().UnixNano()%1000)/1000.0 - 1.0))

	return delay + jitter
}

// isRetryableError determines if an error is retryable
func (tm *TransactionManager) isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()

	// Database connection errors
	retryableErrors := []string{
		"connection",
		"timeout",
		"deadlock",
		"lock wait timeout",
		"connection reset",
		"connection refused",
		"network is unreachable",
		"temporary failure",
		"too many connections",
		"connection pool exhausted",
		"SSL connection has been closed unexpectedly",
		"database is locked",
		"could not serialize access",
		"could not obtain lock",
	}

	for _, retryableErr := range retryableErrors {
		if contains(errStr, retryableErr) {
			return true
		}
	}

	// Check for specific SQL states
	if contains(errStr, "SQLSTATE") {
		// 40001: serialization_failure
		// 40P01: deadlock_detected
		// 53000: insufficient_resources
		// 53300: too_many_connections
		// 57P03: cannot_connect_now
		retryableSQLStates := []string{"40001", "40P01", "53000", "53300", "57P03"}
		for _, state := range retryableSQLStates {
			if contains(errStr, state) {
				return true
			}
		}
	}

	return false
}

// handleDeadLetter handles failed transactions by creating dead letter records
func (tm *TransactionManager) handleDeadLetter(ctx context.Context, err error, retryCount int) error {
	tm.logger.Warn("Creating dead letter record for failed transaction",
		"error", err,
		"retry_count", retryCount,
	)

	// Create dead letter event
	deadLetterEvent := &OutboxEvent{
		ID:            uuid.New(),
		AggregateType: "transaction_failure",
		AggregateID:   fmt.Sprintf("tx_%d", time.Now().UnixNano()),
		EventType:     "TRANSACTION_DEAD_LETTER",
		Payload:       tm.createDeadLetterPayload(err, retryCount),
		TargetStream:  "stream:dead_letter",
		Status:        OutboxStatusDeadLetter,
		RetryCount:    retryCount,
		ErrorMessage:  stringPtr(err.Error()),
		CreatedAt:     time.Now(),
	}

	// Insert dead letter event
	outboxRepo := NewOutboxRepository(tm.db)
	if insertErr := outboxRepo.InsertWithTx(ctx, &deadLetterTxAdapter{tx: nil}, deadLetterEvent); insertErr != nil {
		return fmt.Errorf("failed to create dead letter record: %w", insertErr)
	}

	return nil
}

// createDeadLetterPayload creates payload for dead letter events
func (tm *TransactionManager) createDeadLetterPayload(err error, retryCount int) json.RawMessage {
	payload := map[string]interface{}{
		"error":        err.Error(),
		"error_type":   fmt.Sprintf("%T", err),
		"retry_count":  retryCount,
		"timestamp":    time.Now().UTC().Format(time.RFC3339),
		"service":      "amazon-size-scraper",
		"component":    "transaction_manager",
	}

	data, _ := json.Marshal(payload)
	return data
}

// ProductTransactionService handles product-specific transactions
type ProductTransactionService struct {
	tm      *TransactionManager
	outbox  *OutboxRepository
	logger  *slog.Logger
}

// NewProductTransactionService creates a new product transaction service
func NewProductTransactionService(tm *TransactionManager, outbox *OutboxRepository, logger *slog.Logger) *ProductTransactionService {
	return &ProductTransactionService{
		tm:     tm,
		outbox: outbox,
		logger: logger,
	}
}

// CreateProductWithEvents creates a product and associated events in a single transaction
func (pts *ProductTransactionService) CreateProductWithEvents(
	ctx context.Context,
	product *ProductLifecycle,
	events []*OutboxEvent,
) error {
	return pts.tm.ExecuteInTransaction(ctx, func(tx Tx) error {
		// Insert product
		if err := pts.insertProductLifecycleTx(ctx, tx, product); err != nil {
			return fmt.Errorf("failed to insert product: %w", err)
		}

		// Insert associated events
		for _, event := range events {
			event.AggregateType = "product"
			event.AggregateID = product.ASIN
			if err := pts.outbox.InsertWithTx(ctx, tx, event); err != nil {
				return fmt.Errorf("failed to insert outbox event: %w", err)
			}
		}

		pts.logger.Info("Product and events created successfully",
			"asin", product.ASIN,
			"events_count", len(events),
		)

		return nil
	})
}

// UpdateProductWithEvents updates a product and creates events in a single transaction
func (pts *ProductTransactionService) UpdateProductWithEvents(
	ctx context.Context,
	product *ProductLifecycle,
	events []*OutboxEvent,
) error {
	return pts.tm.ExecuteInTransaction(ctx, func(tx Tx) error {
		// Update product
		if err := pts.updateProductLifecycleTx(ctx, tx, product); err != nil {
			return fmt.Errorf("failed to update product: %w", err)
		}

		// Insert associated events
		for _, event := range events {
			event.AggregateType = "product"
			event.AggregateID = product.ASIN
			if err := pts.outbox.InsertWithTx(ctx, tx, event); err != nil {
				return fmt.Errorf("failed to insert outbox event: %w", err)
			}
		}

		pts.logger.Info("Product and events updated successfully",
			"asin", product.ASIN,
			"events_count", len(events),
		)

		return nil
	})
}

// Helper methods for transaction operations
func (pts *ProductTransactionService) insertProductLifecycleTx(ctx context.Context, tx Tx, p *ProductLifecycle) error {
	// Generate ID if not provided
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
			size_chart_data = EXCLUDED.size_chart_data,
			image_urls = EXCLUDED.image_urls,
			features = EXCLUDED.features,
			current_price = EXCLUDED.current_price,
			currency = EXCLUDED.currency,
			rating = EXCLUDED.rating,
			review_count = EXCLUDED.review_count,
			available_sizes = EXCLUDED.available_sizes,
			gender = EXCLUDED.gender,
			size = EXCLUDED.size,
			color = EXCLUDED.color,
			product_group = EXCLUDED.product_group,
			reviews_content = EXCLUDED.reviews_content,
			material_composition = EXCLUDED.material_composition,
			material_full_text = EXCLUDED.material_full_text,
			care_instructions = EXCLUDED.care_instructions,
			status = EXCLUDED.status,
			updated_at = NOW()
		RETURNING created_at, updated_at`

	// Use the first available size as the size field
	var sizeStr string
	if p.AvailableSizes != nil && len(p.AvailableSizes) > 2 {
		var sizes []string
		if err := json.Unmarshal(p.AvailableSizes, &sizes); err == nil && len(sizes) > 0 {
			sizeStr = sizes[0]
		}
	}

	// Use the first available color as the color field
	var colorStr string
	if p.Colors != nil && len(p.Colors) > 2 {
		var colors []string
		if err := json.Unmarshal(p.Colors, &colors); err == nil && len(colors) > 0 {
			colorStr = colors[0]
		}
	}

	// Extract product_group from ProductGroups JSON array
	var productGroupStr string
	if p.ProductGroups != nil && len(p.ProductGroups) > 2 {
		var productGroups []string
		if err := json.Unmarshal(p.ProductGroups, &productGroups); err == nil && len(productGroups) > 0 {
			productGroupStr = strings.Join(productGroups, " > ")
		}
	}

	// Ensure JSON fields are never nil
	imageURLs := p.ImageURLs
	if imageURLs == nil || len(imageURLs) == 0 {
		imageURLs = json.RawMessage("[]")
	}
	features := p.Features
	if features == nil || len(features) == 0 {
		features = json.RawMessage("[]")
	}
	availableSizes := p.AvailableSizes
	if availableSizes == nil || len(availableSizes) == 0 {
		availableSizes = json.RawMessage("[]")
	}
	materialComposition := p.MaterialComposition
	if materialComposition == nil || len(materialComposition) == 0 {
		materialComposition = json.RawMessage("{}")
	}
	careInstructions := p.CareInstructions
	if careInstructions == nil || len(careInstructions) == 0 {
		careInstructions = json.RawMessage("[]")
	}

	err := tx.QueryRow(ctx, query,
		p.ID, p.ASIN, p.Title, p.Brand, p.DetailPageURL, p.Category, p.Status, p.SizeTable,
		imageURLs, features, p.CurrentPrice, p.Currency, p.Rating, p.ReviewCount,
		availableSizes, p.Gender, sizeStr, colorStr, productGroupStr, p.ReviewsContent,
		materialComposition, p.MaterialFullText, careInstructions,
	).Scan(&p.CreatedAt, &p.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to insert product lifecycle: %w", err)
	}

	return nil
}

func (pts *ProductTransactionService) updateProductLifecycleTx(ctx context.Context, tx Tx, p *ProductLifecycle) error {
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
			color_variations = $18,
			updated_at = NOW()
		WHERE asin = $1`

	// Ensure JSON fields are never nil
	imageURLs := p.ImageURLs
	if imageURLs == nil || len(imageURLs) == 0 {
		imageURLs = json.RawMessage("[]")
	}
	features := p.Features
	if features == nil || len(features) == 0 {
		features = json.RawMessage("[]")
	}
	availableSizes := p.AvailableSizes
	if availableSizes == nil || len(availableSizes) == 0 {
		availableSizes = json.RawMessage("[]")
	}
	materialComposition := p.MaterialComposition
	if materialComposition == nil || len(materialComposition) == 0 {
		materialComposition = json.RawMessage("{}")
	}
	careInstructions := p.CareInstructions
	if careInstructions == nil || len(careInstructions) == 0 {
		careInstructions = json.RawMessage("[]")
	}

	// Prepare color_variations JSON
	colorVariations := p.Colors
	if colorVariations == nil || len(colorVariations) == 0 {
		colorVariations = json.RawMessage("[]")
	}

	result, err := tx.Exec(ctx, query,
		p.ASIN, p.Title, p.Brand, p.DetailPageURL,
		imageURLs, features, p.CurrentPrice, p.Currency,
		p.Rating, p.ReviewCount, p.Status, p.Category,
		availableSizes, p.SizeTable, materialComposition, p.MaterialFullText, careInstructions,
		colorVariations,
	)

	if err != nil {
		return fmt.Errorf("failed to update product with full data: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("product not found: %s", p.ASIN)
	}

	return nil
}

// Helper functions
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr ||
		(len(s) > len(substr) &&
			(s[:len(substr)] == substr ||
			 s[len(s)-len(substr):] == substr ||
			 findSubstring(s, substr))))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func stringPtr(s string) *string {
	return &s
}

// deadLetterTxAdapter is a mock transaction for dead letter insertion
type deadLetterTxAdapter struct {
	tx Tx
}

func (a *deadLetterTxAdapter) Exec(ctx context.Context, sql string, args ...interface{}) (CommandTag, error) {
	if a.tx != nil {
		return a.tx.Exec(ctx, sql, args...)
	}
	// For dead letter events, we need to use the pool directly
	return nil, fmt.Errorf("dead letter transaction not implemented")
}

func (a *deadLetterTxAdapter) QueryRow(ctx context.Context, sql string, args ...interface{}) Row {
	if a.tx != nil {
		return a.tx.QueryRow(ctx, sql, args...)
	}
	return nil
}

func (a *deadLetterTxAdapter) Query(ctx context.Context, sql string, args ...interface{}) (Rows, error) {
	if a.tx != nil {
		return a.tx.Query(ctx, sql, args...)
	}
	return nil, fmt.Errorf("dead letter transaction not implemented")
}

func (a *deadLetterTxAdapter) Commit(ctx context.Context) error {
	return nil
}

func (a *deadLetterTxAdapter) Rollback(ctx context.Context) error {
	return nil
}