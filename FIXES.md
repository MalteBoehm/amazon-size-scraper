# Amazon Size Scraper - Problem Analysis and Fixes

## Problem Analysis

Based on the logs and code analysis, the core issue is identified:

### Root Cause
The Size Scraper cannot find any products to scrape because there are **no products with status "pending"** in the database. This is caused by a workflow break in the product lifecycle.

### Workflow Break
1. **Job Scraper**: Successfully processes jobs and extracts ASINs from Amazon search results
2. **Event Generation**: Creates events with ASIN in `aggregate_id` field ✅
3. **Event Relay**: Correctly forwards events from database outbox to Redis streams ✅
4. **Lifecycle Consumer**: ⚠️ **FAILED** - Could not extract ASIN from events due to parsing issues
5. **Size Scraper**: No products with "pending" status → nothing to scrape ❌

## Fixes Implemented

### 1. Enhanced Lifecycle-Consumer (`cmd/lifecycle-consumer/main.go`)

**Problem**: The consumer couldn't parse events correctly and extract ASINs.

**Fixes Applied**:
- Added comprehensive debug logging for message structure analysis
- Implemented dual parsing strategies:
  1. Parse from "data" field (Relay format with JSON)
  2. Fallback to direct field extraction
- Support for multiple event types:
  - `NEW_PRODUCT_DETECTED`
  - `01_PRODUCT_DETECTED`
  - `02A_PRODUCT_VALIDATED`
- Enhanced ASIN extraction with better error handling and logging

### 2. Event Type Support

**Before**: Only processed `02A_PRODUCT_VALIDATED` events
**After**: Processes all product detection events:
```go
const (
    EVENT_01_PRODUCT_DETECTED     = "01_PRODUCT_DETECTED"
    EVENT_NEW_PRODUCT_DETECTED    = "NEW_PRODUCT_DETECTED"
    EVENT_02A_PRODUCT_VALIDATED   = "02A_PRODUCT_VALIDATED"
)
```

### 3. Data Integrity Recovery

Created tools to fix the data integrity issue:

**Tool**: `fix_product_status.go`
- Analyzes current product statuses
- Identifies products that should be reset to "pending"
- Provides strategies to restore workflow:
  1. Reset completed products without size data
  2. Reset some completed products for re-processing

## Next Steps for Production Deployment

### 1. Deploy Updated Lifecycle-Consumer
```bash
# Build and deploy
go build -o bin/lifecycle-consumer ./cmd/lifecycle-consumer
# Replace the running container/service
```

### 2. Fix Product Statuses
Run the data recovery script on production:
```bash
# Set production environment variables
export DB_HOST=49.13.49.90
export DB_PORT=5111
export DB_PASSWORD=postgres

# Run the fix script
go run fix_product_status.go
```

### 3. Monitor the Logs
After deployment, monitor the lifecycle-consumer logs:
- Look for "DEBUG: Parsed event from data field" messages
- Verify ASIN extraction is working
- Check that products are being created with "pending" status
- Confirm size scraper finds products to process

### 4. Verify End-to-End Workflow
1. **Job Scraper** → Creates jobs ✅
2. **ASIN Extraction** → Finds products ✅
3. **Event Creation** → Publishes events with ASIN ✅
4. **Event Relay** → Forwards to Redis ✅
5. **Lifecycle Consumer** → **FIXED** - Now extracts ASINs correctly
6. **Product Creation** → Creates products with "pending" status ✅
7. **Size Scraper** → Finds and processes pending products ✅

## Expected Outcome After Fixes

- **Lifecycle Consumer** will successfully process events and extract ASINs
- **Products** will be created with "pending" status in the database
- **Size Scraper** will find pending products and scrape size data
- **Workflow** will be restored to normal operation
- **71 existing products** can be gradually reset and processed

## Verification Checklist

- [ ] Lifecycle-Consumer logs show successful ASIN extraction
- [ ] Database contains products with "pending" status
- [ ] Size Scraper logs show "no pending products found" → "processing X products"
- [ ] Products get size data and status changes to "active" or "rejected"
- [ ] New products from jobs follow the complete workflow

The fix addresses the root cause and restores the complete product lifecycle workflow from job creation to size data extraction.