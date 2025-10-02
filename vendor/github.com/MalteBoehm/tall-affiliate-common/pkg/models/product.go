// pkg/models/product.go
package models

import (
	"time"
)

// Product represents an Amazon product
type Product struct {
	ID                  string         `json:"id" db:"id"`
	ASIN                string         `json:"asin" db:"asin"`
	Title               string         `json:"title" db:"title"`
	Brand               string         `json:"brand,omitempty" db:"brand"`
	Description         string         `json:"description,omitempty" db:"description"`
	Features            []string       `json:"features,omitempty" db:"features"`
	Images              *ProductImages `json:"images,omitempty" db:"images"`
	CurrentPrice        float64        `json:"current_price,omitempty" db:"current_price"`
	Currency            string         `json:"currency,omitempty" db:"currency"`
	URL                 string         `json:"url,omitempty" db:"url"`
	Rating              float64        `json:"rating,omitempty" db:"rating"`
	RatingCount         int            `json:"rating_count,omitempty" db:"rating_count"`
	Available           bool           `json:"available" db:"available"`
	PrimaryBrowseNodeID string         `json:"primary_browse_node_id,omitempty" db:"primary_browse_node_id"`
	Category            string         `json:"category,omitempty" db:"category"`
	CategoryAttempts    int            `json:"category_attempts,omitempty" db:"category_attempts"`
	Gender              string         `json:"gender,omitempty" db:"gender"`
	Status              string         `json:"status,omitempty" db:"status"`
	ContentStatus       string         `json:"content_status,omitempty" db:"content_status"`
	ReviewsContent      string         `json:"reviews_content,omitempty" db:"reviews_content"`
	FAQContent          string         `json:"faq_content,omitempty" db:"faq_content"`

	// NEW FIELDS FOR TALL PEOPLE
	// Size & Color Information
	Size           string   `json:"size,omitempty" db:"size"`                       // "XL", "XXL", "Tall", "Long"
	Color          string   `json:"color,omitempty" db:"color"`                     // Color variants
	AvailableSizes []string `json:"available_sizes,omitempty" db:"available_sizes"` // Available sizes

	// Item Dimensions (CRITICAL for tall people)
	HeightCm float64 `json:"height_cm,omitempty" db:"height_cm"` // Physical height in cm
	LengthCm float64 `json:"length_cm,omitempty" db:"length_cm"` // Length in cm (critical for pants)
	WidthCm  float64 `json:"width_cm,omitempty" db:"width_cm"`   // Width in cm
	WeightG  float64 `json:"weight_g,omitempty" db:"weight_g"`   // Weight in grams

	// Product Classification
	ProductGroup   string `json:"product_group,omitempty" db:"product_group"`       // Amazon ProductGroup
	Model          string `json:"model,omitempty" db:"model"`                       // Model number
	IsAdultProduct bool   `json:"is_adult_product,omitempty" db:"is_adult_product"` // Adult product flag

	// Variation Attributes
	VariationAttributes []VariationAttribute `json:"variation_attributes,omitempty" db:"variation_attributes"`

	// Tall-Friendly Scoring
	TallFriendlyScore float64 `json:"tall_friendly_score,omitempty" db:"tall_friendly_score"` // Score (0.00-10.00)
	IsTallFriendly    bool    `json:"is_tall_friendly,omitempty" db:"is_tall_friendly"`       // Binary flag

	// Materials & Care & Media
	MaterialComposition map[string]float64 `json:"material_composition,omitempty" db:"material_composition"` // e.g., {"cotton": 100}
	CareInstructions    []string           `json:"care_instructions,omitempty" db:"care_instructions"`
	ImageVariants       []string           `json:"image_variants,omitempty" db:"image_variants"`

	// Timestamps
	CreatedAt       time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at" db:"updated_at"`
	LastCheckedAt   *time.Time `json:"last_checked_at,omitempty" db:"last_checked_at"`
	LastAvailableAt *time.Time `json:"last_available_at,omitempty" db:"last_available_at"`
}

// ProductImages represents the images for a product
type ProductImages struct {
	Primary  *ProductImage  `json:"primary,omitempty"`
	Variants []ProductImage `json:"variants,omitempty"`
}

// ProductImage represents a single product image
type ProductImage struct {
	URL    string `json:"url,omitempty"`
	Width  int    `json:"width,omitempty"`
	Height int    `json:"height,omitempty"`
}

// VariationAttribute represents a product variation attribute
type VariationAttribute struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// ProductStatus represents a product status value
type ProductStatus string

// ProductStatus constants
const (
	// Existing status values
	ProductStatusPending     ProductStatus = "pending"     // Produkt wartet auf Freigabe/Verarbeitung
	ProductStatusActive      ProductStatus = "active"      // Produkt ist aktiv und sichtbar
	ProductStatusUnavailable ProductStatus = "unavailable" // Legacy status - deprecated
	ProductStatusTerminated  ProductStatus = "terminated"  // Legacy status - deprecated

	// New status values based on database requirements
	ProductStatusUnsupportedCategory ProductStatus = "unsupported_category" // Category ist nicht von unserer Seite unterstützt. Wird blockiert
	ProductStatusPromoted            ProductStatus = "promoted"             // Produkt wird aktiv bevorzugt
	ProductStatusInactive            ProductStatus = "inactive"             // Produkt ist deaktiviert
	ProductStatusDraft               ProductStatus = "draft"                // Produkt ist im Entwurfsmodus
	ProductStatusDeleted             ProductStatus = "deleted"              // Produkt ist gelöscht (soft delete)
	ProductStatusOutOfStock          ProductStatus = "out_of_stock"         // Produkt ist nicht verfügbar
	ProductStatusDiscontinued        ProductStatus = "discontinued"         // Produkt wird nicht mehr geführt
)

// ContentStatus represents a content generation status value
type ContentStatus string

// ContentStatus constants
const (
	ContentStatusPending   ContentStatus = "pending"
	ContentStatusRequested ContentStatus = "requested"
	ContentStatusComplete  ContentStatus = "completed" // Changed from "complete" to "completed" for consistency
	ContentStatusFailed    ContentStatus = "failed"
)

// Gender constants
const (
	GenderMale   = "male"
	GenderFemale = "female"
	GenderUnisex = "unisex"
)
