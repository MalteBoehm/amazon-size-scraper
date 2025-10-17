# Amazon Scraper Service

## Overview

The Amazon Scraper Service is a Go-based microservice that replaces Oxylabs in the Tall Affiliate system. It provides:

- **Oxylabs-compatible endpoints** for size chart and review extraction
- **Job management API** for crawling Amazon category pages
- **Event publishing** to Redis streams for discovered products
- **Chi router** for HTTP handling
- **Playwright** for browser automation

## Architecture

### Service Port
- **8084** - Amazon Scraper Service

### API Endpoints

#### Oxylabs Replacement Endpoints
```
POST /api/v1/scraper/size-chart   - Extract size chart dimensions
POST /api/v1/scraper/reviews      - Extract product reviews
```

#### Job Management
```
POST /api/v1/scraper/jobs         - Create new scraping job
GET  /api/v1/scraper/jobs/{id}    - Get job status
GET  /api/v1/scraper/jobs         - List all jobs
GET  /api/v1/scraper/jobs/{id}/products - Get products found by job
```

#### Statistics
```
GET  /api/v1/stats                - Get scraper statistics
```

## Integration with Existing System

### 1. Product Lifecycle Service Integration
The Product Lifecycle Service calls our size-chart endpoint instead of Oxylabs:

```go
// OLD:
dimensions, err := oxylabsClient.GetSizeChart(asin)

// NEW:
dimensions, err := scraperClient.GetSizeChart(asin)
```

### 2. Content Generation Service Integration
The Content Generation Service calls our reviews endpoint instead of Oxylabs:

```go
// OLD:
reviews, err := oxylabsClient.GetReviews(asin)

// NEW:
reviews, err := scraperClient.GetReviews(asin)
```

### 3. Event Publishing
When new products are found, the service publishes `NEW_PRODUCT_DETECTED` events to Redis:

```
Stream: product:events:stream
Event Type: NEW_PRODUCT_DETECTED
Payload: {
  "asin": "B07ZRD89XF",
  "title": "Product Title",
  "detail_page_url": "https://amazon.de/dp/B07ZRD89XF",
  "source": "scraper"
}
```

## Setup

### Prerequisites
- Go 1.21+
- PostgreSQL 15+
- Redis 7+
- Playwright browsers

### Installation

1. Install Playwright browsers:
```bash
make -f Makefile.scraper install-playwright
```

2. Run database migrations:
```bash
make -f Makefile.scraper migrate-up
```

3. Build the service:
```bash
make -f Makefile.scraper build
```

### Running Locally

```bash
# Set environment variables
export DB_PASSWORD=postgres
export DB_PORT=5433  # Your local PostgreSQL port

# Run the service
make -f Makefile.scraper run
```

### Running with Docker

```bash
# Build and start all services
make -f Makefile.scraper docker-up

# View logs
make -f Makefile.scraper docker-logs

# Stop services
make -f Makefile.scraper docker-down
```

## Configuration

Environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| PORT | 8084 | HTTP server port |
| DB_HOST | localhost | PostgreSQL host |
| DB_PORT | 5432 | PostgreSQL port |
| DB_USER | postgres | PostgreSQL user |
| DB_PASSWORD | - | PostgreSQL password |
| DB_NAME | tall_affiliate | Database name |
| REDIS_ADDR | localhost:6379 | Redis address |
| SCRAPER_HEADLESS | true | Run browser in headless mode |
| SCRAPER_WORKERS | 2 | Number of concurrent workers |
| SCRAPER_RATE_LIMIT | 3 | Seconds between requests |

## Usage Examples

### 1. Extract Size Chart (Oxylabs Replacement)
```bash
curl -X POST http://localhost:8084/api/v1/scraper/size-chart \
  -H "Content-Type: application/json" \
  -d '{
    "asin": "B07ZRD89XF",
    "url": "https://amazon.de/dp/B07ZRD89XF"
  }'
```

Response:
```json
{
  "width_cm": 52.5,
  "length_cm": 74.0,
  "height_cm": 0,
  "size_chart_found": true
}
```

### 2. Extract Reviews (Oxylabs Replacement)
```bash
curl -X POST http://localhost:8084/api/v1/scraper/reviews \
  -H "Content-Type: application/json" \
  -d '{
    "asin": "B07ZRD89XF"
  }'
```

Response:
```json
{
  "reviews": [
    {
      "rating": 5,
      "title": "Great shirt",
      "text": "Perfect fit...",
      "verified_buyer": true,
      "mentions_size": true,
      "mentions_length": false
    }
  ],
  "average_rating": 4.2,
  "total_reviews": 156
}
```

### 3. Create Scraping Job
```bash
curl -X POST http://localhost:8084/api/v1/scraper/jobs \
  -H "Content-Type: application/json" \
  -d '{
    "search_query": "t-shirt größentabelle länge",
    "category": "fashion",
    "max_pages": 5
  }'
```

Response:
```json
{
  "job_id": "550e8400-e29b-41d4-a716-446655440000",
  "status": "pending",
  "message": "Job created successfully"
}
```

### 4. Check Job Status
```bash
curl http://localhost:8084/api/v1/scraper/jobs/550e8400-e29b-41d4-a716-446655440000
```

Response:
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "search_query": "t-shirt größentabelle länge",
  "status": "running",
  "pages_scraped": 3,
  "products_found": 180,
  "products_complete": 45,
  "products_new": 23,
  "products_updated": 22
}
```

## Database Schema

### scraper_jobs
Tracks scraping jobs initiated through the API:
```sql
- id (UUID)
- search_query (TEXT)
- category (VARCHAR)
- status (pending|running|completed|failed)
- pages_scraped
- products_found
- created_at, started_at, completed_at
```

### job_products
Links products to the jobs that discovered them:
```sql
- job_id (UUID)
- asin (VARCHAR)
- page_number (INT)
```

## Testing

```bash
# Run unit tests
make -f Makefile.scraper test

# Run with coverage
make -f Makefile.scraper test-coverage

# Test endpoints
make -f Makefile.scraper test-size-chart
make -f Makefile.scraper test-reviews
make -f Makefile.scraper test-create-job
```

## Monitoring

The service provides structured logging with slog. Key metrics:

- Event publishing success/failure
- Size chart extraction success rate
- Review extraction success rate
- Job processing duration
- Error rates by endpoint

## Troubleshooting

### Common Issues

1. **"Size table button not found"**
   - The product doesn't have a size chart
   - Amazon's HTML structure changed
   - Bot detection triggered

2. **"Failed to connect to Redis"**
   - Check Redis is running
   - Verify REDIS_ADDR configuration
   - Check network connectivity

3. **"Failed to create page"**
   - Playwright browsers not installed
   - Run: `make -f Makefile.scraper install-playwright`

4. **Job stuck in "pending"**
   - Check worker is running
   - Check database connectivity
   - View logs for errors

## Development

### Project Structure
```
/cmd/amazon-scraper/        # Main application
/internal/amazon-scraper/
  /api/                     # HTTP handlers
  /config/                  # Configuration
  /events/                  # Event publishing
  /jobs/                    # Job management
  /scraper/                 # Scraping logic
/migrations/                # Database migrations
```

### Adding New Features

1. **New Scraping Endpoint**:
   - Add handler in `/internal/amazon-scraper/api/handlers.go`
   - Add scraping logic in `/internal/amazon-scraper/scraper/`
   - Add route in `/cmd/amazon-scraper/main.go`

2. **New Event Type**:
   - Add event definition in `/internal/amazon-scraper/events/`
   - Update publisher logic
   - Document in event architecture

## License

Part of the Tall Affiliate system.