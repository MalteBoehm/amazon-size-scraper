# Amazon Size Scraper

A Go-based web scraper for extracting product dimensions and specifications from Amazon.de product pages using Playwright for browser automation.

## Features

- Scrapes product dimensions, weight, and pricing information from Amazon.de
- Built with Playwright-go for robust browser automation
- Adaptive rate limiting to avoid detection
- Configurable concurrency and retry mechanisms
- Structured logging with slog
- Environment-based configuration
- Human-like browsing behavior simulation

## Architecture

The project follows a clean architecture pattern with the following structure:

```
├── cmd/scraper/        # Application entry points
├── internal/           # Private application code
│   ├── browser/        # Browser automation logic
│   ├── config/         # Configuration management
│   ├── models/         # Data models
│   ├── parser/         # HTML parsing logic
│   ├── queue/          # Task queue implementation
│   ├── ratelimit/      # Rate limiting logic
│   └── scraper/        # Core scraping logic
├── pkg/                # Public packages
│   └── logger/         # Logging utilities
└── configs/            # Configuration files
```

## Prerequisites

- Go 1.21 or higher
- Playwright browsers (installed automatically)

## Installation

1. Clone the repository:
```bash
git clone https://github.com/maltedev/amazon-size-scraper.git
cd amazon-size-scraper
```

2. Install dependencies:
```bash
go mod download
```

3. Install Playwright browsers:
```bash
make install-playwright
```

## Configuration

Copy the example environment file and adjust settings:

```bash
cp .env.example .env
```

Key configuration options:

- `SCRAPER_RATE_LIMIT_MIN`: Minimum delay between requests (default: 5s)
- `SCRAPER_RATE_LIMIT_MAX`: Maximum delay between requests (default: 30s)
- `SCRAPER_CONCURRENT_LIMIT`: Number of concurrent scrapers (default: 5)
- `BROWSER_HEADLESS`: Run browser in headless mode (default: true)

## Usage

### Command Line

Scrape by URLs:
```bash
go run cmd/scraper/main.go -urls "https://www.amazon.de/dp/B08N5WRWNW,https://www.amazon.de/dp/B08N5LGQNG"
```

Scrape by ASINs:
```bash
go run cmd/scraper/main.go -asins "B08N5WRWNW,B08N5LGQNG"
```

Scrape from file:
```bash
go run cmd/scraper/main.go -file urls.txt
```

### Build and Run

Build the application:
```bash
make build
```

Run the built binary:
```bash
./bin/amazon-scraper -asins "B08N5WRWNW"
```

## Output Formats

The scraper supports multiple output formats:

- `stdout` (default): Human-readable output
- `json`: JSON format for further processing
- `csv`: CSV format for spreadsheet import

Example:
```bash
go run cmd/scraper/main.go -asins "B08N5WRWNW" -output json
```

## Development

Run tests:
```bash
make test
```

Run with hot reload:
```bash
make dev
```

Format code:
```bash
make fmt
```

Run linter:
```bash
make lint
```

## Important Notes

1. **Legal Compliance**: This tool is for educational purposes. Always respect website terms of service and robots.txt.

2. **Rate Limiting**: The scraper implements adaptive rate limiting. Adjust delays based on your needs and to avoid detection.

3. **Anti-Detection**: While the scraper includes anti-detection measures, it may still be detected by advanced bot protection systems.

4. **Production Use**: This is a prototype. For production use, consider:
   - Implementing proxy rotation
   - Adding database persistence
   - Setting up distributed queuing (RabbitMQ/Redis)
   - Implementing comprehensive error handling
   - Adding monitoring and alerting

## Troubleshooting

1. **Playwright Installation Issues**:
   ```bash
   go run github.com/playwright-community/playwright-go/cmd/playwright install
   ```

2. **Rate Limiting**: If you're getting blocked, increase the rate limit delays in the configuration.

3. **Memory Issues**: Reduce `SCRAPER_CONCURRENT_LIMIT` for systems with limited memory.

## License

This project is for educational purposes only. Use responsibly and in accordance with Amazon's Terms of Service.