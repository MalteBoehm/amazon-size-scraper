package events

import (
	"fmt"
	"time"

	"github.com/maltedev/tall-affiliate/event-contracts/cloudevents"
	"github.com/maltedev/tall-affiliate/event-contracts/validation"
)

// Event type constants - CloudEvents format
const (
	// Product enrichment events
	ProductEnrichmentRequestedV1 = "catalog.product_enrichment_requested.v1"
	ProductEnrichedV1            = "catalog.product_enriched.v1"
	ProductEnrichmentFailedV1    = "catalog.product_enrichment_failed.v1"
	
	// Product lifecycle events
	ProductDeletedV1             = "product.deleted.v1"
	ProductStatusChangedV1       = "product.status.changed.v1"
	ProductAvailabilityChangedV1 = "product.availability.changed.v1"
)

// Priority levels for enrichment requests
const (
	PriorityNormal = "normal"
	PriorityHigh   = "high"
)

// ProductEnrichmentRequestedData represents the data for a product enrichment request event
type ProductEnrichmentRequestedData struct {
	ASIN       string `json:"asin"`
	Region     string `json:"region"`
	PartnerTag string `json:"partner_tag,omitempty"`
	RequestID  string `json:"request_id"`
	Priority   string `json:"priority"`
	RetryCount int    `json:"retry_count"`
}

// Validate validates the enrichment request data
func (d *ProductEnrichmentRequestedData) Validate() error {
	if d.ASIN == "" {
		return fmt.Errorf("asin is required")
	}
	// Validate ASIN format
	if err := validation.ValidateASIN(d.ASIN); err != nil {
		return err
	}
	
	if d.Region == "" {
		return fmt.Errorf("region is required")
	}
	// Validate region code
	if err := validation.ValidateRegion(d.Region); err != nil {
		return err
	}
	
	if d.RequestID == "" {
		return fmt.Errorf("request_id is required")
	}
	if d.Priority != PriorityNormal && d.Priority != PriorityHigh {
		return fmt.Errorf("priority must be 'normal' or 'high'")
	}
	if d.RetryCount < 0 {
		return fmt.Errorf("retry_count cannot be negative")
	}
	return nil
}

// ColorVariant represents a single color variant of a product
type ColorVariant struct {
	ColorName string   `json:"color_name"`
	ASIN      string   `json:"asin"`
	Images    ImageSet `json:"images"`
}

// Validate validates the color variant
func (cv *ColorVariant) Validate() error {
	if cv.ColorName == "" {
		return fmt.Errorf("color_name is required")
	}
	if cv.ASIN == "" {
		return fmt.Errorf("asin is required")
	}
	// Validate ASIN format
	if err := validation.ValidateASIN(cv.ASIN); err != nil {
		return err
	}
	return cv.Images.Validate()
}

// ImageSet contains different image sizes for a product
type ImageSet struct {
	Primary  ImageUrls   `json:"primary"`
	Variants []ImageUrls `json:"variants"`
}

// Validate validates the image set
func (is *ImageSet) Validate() error {
	if err := is.Primary.Validate(); err != nil {
		return fmt.Errorf("primary images validation failed: %w", err)
	}
	for i, variant := range is.Variants {
		if err := variant.Validate(); err != nil {
			return fmt.Errorf("variant %d images validation failed: %w", i, err)
		}
	}
	return nil
}

// ImageUrls contains URLs for different image sizes
type ImageUrls struct {
	Tiny   string `json:"tiny"`
	Small  string `json:"small"`
	Medium string `json:"medium"`
	Large  string `json:"large"`
	HiRes  string `json:"hires"`
}

// Validate validates image URLs (at least one URL must be present)
func (iu *ImageUrls) Validate() error {
	if iu.Tiny == "" && iu.Small == "" && iu.Medium == "" && iu.Large == "" && iu.HiRes == "" {
		return fmt.Errorf("at least one image URL is required")
	}
	return nil
}

// ProductEnrichedData represents the data for a successful product enrichment event
type ProductEnrichedData struct {
	ASIN             string         `json:"asin"`
	Region           string         `json:"region"`
	ColorVariants    []ColorVariant `json:"color_variants"`
	EnrichmentSource string         `json:"enrichment_source"`
	RequestID        string         `json:"request_id"`
	ProcessingMS     int64          `json:"processing_ms"`
	EnrichedAt       time.Time      `json:"enriched_at"`
	VariantsCount    int            `json:"variants_count"`
}

// Validate validates the enriched product data
func (d *ProductEnrichedData) Validate() error {
	if d.ASIN == "" {
		return fmt.Errorf("asin is required")
	}
	// Validate ASIN format
	if err := validation.ValidateASIN(d.ASIN); err != nil {
		return err
	}
	
	if d.Region == "" {
		return fmt.Errorf("region is required")
	}
	// Validate region code
	if err := validation.ValidateRegion(d.Region); err != nil {
		return err
	}
	
	if d.EnrichmentSource == "" {
		return fmt.Errorf("enrichment_source is required")
	}
	if d.RequestID == "" {
		return fmt.Errorf("request_id is required")
	}
	for i, cv := range d.ColorVariants {
		if err := cv.Validate(); err != nil {
			return fmt.Errorf("color_variant %d validation failed: %w", i, err)
		}
	}
	return nil
}

// ProductEnrichmentFailedData represents the data for a failed product enrichment event
type ProductEnrichmentFailedData struct {
	ASIN         string    `json:"asin"`
	Region       string    `json:"region"`
	ErrorCode    string    `json:"error_code"`
	ErrorMessage string    `json:"error_message"`
	Retryable    bool      `json:"retryable"`
	RequestID    string    `json:"request_id"`
	FailedAt     time.Time `json:"failed_at"`
	RetryCount   int       `json:"retry_count"`
}

// Validate validates the failed enrichment data
func (d *ProductEnrichmentFailedData) Validate() error {
	if d.ASIN == "" {
		return fmt.Errorf("asin is required")
	}
	// Validate ASIN format
	if err := validation.ValidateASIN(d.ASIN); err != nil {
		return err
	}
	
	if d.Region == "" {
		return fmt.Errorf("region is required")
	}
	// Validate region code
	if err := validation.ValidateRegion(d.Region); err != nil {
		return err
	}
	
	if d.ErrorCode == "" {
		return fmt.Errorf("error_code is required")
	}
	if d.ErrorMessage == "" {
		return fmt.Errorf("error_message is required")
	}
	if d.RequestID == "" {
		return fmt.Errorf("request_id is required")
	}
	return nil
}

// Helper functions to create events

// NewProductEnrichmentRequestedEvent creates a new product enrichment requested event
func NewProductEnrichmentRequestedEvent(source string, data *ProductEnrichmentRequestedData) (*cloudevents.CloudEvent, error) {
	if err := data.Validate(); err != nil {
		return nil, fmt.Errorf("invalid request data: %w", err)
	}
	
	event := cloudevents.NewCloudEvent(ProductEnrichmentRequestedV1, source)
	event.Subject = fmt.Sprintf("asin:%s", data.ASIN)
	
	if err := event.SetData(data); err != nil {
		return nil, err
	}
	
	return event, nil
}

// NewProductEnrichedEvent creates a new product enriched event
func NewProductEnrichedEvent(source string, data *ProductEnrichedData) (*cloudevents.CloudEvent, error) {
	if err := data.Validate(); err != nil {
		return nil, fmt.Errorf("invalid enriched data: %w", err)
	}
	
	event := cloudevents.NewCloudEvent(ProductEnrichedV1, source)
	event.Subject = fmt.Sprintf("asin:%s", data.ASIN)
	
	if err := event.SetData(data); err != nil {
		return nil, err
	}
	
	return event, nil
}

// NewProductEnrichmentFailedEvent creates a new product enrichment failed event
func NewProductEnrichmentFailedEvent(source string, data *ProductEnrichmentFailedData) (*cloudevents.CloudEvent, error) {
	if err := data.Validate(); err != nil {
		return nil, fmt.Errorf("invalid failed data: %w", err)
	}
	
	event := cloudevents.NewCloudEvent(ProductEnrichmentFailedV1, source)
	event.Subject = fmt.Sprintf("asin:%s", data.ASIN)
	
	if err := event.SetData(data); err != nil {
		return nil, err
	}
	
	return event, nil
}

// Helper to calculate processing time
func CalculateProcessingTime(start time.Time) int64 {
	return time.Since(start).Milliseconds()
}