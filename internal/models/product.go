package models

import (
	"time"
)

type Product struct {
	ID          string    `json:"id"`
	ASIN        string    `json:"asin"`
	URL         string    `json:"url"`
	Title       string    `json:"title"`
	Brand       string    `json:"brand"`
	Category    string    `json:"category"`
	Material    string    `json:"material"`
	Dimensions  Dimension `json:"dimensions"`
	Weight      Weight    `json:"weight"`
	Price       Price     `json:"price"`
	Images      []string  `json:"images"`
	ScrapedAt   time.Time `json:"scraped_at"`
	LastUpdated time.Time `json:"last_updated"`
}

type Dimension struct {
	Length     float64 `json:"length"`
	Width      float64 `json:"width"`
	Height     float64 `json:"height"`
	Unit       string  `json:"unit"`
	PackageL   float64 `json:"package_length,omitempty"`
	PackageW   float64 `json:"package_width,omitempty"`
	PackageH   float64 `json:"package_height,omitempty"`
	PackageUnit string `json:"package_unit,omitempty"`
}

type Weight struct {
	Value       float64 `json:"value"`
	Unit        string  `json:"unit"`
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

type Error struct {
	Code    string    `json:"code"`
	Message string    `json:"message"`
	Time    time.Time `json:"time"`
	URL     string    `json:"url,omitempty"`
}

func NewProduct(asin string) *Product {
	now := time.Now()
	return &Product{
		ID:          asin,
		ASIN:        asin,
		ScrapedAt:   now,
		LastUpdated: now,
		Images:      make([]string, 0),
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