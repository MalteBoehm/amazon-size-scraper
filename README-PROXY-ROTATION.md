# Proxy Rotation for Amazon Scraper

This implementation adds proxy rotation capabilities to the Amazon size scraper to help avoid bot detection.

## Features

- **Proxy Management**: Automatic parsing and rotation of proxy list
- **Browser Pool**: Multiple browsers with different proxies
- **Health Monitoring**: Automatic failover for failed proxies
- **Configurable Rotation**: Request-based and time-based rotation
- **Anti-Detection**: Combined with existing stealth measures

## Setup

### 1. Proxy List File

Create a `proxy_list` file with your Packetstream proxies:

```
letsgrowdigital:2dc91789a1ca3824_country-Germany_session-a9CBe528:proxy.packetstream.io:31111
letsgrowdigital:2dc91789a1ca3824_country-Germany_session-tTBXMgBo:proxy.packetstream.io:31111
...
```

### 2. Environment Configuration

Set the following environment variables or use command line flags:

```bash
export USE_PROXIES=true
export PROXY_LIST_FILE=proxy_list
export BROWSER_POOL_SIZE=3
export MAX_REQUESTS_PER_BROWSER=20
```

### 3. Running with Proxy Rotation

```bash
# Use command line flags
./bin/size-scraper -use-proxies=true -proxy-list=proxy_list -pool-size=3 -max-requests=20

# Or run the test utility
go run cmd/test-proxy/main.go -proxy-list=proxy_list -asin=B08N5WRWNW
```

## Configuration Options

| Flag | Environment Variable | Default | Description |
|------|---------------------|---------|-------------|
| `--use-proxies` | `USE_PROXIES` | `true` | Enable proxy rotation |
| `--proxy-list` | `PROXY_LIST_FILE` | `proxy_list` | Path to proxy list file |
| `--pool-size` | `BROWSER_POOL_SIZE` | `3` | Number of browsers per pool |
| `--max-requests` | `MAX_REQUESTS_PER_BROWSER` | `20` | Requests before browser recreation |

## How It Works

### Proxy Manager
- Parses Packetstream format: `username:password@host:port`
- Implements round-robin rotation with health tracking
- Automatically blacklists failed proxies (3 failures max)
- Resets blacklist when all proxies are exhausted

### Browser Pool
- Creates multiple browser instances with different proxies
- Tracks request count per browser for rotation
- Automatically recreates browsers after max requests
- Handles proxy failures with automatic replacement

### Integration with Scraper
- `AmazonScraper` updated to use browser pools
- Automatic proxy failover on bot detection
- Enhanced error handling for proxy-related issues
- Statistics tracking for monitoring

## Testing Proxy Rotation

Use the test utility to verify proxy rotation:

```bash
go run cmd/test-proxy/main.go -proxy-list=proxy_list
```

This will:
1. Load your proxy list
2. Create a browser pool with proxy rotation
3. Perform multiple scraping attempts
4. Show proxy rotation and statistics

## Bot Detection Avoidance

The proxy rotation system combines with existing anti-detection measures:

- **IP Rotation**: Different proxy for each browser
- **User Agent Rotation**: Random user agents per session
- **Human-like Behavior**: Mouse movements and delays
- **Request Timing**: Rate limiting between requests
- **Browser Recreation**: Fresh browser instances periodically

## Monitoring and Statistics

Get real-time statistics about proxy usage:

```go
stats := scraper.GetPoolStats()
// Returns:
// {
//   "pool_size": 3,
//   "max_requests": 20,
//   "request_counts": [5, 3, 7],
//   "healthy_proxies": 98,
//   "total_proxies": 100
// }
```

## Troubleshooting

### Common Issues

1. **"No proxies available"**
   - Check proxy list file path
   - Verify proxy format is correct
   - Ensure proxies are not all blacklisted

2. **"Failed to create browser with proxy"**
   - Verify proxy credentials
   - Check proxy server connectivity
   - Try reducing pool size

3. **"All proxies blacklisted"**
   - Failure counts reset automatically
   - Check proxy server status
   - Increase max failures threshold

### Debug Logging

Enable debug logging to see detailed proxy usage:

```bash
export LOG_LEVEL=debug
./bin/size-scraper -use-proxies=true
```

## Architecture

```
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│   Proxy List    │───▶│  Proxy Manager   │───▶│  Browser Pool   │
│   (100 proxies) │    │  (Round Robin)   │    │  (3 browsers)   │
└─────────────────┘    └──────────────────┘    └─────────────────┘
                                │                        │
                                ▼                        ▼
                       ┌──────────────────┐    ┌─────────────────┐
                       │ Health Tracking  │    │ Amazon Scraper  │
                       │ (Failure Count)  │    │ (with rotation) │
                       └──────────────────┘    └─────────────────┘
```

This implementation provides robust proxy rotation to help avoid Amazon's bot detection while maintaining scraping performance and reliability.