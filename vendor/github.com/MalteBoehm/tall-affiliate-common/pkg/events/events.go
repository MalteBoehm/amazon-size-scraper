// pkg/events/events.go
package events

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Event types with numbered convention (Phase_Option_Name)
// Numbers indicate sequence, Letters indicate path (A=Success, B=Failure, C=Manual, D=Retry)
const (
	// Phase 1: Product Discovery (01-02)
	Event_01_ProductDetected        = "01_PRODUCT_DETECTED"
	Event_02A_ProductValidated      = "02A_PRODUCT_VALIDATED"
	Event_02B_ProductIgnored        = "02B_PRODUCT_IGNORED"
	Event_02C_ProductReviewRequired = "02C_PRODUCT_REVIEW_REQUIRED"

	// Phase 2: Enrichment Orchestration (03-05)
	Event_03_EnrichmentOrchestrationStarted = "03_ENRICHMENT_ORCHESTRATION_STARTED"
	// DEPRECATED: Event_04A_DimensionEnrichmentRequested removed - use CatalogProductEnrichmentRequestedV1 instead
	Event_04B_ColorEnrichmentRequested     = "04B_COLOR_ENRICHMENT_REQUESTED"
	// DEPRECATED: Event_04C_BrowseNodeRequested removed - use CatalogProductEnrichmentRequestedV1 instead
	Event_04D_VariantsEnrichmentRequested = "04D_VARIANTS_ENRICHMENT_REQUESTED"
	Event_05A_EnrichmentCompleted         = EnrichmentCompletedV1         // Migrated to CloudEvents
	Event_05B_EnrichmentFailed            = EnrichmentFailedV1            // Migrated to CloudEvents
	Event_05C_VariantsEnriched            = VariantsEnrichedV1            // Migrated to CloudEvents
	Event_05D_EnrichmentRetry             = EnrichmentRetryV1             // Migrated to CloudEvents

	// Phase 3: Quality Assessment (06-07)
	Event_06_QualityAssessmentRequested  = QualityAssessmentRequestedV1  // Migrated to CloudEvents
	Event_07A_QualityAssessmentCompleted = QualityAssessmentCompletedV1 // Migrated to CloudEvents
	Event_07B_QualityAssessmentFailed    = QualityAssessmentFailedV1    // Migrated to CloudEvents

	// Phase 4: Content & Reviews Generation (08-12)
	Event_08A_ContentGenerationRequested = "08A_CONTENT_GENERATION_REQUESTED"
	Event_08B_ReviewsRequested           = ReviewsRequestedV1           // Migrated to CloudEvents
	Event_09A_ContentGenerationStarted   = "09A_CONTENT_GENERATION_STARTED"
	Event_09B_ReviewsFetched             = ReviewsFetchedV1             // Migrated to CloudEvents
	Event_10A_ContentGenerated           = "10A_CONTENT_GENERATED"
	Event_10B_ContentGenerationFailed    = "10B_CONTENT_GENERATION_FAILED"
	Event_10C_ReviewsProcessed           = ReviewsProcessedV1           // Migrated to CloudEvents
	Event_10D_ContentGenerationRetried   = ContentGenerationRetriedV1   // Migrated to CloudEvents
	Event_11A_ReviewsValidated           = ReviewsValidatedV1           // Migrated to CloudEvents
	Event_11B_ReviewsFetchFailed         = ReviewsFetchFailedV1         // Migrated to CloudEvents
	Event_12A_ReviewsStored              = ReviewsStoredV1              // Migrated to CloudEvents
	Event_12B_ReviewsError               = ReviewsErrorV1               // Migrated to CloudEvents

	// Phase 5: Publication & Monitoring (13-14)
	Event_13_ProductReadyForPublication  = ProductReadyForPublicationV1  // Migrated to CloudEvents
	Event_14A_PriceMonitoringScheduled   = PriceMonitoringScheduledV1   // Migrated to CloudEvents
	Event_14B_AvailabilityCheckScheduled = AvailabilityCheckScheduledV1 // Migrated to CloudEvents
	Event_14C_PeriodicUpdateScheduled    = PeriodicUpdateScheduledV1    // Migrated to CloudEvents

	// Additional Events (15+)
	Event_15A_PriceUpdated              = PriceUpdatedV1              // Migrated to CloudEvents
	Event_15B_PriceUpdateFailed         = PriceUpdateFailedV1         // Migrated to CloudEvents
	Event_16A_ProductUpdated            = ProductUpdatedV1            // Migrated to CloudEvents
	Event_16B_ProductUpdateFailed       = ProductUpdateFailedV1       // Migrated to CloudEvents
	Event_17_ProductAvailabilityChanged = "17_PRODUCT_AVAILABILITY_CHANGED"
	Event_18_ProductStatusChanged       = ProductStatusChangedV1      // Migrated to CloudEvents
	Event_19_ProductDeleted             = ProductDeletedV1            // Migrated to CloudEvents

	// Legacy event names (deprecated - for backward compatibility)
	// DEPRECATED: Use numbered constants instead of plain strings:
	// - "PRODUCT_CREATED" → use Event_02A_ProductValidated or Event_08A_ContentGenerationRequested
	// - "PRODUCT_IGNORED" → use Event_02B_ProductIgnored
	// - "CONTENT_GENERATION_REQUESTED" → use Event_08A_ContentGenerationRequested
	// - "04D_VARIANTS_ENRICHMENT_REQUESTED" → use VariationEnrichmentRequested (04B_VARIATION_ENRICHMENT_REQUESTED)
	EventTypeNewProductDetected         = Event_01_ProductDetected
	EventTypeProductValidated           = Event_02A_ProductValidated
	EventTypeProductIgnored             = Event_02B_ProductIgnored
	EventTypeProductReviewRequired      = Event_02C_ProductReviewRequired
	EventTypeContentGenerationRequested = Event_08A_ContentGenerationRequested
	EventTypeContentGenerationStarted   = Event_09A_ContentGenerationStarted
	EventTypeContentGenerated           = Event_10A_ContentGenerated
	EventTypeContentGenerationFailed    = Event_10B_ContentGenerationFailed
	EventTypeContentGenerationRetried   = Event_10D_ContentGenerationRetried
	EventTypeReviewsRequested           = Event_08B_ReviewsRequested
	EventTypeReviewsFetched             = Event_09B_ReviewsFetched
	EventTypeReviewsProcessed           = Event_10C_ReviewsProcessed
	EventTypeReviewsValidated           = Event_11A_ReviewsValidated
	EventTypeReviewsFetchFailed         = Event_11B_ReviewsFetchFailed
	EventTypeReviewsStored              = Event_12A_ReviewsStored
	EventTypeReviewsError               = Event_12B_ReviewsError
	// DEPRECATED: Browse Node events removed - use CatalogProductEnrichment events instead
	EventTypeCheckPrice                 = "CHECK_PRICE"
	EventTypePriceUpdated               = Event_15A_PriceUpdated
	EventTypePriceUpdateFailed          = Event_15B_PriceUpdateFailed
	EventTypeProductUpdated             = Event_16A_ProductUpdated
	EventTypeProductAvailabilityChanged = Event_17_ProductAvailabilityChanged
	EventTypeProductStatusChanged       = Event_18_ProductStatusChanged
	EventTypeProductDeleted             = Event_19_ProductDeleted

	// Deprecated - to be removed
	EventTypeProductUnavailable     = "PRODUCT_UNAVAILABLE"
	EventTypeProductCreated         = "PRODUCT_CREATED"
	EventTypeProductUpdateRequested = "PRODUCT_UPDATE_REQUESTED"
	EventTypeContentUpdateRequested = "CONTENT_UPDATE_REQUESTED"
	EventTypeContentUpdated         = "CONTENT_UPDATED"
	EventTypeContentAnalysisFailed  = "CONTENT_ANALYSIS_FAILED"
	EventTypeReviewsCollected       = "REVIEWS_COLLECTED"
	EventTypeReviewsEnriched        = "REVIEWS_ENRICHED"
	EventTypeReviewsCached          = "REVIEWS_CACHED"
	EventTypeReviewsExpired         = "REVIEWS_EXPIRED"
	EventTypeReviewsDeleted         = "REVIEWS_DELETED"
	// DEPRECATED: Browse node detection removed - handled internally by services
)

// Legacy orchestration event names (mapped to new convention)
const (
	// DEPRECATED: Dimension Enrichment events removed - use CatalogProductEnrichment events instead

	// Quality Assessment
	QualityAssessmentRequested = Event_06_QualityAssessmentRequested
	QualityAssessmentCompleted = Event_07A_QualityAssessmentCompleted
	QualityAssessmentFailed    = Event_07B_QualityAssessmentFailed

	// Scheduling
	PriceMonitoringScheduled   = Event_14A_PriceMonitoringScheduled
	AvailabilityCheckScheduled = Event_14B_AvailabilityCheckScheduled
	PeriodicUpdateScheduled    = Event_14C_PeriodicUpdateScheduled

	// PA-API Color Enrichment
	ColorEnrichmentRequested     = Event_04B_ColorEnrichmentRequested
	ColorEnrichmentCompleted     = "05A_COLOR_ENRICHMENT_COMPLETED"
	ColorEnrichmentFailed        = "05B_COLOR_ENRICHMENT_FAILED"
	VariationEnrichmentRequested = "04B_VARIATION_ENRICHMENT_REQUESTED"
	VariationEnrichmentCompleted = "05A_VARIATION_ENRICHMENT_COMPLETED"
	VariationEnrichmentFailed    = "05B_VARIATION_ENRICHMENT_FAILED"

	// PA-API Variant Enrichment (single call for both color and image variants)
	VariantsEnrichmentRequested = Event_04D_VariantsEnrichmentRequested
	VariantsEnrichmentCompleted = Event_05C_VariantsEnriched
	VariantsEnrichmentFailed    = "05B_VARIANTS_ENRICHMENT_FAILED"

	// PA-API Enrichment Event Types (CloudEvent format)
	// DEPRECATED: Use catalog.* variants instead for consistency
	ProductEnrichmentRequestedV1  = "product.enrichment.requested.v1"
	ProductEnrichmentCompletedV1  = "product.enrichment.completed.v1"
	ProductEnrichmentFailedV1     = "product.enrichment.failed.v1"
	VariantsEnrichmentRequestedV1 = "product.variants.enrichment.requested.v1"
	VariantsEnrichmentCompletedV1 = "product.variants.enrichment.completed.v1"
	VariantsEnrichmentFailedV1    = "product.variants.enrichment.failed.v1"
)

// Canonical CloudEvents types (domain.subdomain.action.v1 format)
// Use these for new implementations; they follow CloudEvents naming conventions
const (
	// Product Discovery Events
	CatalogProductDetectedV1       = "catalog.product.detected.v1"
	CatalogProductValidatedV1      = "catalog.product.validated.v1"
	CatalogProductIgnoredV1        = "catalog.product.ignored.v1"
	CatalogProductReviewRequiredV1 = "catalog.product.review_required.v1"

	// Content Generation Events
	ContentGenerationRequestedV1 = "content.generation.requested.v1"
	ContentGenerationStartedV1   = "content.generation.started.v1"
	ContentGeneratedV1           = "content.generated.v1"
	ContentGenerationFailedV1    = "content.generation.failed.v1"
	ContentGenerationRetriedV1   = "content.generation.retried.v1"

	// Reviews Events
	ReviewsRequestedV1   = "reviews.requested.v1"
	ReviewsFetchedV1     = "reviews.fetched.v1"
	ReviewsProcessedV1   = "reviews.processed.v1"
	ReviewsValidatedV1   = "reviews.validated.v1"
	ReviewsFetchFailedV1 = "reviews.fetch_failed.v1"
	ReviewsStoredV1      = "reviews.stored.v1"
	ReviewsErrorV1       = "reviews.error.v1"

	// Product Enrichment Events (replaces 04A/04C flows)
	CatalogProductEnrichmentRequestedV1 = "catalog.product.enrichment.requested.v1"
	CatalogProductEnrichmentCompletedV1 = "catalog.product.enrichment.completed.v1"
	CatalogProductEnrichmentFailedV1    = "catalog.product.enrichment.failed.v1"
	
	// Enrichment Events (Legacy 05 series)
	EnrichmentCompletedV1 = "enrichment.completed.v1"
	EnrichmentFailedV1    = "enrichment.failed.v1"
	VariantsEnrichedV1    = "variants.enriched.v1"
	EnrichmentRetryV1     = "enrichment.retry.v1"

	// Quality Assessment Events
	QualityAssessmentRequestedV1 = "quality.assessment.requested.v1"
	QualityAssessmentCompletedV1 = "quality.assessment.completed.v1"
	QualityAssessmentFailedV1    = "quality.assessment.failed.v1"

	// Product Lifecycle Events
	ProductReadyForPublicationV1 = "product.ready_for_publication.v1"
	ProductUpdatedV1             = "product.updated.v1"
	ProductUpdateFailedV1        = "product.update_failed.v1"
	ProductAvailabilityChangedV1 = "product.availability.changed.v1"
	ProductStatusChangedV1       = "product.status.changed.v1"
	ProductDeletedV1             = "product.deleted.v1"

	// Price Monitoring Events
	PriceUpdatedV1               = "price.updated.v1"
	PriceUpdateFailedV1          = "price.update_failed.v1"
	PriceMonitoringScheduledV1   = "price.monitoring.scheduled.v1"
	AvailabilityCheckScheduledV1 = "product.availability_check.scheduled.v1"
	PeriodicUpdateScheduledV1    = "product.periodic_update.scheduled.v1"
)

// Event represents a domain event
type Event struct {
	ID            string         `json:"id"`
	Type          string         `json:"type"`
	AggregateType string         `json:"aggregate_type"`
	AggregateID   string         `json:"aggregate_id"`
	Payload       any            `json:"payload"`
	Timestamp     time.Time      `json:"timestamp"`
	Metadata      map[string]any `json:"metadata,omitempty"`
}

// ProductCreatedPayload represents the payload for a PRODUCT_CREATED event
type ProductCreatedPayload struct {
	ASIN           string   `json:"asin"`
	Title          string   `json:"title"`
	Brand          string   `json:"brand,omitempty"`
	Category       string   `json:"category,omitempty"`
	Gender         string   `json:"gender,omitempty"`
	CurrentPrice   float64  `json:"current_price,omitempty"`
	Currency       string   `json:"currency,omitempty"`
	DetailPageURL  string   `json:"detail_page_url,omitempty"`
	ImageUrls      []string `json:"image_urls,omitempty"`
	Features       []string `json:"features,omitempty"`
	BrowseNodeID   string   `json:"browse_node_id,omitempty"`
	BrowseNodeTags []string `json:"browse_node_tags,omitempty"`
}

// ProductUpdatedPayload represents the payload for a PRODUCT_UPDATED event
type ProductUpdatedPayload struct {
	ASIN      string  `json:"asin"`
	Title     string  `json:"title,omitempty"`
	Brand     string  `json:"brand,omitempty"`
	Price     float64 `json:"price,omitempty"`
	Currency  string  `json:"currency,omitempty"`
	Available bool    `json:"available"`
}

// ContentGenerationRequestedPayload represents the payload for a CONTENT_GENERATION_REQUESTED event
type ContentGenerationRequestedPayload struct {
	ASIN        string    `json:"asin"`
	ProductID   string    `json:"product_id"`
	Priority    int       `json:"priority,omitempty"` // 1-5, wobei 5 die höchste Priorität ist
	RequestedAt time.Time `json:"requested_at"`
}

// DEPRECATED: BrowseNode payload types removed
// Use ProductEnrichmentRequestedData, ProductEnrichedData, and ProductEnrichmentFailedData instead

// ContentGeneratedPayload represents the payload for a CONTENT_GENERATED event
type ContentGeneratedPayload struct {
	ASIN           string    `json:"asin"`
	ProductID      string    `json:"product_id"`
	ContentType    string    `json:"content_type"` // z.B. "description", "faq"
	ContentID      string    `json:"content_id,omitempty"`
	GeneratedAt    time.Time `json:"generated_at"`
	ReviewsContent string    `json:"reviews_content,omitempty"`
	FAQContent     string    `json:"faq_content,omitempty"`
	Gender         string    `json:"gender,omitempty"`
}

// ContentGenerationFailedPayload represents the payload for a CONTENT_GENERATION_FAILED event
type ContentGenerationFailedPayload struct {
	ASIN      string    `json:"asin"`
	ProductID string    `json:"product_id"`
	Reason    string    `json:"reason"`
	FailedAt  time.Time `json:"failed_at"`
}

// ReviewsCollectedPayload represents the payload for a REVIEWS_COLLECTED event
type ReviewsCollectedPayload struct {
	ASIN        string    `json:"asin"`
	ProductID   string    `json:"product_id"`
	ReviewCount int       `json:"review_count"`
	CollectedAt time.Time `json:"collected_at"`
}

// NewProductDetectedPayload represents the payload for a NEW_PRODUCT_DETECTED event
type NewProductDetectedPayload struct {
	ASIN           string                 `json:"asin"`
	Title          string                 `json:"title"`
	Brand          string                 `json:"brand,omitempty"`
	Category       string                 `json:"category,omitempty"`
	Gender         string                 `json:"gender,omitempty"`
	CurrentPrice   float64                `json:"current_price,omitempty"`
	Currency       string                 `json:"currency,omitempty"`
	DetailPageURL  string                 `json:"detail_page_url,omitempty"`
	ImageUrls      []string               `json:"image_urls,omitempty"`
	Features       []string               `json:"features,omitempty"`
	BrowseNodeID   string                 `json:"browse_node_id,omitempty"`
	BrowseNodeTags []string               `json:"browse_node_tags,omitempty"` // Browse node IDs for categorization
	AmazonData     map[string]interface{} `json:"amazon_data,omitempty"`      // Complete Amazon product data

	// ENHANCED FILTER FIELDS FOR TALL PEOPLE (CRITICAL for validation)
	// Size & Color Information
	Size           string   `json:"size,omitempty"`            // "XL", "XXL", "Tall", "Long"
	Color          string   `json:"color,omitempty"`           // Color variants
	AvailableSizes []string `json:"available_sizes,omitempty"` // Available sizes

	// Item Dimensions (CRITICAL for tall people)
	HeightCm *float64 `json:"height_cm,omitempty"` // Physical height in cm
	LengthCm *float64 `json:"length_cm,omitempty"` // Length in cm (critical for pants)
	WidthCm  *float64 `json:"width_cm,omitempty"`  // Width in cm
	WeightG  *float64 `json:"weight_g,omitempty"`  // Weight in grams

	// Product Classification
	ProductGroup   string `json:"product_group,omitempty"`    // Amazon ProductGroup
	Model          string `json:"model,omitempty"`            // Product model
	IsAdultProduct *bool  `json:"is_adult_product,omitempty"` // Adult product flag

	// Variation Attributes
	VariationAttributes []map[string]string `json:"variation_attributes,omitempty"` // Size, Color variations

	// Tall-Friendly Scoring
	TallFriendlyScore *float64 `json:"tall_friendly_score,omitempty"` // Pre-calculated score
	IsTallFriendly    *bool    `json:"is_tall_friendly,omitempty"`    // Pre-calculated flag

	// Quality Assessment Fields (CRITICAL for validation)
	Rating      *float64 `json:"rating,omitempty"`       // Product rating (1-5 stars)
	ReviewCount *int64   `json:"review_count,omitempty"` // Number of reviews

	// Dimension Data from Amazon Scraper
	Dimensions    map[string]interface{} `json:"dimensions,omitempty"`     // Size chart dimensions from scraper
	HasDimensions bool                   `json:"has_dimensions,omitempty"` // Whether dimensions were found
}

// ProductIgnoredPayload represents the payload for a PRODUCT_IGNORED event
type ProductIgnoredPayload struct {
	ASIN          string    `json:"asin"`
	ProductID     string    `json:"product_id"`
	Reason        string    `json:"reason"` // e.g., "no_size_information", "no_reviews", "no_images", "price_out_of_range"
	MissingFields []string  `json:"missing_fields,omitempty"`
	IgnoredAt     time.Time `json:"ignored_at"`
}

// ProductReviewRequiredPayload represents the payload for a PRODUCT_REVIEW_REQUIRED event
type ProductReviewRequiredPayload struct {
	ASIN            string    `json:"asin"`
	ProductID       string    `json:"product_id"`
	Reason          string    `json:"reason"`           // e.g., "borderline_quality", "size_information_unclear", "low_rating"
	SuggestedAction string    `json:"suggested_action"` // e.g., "manual_size_check", "quality_review"
	Score           float64   `json:"score,omitempty"`  // Quality score if applicable
	RequiredAt      time.Time `json:"required_at"`
}

// DEPRECATED: DimensionEnrichment payload types removed
// Use ProductEnrichmentRequestedData, ProductEnrichedData, and ProductEnrichmentFailedData instead

// QualityAssessmentRequestedPayload represents the payload for quality assessment request
type QualityAssessmentRequestedPayload struct {
	ASIN        string                 `json:"asin"`
	ProductID   string                 `json:"product_id"`
	ProductData map[string]interface{} `json:"product_data"` // All product data including dimensions
	RequestedAt time.Time              `json:"requested_at"`
}

// QualityAssessmentCompletedPayload represents the payload for completed quality assessment
type QualityAssessmentCompletedPayload struct {
	ASIN            string    `json:"asin"`
	ProductID       string    `json:"product_id"`
	QualityScore    float64   `json:"quality_score"`
	Status          string    `json:"status"` // "validated", "ignored", "review_required"
	Reason          string    `json:"reason,omitempty"`
	MissingFields   []string  `json:"missing_fields,omitempty"`
	SuggestedAction string    `json:"suggested_action,omitempty"`
	CompletedAt     time.Time `json:"completed_at"`
}

// QualityAssessmentFailedPayload represents the payload for failed quality assessment
type QualityAssessmentFailedPayload struct {
	ASIN      string    `json:"asin"`
	ProductID string    `json:"product_id"`
	Reason    string    `json:"reason"`
	FailedAt  time.Time `json:"failed_at"`
}

// ColorEnrichmentRequestedPayload represents the payload for PA-API color enrichment request
type ColorEnrichmentRequestedPayload struct {
	ASIN        string    `json:"asin"`
	ProductID   string    `json:"product_id"`
	Title       string    `json:"title,omitempty"`
	RequestedAt time.Time `json:"requested_at"`
}

// ColorEnrichmentCompletedPayload represents the payload for successful color enrichment
type ColorEnrichmentCompletedPayload struct {
	ASIN             string                   `json:"asin"`
	ProductID        string                   `json:"product_id"`
	ColorVariations  []map[string]interface{} `json:"color_variations"`
	ParentASIN       string                   `json:"parent_asin,omitempty"`
	EnrichmentSource string                   `json:"enrichment_source"` // "pa-api"
	CompletedAt      time.Time                `json:"completed_at"`
}

// ColorEnrichmentFailedPayload represents the payload for failed color enrichment
type ColorEnrichmentFailedPayload struct {
	ASIN      string    `json:"asin"`
	ProductID string    `json:"product_id"`
	Reason    string    `json:"reason"`
	FailedAt  time.Time `json:"failed_at"`
}

// VariationEnrichmentRequestedPayload for getting all product variations
type VariationEnrichmentRequestedPayload struct {
	ASIN        string    `json:"asin"`
	ParentASIN  string    `json:"parent_asin,omitempty"`
	ProductID   string    `json:"product_id"`
	RequestedAt time.Time `json:"requested_at"`
}

// VariationEnrichmentCompletedPayload for successful variation data
type VariationEnrichmentCompletedPayload struct {
	ASIN        string                   `json:"asin"`
	ProductID   string                   `json:"product_id"`
	Variations  []map[string]interface{} `json:"variations"`
	CompletedAt time.Time                `json:"completed_at"`
}

// VariationEnrichmentFailedPayload for failed variation enrichment
type VariationEnrichmentFailedPayload struct {
	ASIN      string    `json:"asin"`
	ProductID string    `json:"product_id"`
	Reason    string    `json:"reason"`
	FailedAt  time.Time `json:"failed_at"`
}

// ColorVariant represents a product color variation
type ColorVariant struct {
	ColorName string     `json:"color_name"`
	ASIN      string     `json:"asin"`
	Images    []ImageSet `json:"images"`
}

// ImageSet represents product images at different sizes
type ImageSet struct {
	Small  string `json:"small"`
	Medium string `json:"medium"`
	Large  string `json:"large"`
}

// ProductEnrichmentRequestedData represents a PA-API enrichment request
type ProductEnrichmentRequestedData struct {
	ASIN       string `json:"asin"`
	Region     string `json:"region"`
	RequestID  string `json:"request_id"`
	RetryCount int    `json:"retry_count"`
}

func (p *ProductEnrichmentRequestedData) Validate() error {
	if p.ASIN == "" {
		return fmt.Errorf("ASIN is required")
	}
	if p.Region == "" {
		return fmt.Errorf("region is required")
	}
	if p.RequestID == "" {
		return fmt.Errorf("request_id is required")
	}
	return nil
}

// ProductEnrichedData represents successful PA-API enrichment
type ProductEnrichedData struct {
	ASIN          string         `json:"asin"`
	Region        string         `json:"region"`
	RequestID     string         `json:"request_id"`
	ColorVariants []ColorVariant `json:"color_variants"`
	ProcessingMS  int64          `json:"processing_ms"`
	EnrichedAt    time.Time      `json:"enriched_at"`
}

// ProductEnrichmentFailedData represents failed PA-API enrichment
type ProductEnrichmentFailedData struct {
	ASIN         string    `json:"asin"`
	Region       string    `json:"region"`
	RequestID    string    `json:"request_id"`
	ErrorCode    string    `json:"error_code"`
	ErrorMessage string    `json:"error_message"`
	FailedAt     time.Time `json:"failed_at"`
	RetryCount   int       `json:"retry_count"`
}

// PriceMonitoringScheduledPayload represents the payload for scheduled price monitoring
type PriceMonitoringScheduledPayload struct {
	ASIN        string    `json:"asin"`
	ProductID   string    `json:"product_id"`
	ScheduledAt time.Time `json:"scheduled_at"`
	NextCheckAt time.Time `json:"next_check_at"`
}

// AvailabilityCheckScheduledPayload represents the payload for scheduled availability check
type AvailabilityCheckScheduledPayload struct {
	ASIN        string    `json:"asin"`
	ProductID   string    `json:"product_id"`
	ScheduledAt time.Time `json:"scheduled_at"`
	NextCheckAt time.Time `json:"next_check_at"`
}

// PeriodicUpdateScheduledPayload represents the payload for scheduled periodic updates
type PeriodicUpdateScheduledPayload struct {
	ASIN        string    `json:"asin"`
	ProductID   string    `json:"product_id"`
	UpdateType  string    `json:"update_type"` // "full", "price_only", "availability_only"
	ScheduledAt time.Time `json:"scheduled_at"`
	NextCheckAt time.Time `json:"next_check_at"`
}

// Helper functions for event creation
func NewEvent(eventType, aggregateType, aggregateID string, payload any) (*Event, error) {
	return &Event{
		ID:            uuid.New().String(),
		Type:          eventType,
		AggregateType: aggregateType,
		AggregateID:   aggregateID,
		Payload:       payload,
		Timestamp:     time.Now().UTC(),
		Metadata:      make(map[string]any),
	}, nil
}

// UnmarshalPayload unmarshals the event payload into the given value
func (e *Event) UnmarshalPayload(v any) error {
	payloadBytes, err := json.Marshal(e.Payload)
	if err != nil {
		return err
	}
	return json.Unmarshal(payloadBytes, v)
}

// Reviews-related constants and types
const (
	ReviewsSourceAmazonAPI = "amazon_api"
	ReviewsSourceManual    = "manual"
)

const (
	ReviewsErrorTypeFetch      = "fetch_error"
	ReviewsErrorTypeProcess    = "process_error"
	ReviewsErrorTypeValidation = "validation_error"
	ReviewsErrorTypeCache      = "cache_error"
	ReviewsErrorTypeDatabase   = "database_error"
)

// Reviews payload structures
type ReviewsRequestedPayload struct {
	ASIN      string         `json:"asin"`
	ProductID string         `json:"product_id"`
	Source    string         `json:"source"`
	Options   map[string]any `json:"options,omitempty"`
}

type ReviewsErrorPayload struct {
	ASIN         string `json:"asin"`
	ProductID    string `json:"product_id"`
	Source       string `json:"source"`
	ErrorType    string `json:"error_type"`
	ErrorMessage string `json:"error_message"`
	RetryCount   int    `json:"retry_count"`
}

// Reviews helper functions
func NewReviewsRequestedEvent(asin, productID, source string, options map[string]any) *Event {
	payload := ReviewsRequestedPayload{
		ASIN:      asin,
		ProductID: productID,
		Source:    source,
		Options:   options,
	}

	event, _ := NewEvent(EventTypeReviewsRequested, "reviews", productID, payload)
	return event
}

func NewReviewsErrorEvent(asin, productID, source, errorType, errorMessage string, retryCount int) *Event {
	payload := ReviewsErrorPayload{
		ASIN:         asin,
		ProductID:    productID,
		Source:       source,
		ErrorType:    errorType,
		ErrorMessage: errorMessage,
		RetryCount:   retryCount,
	}

	event, _ := NewEvent(EventTypeReviewsError, "reviews", productID, payload)
	return event
}

func IsReviewsEvent(eventType string) bool {
	reviewsEvents := []string{
		EventTypeReviewsRequested,
		EventTypeReviewsFetched,
		EventTypeReviewsStored,
		EventTypeReviewsCollected,
		EventTypeReviewsFetchFailed,
		EventTypeReviewsProcessed,
		EventTypeReviewsValidated,
		EventTypeReviewsEnriched,
		EventTypeReviewsCached,
		EventTypeReviewsExpired,
		EventTypeReviewsError,
		EventTypeReviewsDeleted,
	}

	for _, re := range reviewsEvents {
		if re == eventType {
			return true
		}
	}
	return false
}

func GetReviewsEventPriority(eventType string) int {
	priorities := map[string]int{
		EventTypeReviewsError:       1, // Highest priority
		EventTypeReviewsRequested:   2,
		EventTypeReviewsFetched:     3,
		EventTypeReviewsStored:      4,
		EventTypeReviewsProcessed:   5,
		EventTypeReviewsValidated:   6,
		EventTypeReviewsEnriched:    7,
		EventTypeReviewsCached:      8,
		EventTypeReviewsExpired:     9,
		EventTypeReviewsDeleted:     10, // Lowest priority
		EventTypeReviewsCollected:   5,
		EventTypeReviewsFetchFailed: 2,
	}

	if priority, exists := priorities[eventType]; exists {
		return priority
	}
	return 999 // Unknown event
}

// Helper functions for orchestration events

// DEPRECATED: Dimension Enrichment constructor functions removed
// Use PA-API enrichment events instead:
// - NewProductEnrichmentRequestedEvent for requesting enrichment
// - NewProductEnrichedEvent for completed enrichment
// - NewProductEnrichmentFailedEvent for failed enrichment
// See MIGRATION_DIMENSION_ENRICHMENT_REMOVAL.md for migration guide

// NewQualityAssessmentRequestedEvent creates a new quality assessment requested event
func NewQualityAssessmentRequestedEvent(asin, productID string, productData map[string]interface{}) *Event {
	payload := QualityAssessmentRequestedPayload{
		ASIN:        asin,
		ProductID:   productID,
		ProductData: productData,
		RequestedAt: time.Now().UTC(),
	}

	event, _ := NewEvent(QualityAssessmentRequested, "product", productID, payload)
	return event
}

// NewQualityAssessmentCompletedEvent creates a new quality assessment completed event
func NewQualityAssessmentCompletedEvent(asin, productID string, score float64, status, reason string) *Event {
	payload := QualityAssessmentCompletedPayload{
		ASIN:         asin,
		ProductID:    productID,
		QualityScore: score,
		Status:       status,
		Reason:       reason,
		CompletedAt:  time.Now().UTC(),
	}

	event, _ := NewEvent(QualityAssessmentCompleted, "product", productID, payload)
	return event
}

// NewQualityAssessmentFailedEvent creates a new quality assessment failed event
func NewQualityAssessmentFailedEvent(asin, productID, reason string) *Event {
	payload := QualityAssessmentFailedPayload{
		ASIN:      asin,
		ProductID: productID,
		Reason:    reason,
		FailedAt:  time.Now().UTC(),
	}

	event, _ := NewEvent(QualityAssessmentFailed, "product", productID, payload)
	return event
}

// NewColorEnrichmentRequestedEvent creates a new color enrichment requested event
func NewColorEnrichmentRequestedEvent(asin, productID, title string) *Event {
	payload := ColorEnrichmentRequestedPayload{
		ASIN:        asin,
		ProductID:   productID,
		Title:       title,
		RequestedAt: time.Now().UTC(),
	}

	event, _ := NewEvent(ColorEnrichmentRequested, "product", productID, payload)
	return event
}

// NewColorEnrichmentCompletedEvent creates a new color enrichment completed event
func NewColorEnrichmentCompletedEvent(asin, productID string, colorVariations []map[string]interface{}, parentASIN string) *Event {
	payload := ColorEnrichmentCompletedPayload{
		ASIN:             asin,
		ProductID:        productID,
		ColorVariations:  colorVariations,
		ParentASIN:       parentASIN,
		EnrichmentSource: "pa-api",
		CompletedAt:      time.Now().UTC(),
	}

	event, _ := NewEvent(ColorEnrichmentCompleted, "product", productID, payload)
	return event
}

// NewColorEnrichmentFailedEvent creates a new color enrichment failed event
func NewColorEnrichmentFailedEvent(asin, productID, reason string) *Event {
	payload := ColorEnrichmentFailedPayload{
		ASIN:      asin,
		ProductID: productID,
		Reason:    reason,
		FailedAt:  time.Now().UTC(),
	}

	event, _ := NewEvent(ColorEnrichmentFailed, "product", productID, payload)
	return event
}

// NewPriceMonitoringScheduledEvent creates a new price monitoring scheduled event
func NewPriceMonitoringScheduledEvent(asin, productID string, nextCheckAt time.Time) *Event {
	payload := PriceMonitoringScheduledPayload{
		ASIN:        asin,
		ProductID:   productID,
		ScheduledAt: time.Now().UTC(),
		NextCheckAt: nextCheckAt,
	}

	event, _ := NewEvent(PriceMonitoringScheduled, "product", productID, payload)
	return event
}

// NewAvailabilityCheckScheduledEvent creates a new availability check scheduled event
func NewAvailabilityCheckScheduledEvent(asin, productID string, nextCheckAt time.Time) *Event {
	payload := AvailabilityCheckScheduledPayload{
		ASIN:        asin,
		ProductID:   productID,
		ScheduledAt: time.Now().UTC(),
		NextCheckAt: nextCheckAt,
	}

	event, _ := NewEvent(AvailabilityCheckScheduled, "product", productID, payload)
	return event
}

// NewPeriodicUpdateScheduledEvent creates a new periodic update scheduled event
func NewPeriodicUpdateScheduledEvent(asin, productID, updateType string, nextCheckAt time.Time) *Event {
	payload := PeriodicUpdateScheduledPayload{
		ASIN:        asin,
		ProductID:   productID,
		UpdateType:  updateType,
		ScheduledAt: time.Now().UTC(),
		NextCheckAt: nextCheckAt,
	}

	event, _ := NewEvent(PeriodicUpdateScheduled, "product", productID, payload)
	return event
}

// NewProductIgnoredEvent creates a new product ignored event
func NewProductIgnoredEvent(asin, reason string) *Event {
	payload := ProductIgnoredPayload{
		ASIN:      asin,
		ProductID: asin, // Use ASIN as product ID for ignored products
		Reason:    reason,
		IgnoredAt: time.Now().UTC(),
	}

	event, _ := NewEvent("PRODUCT_IGNORED", "product", asin, payload)
	return event
}

// NewProductEnrichmentRequestedEvent creates a new PA-API enrichment request event
func NewProductEnrichmentRequestedEvent(source string, data *ProductEnrichmentRequestedData) (*Event, error) {
	if err := data.Validate(); err != nil {
		return nil, err
	}
	return NewEvent(ProductEnrichmentRequestedV1, "product", data.ASIN, data)
}

// NewProductEnrichedEvent creates a new PA-API enrichment success event
func NewProductEnrichedEvent(source string, data *ProductEnrichedData) (*Event, error) {
	return NewEvent(ProductEnrichmentCompletedV1, "product", data.ASIN, data)
}

// NormalizeEventType maps legacy/ad-hoc event strings to canonical numbered constants.
// Returns (canonical_event, true) if mapping exists, otherwise (original_string, false).
// This helper allows services to centrally handle legacy event names during migration.
func NormalizeEventType(s string) (string, bool) {
	switch s {
	case "CONTENT_GENERATION_REQUESTED":
		return Event_08A_ContentGenerationRequested, true
	case "CONTENT_GENERATED":
		return Event_10A_ContentGenerated, true
	case "CONTENT_GENERATION_FAILED":
		return Event_10B_ContentGenerationFailed, true
	case "PRODUCT_VALIDATED":
		return Event_02A_ProductValidated, true
	case "PRODUCT_IGNORED":
		return Event_02B_ProductIgnored, true
	case "PRODUCT_REVIEW_REQUIRED":
		return Event_02C_ProductReviewRequired, true
	case "04D_VARIANTS_ENRICHMENT_REQUESTED":
		return VariationEnrichmentRequested, true
	case "ENRICHMENT_RETRY":
		return Event_05D_EnrichmentRetry, true
	case "QUALITY_ASSESSMENT_REQUESTED":
		return Event_06_QualityAssessmentRequested, true
	case "QUALITY_ASSESSMENT_COMPLETED":
		return Event_07A_QualityAssessmentCompleted, true
	case "QUALITY_ASSESSMENT_FAILED":
		return Event_07B_QualityAssessmentFailed, true
	// Product lifecycle events - CloudEvents format mapping
	case "product.deleted.v1":
		return Event_19_ProductDeleted, true
	case "product.status.changed.v1":
		return Event_18_ProductStatusChanged, true
	case "product.availability.changed.v1":
		return Event_17_ProductAvailabilityChanged, true
	case "product.updated.v1":
		return Event_16A_ProductUpdated, true
	case "product.update_failed.v1":
		return Event_16B_ProductUpdateFailed, true
	case "price.updated.v1":
		return Event_15A_PriceUpdated, true
	case "price.update_failed.v1":
		return Event_15B_PriceUpdateFailed, true
	case "product.ready_for_publication.v1":
		return Event_13_ProductReadyForPublication, true
	// Scheduling events
	case "price.monitoring.scheduled.v1":
		return Event_14A_PriceMonitoringScheduled, true
	case "product.availability_check.scheduled.v1":
		return Event_14B_AvailabilityCheckScheduled, true
	case "product.periodic_update.scheduled.v1":
		return Event_14C_PeriodicUpdateScheduled, true
	// Content generation events
	case "content.generation.requested.v1":
		return Event_08A_ContentGenerationRequested, true
	case "content.generation.started.v1":
		return Event_09A_ContentGenerationStarted, true
	case "content.generated.v1":
		return Event_10A_ContentGenerated, true
	case "content.generation.failed.v1":
		return Event_10B_ContentGenerationFailed, true
	case "content.generation.retried.v1":
		return Event_10D_ContentGenerationRetried, true
	// Reviews events
	case "reviews.requested.v1":
		return Event_08B_ReviewsRequested, true
	case "reviews.fetched.v1":
		return Event_09B_ReviewsFetched, true
	case "reviews.processed.v1":
		return Event_10C_ReviewsProcessed, true
	case "reviews.validated.v1":
		return Event_11A_ReviewsValidated, true
	case "reviews.fetch_failed.v1":
		return Event_11B_ReviewsFetchFailed, true
	case "reviews.stored.v1":
		return Event_12A_ReviewsStored, true
	case "reviews.error.v1":
		return Event_12B_ReviewsError, true
	// Quality assessment events
	case "quality.assessment.requested.v1":
		return Event_06_QualityAssessmentRequested, true
	case "quality.assessment.completed.v1":
		return Event_07A_QualityAssessmentCompleted, true
	case "quality.assessment.failed.v1":
		return Event_07B_QualityAssessmentFailed, true
	// Catalog enrichment events mapping
	case "catalog.product.enrichment.requested.v1":
		return CatalogProductEnrichmentRequestedV1, true
	case "catalog.product.enrichment.completed.v1":
		return CatalogProductEnrichmentCompletedV1, true
	case "catalog.product.enrichment.failed.v1":
		return CatalogProductEnrichmentFailedV1, true
	// Product enrichment events (keep for backward compatibility)
	case "product.enrichment.requested.v1":
		return ProductEnrichmentRequestedV1, true
	case "product.enrichment.completed.v1":
		return ProductEnrichmentCompletedV1, true
	case "product.enrichment.failed.v1":
		return ProductEnrichmentFailedV1, true
	default:
		return s, false
	}
}

// NewProductEnrichmentFailedEvent creates a new PA-API enrichment failure event
func NewProductEnrichmentFailedEvent(source string, data *ProductEnrichmentFailedData) (*Event, error) {
	return NewEvent(ProductEnrichmentFailedV1, "product", data.ASIN, data)
}

// Catalog event constructor aliases for PA-API enrichment

// NewCatalogProductEnrichmentRequestedEvent creates a new catalog PA-API enrichment request event
// This is the preferred function for new implementations
func NewCatalogProductEnrichmentRequestedEvent(source string, data *ProductEnrichmentRequestedData) (*Event, error) {
	if err := data.Validate(); err != nil {
		return nil, err
	}
	return NewEvent(CatalogProductEnrichmentRequestedV1, "catalog.product", data.ASIN, data)
}

// NewCatalogProductEnrichmentCompletedEvent creates a new catalog PA-API enrichment success event
// This is the preferred function for new implementations
func NewCatalogProductEnrichmentCompletedEvent(source string, data *ProductEnrichedData) (*Event, error) {
	return NewEvent(CatalogProductEnrichmentCompletedV1, "catalog.product", data.ASIN, data)
}

// NewCatalogProductEnrichmentFailedEvent creates a new catalog PA-API enrichment failure event
// This is the preferred function for new implementations
func NewCatalogProductEnrichmentFailedEvent(source string, data *ProductEnrichmentFailedData) (*Event, error) {
	return NewEvent(CatalogProductEnrichmentFailedV1, "catalog.product", data.ASIN, data)
}
