# PA-API Color Extraction Migration Guide

## Overview
This document describes the migration from scraper-based color extraction to PA-API Variations for robust and complete color information retrieval.

## Migration Status
✅ **Completed** - The system now supports full PA-API v5 integration for color extraction.

## Key Changes

### 1. PA-API Client Implementation
- **Location**: `internal/paapi/client.go`
- **Features**:
  - Full AWS SigV4 authentication
  - GetItems method for batch processing (≤10 ASINs)
  - GetVariations method with pagination support
  - Rate limiting (1 request/second) with token bucket
  - Exponential backoff for retries
  - Comprehensive error handling

### 2. Color Normalization Utility
- **Location**: `internal/util/colors/normalizer.go`
- **Features**:
  - Technical prefix removal (e.g., "X123 Navy Blue" → "navy blue")
  - Parentheses content handling
  - Case normalization
  - Deduplication by normalized value
  - Validation to exclude non-color variations

### 3. Batch Enricher Updates
- **Location**: `cmd/batch-enricher/main.go`
- **Changes**:
  - Integration with PA-API client
  - Support for parent ASIN storage
  - Enhanced database updates with enrichment source tracking
  - Batch processing with configurable limits

### 4. Database Schema Updates
- **Migration**: `migrations/002_add_parent_asin.sql`
- **New Fields**:
  - `parent_asin`: Stores parent ASIN for variation products
  - `enrichment_source`: Tracks data source ('scraper' or 'pa-api')
  - Indexes for efficient querying

### 5. Prometheus Metrics
- **Location**: `internal/metrics/paapi_metrics.go`
- **Metrics**:
  - Request counts and duration
  - Color enrichment success/failure rates
  - Variations found per product
  - Rate limit hits
  - Retry counts
  - Batch sizes
  - Pagination metrics

### 6. Scraper Changes
- **REMOVED**: HTML-based color extraction has been completely removed from the scraper
- All color data now comes exclusively from PA-API via the batch enricher
- This ensures reliable, complete color variation data

## Configuration

### Environment Variables
```bash
# PA-API Credentials (Required)
AMAZON_ACCESS_KEY=your_access_key
AMAZON_SECRET_KEY=your_secret_key
AMAZON_PARTNER_TAG=your_partner_tag
AMAZON_REGION=de  # or us, uk, fr, etc.

# Note: Scraper color extraction has been removed
# All color data now comes from PA-API
```

### Running the Batch Enricher
```bash
# Build the enricher
cd amzon-size-scraper
go build -o bin/batch-enricher cmd/batch-enricher/main.go

# Run enrichment for products missing colors
./bin/batch-enricher

# Show statistics only
./bin/batch-enricher -stats

# Dry run (no database updates)
./bin/batch-enricher -dry-run
```

## Testing

### Unit Tests
```bash
# Run color normalizer tests
go test ./internal/util/colors/...

# Run PA-API client tests
go test ./internal/paapi/...
```

### Integration Tests
```bash
# Run with real PA-API credentials
go test -tags=integration ./internal/paapi/...
```

### Test ASINs (German Marketplace)
- **With Color Variations**: B08N5WRWNW (Echo Dot), B07PJV3JPR (Fire TV Stick)
- **Single Color**: B08C1LR9RC (Fire TV Stick 4K)

## Data Flow

1. **Candidate Selection**: Query products where colors are missing or from scraper
2. **Batch Processing**: Process ASINs in batches of ≤10
3. **PA-API GetItems**: Retrieve product information and parent ASINs
4. **Variation Check**: Determine if product has color variations
5. **PA-API GetVariations**: Fetch all color variations for parent products
6. **Normalization**: Apply color name normalization and deduplication
7. **Database Update**: Store colors, parent_asin, and enrichment_source

## Monitoring

### Key Metrics to Watch
- `paapi_requests_total`: Total API requests by operation and status
- `paapi_color_enrichment_total`: Success/failure rates for enrichment
- `paapi_variations_found`: Distribution of variation counts
- `paapi_rate_limit_hits_total`: Rate limiting incidents
- `paapi_errors_total`: API errors by type

### Logs
The system provides structured logging with the following levels:
- **INFO**: Normal operations, enrichment results
- **WARN**: Non-critical issues (e.g., no variations found)
- **ERROR**: API failures, database errors
- **DEBUG**: Detailed request/response information

## Migration Notes

The scraper-based color extraction has been completely removed. To populate color data:

1. Ensure PA-API credentials are configured
2. Run the batch enricher: `./bin/batch-enricher`
3. The enricher will automatically find and process products without colors
4. Schedule regular runs via cron for continuous enrichment

## Benefits of PA-API Migration

1. **Completeness**: Access to all color variations, not just visible ones
2. **Accuracy**: Direct from Amazon's catalog, no HTML parsing errors
3. **Parent ASIN**: Proper variation grouping for better data organization
4. **Availability**: Real-time availability status for each variation
5. **Performance**: Batch processing reduces overall API calls
6. **Reliability**: No dependency on HTML structure changes

## Acceptance Criteria ✅

- [x] Products with color variations have `colors` with ≥2 VariationItems
- [x] Products without variations but with ItemInfo.Color have exactly 1 VariationItem
- [x] `Value` field is normalized (lowercase, cleaned, deduplicated)
- [x] `DisplayName` preserves original formatting
- [x] `parent_asin` is populated for variation products
- [x] `enrichment_source` = 'pa-api' for enriched products
- [x] Metrics and logs show successful operations
- [x] Scraper color extraction has been completely removed

## Future Enhancements

1. **Caching**: Implement Redis caching for PA-API responses
2. **Scheduling**: Add cron job for regular enrichment runs
3. **Webhooks**: Notify downstream services of enrichment completion
4. **Multi-region**: Support multiple Amazon marketplaces simultaneously
5. **ML Integration**: Use color data for improved product recommendations

## Support

For issues or questions about the PA-API migration:
1. Check logs in the batch-enricher output
2. Review Prometheus metrics for anomalies
3. Verify PA-API credentials are correctly configured
4. Ensure database migrations have been applied

---
*Last Updated: 2025-01-11*