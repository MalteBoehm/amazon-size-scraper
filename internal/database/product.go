package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/maltedev/amazon-size-scraper/internal/models"
)

type ProductStatus string

const (
	StatusPending   ProductStatus = "pending"
	StatusCompleted ProductStatus = "completed"
	StatusFailed    ProductStatus = "failed"
)

type Product struct {
	ASIN         string          `db:"asin"`
	Title        string          `db:"title"`
	Brand        sql.NullString  `db:"brand"`
	Category     sql.NullString  `db:"category"`
	URL          string          `db:"detail_page_url"`
	SizeTable    json.RawMessage `db:"size_chart_data"`
	Status       ProductStatus   `db:"status"`
	ErrorMessage sql.NullString  `db:"error_message"`
	CreatedAt    time.Time       `db:"created_at"`
	UpdatedAt    time.Time       `db:"updated_at"`
}

type SizeTable struct {
	Sizes        []string                      `json:"sizes"`
	Measurements map[string]map[string]float64 `json:"measurements"`
	Unit         string                        `json:"unit"`
}

// InsertProduct inserts a new product or updates if exists
// Deprecated: Use InsertProductLifecycle for the new product table
func (db *DB) InsertProduct(ctx context.Context, p *Product) error {
	query := `
		INSERT INTO product (asin, title, brand, category, detail_page_url, status)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (asin) DO UPDATE SET
			title = EXCLUDED.title,
			brand = EXCLUDED.brand,
			category = EXCLUDED.category,
			detail_page_url = EXCLUDED.detail_page_url,
			updated_at = CURRENT_TIMESTAMP
		RETURNING created_at, updated_at`

	err := db.pool.QueryRow(ctx, query,
		p.ASIN, p.Title, p.Brand, p.Category, p.URL, p.Status,
	).Scan(&p.CreatedAt, &p.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to insert product: %w", err)
	}

	return nil
}

// UpdateProductSizes updates the size data for a product
// Deprecated: Use UpdateProductLifecycleSizeTable for the new product table
func (db *DB) UpdateProductSizes(ctx context.Context, asin string, sizeTable *SizeTable) error {
	sizeJSON, err := json.Marshal(sizeTable)
	if err != nil {
		return fmt.Errorf("failed to marshal size table: %w", err)
	}

	query := `
		UPDATE product SET
			size_chart_data = $2,
			status = $3,
			updated_at = CURRENT_TIMESTAMP
		WHERE asin = $1`

	_, err = db.pool.Exec(ctx, query,
		asin, sizeJSON, StatusCompleted,
	)

	if err != nil {
		return fmt.Errorf("failed to update product sizes: %w", err)
	}

	return nil
}

// UpdateProductColors updates the color variations for a product
// Deprecated: This method was used by the scraper's color extraction which has been removed.
// Use PA-API batch enricher instead which updates colors directly with parent_asin and enrichment_source.
func (db *DB) UpdateProductColors(ctx context.Context, asin string, colors []*models.VariationItem) error {
	// Convert colors to JSON
	colorsJSON, err := json.Marshal(colors)
	if err != nil {
		return fmt.Errorf("failed to marshal colors: %w", err)
	}

	// Try to update the products table first (if it exists)
	query := `
		UPDATE product SET
			colors = $2,
			updated_at = CURRENT_TIMESTAMP
		WHERE asin = $1`

	result, err := db.pool.Exec(ctx, query, asin, colorsJSON)
	if err != nil {
		// If products table doesn't have color_variations column, try product lifecycle table
		queryLifecycle := `
			UPDATE product SET
				colors = $2,
				enrichment_source = 'scraper',
				updated_at = CURRENT_TIMESTAMP
			WHERE asin = $1`

		result, err = db.pool.Exec(ctx, queryLifecycle, asin, colorsJSON)
		if err != nil {
			return fmt.Errorf("failed to update product colors: %w", err)
		}
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("no product found with ASIN: %s", asin)
	}

	return nil
}

// UpdateProductStatus updates the status and error message
// Deprecated: Use product lifecycle table methods instead
func (db *DB) UpdateProductStatus(ctx context.Context, asin string, status ProductStatus, errorMsg string) error {
	query := `
		UPDATE product SET
			status = $2,
			updated_at = CURRENT_TIMESTAMP
		WHERE asin = $1`

	_, err := db.pool.Exec(ctx, query, asin, status)
	if err != nil {
		return fmt.Errorf("failed to update product status: %w", err)
	}

	return nil
}

// GetPendingProducts returns products that need to be scraped
// Deprecated: Use product lifecycle table methods instead
func (db *DB) GetPendingProducts(ctx context.Context, limit int) ([]*Product, error) {
	query := `
		SELECT asin, title, brand, category, detail_page_url, status, created_at, updated_at
		FROM product
		WHERE status = $1
		ORDER BY created_at ASC
		LIMIT $2`

	rows, err := db.pool.Query(ctx, query, StatusPending, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query pending products: %w", err)
	}
	defer rows.Close()

	var products []*Product
	for rows.Next() {
		p := &Product{}
		err := rows.Scan(
			&p.ASIN, &p.Title, &p.Brand, &p.Category, &p.URL,
			&p.Status, &p.CreatedAt, &p.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan product: %w", err)
		}
		products = append(products, p)
	}

	return products, nil
}

// GetProduct retrieves a single product by ASIN
// Deprecated: Use GetProductLifecycleByASIN for the new product table
func (db *DB) GetProduct(ctx context.Context, asin string) (*Product, error) {
	query := `
		SELECT asin, title, brand, category, detail_page_url, size_chart_data,
			   status, created_at, updated_at
		FROM product
		WHERE asin = $1`

	p := &Product{}
	err := db.pool.QueryRow(ctx, query, asin).Scan(
		&p.ASIN, &p.Title, &p.Brand, &p.Category, &p.URL, &p.SizeTable,
		&p.Status, &p.CreatedAt, &p.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get product: %w", err)
	}

	return p, nil
}

// CountProductsByStatus returns count of products by status
func (db *DB) CountProductsByStatus(ctx context.Context) (map[ProductStatus]int, error) {
	query := `
		SELECT status, COUNT(*) as count
		FROM product
		GROUP BY status`

	rows, err := db.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to count products: %w", err)
	}
	defer rows.Close()

	counts := make(map[ProductStatus]int)
	for rows.Next() {
		var status ProductStatus
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return nil, fmt.Errorf("failed to scan count: %w", err)
		}
		counts[status] = count
	}

	return counts, nil
}
