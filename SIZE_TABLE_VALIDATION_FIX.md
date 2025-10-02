# Size Table Validation Fix

## Problem
- "Länge (cm)" in size tables refers to T-shirt length (e.g., 73-77cm), NOT arm length
- **CRITICAL BUG**: "Armlänge (cm)" was being incorrectly mapped to "length" because it contains "länge"
- This caused products with only arm/sleeve measurements (19-28cm) to be accepted as valid
- Some products had incomplete size tables missing crucial measurements (chest/length)
- Products without chest circumference or length measurements should be ignored

## Solution Implemented

### 1. Enhanced Size Table Validation
- Updated `ValidateSizeTable()` in `internal/database/product_lifecycle.go`
  - Now requires BOTH chest/bust AND length measurements for ALL sizes
  - Returns false if ANY size is missing either measurement
- Added `ValidateSizeTableWithDetails()` 
  - Returns validation result plus list of missing fields
  - Helps with debugging and detailed error messages

### 2. Fixed Size Table Parsing
- Updated `parseFullSizeTable()` in `internal/amazon-scraper/scraper/product_extractor.go`
  - Now checks for "armlänge" BEFORE checking for "länge"
  - "Armlänge (cm)" correctly maps to "sleeve" measurement
  - Only standalone "Länge (cm)" maps to "length" measurement
  - Added warning logs when tables contain sleeve but no length measurements

### 3. Worker Logic Updates
- Modified `extractCompleteProductData()` in `internal/amazon-scraper/jobs/worker.go`
  - Uses new detailed validation to check size tables
  - Returns specific error messages about missing measurements
- Enhanced PRODUCT_IGNORED event publishing
  - Extracts specific missing fields from validation
  - Sets appropriate reason: "missing_required_measurements"
  - Includes detailed missing fields list

### 3. Event Publisher
- Existing `PublishProductIgnored()` method in `internal/amazon-scraper/events/publisher.go`
  - Already supports ProductIgnoredPayload with reason and missing fields
  - Publishes to "stream:product_lifecycle" for processing

### 4. Enhanced Logging
- Added detailed logging in `parseFullSizeTable()` 
  - Logs what measurements were found for each product
  - Helps debug which products are missing what data

## Testing
- Created comprehensive unit tests in `internal/database/product_lifecycle_test.go`
- Tests cover:
  - Products with only chest measurements (invalid)
  - Products with only length measurements (invalid)
  - Products with both measurements (valid)
  - Real example from user with missing length (invalid)
  - **NEW**: Products with "Armlänge" instead of T-shirt length (invalid)
- All tests passing ✅

## Key Fix for "Armlänge" Issue
The parsing order is critical:
```go
// IMPORTANT: Check for "Armlänge" BEFORE checking for "länge" to avoid false positives!
if strings.Contains(measurementType, "armlänge") || strings.Contains(measurementType, "arm length") || 
    strings.Contains(measurementType, "ärmel") || strings.Contains(measurementType, "sleeve") {
    // Armlänge/Ärmellänge is sleeve length, NOT T-shirt length!
    measurementKey = "sleeve"
} else if strings.Contains(measurementType, "länge") || strings.Contains(measurementType, "length") {
    // Only map to "length" if it's NOT "Armlänge"
    measurementKey = "length"
}
```

## Event Flow
1. Amazon Scraper extracts product with size table
2. `ValidateSizeTable()` checks for chest AND length measurements
3. If validation fails:
   - Worker publishes PRODUCT_IGNORED event
   - Event includes reason and specific missing fields
   - Product doesn't enter content generation pipeline
4. If validation passes:
   - Worker publishes NEW_PRODUCT_DETECTED event
   - Product continues through normal pipeline

## Benefits
- Prevents products with incomplete size data from entering the pipeline
- Ensures only products suitable for tall people are processed
- Clear audit trail of why products were ignored
- Uses existing event architecture (no new events needed)