# Amazon Size Scraper - Docker Setup

This guide explains how to run the Amazon Size Scraper using Docker Compose, both for development and production deployment (including Coolify).

## Quick Start

### Development (Local)
```bash
# Copy environment file and customize
cp example.env .env

# Start all services
docker-compose up -d

# View logs
docker-compose logs -f size-scraper

# Stop services
docker-compose down
```

### Production (Coolify)
1. **Repository Setup**: Ensure your repository contains these files:
   - `docker-compose.yml` (main services)
   - `Dockerfile.scraper` (application image)
   - `.env` (production environment variables)
   - `proxy_list` (if using proxies)

2. **Coolify Configuration**:
   - Service Type: Docker Compose
   - Main Compose File: `docker-compose.yml`
   - Environment Variables: Set in Coolify UI or `.env` file
   - Build Context: Root directory
   - Dockerfile: `Dockerfile.scraper`

3. **Required Environment Variables for Production**:
   ```bash
   DB_PASSWORD=your_secure_password
   BROWSER_HEADLESS=true
   DEBUG_MODE=false
   LOG_LEVEL=info
   SCRAPER_CONCURRENT_LIMIT=2
   ```

## Services Overview

### Main Services
- **size-scraper**: Main scraper application
- **postgres**: PostgreSQL database
- **redis**: Redis for caching and job queues
- **lifecycle-consumer**: Optional event consumer (profile-based)

### Service Dependencies
```
postgres → (health check) → size-scraper
redis → (health check) → size-scraper
```

## Environment Variables

### Required for All Deployments
- `DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASSWORD`, `DB_NAME`
- `REDIS_ADDR`

### Browser Configuration
- `BROWSER_HEADLESS`: Set to `true` for production
- `BROWSER_TIMEOUT`: Browser operation timeout
- `BROWSER_VIEWPORT_WIDTH/HEIGHT`: Browser dimensions

### Scraper Configuration
- `SCRAPER_RATE_LIMIT_MIN/MAX`: Rate limiting (important!)
- `SCRAPER_CONCURRENT_LIMIT`: Number of concurrent scrapers
- `USE_PROXIES`: Enable proxy rotation if you have proxy_list

### Development vs Production
- **Development**: `DEBUG_MODE=true`, `BROWSER_HEADLESS=false`
- **Production**: `DEBUG_MODE=false`, `BROWSER_HEADLESS=true`

## Usage Examples

### Run Single Product Scrape (Debug Mode)
```bash
# Enable debug mode in .env
DEBUG_MODE=true

# Run with single URL
docker-compose run --rm size-scraper \
  -url "https://www.amazon.de/dp/ASIN_HERE"
```

### Run Search Crawling
```bash
# Add search URL to environment
SEARCH_URL="https://www.amazon.de/s?k=t-shirt+größentabelle"

# Start crawler
docker-compose up size-scraper
```

### Enable Lifecycle Consumer
```bash
# Start with consumer profile
docker-compose --profile consumer up -d
```

## Volumes and Persistence

- **postgres_data**: Database persistence
- **redis_data**: Redis persistence
- **scraper_logs**: Application logs

## Development Features (Override File)

The `docker-compose.override.yml` automatically applies during development:
- Debug mode enabled
- Browser visible (headless=false)
- Source code mounting (uncomment for hot reload)
- External database access ports
- Faster rate limiting for testing

## Troubleshooting

### Common Issues
1. **Browser startup issues**: Check Playwright dependencies
2. **Database connection**: Verify PostgreSQL health check
3. **Rate limiting**: Increase delays if blocked by Amazon
4. **Permission issues**: Check proxy_list file permissions

### Logs
```bash
# View all logs
docker-compose logs -f

# View specific service
docker-compose logs -f size-scraper
docker-compose logs -f postgres

# View recent logs
docker-compose logs --tail=100 size-scraper
```

### Health Checks
```bash
# Check service health
docker-compose ps

# Manual health check
curl http://localhost:8084/health
```

## Security Notes

- Change default passwords in production
- Use proxy rotation for large-scale scraping
- Monitor rate limits to avoid IP blocking
- Enable Redis authentication in production

## Coolify Specific Tips

1. **Build Optimization**: The Dockerfile uses multi-stage builds for smaller images
2. **Health Checks**: Built-in health checks for automatic restarts
3. **Resource Limits**: Set memory/CPU limits in Coolify if needed
4. **Environment**: Use Coolify's environment variable management for secrets
5. **Profiles**: Use Docker Compose profiles for optional services

## Monitoring

### Application Metrics
- Health endpoint: `GET /health`
- Application runs on port 8084
- Structured JSON logging

### Database Access
```bash
# Connect to PostgreSQL
docker-compose exec postgres psql -U postgres -d amazon_scraper

# View tables
\dt

# Check product status
SELECT COUNT(*), status FROM products GROUP BY status;
```

This setup provides a complete, production-ready Docker environment for the Amazon Size Scraper that works seamlessly with Coolify.