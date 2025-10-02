# Automated Batch Enricher Service

## Overview
The Batch Enricher Service is an automated background service that continuously monitors the database for products needing color enrichment and automatically processes them via Amazon PA-API.

## Features
- **Automatic Scheduling**: Runs every 3 minutes by default (configurable)
- **Smart Detection**: Finds products without colors or with outdated scraper-based colors
- **PA-API Integration**: Fetches accurate color data from Amazon's Product Advertising API
- **Graceful Fallback**: Works even without PA-API credentials (uses placeholder data)
- **Docker Ready**: Includes Dockerfile and docker-compose configuration

## Installation

### Local Development
```bash
# Build the service
go build -o bin/batch-enricher-service cmd/batch-enricher-service/main.go

# Run the service
DATABASE_URL="postgres://user:pass@localhost/db" \
ENRICHMENT_CHECK_INTERVAL=3m \
./bin/batch-enricher-service
```

### Docker Deployment
```bash
# Build the Docker image
docker build -f Dockerfile.enricher -t batch-enricher .

# Run with docker-compose
docker-compose up batch-enricher
```

## Configuration

### Environment Variables
| Variable | Default | Description |
|----------|---------|-------------|
| `DATABASE_URL` | - | PostgreSQL connection string (required) |
| `ENRICHMENT_CHECK_INTERVAL` | `3m` | How often to check for products needing enrichment |
| `ENRICHMENT_BATCH_SIZE` | `10` | Number of products to process per batch |
| `ENRICHMENT_ENABLED` | `true` | Enable/disable the enrichment service |
| `ENRICHMENT_DRY_RUN` | `false` | Run without making database updates |
| `ENRICHMENT_STATS_ONLY` | `false` | Only show statistics, don't process |
| `LOG_LEVEL` | `info` | Logging level (debug, info, warn, error) |
| `LOG_FORMAT` | `json` | Log format (json or text) |
| `AMAZON_ACCESS_KEY` | - | PA-API access key (optional) |
| `AMAZON_SECRET_KEY` | - | PA-API secret key (optional) |
| `AMAZON_PARTNER_TAG` | - | Amazon partner tag (optional) |
| `AMAZON_REGION` | `de` | Amazon region (de, us, uk, etc.) |

### Scheduling Examples
```bash
# Every minute (for testing)
ENRICHMENT_CHECK_INTERVAL=1m

# Every 10 minutes
ENRICHMENT_CHECK_INTERVAL=10m

# Every hour
ENRICHMENT_CHECK_INTERVAL=1h

# Every 30 seconds (aggressive)
ENRICHMENT_CHECK_INTERVAL=30s
```

## How It Works

### Workflow
1. **Periodic Check**: Service wakes up every interval (default 3 minutes)
2. **Find Candidates**: Queries database for products needing enrichment:
   - Products without any colors
   - Products with scraper-sourced colors (unreliable)
3. **Batch Processing**: Processes up to 10 products at a time (PA-API limit)
4. **PA-API Call**: Fetches color variations from Amazon
5. **Database Update**: Updates products with enriched color data
6. **Sleep**: Waits for next interval

### Selection Criteria
Products are selected for enrichment if they meet ANY of these criteria:
- `color_variations IS NULL` (no colors at all)
- `jsonb_array_length(color_variations) = 0` (empty color array)
- `enrichment_source = 'scraper'` (old scraper-based colors)

### PA-API Integration
When PA-API credentials are configured:
- Calls the batch-enricher binary to perform actual enrichment
- Processes products in batches of 10 (API limit)
- Applies rate limiting (1 request/second)
- Handles pagination for products with many variations

When PA-API credentials are NOT configured:
- Updates products with placeholder data
- Sets `enrichment_source = 'placeholder'`
- Prevents repeated processing attempts

## Monitoring

### Logs
The service provides detailed logging:
```json
{
  "time": "2025-01-11T10:00:00Z",
  "level": "INFO",
  "msg": "enrichment cycle starting",
  "total_products": 500,
  "without_colors": 50,
  "scraper_colors": 100,
  "need_enrichment": 150
}
```

### Statistics Mode
Run with statistics only to see the current state:
```bash
./bin/batch-enricher-service -stats
```

Output:
```
Total Products: 500
Products Without Colors: 50
Products With Scraper Colors: 100
Products Needing Enrichment: 150
```

### Health Checks
Docker health check monitors if the service is running:
```yaml
healthcheck:
  test: ["CMD", "ps", "aux", "|", "grep", "batch-enricher-service"]
  interval: 60s
```

## Database Schema

The service expects these columns in the `product` table:
```sql
-- Color data storage
color_variations JSONB,

-- Parent ASIN for variation products
parent_asin VARCHAR(20),

-- Source of enrichment data
enrichment_source VARCHAR(20), -- 'scraper', 'pa-api', 'placeholder'

-- Timestamp of last enrichment
colors_enriched_at TIMESTAMP
```

## Performance

### Resource Usage
- **CPU**: Minimal (mostly idle between checks)
- **Memory**: ~20-50MB
- **Database Connections**: 1 persistent connection
- **Network**: Depends on batch size and interval

### Optimization Tips
1. **Adjust Interval**: Balance between freshness and resource usage
2. **Batch Size**: Larger batches are more efficient but risk timeouts
3. **Database Indexes**: Ensure indexes on enrichment criteria columns

## Troubleshooting

### Service Not Processing Products
1. Check if `ENRICHMENT_ENABLED=true`
2. Verify database connection with logs
3. Check if products meet selection criteria
4. Look for PA-API credential warnings in logs

### High Resource Usage
1. Increase check interval (e.g., `10m` instead of `3m`)
2. Reduce batch size
3. Add database indexes for query optimization

### PA-API Errors
1. Verify credentials are correct
2. Check rate limiting (1 req/sec)
3. Ensure region matches product marketplace
4. Check PA-API account status

## Docker Compose Integration

The service is included in `docker-compose.yml`:
```yaml
batch-enricher:
  build:
    context: ./amzon-size-scraper
    dockerfile: Dockerfile.enricher
  environment:
    ENRICHMENT_CHECK_INTERVAL: 3m
    ENRICHMENT_BATCH_SIZE: 10
    DATABASE_URL: ${DATABASE_URL}
    AMAZON_ACCESS_KEY: ${AMAZON_ACCESS_KEY}
    AMAZON_SECRET_KEY: ${AMAZON_SECRET_KEY}
    AMAZON_PARTNER_TAG: ${AMAZON_PARTNER_TAG}
  restart: unless-stopped
```

## Future Enhancements

1. **Webhook Notifications**: Notify when enrichment completes
2. **Priority Queue**: Process newer products first
3. **Retry Logic**: Automatic retry for failed enrichments
4. **Metrics Export**: Prometheus metrics for monitoring
5. **Dynamic Intervals**: Adjust frequency based on queue size
6. **Multi-Region Support**: Process different Amazon marketplaces

## Related Documentation

- [PA-API Migration Guide](README_PA_API_MIGRATION.md)
- [Batch Enricher Manual](cmd/batch-enricher/README.md)
- [Docker Deployment](../README.md#docker-deployment)

---
*Last Updated: 2025-01-11*