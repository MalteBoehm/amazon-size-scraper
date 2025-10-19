package events

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/MalteBoehm/tall-affiliate-common/pkg/events"
	intdb "github.com/maltedev/amazon-size-scraper/internal/database"
	"github.com/maltedev/amazon-size-scraper/internal/scraper"
)

// ProductEventValidator validates and enriches product events
type ProductEventValidator struct {
	logger *slog.Logger
}

// NewProductEventValidator creates a new product event validator
func NewProductEventValidator(logger *slog.Logger) *ProductEventValidator {
	return &ProductEventValidator{
		logger: logger.With("component", "product_event_validator"),
	}
}

// CreateProductDetectedEvent creates an enhanced NEW_PRODUCT_DETECTED event
func (v *ProductEventValidator) CreateProductDetectedEvent(result scraper.SearchResult, jobID string) (*intdb.OutboxEvent, error) {
	// Validate required fields
	if err := v.validateSearchResult(result); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// Create enhanced payload with tall-specific fields
	payload := v.createEnhancedPayload(result, jobID)

	// Serialize payload with validation
	payloadBytes, err := v.serializePayload(payload)
	if err != nil {
		return nil, fmt.Errorf("payload serialization failed: %w", err)
	}

	// Create outbox event with enhanced metadata
	event := &intdb.OutboxEvent{
		ID:            uuid.New(),
		AggregateType: "product",
		AggregateID:   result.ASIN,
		EventType:     events.Event_01_ProductDetected, // Use the canonical numbered event type
		Payload:       payloadBytes,
		TargetStream:  "stream:product_lifecycle",
		Status:        intdb.OutboxStatusPending,
		RetryCount:    0,
	}

	v.logger.Info("Created enhanced NEW_PRODUCT_DETECTED event",
		"asin", result.ASIN,
		"event_type", events.Event_01_ProductDetected,
		"has_dimensions", payload.HasDimensions,
		"price", payload.CurrentPrice,
		"brand", payload.Brand,
		"job_id", jobID)

	return event, nil
}

// validateSearchResult validates the search result before event creation
func (v *ProductEventValidator) validateSearchResult(result scraper.SearchResult) error {
	if result.ASIN == "" {
		return fmt.Errorf("ASIN is required")
	}
	if result.Title == "" {
		return fmt.Errorf("title is required")
	}
	if result.URL == "" {
		return fmt.Errorf("URL is required")
	}
	return nil
}

// createEnhancedPayload creates an enhanced payload with tall-specific fields
func (v *ProductEventValidator) createEnhancedPayload(result scraper.SearchResult, jobID string) events.NewProductDetectedPayload {
	payload := events.NewProductDetectedPayload{
		ASIN:           result.ASIN,
		Title:          result.Title,
		DetailPageURL:  result.URL,
		HasDimensions:  result.HasTable,
		BrowseNodeTags: []string{}, // Initialize empty array for consistency
		AmazonData:     make(map[string]interface{}),
	}

	// Add job metadata
	payload.AmazonData["job_id"] = jobID
	payload.AmazonData["scraper_timestamp"] = time.Now().UTC().Format(time.RFC3339)

	// Parse and validate price
	if result.Price != "" {
		if price, currency, err := v.parsePrice(result.Price); err == nil {
			payload.CurrentPrice = price
			payload.Currency = currency
		} else {
			v.logger.Warn("Failed to parse price",
				"asin", result.ASIN,
				"price", result.Price,
				"error", err)
		}
	}

	// Extract brand with enhanced logic
	payload.Brand = v.extractBrand(result.Title)

	// Extract gender information for tall clothing
	payload.Gender = v.extractGender(result.Title, payload.Brand)

	// Size and color fields will be added when they become available in SearchResult
	// Currently SearchResult only has: ASIN, Title, URL, Price, HasTable

	// Store original scraper data for reference
	payload.AmazonData["original_price"] = result.Price
	// Size, color, and image fields will be added when available in SearchResult

	return payload
}

// parsePrice parses price string and returns price and currency
func (v *ProductEventValidator) parsePrice(priceStr string) (float64, string, error) {
	// Clean price string
	priceStr = v.cleanPriceString(priceStr)

	// Default currency
	currency := "EUR" // Default for German Amazon

	// Check for currency indicators
	if contains(priceStr, "$") || contains(priceStr, "USD") {
		currency = "USD"
	} else if contains(priceStr, "£") || contains(priceStr, "GBP") {
		currency = "GBP"
	}

	// Extract numeric price
	var price float64
	n, err := fmt.Sscanf(priceStr, "%f", &price)
	if err != nil || n != 1 {
		return 0, "", fmt.Errorf("failed to parse price from: %s", priceStr)
	}

	return price, currency, nil
}

// cleanPriceString cleans price string for parsing
func (v *ProductEventValidator) cleanPriceString(priceStr string) string {
	// Replace common price separators
	priceStr = replaceAll(priceStr, ",", ".")
	priceStr = replaceAll(priceStr, "€", "")
	priceStr = replaceAll(priceStr, "$", "")
	priceStr = replaceAll(priceStr, "£", "")
	priceStr = replaceAll(priceStr, "USD", "")
	priceStr = replaceAll(priceStr, "EUR", "")
	priceStr = replaceAll(priceStr, "GBP", "")

	// Trim whitespace
	return trimSpace(priceStr)
}

// extractBrand extracts brand from title with enhanced logic
func (v *ProductEventValidator) extractBrand(title string) string {
	// Common tall-friendly brands to prioritize
	tallBrands := []string{
		"Longline", "Longtall", "Tallmen", "Tallwomen", "Height",
		"American Tall", "FortheTall", "Tall Order", "Tall Size",
	}

	titleUpper := toUpper(title)

	// Check for tall-specific brands first
	for _, brand := range tallBrands {
		if contains(titleUpper, toUpper(brand)) {
			return brand
		}
	}

	// Extract first word as fallback brand
	words := split(title, " ")
	if len(words) > 0 {
		return words[0]
	}

	return "Unknown"
}

// extractGender extracts gender information from title and brand
func (v *ProductEventValidator) extractGender(title, brand string) string {
	titleUpper := toUpper(title)
	brandUpper := toUpper(brand)

	// Male indicators
	maleIndicators := []string{"MEN", "HERREN", "MAN", "MÄNNER", "MALE"}
	for _, indicator := range maleIndicators {
		if contains(titleUpper, indicator) || contains(brandUpper, indicator) {
			return "male"
		}
	}

	// Female indicators
	femaleIndicators := []string{"WOMEN", "DAMEN", "WOMAN", "FRAU", "FEMALE"}
	for _, indicator := range femaleIndicators {
		if contains(titleUpper, indicator) || contains(brandUpper, indicator) {
			return "female"
		}
	}

	// Unisex indicators
	unisexIndicators := []string{"UNISEX", "UNI", "ALL"}
	for _, indicator := range unisexIndicators {
		if contains(titleUpper, indicator) {
			return "unisex"
		}
	}

	return "unknown"
}

// serializePayload serializes payload with validation
func (v *ProductEventValidator) serializePayload(payload events.NewProductDetectedPayload) (json.RawMessage, error) {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}

	// Validate that it's valid JSON
	var test map[string]interface{}
	if err := json.Unmarshal(payloadBytes, &test); err != nil {
		return nil, fmt.Errorf("payload is not valid JSON: %w", err)
	}

	return payloadBytes, nil
}

