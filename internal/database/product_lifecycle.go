package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ProductLifecycle represents a product in the lifecycle product table
type ProductLifecycle struct {
	ID                  uuid.UUID       `db:"id"`
	ASIN                string          `db:"asin"`
	Title               string          `db:"title"`
	Brand               string          `db:"brand"`
	DetailPageURL       string          `db:"detail_page_url"`
	ImageURLs           json.RawMessage `db:"image_urls"`
	Features            json.RawMessage `db:"features"`
	CurrentPrice        *float64        `db:"current_price"`
	Currency            string          `db:"currency"`
	Rating              *float64        `db:"rating"`
	ReviewCount         *int            `db:"review_count"`
	Status              string          `db:"status"`
	Category            string          `db:"category"`
	AvailableSizes      json.RawMessage `db:"available_sizes"`
	VariationAttributes json.RawMessage `db:"variation_attributes"`
	Model               string          `db:"model"`
	MaterialComposition json.RawMessage `db:"material_composition"`
	MaterialFullText    string          `db:"material_full_text"`
	CareInstructions    json.RawMessage `db:"care_instructions"`
	SizeTable           json.RawMessage `db:"size_table"`
	IsPrimeEligible     bool            `db:"is_prime_eligible"`
	InStock             bool            `db:"in_stock"`
	Colors              json.RawMessage `db:"colors"`
	Material            string          `db:"material"`
	Gender              string          `db:"gender"`
	ProductGroups       json.RawMessage `db:"product_groups"`
	ReviewsContent      string          `db:"reviews_content"`
	CreatedAt           time.Time       `db:"created_at"`
	UpdatedAt           time.Time       `db:"updated_at"`
}

// InsertProductLifecycle inserts a new product into the product table or updates if exists
func (db *DB) InsertProductLifecycle(ctx context.Context, p *ProductLifecycle) error {
	// Generate ID if not provided
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}

	query := `
		INSERT INTO product (
			asin, title, brand, detail_page_url, category, status, size_chart_data,
			image_urls, features, current_price, currency, rating, review_count,
			available_sizes, gender, size, color, product_group, reviews_content,
			material_composition, material_full_text, care_instructions
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22
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
		RETURNING asin, created_at, updated_at`
	// Size table data is stored but dimensions are not extracted to separate fields anymore
	// All dimensional data stays in size_chart_data JSON for content generation

	// Use the first available size as the size field
	var sizeStr string
	if p.AvailableSizes != nil && len(p.AvailableSizes) > 2 { // At least "[]"
		var sizes []string
		if err := json.Unmarshal(p.AvailableSizes, &sizes); err == nil && len(sizes) > 0 {
			sizeStr = sizes[0]
		}
	}

	// Use the first available color as the color field
	var colorStr string
	if p.Colors != nil && len(p.Colors) > 2 { // At least "[]"
		var colors []string
		if err := json.Unmarshal(p.Colors, &colors); err == nil && len(colors) > 0 {
			colorStr = colors[0]
		}
	}

	// Extract product_group from ProductGroups JSON array (use first element)
	var productGroupStr string
	if p.ProductGroups != nil && len(p.ProductGroups) > 2 { // At least "[]"
		var productGroups []string
		if err := json.Unmarshal(p.ProductGroups, &productGroups); err == nil && len(productGroups) > 0 {
			// Join all groups with " > " to create a breadcrumb string
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

	err := db.pool.QueryRow(ctx, query,
		p.ASIN, p.Title, p.Brand, p.DetailPageURL, p.Category, p.Status, p.SizeTable,
		imageURLs, features, p.CurrentPrice, p.Currency, p.Rating, p.ReviewCount,
		availableSizes, p.Gender, sizeStr, colorStr, productGroupStr, p.ReviewsContent,
		materialComposition, p.MaterialFullText, careInstructions,
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
			available_sizes, size_chart_data, created_at, updated_at
		FROM product
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

	return &p, nil
}

// UpdateProductLifecycleSizeTable updates the size table and status for a product
func (db *DB) UpdateProductLifecycleSizeTable(ctx context.Context, asin string, sizeTable *SizeTable) error {
	sizeTableJSON, err := json.Marshal(sizeTable)
	if err != nil {
		return fmt.Errorf("failed to marshal size table: %w", err)
	}

	query := `
		UPDATE product SET
			size_chart_data = $2,
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

// isRealisticMeasurement checks if a measurement value is within realistic ranges
func isRealisticMeasurement(measurementType string, value float64) bool {
	// Define realistic ranges for different measurement types (in cm)
	switch measurementType {
	case "chest", "bust", "brustumfang":
		// Normal chest measurements: 70-150cm
		// Anything above 150cm is likely a parsing error
		return value >= 70 && value <= 150
	case "length", "länge":
		// Normal T-shirt/shirt length: 50-100cm
		// Anything above 100cm is likely a parsing error
		return value >= 50 && value <= 100
	case "shoulder", "schulter":
		// Normal shoulder width: 35-60cm
		return value >= 35 && value <= 60
	case "sleeve", "arm", "ärmel", "armlänge":
		// Normal sleeve length: 15-90cm
		return value >= 15 && value <= 90
	case "waist", "taille", "bund":
		// Normal waist: 60-150cm
		return value >= 60 && value <= 150
	case "hip", "hips", "hüfte":
		// Normal hips: 70-150cm
		return value >= 70 && value <= 150
	case "collar", "neck", "kragen":
		// Normal collar: 35-50cm
		return value >= 35 && value <= 50
	case "width", "breite":
		// Normal width (half chest): 40-80cm
		return value >= 40 && value <= 80
	case "height", "höhe":
		// This could be product height or body height, be more lenient
		return value >= 50 && value <= 250
	default:
		// For unknown measurement types, accept reasonable clothing ranges
		return value >= 10 && value <= 200
	}
}

// ValidateSizeTable checks if a size table has valid measurements
func ValidateSizeTable(st *SizeTable) bool {
	if st == nil || len(st.Sizes) == 0 || len(st.Measurements) == 0 {
		return false
	}

	// For clothing, we MUST have both chest/bust AND length measurements
	// This ensures we only process products suitable for tall people
	hasValidMeasurements := true

	for _, measurements := range st.Measurements {
		// Check if this size has required measurements
		hasChest := false
		hasLength := false
		hasRealisticValues := true

		for measurementType, value := range measurements {
			if value > 0 {
				// Check for unrealistic values first
				if !isRealisticMeasurement(measurementType, value) {
					hasRealisticValues = false
					break
				}
				
				// Check for chest/bust measurements
				if measurementType == "chest" || measurementType == "bust" {
					hasChest = true
				}
				// Check for length measurements (T-shirt length, not arm length)
				if measurementType == "length" {
					hasLength = true
				}
			}
		}

		// If any size has unrealistic values or is missing chest OR length, the table is invalid
		if !hasRealisticValues || !hasChest || !hasLength {
			hasValidMeasurements = false
			break
		}
	}

	return hasValidMeasurements
}

// ValidateSizeTableWithDetails checks if a size table has valid measurements and returns details
func ValidateSizeTableWithDetails(st *SizeTable) (bool, []string) {
	if st == nil || len(st.Sizes) == 0 || len(st.Measurements) == 0 {
		return false, []string{"no_size_table"}
	}

	hasAnyChest := false
	hasAnyLength := false
	unrealisticMeasurements := []string{}

	// Check all sizes to see what measurements we have
	for sizeName, measurements := range st.Measurements {
		for measurementType, value := range measurements {
			if value > 0 {
				// Check for unrealistic values
				if !isRealisticMeasurement(measurementType, value) {
					unrealisticMeasurements = append(unrealisticMeasurements, 
						fmt.Sprintf("unrealistic_%s_%.1fcm_for_size_%s", measurementType, value, sizeName))
				}
				
				if measurementType == "chest" || measurementType == "bust" {
					hasAnyChest = true
				}
				if measurementType == "length" {
					hasAnyLength = true
				}
			}
		}
	}

	// Build missing/invalid fields list
	missingFields := []string{}
	
	// First check for unrealistic measurements
	if len(unrealisticMeasurements) > 0 {
		missingFields = append(missingFields, unrealisticMeasurements...)
	}
	
	// Then check for missing required measurements
	if !hasAnyChest {
		missingFields = append(missingFields, "chest_measurements")
	}
	if !hasAnyLength {
		missingFields = append(missingFields, "length_measurements")
	}

	isValid := hasAnyChest && hasAnyLength && len(unrealisticMeasurements) == 0
	return isValid, missingFields
}

// UpdateProductLifecycleWithFullData updates a product with complete scraped data
func (db *DB) UpdateProductLifecycleWithFullData(ctx context.Context, p *ProductLifecycle) error {
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
			created_at = NOW(),
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

	result, err := db.pool.Exec(ctx, query,
		p.ASIN, p.Title, p.Brand, p.DetailPageURL,
		imageURLs, features, p.CurrentPrice, p.Currency,
		p.Rating, p.ReviewCount, p.Status, p.Category,
		availableSizes, p.SizeTable, materialComposition, p.MaterialFullText, careInstructions,
	)

	if err != nil {
		return fmt.Errorf("failed to update product with full data: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("product not found: %s", p.ASIN)
	}

	return nil
}
