# Amazon Size Scraper - System Architecture

## Overview

Amazon Size Scraper is a Go-based microservice system designed to extract product dimension data from Amazon.de. The architecture prioritizes reliability, scalability, and anti-detection capabilities.

## Core Components

### 1. Scraper Service (Go)
- Primary service responsible for coordinating scraping operations
- Manages browser automation through WebSocket connections
- Implements retry logic and error handling
- Distributes work across multiple browser instances

### 2. Browser Automation Layer
- **Current Evaluation**: Camoufox (not production-ready)
- **Alternatives**: 
  - Playwright-go with stealth plugins
  - Chromedp for direct Chrome DevTools Protocol
  - Commercial scraping APIs (Scrapfly, Bright Data)

### 3. Data Processing Pipeline
- Extracts product dimensions from HTML
- Normalizes size data across different categories
- Validates extracted information
- Stores results in structured format

### 4. Request Management
- Rate limiting to avoid detection
- Proxy rotation for IP diversity
- Session management for authentication
- Request queuing and prioritization

## Technology Stack

- **Language**: Go
- **Browser Automation**: Playwright-go / Chromedp
- **Data Storage**: PostgreSQL / Redis for caching
- **Message Queue**: RabbitMQ / Redis Pub/Sub
- **Monitoring**: Prometheus + Grafana
- **Container**: Docker + Kubernetes

## Data Flow

```
[Product URLs] → [Queue] → [Scraper Service] → [Browser Instance]
                              ↓                        ↓
                         [Rate Limiter]          [HTML Parser]
                              ↓                        ↓
                         [Proxy Manager]         [Data Extractor]
                                                      ↓
                                               [Data Validator]
                                                      ↓
                                                 [Database]
```

## Anti-Detection Strategy

1. **Request Patterns**
   - Randomized delays between requests (5-30 seconds)
   - Human-like browsing patterns
   - Varying user agents and browser fingerprints

2. **Browser Configuration**
   - Disable automation indicators
   - Randomize viewport sizes
   - Enable JavaScript and cookies
   - Use residential proxies

3. **Session Management**
   - Maintain cookies across requests
   - Handle captchas manually or via service
   - Rotate sessions periodically

## Scalability Considerations

- Horizontal scaling of scraper instances
- Load balancing across browser pools
- Distributed queue for work distribution
- Database sharding for large datasets

## Error Handling

- Automatic retry with exponential backoff
- Circuit breaker for failing proxies
- Graceful degradation on rate limits
- Comprehensive logging and alerting

## Security Measures

- Encrypted proxy credentials
- Secure storage of session data
- API key rotation for commercial services
- Network isolation between components

## Performance Metrics

- Target: 100-500 products/hour per instance
- Memory: 200-400MB per browser instance
- CPU: 0.5-1 core per active scraper
- Network: Variable based on proxy quality

## Future Considerations

- Integration with Amazon Product Advertising API
- Machine learning for better data extraction
- Distributed tracing for debugging
- Auto-scaling based on queue depth