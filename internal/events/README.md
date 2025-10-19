# Event Structure & Outbox Integration

This package provides enhanced event handling for the Amazon Scraper with improved validation, enrichment, and compatibility with the tall-affiliate-common event system.

## Overview

The event system has been optimized to provide:

1. **Enhanced Event Validation** - Comprehensive validation of event structures and payloads
2. **Tall-Specific Enrichment** - Automatic enrichment with tall-friendly attributes
3. **Multiple Format Support** - Support for legacy, CloudEvents, and numbered event formats
4. **Improved Outbox Integration** - Better integration with the transactional outbox pattern
5. **Enhanced Relay Service** - Improved Redis Stream publishing with metadata

## Components

### ProductEventValidator

Validates and enriches product events with tall-specific attributes:

```go
validator := events.NewProductEventValidator(logger)
outboxEvent, err := validator.CreateProductDetectedEvent(result, jobID)
```

**Features:**
- Validates required fields (ASIN, title, URL)
- Parses prices with currency detection
- Extracts brand information with tall-friendly brand detection
- Determines gender from product titles
- Adds size and color information
- Enhances metadata with job context

### EventCompatibilityManager

Handles compatibility between different event formats:

```go
manager := events.NewEventCompatibilityManager(logger)

// Legacy format
legacyEvent, err := manager.CreateLegacyProductDetectedEvent(result, jobID)

// CloudEvents format
cloudEvent, err := manager.CreateCloudEventsProductDetected(result, jobID)

// Numbered format (tall-affiliate-common convention)
numberedEvent, err := manager.CreateNumberedEventProductDetected(result, jobID)
```

**Supported Event Types:**
- **Legacy**: `NEW_PRODUCT_DETECTED`
- **CloudEvents**: `catalog.product.detected.v1`
- **Numbered**: `01_PRODUCT_DETECTED`

### EnhancedRelay

Extends the base relay with validation and enrichment:

```go
config := events.EnhancedRelayConfig{
    RelayConfig: intdb.RelayConfig{
        PollInterval: 5 * time.Second,
        BatchSize:    100,
    },
    EnableEventValidation: true,
    EnableMetrics:         true,
}

relay := events.NewEnhancedRelay(db, redisClient, logger, config)
```

**Enhancements:**
- Event validation before publishing
- Automatic metadata enrichment
- Event categorization and prioritization
- CloudEvents specification support
- Enhanced error handling and logging

## Event Payload Structure

### Enhanced NewProductDetectedPayload

```json
{
  "asin": "B0D3M4HXHM",
  "title": "Longline Business Shirt for Tall Men",
  "brand": "Longline",
  "gender": "male",
  "current_price": 49.99,
  "currency": "EUR",
  "detail_page_url": "https://amazon.de/dp/B0D3M4HXHM",
  "has_dimensions": true,
  "size": "XL",
  "color": "White",
  "available_sizes": ["XL", "XXL"],
  "browse_node_tags": [],
  "amazon_data": {
    "job_id": "job_123",
    "scraper_version": "v2.0.0",
    "processed_at": "2025-01-19T10:30:00Z",
    "original_price": "49,99 €",
    "original_size": "XL",
    "original_color": "Weiß",
    "original_image": "https://images.amazon.com/..."
  },
  "dimensions": {},
  "variation_attributes": [],
  "tall_friendly_score": null,
  "is_tall_friendly": null,
  "rating": null,
  "review_count": null
}
```

## Stream Data Format

### Enhanced Redis Stream Entry

```json
{
  "id": "event-uuid",
  "type": "01_PRODUCT_DETECTED",
  "aggregate_type": "product",
  "aggregate_id": "B0D3M4HXHM",
  "timestamp": "2025-01-19T10:30:00Z",
  "payload": { /* Enhanced payload */ },
  "metadata": {
    "source": "amazon-scraper",
    "outbox_id": "event-uuid",
    "retry_count": 0,
    "target_stream": "stream:product_lifecycle",
    "relay_version": "v1.0.0",
    "relay_processed": "2025-01-19T10:30:01Z",
    "event_category": "product_discovery",
    "priority": 10
  },
  "specversion": "1.0",
  "source": "/amazon-scraper/product-detection",
  "subject": "B0D3M4HXHM"
}
```

## Integration with Job Scraper

The job scraper has been updated to use the enhanced event system:

```go
func createProductDetectedOutboxEvent(ctx context.Context, tx intdb.Tx, result scraper.SearchResult, logger *slog.Logger) error {
    validator := events.NewProductEventValidator(logger)
    outboxEvent, err := validator.CreateProductDetectedEvent(result, "")
    if err != nil {
        return fmt.Errorf("failed to create enhanced product detected event: %w", err)
    }

    outboxRepo := intdb.NewOutboxRepository(nil)
    if err := outboxRepo.InsertWithTx(ctx, tx, outboxEvent); err != nil {
        return fmt.Errorf("failed to insert enhanced outbox event: %w", err)
    }

    logger.Info("Created enhanced NEW_PRODUCT_DETECTED outbox event",
        "asin", result.ASIN,
        "event_type", tallEvents.Event_01_ProductDetected,
        "has_dimensions", result.HasTable,
        "price", result.Price)

    return nil
}
```

## Event Prioritization

Events are automatically categorized and prioritized:

| Category | Priority | Event Types |
|----------|----------|-------------|
| product_discovery | 10 | `01_PRODUCT_DETECTED`, `NEW_PRODUCT_DETECTED` |
| product_enrichment | 6 | `ENRICHMENT_REQUESTED`, `ENRICHMENT_COMPLETED` |
| quality_assessment | 5 | `QUALITY_ASSESSMENT_REQUESTED`, `QUALITY_ASSESSMENT_COMPLETED` |
| content_generation | 4 | `CONTENT_GENERATION_REQUESTED`, `CONTENT_GENERATED` |
| price_tracking | 3 | `PRICE_UPDATED`, `AVAILABILITY_CHANGED` |
| reviews_processing | 2 | `REVIEWS_REQUESTED`, `REVIEWS_FETCHED` |
| general | 1 | All other events |

## Validation Rules

### Required Fields
- `ASIN` - Valid 10-character Amazon identifier
- `title` - Non-empty product title
- `detail_page_url` - Valid HTTP/HTTPS URL
- `payload` - Valid JSON structure

### Tall-Specific Validation
- Brand detection prioritizes tall-friendly brands
- Gender extraction from product titles
- Size information validation
- Price parsing with currency detection

### JSON Schema Validation
- Payload must be valid JSON
- Required fields must be present and non-null
- Optional fields are validated if present

## Migration Notes

### From Legacy Events
Legacy events using `NEW_PRODUCT_DETECTED` are automatically mapped to the canonical `01_PRODUCT_DETECTED` event type.

### Event Type Normalization
The system normalizes event types to the tall-affiliate-common canonical format:
- `PRODUCT_CREATED` → `02A_PRODUCT_VALIDATED`
- `CONTENT_GENERATION_REQUESTED` → `08A_CONTENT_GENERATION_REQUESTED`
- `product.deleted.v1` → `19_PRODUCT_DELETED`

### Backward Compatibility
The system maintains backward compatibility with existing consumers by:
- Supporting legacy event types
- Providing event type normalization
- Maintaining payload structure compatibility

## Error Handling

### Validation Errors
Events that fail validation are not published and are marked as failed in the outbox with detailed error messages.

### Serialization Errors
JSON serialization errors are caught and logged with the problematic payload for debugging.

### Relay Errors
Publishing errors trigger the standard retry mechanism with exponential backoff.

## Monitoring and Metrics

### Event Metrics
```go
metrics, err := relay.GetEventMetrics(ctx)
// Returns:
// - pending_events: Number of events waiting to be processed
// - dead_letter_events: Number of events that failed processing
// - relay_version: Version of the relay service
// - last_updated: Last metrics update timestamp
```

### Logging
Enhanced logging provides detailed information about:
- Event creation and validation
- Payload enrichment details
- Stream publishing status
- Error conditions and retry attempts

## Usage Examples

### Creating Enhanced Events
```go
// Initialize validator
validator := events.NewProductEventValidator(logger)

// Create enhanced event for product detection
event, err := validator.CreateProductDetectedEvent(searchResult, "job_123")
if err != nil {
    log.Fatal("Failed to create event:", err)
}

// Insert into outbox within transaction
err = db.Transaction(ctx, func(tx intdb.Tx) error {
    return outboxRepo.InsertWithTx(ctx, tx, event)
})
```

### Using Enhanced Relay
```go
// Initialize enhanced relay
relay := events.NewEnhancedRelay(db, redisClient, logger, config)

// Start processing events
go func() {
    if err := relay.Start(ctx); err != nil {
        log.Printf("Relay error: %v", err)
    }
}()

// Get metrics
metrics, err := relay.GetEventMetrics(ctx)
fmt.Printf("Pending events: %d\n", metrics["pending_events"])
```

This enhanced event system provides robust event handling with comprehensive validation, enrichment, and compatibility features for the Amazon Scraper integration with the tall-affiliate ecosystem.