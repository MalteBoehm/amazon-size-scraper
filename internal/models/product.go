package models

import (
	"fmt"
	"time"
)

type Product struct {
	ID            string    `json:"id"`
	ASIN          string    `json:"asin"`
	URL           string    `json:"url"`
	Title         string    `json:"title"`
	Brand         string    `json:"brand"`
	Category      string    `json:"category"`
	Features      []string  `json:"features"`       // Features aus "Info zu diesem Artikel"
	ProductGroups []string  `json:"product_groups"` // Breadcrumb-Kategorien
	Dimensions    Dimension `json:"dimensions"`
	Weight        Weight    `json:"weight"`
	Price         Price     `json:"price"`
	Images        []string  `json:"images"`
	ScrapedAt     time.Time `json:"scraped_at"`
	LastUpdated   time.Time `json:"last_updated"`

	// Newly extracted commerce attributes (useful for mapping to central product table)
	Size                string               `json:"size,omitempty"`
	Color               string               `json:"color,omitempty"`
	AvailableSizes      []string             `json:"available_sizes,omitempty"`
	VariationAttributes []VariationAttribute `json:"variation_attributes,omitempty"`
	Model               string               `json:"model,omitempty"`
	MaterialComposition map[string]float64   `json:"material_composition,omitempty"`
	MaterialFullText    string               `json:"material_full_text,omitempty"`
	MaterialInfo        *MaterialInfo        `json:"material_info,omitempty"`
	CareInstructions    []string             `json:"care_instructions,omitempty"`
	
	// Enhanced color data from dropdown interaction
	ColorVariations     []VariationItem      `json:"color_variations,omitempty"`
	ParentASIN          string               `json:"parent_asin,omitempty"`
	EnrichmentSource    string               `json:"enrichment_source,omitempty"` // "scraper" or "pa-api"
	ColorsEnrichedAt    *time.Time           `json:"colors_enriched_at,omitempty"`

	// TODO: consider adding a field like 'Platform' or 'ScrapingSource' to distinguish
	// where the data came from (e.g., 'amazon', 'zalando', etc.). This will become important
	// once we support multiple marketplaces beyond Amazon.

}

// VariationAttribute represents a variation dimension like size or color
type VariationAttribute struct {
	Type   string           `json:"type"`
	Values []string         `json:"values"`
	// Enhanced color information with availability
	Items  []VariationItem  `json:"items,omitempty"`
}

// VariationItem represents a single variation option with availability
type VariationItem struct {
	Value       string `json:"value"`
	ASIN        string `json:"asin,omitempty"`        // Child ASIN for this variation
	Available   bool   `json:"available"`              // Whether this variation is available
	DisplayName string `json:"display_name,omitempty"` // Display name (e.g., "Rot", "Blau (Königsblau)")
}

// Validate checks if the VariationItem is valid
func (v *VariationItem) Validate() error {
	if v.Value == "" {
		return fmt.Errorf("variation value cannot be empty")
	}
	// ASIN is optional for inline twisters
	// DisplayName is optional (defaults to Value)
	return nil
}

type Dimension struct {
	Length      float64 `json:"length"`
	Width       float64 `json:"width"`
	Height      float64 `json:"height"`
	Unit        string  `json:"unit"`
	PackageL    float64 `json:"package_length,omitempty"`
	PackageW    float64 `json:"package_width,omitempty"`
	PackageH    float64 `json:"package_height,omitempty"`
	PackageUnit string  `json:"package_unit,omitempty"`
}

type Weight struct {
	Value         float64 `json:"value"`
	Unit          string  `json:"unit"`
	PackageWeight float64 `json:"package_weight,omitempty"`
	PackageUnit   string  `json:"package_unit,omitempty"`
}

type Price struct {
	Amount   float64 `json:"amount"`
	Currency string  `json:"currency"`
	Original float64 `json:"original,omitempty"`
	Discount float64 `json:"discount,omitempty"`
}

type ScrapeResult struct {
	Product *Product `json:"product,omitempty"`
	Error   *Error   `json:"error,omitempty"`
	Success bool     `json:"success"`
}

// MaterialInfo represents detailed material composition information
type MaterialInfo struct {
	Materials         map[string]float64 `json:"materials"`
	RawText           string             `json:"raw_text"`
	Confidence        float64            `json:"confidence"`
	Source            string             `json:"source"`
	ExtractionMethod  string             `json:"extraction_method"`
	AlternativeTexts  []string           `json:"alternative_texts,omitempty"`
}

type Error struct {
	Code    string    `json:"code"`
	Message string    `json:"message"`
	Time    time.Time `json:"time"`
	URL     string    `json:"url,omitempty"`
}

func NewProduct(asin string) *Product {
	now := time.Now()
	return &Product{
		ID:            asin,
		ASIN:          asin,
		ScrapedAt:     now,
		LastUpdated:   now,
		Images:        make([]string, 0),
		Features:      make([]string, 0),
		ProductGroups: make([]string, 0),
	}
}

func (d *Dimension) IsValid() bool {
	return d.Length > 0 && d.Width > 0 && d.Height > 0 && d.Unit != ""
}

func (w *Weight) IsValid() bool {
	return w.Value > 0 && w.Unit != ""
}

func (p *Price) IsValid() bool {
	return p.Amount >= 0 && p.Currency != ""
}

func (p *Product) Validate() []string {
	var errors []string

	if p.ASIN == "" {
		errors = append(errors, "ASIN is required")
	}

	if p.Title == "" {
		errors = append(errors, "Title is required")
	}

	if !p.Dimensions.IsValid() {
		errors = append(errors, "Invalid dimensions")
	}

	if !p.Weight.IsValid() {
		errors = append(errors, "Invalid weight")
	}

	return errors
}
