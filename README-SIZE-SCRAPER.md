# Amazon Size Table Scraper

A sophisticated Go-based web scraper for extracting product dimension data and size tables from Amazon.de. This tool automatically navigates search results, collects product ASINs, and extracts detailed size information from product pages, specifically designed for clothing items where physical measurements are crucial.

## Features

- **Automated Search Crawling**: Crawls Amazon search results and collects all product ASINs
- **Bot Protection Bypass**: Automatically detects and bypasses Amazon's "Weiter shoppen" bot check
- **Size Table Extraction**: Clicks "Größentabelle" button and extracts complete size data
- **Database Storage**: Stores all product data and size information in PostgreSQL
- **Concurrent Scraping**: Supports multiple concurrent scrapers for faster processing
- **Progress Tracking**: Resume capability with database-backed state management
- **Anti-Detection**: Human-like behavior simulation using Playwright
- **Structured Data**: Extracts and normalizes German size measurements

## Prerequisites

- Go 1.21 or higher
- PostgreSQL 12 or higher
- Playwright browsers (installed automatically)

## Installation

```bash
# Clone the repository
git clone https://github.com/yourusername/amazon-size-scraper.git
cd amazon-size-scraper

# Install dependencies
go mod download

# Install Playwright browsers
make install-playwright

# Set up database
createdb amazon_scraper
./scripts/setup_db.sh

# Build the application
make build
```

## Usage

### Complete Pipeline (Search + Scrape)

```bash
# Crawl search results and scrape all products
./bin/size-scraper -search "https://www.amazon.de/s?k=t-shirt+größentabelle+länge&i=fashion"

# With custom database settings
./bin/size-scraper \
  -search "https://www.amazon.de/s?k=t-shirt+größentabelle+länge&i=fashion" \
  -db-host localhost \
  -db-name amazon_scraper \
  -db-user postgres \
  -concurrent 3
```

### Scrape Only (Resume Previous Crawl)

```bash
# Only scrape pending products in database
./bin/size-scraper -scrape-only -concurrent 5
```

### Environment Variables

Create a `.env` file based on `example.env`:

```bash
# Database configuration
DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=yourpassword
DB_NAME=amazon_scraper

# Browser configuration
HEADLESS=false  # Set to true for production

# Scraping configuration
CONCURRENT_SCRAPERS=3
```

## Database Schema

The scraper stores data in a PostgreSQL database with the following structure:

- **products** table:
  - `asin` (primary key): Amazon Standard Identification Number
  - `title`: Product title
  - `brand`: Brand name
  - `category`: Product category
  - `url`: Product URL
  - `size_table`: Complete size table data (JSON)
  - `width_cm`: Extracted width in centimeters
  - `length_cm`: Extracted length in centimeters
  - `height_cm`: Extracted height in centimeters (if available)
  - `status`: pending/completed/failed
  - `error_message`: Error details if failed
  - Timestamps for tracking

## How It Works

1. **Search Phase**:
   - Navigates to Amazon search URL
   - Detects and bypasses bot protection
   - Extracts all product ASINs from search results
   - Handles pagination automatically
   - Stores products with "pending" status

2. **Scraping Phase**:
   - Retrieves pending products from database
   - Visits each product page
   - Clicks "Größentabelle" (size table) button
   - Extracts size data from modal/popup
   - Normalizes measurements (German → English)
   - Calculates key dimensions (width = chest/2)
   - Updates database with results

## Extracted Data

The scraper extracts and normalizes:
- **Sizes**: S, M, L, XL, XXL, etc.
- **Measurements**:
  - Länge → length
  - Breite → width
  - Brustumfang → chest (divided by 2 for width)
  - Schulter → shoulder
  - Ärmel → sleeve
  - Höhe → height

## Development

```bash
# Run tests
make test

# Run with hot reload
make dev

# Format code
make fmt

# Run linter
make lint

# View database
psql -d amazon_scraper -c "SELECT asin, title, status, width_cm, length_cm FROM products;"
```

## Monitoring Progress

```sql
-- Check scraping progress
SELECT status, COUNT(*) FROM products GROUP BY status;

-- View completed products with sizes
SELECT asin, title, width_cm, length_cm 
FROM products 
WHERE status = 'completed' AND length_cm > 0;

-- Check failed products
SELECT asin, title, error_message 
FROM products 
WHERE status = 'failed';
```

## Architecture

The scraper uses a modular architecture:

- **Browser Package**: Playwright wrapper with bot detection bypass
- **Database Package**: PostgreSQL operations with pgx
- **SearchCrawler**: Extracts ASINs from search results
- **ProductScraper**: Extracts size data from product pages
- **Main Orchestrator**: Coordinates the complete pipeline

## Error Handling

- Automatic retry with exponential backoff
- Failed products marked in database with error messages
- Can resume from any point using database state
- Comprehensive logging for debugging

## Example Output

```json
{
  "asin": "B07KXV71TG",
  "title": "Herren T-Shirt Baumwolle",
  "size_table": {
    "sizes": ["S", "M", "L", "XL", "XXL"],
    "measurements": {
      "XL": {
        "length": 75.0,
        "chest": 112.0,
        "shoulder": 48.0
      }
    },
    "unit": "cm"
  },
  "width_cm": 56.0,
  "length_cm": 75.0,
  "status": "completed"
}
```

## License

MIT