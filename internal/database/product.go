package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
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
	URL          string          `db:"url"`
	SizeTable    json.RawMessage `db:"size_table"`
	Status       ProductStatus   `db:"status"`
	ErrorMessage sql.NullString  `db:"error_message"`
	ScrapedAt    sql.NullTime    `db:"scraped_at"`
	CreatedAt    time.Time       `db:"created_at"`
	UpdatedAt    time.Time       `db:"updated_at"`
}

type SizeTable struct {
	Sizes        []string                       `json:"sizes"`
	Measurements map[string]map[string]float64  `json:"measurements"`
	Unit         string                        `json:"unit"`
}

// InsertProduct inserts a new product or updates if exists
// Deprecated: Use InsertProductLifecycle for the new product table
func (db *DB) InsertProduct(ctx context.Context, p *Product) error {
	query := `
		INSERT INTO products (asin, title, brand, category, url, status)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (asin) DO UPDATE SET
			title = EXCLUDED.title,
			brand = EXCLUDED.brand,
			category = EXCLUDED.category,
			url = EXCLUDED.url,
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
		UPDATE products SET
			size_table = $2,
			status = $3,
			scraped_at = CURRENT_TIMESTAMP,
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

// UpdateProductMaterial updates the material data for a product
func (db *DB) UpdateProductMaterial(ctx context.Context, asin string, materialComposition *models.MaterialComposition, materialFullText string) error {
	var materialCompositionJSON []byte
	var err error

	if materialComposition != nil {
		materialCompositionJSON, err = json.Marshal(materialComposition)
		if err != nil {
			return fmt.Errorf("failed to marshal material composition: %w", err)
		}
	}

	query := `
		UPDATE products SET
			material_composition = $2,
			material_full_text = $3,
			updated_at = CURRENT_TIMESTAMP
		WHERE asin = $1`

	_, err = db.pool.Exec(ctx, query,
		asin, materialCompositionJSON, materialFullText,
	)

	if err != nil {
		return fmt.Errorf("failed to update product material: %w", err)
	}

	return nil
}

// UpdateProductWithMaterialAndSize updates both material and size data for a product
func (db *DB) UpdateProductWithMaterialAndSize(ctx context.Context, asin string, sizeTable *SizeTable, materialComposition *models.MaterialComposition, materialFullText string) error {
	var sizeJSON []byte
	var materialCompositionJSON []byte
	var err error

	if sizeTable != nil {
		sizeJSON, err = json.Marshal(sizeTable)
		if err != nil {
			return fmt.Errorf("failed to marshal size table: %w", err)
		}
	}

	if materialComposition != nil {
		materialCompositionJSON, err = json.Marshal(materialComposition)
		if err != nil {
			return fmt.Errorf("failed to marshal material composition: %w", err)
		}
	}

	query := `
		UPDATE products SET
			size_table = $2,
			material_composition = $3,
			material_full_text = $4,
			status = $5,
			scraped_at = CURRENT_TIMESTAMP,
			updated_at = CURRENT_TIMESTAMP
		WHERE asin = $1`

	_, err = db.pool.Exec(ctx, query,
		asin, sizeJSON, materialCompositionJSON, materialFullText, StatusCompleted,
	)

	if err != nil {
		return fmt.Errorf("failed to update product with material and size: %w", err)
	}

	return nil
}

// UpdateProductStatus updates the status and error message
// Deprecated: Use product lifecycle table methods instead
func (db *DB) UpdateProductStatus(ctx context.Context, asin string, status ProductStatus, errorMsg string) error {
	query := `
		UPDATE products SET
			status = $2,
			error_message = $3,
			updated_at = CURRENT_TIMESTAMP
		WHERE asin = $1`

	_, err := db.pool.Exec(ctx, query, asin, status, errorMsg)
	if err != nil {
		return fmt.Errorf("failed to update product status: %w", err)
	}

	return nil
}

// GetPendingProducts returns products that need to be scraped
// Deprecated: Use product lifecycle table methods instead
func (db *DB) GetPendingProducts(ctx context.Context, limit int) ([]*Product, error) {
	query := `
		SELECT asin, title, brand, category, url, status, created_at, updated_at
		FROM products
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
		SELECT asin, title, brand, category, url, size_table, 
			   status, error_message, scraped_at, created_at, updated_at
		FROM products
		WHERE asin = $1`

	p := &Product{}
	err := db.pool.QueryRow(ctx, query, asin).Scan(
		&p.ASIN, &p.Title, &p.Brand, &p.Category, &p.URL, &p.SizeTable,
		&p.Status, &p.ErrorMessage, &p.ScrapedAt, &p.CreatedAt, &p.UpdatedAt,
	)

	if err == pgx.ErrNoRows {
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
		FROM products
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