package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// ProductLifecycle represents a product in the lifecycle product table
type ProductLifecycle struct {
	ID                 uuid.UUID       `db:"id"`
	ASIN               string          `db:"asin"`
	Title              string          `db:"title"`
	Brand              string          `db:"brand"`
	DetailPageURL      string          `db:"detail_page_url"`
	ImageURLs          json.RawMessage `db:"image_urls"`
	Features           json.RawMessage `db:"features"`
	CurrentPrice       *float64        `db:"current_price"`
	Currency           string          `db:"currency"`
	Rating             *float64        `db:"rating"`
	ReviewCount        *int            `db:"review_count"`
	Status             string          `db:"status"`
	Category           string          `db:"category"`
	AvailableSizes     json.RawMessage `db:"available_sizes"`
	SizeTable          json.RawMessage `db:"size_table"`
	CreatedAt          time.Time       `db:"created_at"`
	UpdatedAt          time.Time       `db:"updated_at"`
}

// InsertProductLifecycle inserts a new product into the product table or updates if exists
func (db *DB) InsertProductLifecycle(ctx context.Context, p *ProductLifecycle) error {
	// Generate ID if not provided
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}

	query := `
		INSERT INTO products (
			asin, title, brand, url,
			category, status, size_table
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7
		)
		ON CONFLICT (asin) DO UPDATE SET
			title = EXCLUDED.title,
			brand = EXCLUDED.brand,
			url = EXCLUDED.url,
			category = EXCLUDED.category,
			size_table = EXCLUDED.size_table,
			status = EXCLUDED.status,
			updated_at = NOW()
		RETURNING asin, created_at, updated_at`

	err := db.pool.QueryRow(ctx, query,
		p.ASIN, p.Title, p.Brand, p.DetailPageURL,
		p.Category, p.Status, p.SizeTable,
	).Scan(&p.ASIN, &p.CreatedAt, &p.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to insert product lifecycle: %w", err)
	}

	return nil
}

// GetProductLifecycleByASIN retrieves a product from the product table by ASIN
func (db *DB) GetProductLifecycleByASIN(ctx context.Context, asin string) (*ProductLifecycle, error) {
	query := `
		SELECT 
			id, asin, title, brand, detail_page_url,
			image_urls, features, current_price, currency,
			rating, review_count, status, category,
			available_sizes, size_table, created_at, updated_at
		FROM products
		WHERE asin = $1`

	var p ProductLifecycle
	var imageURLs, features, availableSizes, sizeTable sql.NullString
	
	err := db.pool.QueryRow(ctx, query, asin).Scan(
		&p.ID, &p.ASIN, &p.Title, &p.Brand, &p.DetailPageURL,
		&imageURLs, &features, &p.CurrentPrice, &p.Currency,
		&p.Rating, &p.ReviewCount, &p.Status, &p.Category,
		&availableSizes, &sizeTable, &p.CreatedAt, &p.UpdatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
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

	return &p, nil
}

// UpdateProductLifecycleSizeTable updates the size table and status for a product
func (db *DB) UpdateProductLifecycleSizeTable(ctx context.Context, asin string, sizeTable *SizeTable) error {
	sizeTableJSON, err := json.Marshal(sizeTable)
	if err != nil {
		return fmt.Errorf("failed to marshal size table: %w", err)
	}

	query := `
		UPDATE products SET
			size_table = $2,
			status = 'SCRAPED',
			updated_at = NOW()
		WHERE asin = $1`

	result, err := db.pool.Exec(ctx, query, asin, sizeTableJSON)
	if err != nil {
		return fmt.Errorf("failed to update product size table: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("product not found: %s", asin)
	}

	return nil
}

// ValidateSizeTable checks if a size table has both length and chest measurements
func ValidateSizeTable(st *SizeTable) bool {
	if st == nil || len(st.Sizes) == 0 || len(st.Measurements) == 0 {
		return false
	}

	// Check that at least one size has both length and chest
	for _, measurements := range st.Measurements {
		if _, hasLength := measurements["length"]; !hasLength {
			continue
		}
		if _, hasChest := measurements["chest"]; !hasChest {
			continue
		}
		// Found at least one size with both length and chest
		return true
	}

	return false
}

// UpdateProductLifecycleWithFullData updates a product with complete scraped data
func (db *DB) UpdateProductLifecycleWithFullData(ctx context.Context, p *ProductLifecycle) error {
	query := `
		UPDATE products SET
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
			size_table = $14,
			updated_at = NOW()
		WHERE asin = $1`

	result, err := db.pool.Exec(ctx, query,
		p.ASIN, p.Title, p.Brand, p.DetailPageURL,
		p.ImageURLs, p.Features, p.CurrentPrice, p.Currency,
		p.Rating, p.ReviewCount, p.Status, p.Category,
		p.AvailableSizes, p.SizeTable,
	)

	if err != nil {
		return fmt.Errorf("failed to update product with full data: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("product not found: %s", p.ASIN)
	}

	return nil
}