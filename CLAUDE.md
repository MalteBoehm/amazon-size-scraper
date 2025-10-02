# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build and Development Commands

### Essential Commands
```bash
# Build the application
make build

# Run tests with race detection
make test

# Run a single test
go test -v -run TestFunctionName ./internal/package_name

# Run tests with coverage
make test-coverage

# Install Playwright browsers (required for first-time setup)
make install-playwright

# Run the scraper
./bin/amazon-scraper -asins "B08N5WRWNW,B08N5LGQNG"
go run cmd/scraper/main.go -urls "https://www.amazon.de/dp/B08N5WRWNW"
go run cmd/scraper/main.go -file urls.txt

# Development with hot reload (requires air)
make dev

# Lint and format
make lint
make fmt
```

## Architecture Overview

### Core Abstraction Layers

1. **Browser Automation Layer** (`internal/browser/`)
   - Wraps Playwright-go functionality
   - Implements anti-detection measures (user agent rotation, viewport randomization)
   - Manages browser lifecycle and page creation
   - Key method: `NavigateWithRetry()` handles flaky network conditions

2. **Parser-Scraper Separation**
   - **Scraper** (`internal/scraper/`): Handles browser interaction, navigation, rate limiting
   - **Parser** (`internal/parser/`): Pure HTML parsing logic with regex patterns for German Amazon pages
   - This separation allows testing parsing logic without browser overhead

3. **Rate Limiting Strategy** (`internal/ratelimit/`)
   - Three implementations: Simple, Adaptive, TokenBucket
   - Adaptive rate limiter adjusts delays based on success/error rates
   - Critical for avoiding Amazon's anti-bot detection

### Data Flow Architecture

```
CLI Input → Queue → Scraper → Browser → Parser → Output
                ↓
           RateLimiter
```

- **Queue**: In-memory priority queue, designed for future Redis/RabbitMQ migration
- **Models**: Product struct includes nested Dimension, Weight, Price structs
- **Config**: Environment-based configuration with sensible defaults

### Anti-Detection Implementation

The scraper implements several anti-detection measures:
- Human-like mouse movements in `humanizeInteraction()`
- Captcha detection in `checkIfBlocked()`
- Configurable delays between requests (5-30 seconds default)
- Browser fingerprint randomization through Playwright options

### Parser Design

The parser uses compiled regex patterns to extract:
- Dimensions in various formats (cm, mm, inch, zoll)
- Weight in German units (Kilogramm, Gramm)
- Handles German decimal notation (comma as decimal separator)

Key extraction methods:
- `ExtractDimensions()`: Searches multiple page sections for size data
- `extractProductDetails()`: Aggregates text from various Amazon product detail sections

### Configuration Strategy

Environment variables control behavior without code changes:
- `SCRAPER_RATE_LIMIT_MIN/MAX`: Adaptive delay range
- `BROWSER_HEADLESS`: Toggle for debugging
- `SCRAPER_CONCURRENT_LIMIT`: Control resource usage

### Error Handling Pattern

The codebase uses wrapped errors throughout:
```go
return nil, fmt.Errorf("failed to extract ASIN: %w", err)
```

This enables error chain inspection and proper logging context.

## Key Design Decisions

1. **Playwright over Chromedp**: Chosen for better stealth capabilities and cross-browser support
2. **In-memory queue**: Simple implementation ready for distributed queue migration
3. **Functional options pattern**: Browser configuration uses options struct for flexibility
4. **Table-driven regex**: Parser uses multiple patterns to handle Amazon's varying HTML structures

## Testing Approach

- Unit tests for parser logic (regex patterns)
- Integration tests would require mock HTML responses
- Browser tests need Playwright installation

## Future Extension Points

1. **Proxy Support**: Browser struct has ProxyServer field ready for implementation
2. **Database Layer**: Models are JSON-tagged for easy persistence
3. **Distributed Queue**: Queue interface allows swapping implementations
4. **Multiple Parsers**: Parser interface enables supporting other Amazon domains