package events

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	tallEvents "github.com/MalteBoehm/tall-affiliate-common/pkg/events"
	intdb "github.com/maltedev/amazon-size-scraper/internal/database"
	"github.com/maltedev/amazon-size-scraper/internal/scraper"
)

// EventCompatibilityManager handles compatibility between event formats
type EventCompatibilityManager struct {
	logger *slog.Logger
}

// NewEventCompatibilityManager creates a new event compatibility manager
func NewEventCompatibilityManager(logger *slog.Logger) *EventCompatibilityManager {
	return &EventCompatibilityManager{
		logger: logger.With("component", "event_compatibility"),
	}
}

// CreateLegacyProductDetectedEvent creates a legacy-compatible event
func (m *EventCompatibilityManager) CreateLegacyProductDetectedEvent(result scraper.SearchResult, jobID string) (*intdb.OutboxEvent, error) {
	// Create legacy payload format
	payload := tallEvents.NewProductDetectedPayload{
		ASIN:           result.ASIN,
		Title:          result.Title,
		DetailPageURL:  result.URL,
		HasDimensions:  result.HasTable,
		BrowseNodeTags: []string{},
		AmazonData: map[string]interface{}{
			"job_id":           jobID,
			"scraper_version":  "v2.0.0",
			"processed_at":     time.Now().UTC().Format(time.RFC3339),
			"original_price":   result.Price,
			// Size and color fields will be added when available in SearchResult
		},
	}

	// Parse price with currency detection
	if result.Price != "" {
		price, currency := m.parsePriceWithCurrency(result.Price)
		payload.CurrentPrice = price
		payload.Currency = currency
	}

	// Extract brand with enhanced logic
	payload.Brand = m.extractBrandFromTitle(result.Title)

	// Add gender information
	payload.Gender = m.extractGenderFromTitle(result.Title)

	// Size and color fields will be added when they become available in SearchResult
	// Currently SearchResult only has: ASIN, Title, URL, Price, HasTable

	// Serialize payload
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal legacy payload: %w", err)
	}

	// Create outbox event with legacy event type
	event := &intdb.OutboxEvent{
		AggregateType: "product",
		AggregateID:   result.ASIN,
		EventType:     tallEvents.EventTypeNewProductDetected,
		Payload:       payloadBytes,
		TargetStream:  "stream:product_lifecycle",
		Status:        intdb.OutboxStatusPending,
		RetryCount:    0,
	}

	m.logger.Info("Created legacy-compatible NEW_PRODUCT_DETECTED event",
		"asin", result.ASIN,
		"event_type", tallEvents.EventTypeNewProductDetected,
		"has_dimensions", payload.HasDimensions,
		"brand", payload.Brand,
		"gender", payload.Gender)

	return event, nil
}

// CreateCloudEventsProductDetected creates a CloudEvents-format event
func (m *EventCompatibilityManager) CreateCloudEventsProductDetected(result scraper.SearchResult, jobID string) (*intdb.OutboxEvent, error) {
	// Create CloudEvents payload format
	payload := map[string]interface{}{
		"asin":             result.ASIN,
		"title":            result.Title,
		"detail_page_url":  result.URL,
		"has_dimensions":   result.HasTable,
		"brand":            m.extractBrandFromTitle(result.Title),
		"gender":           m.extractGenderFromTitle(result.Title),
		// Size, color, and image_url fields will be added when available in SearchResult
		"scraper_metadata": map[string]interface{}{
			"job_id":          jobID,
			"scraper_version": "v2.0.0",
			"processed_at":    time.Now().UTC().Format(time.RFC3339),
		},
	}

	// Add price information
	if result.Price != "" {
		price, currency := m.parsePriceWithCurrency(result.Price)
		payload["current_price"] = price
		payload["currency"] = currency
		payload["original_price"] = result.Price
	}

	// Serialize payload
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal CloudEvents payload: %w", err)
	}

	// Create outbox event with CloudEvents event type
	event := &intdb.OutboxEvent{
		AggregateType: "catalog.product",
		AggregateID:   result.ASIN,
		EventType:     tallEvents.CatalogProductDetectedV1,
		Payload:       payloadBytes,
		TargetStream:  "stream:product_lifecycle",
		Status:        intdb.OutboxStatusPending,
		RetryCount:    0,
	}

	m.logger.Info("Created CloudEvents-format product detected event",
		"asin", result.ASIN,
		"event_type", tallEvents.CatalogProductDetectedV1,
		"spec_version", "1.0",
		"source", "/amazon-scraper/product-detection")

	return event, nil
}

// CreateNumberedEventProductDetected creates a numbered-format event
func (m *EventCompatibilityManager) CreateNumberedEventProductDetected(result scraper.SearchResult, jobID string) (*intdb.OutboxEvent, error) {
	// Create numbered event payload (following tall-affiliate-common conventions)
	payload := tallEvents.NewProductDetectedPayload{
		ASIN:           result.ASIN,
		Title:          result.Title,
		DetailPageURL:  result.URL,
		HasDimensions:  result.HasTable,
		BrowseNodeTags: []string{},
		AmazonData: map[string]interface{}{
			"job_id":           jobID,
			"scraper_version":  "v2.0.0",
			"sequence_number":  "01", // Phase 1, Option 1
			"processed_at":     time.Now().UTC().Format(time.RFC3339),
			"legacy_event_id":  fmt.Sprintf("product_%s_%d", result.ASIN, time.Now().Unix()),
		},
	}

	// Parse price with currency detection
	if result.Price != "" {
		price, currency := m.parsePriceWithCurrency(result.Price)
		payload.CurrentPrice = price
		payload.Currency = currency
	}

	// Extract brand and gender
	payload.Brand = m.extractBrandFromTitle(result.Title)
	payload.Gender = m.extractGenderFromTitle(result.Title)

	// Size and color fields will be added when they become available in SearchResult
	// Currently SearchResult only has: ASIN, Title, URL, Price, HasTable

	// Serialize payload
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal numbered event payload: %w", err)
	}

	// Create outbox event with numbered event type
	event := &intdb.OutboxEvent{
		AggregateType: "product",
		AggregateID:   result.ASIN,
		EventType:     tallEvents.Event_01_ProductDetected,
		Payload:       payloadBytes,
		TargetStream:  "stream:product_lifecycle",
		Status:        intdb.OutboxStatusPending,
		RetryCount:    0,
	}

	m.logger.Info("Created numbered-format product detected event",
		"asin", result.ASIN,
		"event_type", tallEvents.Event_01_ProductDetected,
		"sequence", "01",
		"phase", "product_discovery")

	return event, nil
}

// parsePriceWithCurrency parses price string and detects currency
func (m *EventCompatibilityManager) parsePriceWithCurrency(priceStr string) (float64, string) {
	currency := "EUR" // Default for German Amazon
	price := 0.0

	// Clean price string
	cleanPrice := m.cleanPriceString(priceStr)

	// Detect currency
	if contains(cleanPrice, "$") || contains(cleanPrice, "USD") {
		currency = "USD"
	} else if contains(cleanPrice, "£") || contains(cleanPrice, "GBP") {
		currency = "GBP"
	}

	// Extract numeric value
	fmt.Sscanf(cleanPrice, "%f", &price)

	return price, currency
}

// cleanPriceString cleans price string for parsing
func (m *EventCompatibilityManager) cleanPriceString(priceStr string) string {
	priceStr = replaceAll(priceStr, ",", ".")
	priceStr = replaceAll(priceStr, "€", "")
	priceStr = replaceAll(priceStr, "$", "")
	priceStr = replaceAll(priceStr, "£", "")
	priceStr = replaceAll(priceStr, "USD", "")
	priceStr = replaceAll(priceStr, "EUR", "")
	priceStr = replaceAll(priceStr, "GBP", "")
	return trimSpace(priceStr)
}

// extractBrandFromTitle extracts brand from title
func (m *EventCompatibilityManager) extractBrandFromTitle(title string) string {
	// Common tall-friendly brands
	tallBrands := []string{
		"Longline", "Longtall", "Tallmen", "Tallwomen", "Height",
		"American Tall", "FortheTall", "Tall Order", "Tall Size",
		"Pierre Cardin", "JC Rendezvous", "Camouflag", "Albamoda",
	}

	titleUpper := toUpper(title)

	// Check for known brands
	for _, brand := range tallBrands {
		if contains(titleUpper, toUpper(brand)) {
			return brand
		}
	}

	// Extract first word as fallback
	return extractFirstWord(title)
}

// extractGenderFromTitle extracts gender from title
func (m *EventCompatibilityManager) extractGenderFromTitle(title string) string {
	titleUpper := toUpper(title)

	// Male indicators
	maleIndicators := []string{"MEN", "HERREN", "MAN", "MÄNNER"}
	for _, indicator := range maleIndicators {
		if contains(titleUpper, indicator) {
			return "male"
		}
	}

	// Female indicators
	femaleIndicators := []string{"WOMEN", "DAMEN", "WOMAN", "FRAU"}
	for _, indicator := range femaleIndicators {
		if contains(titleUpper, indicator) {
			return "female"
		}
	}

	return "unisex"
}

// GetSupportedEventTypes returns all supported event types for compatibility
func (m *EventCompatibilityManager) GetSupportedEventTypes() map[string]string {
	return map[string]string{
		"legacy":     tallEvents.EventTypeNewProductDetected,
		"cloudevents": tallEvents.CatalogProductDetectedV1,
		"numbered":   tallEvents.Event_01_ProductDetected,
	}
}

// NormalizeEventType normalizes event type to canonical format
func (m *EventCompatibilityManager) NormalizeEventType(eventType string) string {
	if canonical, ok := tallEvents.NormalizeEventType(eventType); ok {
		return canonical
	}
	return eventType
}

// ValidateEventPayload validates event payload against expected schema
func (m *EventCompatibilityManager) ValidateEventPayload(eventType string, payload []byte) error {
	switch eventType {
	case tallEvents.EventTypeNewProductDetected: // This maps to Event_01_ProductDetected
		var testPayload tallEvents.NewProductDetectedPayload
		return json.Unmarshal(payload, &testPayload)
	case tallEvents.CatalogProductDetectedV1:
		var testPayload map[string]interface{}
		if err := json.Unmarshal(payload, &testPayload); err != nil {
			return err
		}
		// Validate required fields for CloudEvents format
		if testPayload["asin"] == nil {
			return fmt.Errorf("asin field is required for CloudEvents format")
		}
		if testPayload["title"] == nil {
			return fmt.Errorf("title field is required for CloudEvents format")
		}
		return nil
	default:
		// Basic JSON validation for unknown event types
		var testPayload map[string]interface{}
		return json.Unmarshal(payload, &testPayload)
	}
}