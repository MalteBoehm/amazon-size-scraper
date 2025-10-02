# Reviews Lifecycle Events

This document describes the Reviews Lifecycle Events in the Tall Affiliate system.

## Overview

The Reviews system uses a comprehensive event-driven architecture to track the complete lifecycle of product reviews from request to deletion. This ensures reliable processing, caching, and error handling.

## Event Flow

```
REVIEWS_REQUESTED → REVIEWS_FETCHED → REVIEWS_STORED → REVIEWS_PROCESSED → REVIEWS_VALIDATED → REVIEWS_ENRICHED → REVIEWS_CACHED
                                                                                                                        ↓
REVIEWS_DELETED ← REVIEWS_EXPIRED ← REVIEWS_CACHED
                                    ↑
                              REVIEWS_ERROR (can occur at any stage)
```

## Event Types

### 1. REVIEWS_REQUESTED
**Trigger**: When reviews are requested for a product
**Payload**: `ReviewsRequestedPayload`
- `asin`: Product ASIN
- `product_id`: Internal product ID
- `source`: Reviews source (oxylabs, amazon_api, manual)
- `requested_at`: Timestamp when requested
- `options`: Additional options (pages, filters, etc.)

### 2. REVIEWS_FETCHED
**Trigger**: When reviews are successfully fetched from external source
**Payload**: `ReviewsFetchedPayload`
- `asin`: Product ASIN
- `product_id`: Internal product ID
- `source`: Reviews source
- `review_count`: Number of reviews fetched
- `fetched_at`: Timestamp when fetched
- `raw_data_size_bytes`: Size of raw data in bytes

### 3. REVIEWS_STORED
**Trigger**: When reviews are stored in the database
**Payload**: `ReviewsStoredPayload`
- `asin`: Product ASIN
- `product_id`: Internal product ID
- `source`: Reviews source
- `review_count`: Number of reviews stored
- `stored_at`: Timestamp when stored
- `database_id`: Database record ID

### 4. REVIEWS_PROCESSED
**Trigger**: When reviews are processed/cleaned
**Payload**: `ReviewsProcessedPayload`
- `asin`: Product ASIN
- `product_id`: Internal product ID
- `source`: Reviews source
- `processed_count`: Number of reviews processed
- `processed_at`: Timestamp when processed
- `processing_duration`: Time taken to process

### 5. REVIEWS_VALIDATED
**Trigger**: When reviews are validated for quality
**Payload**: `ReviewsValidatedPayload`
- `asin`: Product ASIN
- `product_id`: Internal product ID
- `source`: Reviews source
- `valid_count`: Number of valid reviews
- `invalid_count`: Number of invalid reviews
- `validated_at`: Timestamp when validated

### 6. REVIEWS_ENRICHED
**Trigger**: When reviews are enriched with additional data
**Payload**: `ReviewsEnrichedPayload`
- `asin`: Product ASIN
- `product_id`: Internal product ID
- `source`: Reviews source
- `enrichment_type`: Type of enrichment (sentiment, fit_analysis, etc.)
- `enriched_at`: Timestamp when enriched
- `additional_data`: Additional enrichment data

### 7. REVIEWS_CACHED
**Trigger**: When reviews are cached for fast access
**Payload**: `ReviewsCachedPayload`
- `asin`: Product ASIN
- `product_id`: Internal product ID
- `source`: Reviews source
- `cache_key`: Cache key used
- `ttl`: Time to live for cache
- `cached_at`: Timestamp when cached

### 8. REVIEWS_EXPIRED
**Trigger**: When cached reviews expire
**Payload**: `ReviewsExpiredPayload`
- `asin`: Product ASIN
- `product_id`: Internal product ID
- `source`: Reviews source
- `cache_key`: Cache key that expired
- `expired_at`: Timestamp when expired

### 9. REVIEWS_ERROR
**Trigger**: When an error occurs during any stage
**Payload**: `ReviewsErrorPayload`
- `asin`: Product ASIN
- `product_id`: Internal product ID
- `source`: Reviews source
- `error_type`: Type of error (fetch_error, process_error, etc.)
- `error_message`: Detailed error message
- `failed_at`: Timestamp when error occurred
- `retry_count`: Number of retry attempts

### 10. REVIEWS_DELETED
**Trigger**: When reviews are deleted
**Payload**: `ReviewsDeletedPayload`
- `asin`: Product ASIN
- `product_id`: Internal product ID
- `source`: Reviews source
- `deleted_at`: Timestamp when deleted
- `reason`: Reason for deletion (optional)

## Streams and Consumer Groups

### Streams
- `Product_Lifecycle_Stream`: Main stream for all reviews events
- `Reviews_Stream`: Dedicated reviews stream (optional)

### Consumer Groups
- `group:reviews`: Consumer group for reviews processing

## Error Types

- `fetch_error`: Error during fetching from external source
- `process_error`: Error during processing/cleaning
- `validation_error`: Error during validation
- `cache_error`: Error during caching operations
- `database_error`: Error during database operations

## Sources

- `oxylabs`: Oxylabs API
- `amazon_api`: Amazon Product Advertising API
- `manual`: Manually entered reviews

## Usage Examples

```go
// Create a reviews requested event
event := NewReviewsRequestedEvent("B00EXAMPLE", "product-123", "oxylabs", map[string]interface{}{
    "pages": 3,
    "sort": "recent",
})

// Create a reviews error event
errorEvent := NewReviewsErrorEvent("B00EXAMPLE", "product-123", "oxylabs", "fetch_error", "API rate limit exceeded", 2)

// Check if event is reviews-related
if IsReviewsEvent(event.Type) {
    priority := GetReviewsEventPriority(event.Type)
    // Handle based on priority
}
```

## Performance Considerations

- Events are processed asynchronously
- High-priority events (errors) are processed first
- Cache events help reduce API calls
- Validation events ensure data quality
- Enrichment events add value for content generation
