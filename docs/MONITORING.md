# Monitoring and Metrics Guide

This guide explains how to use the monitoring and metrics features in the Amazon Size Scraper.

## Overview

The scraper includes comprehensive Prometheus metrics for monitoring extraction success rates, performance, and anti-bot detection.

## Available Metrics

### Extraction Metrics

- **`scraper_field_extraction_total`** (Counter)
  - Labels: `field`, `status` (success/failure)
  - Description: Total number of field extraction attempts
  
- **`scraper_field_extraction_duration_seconds`** (Histogram)
  - Labels: `field`
  - Description: Duration of field extraction in seconds
  
- **`scraper_fallback_usage_total`** (Counter)
  - Labels: `field`, `fallback_level`
  - Description: Number of times fallback extraction methods were used
  
- **`scraper_product_extraction_total`** (Counter)
  - Labels: `status` (complete/partial/failed)
  - Description: Total number of product extractions
  
- **`scraper_product_extraction_duration_seconds`** (Histogram)
  - Description: Duration of complete product extraction
  
- **`scraper_size_table_validation_total`** (Counter)
  - Labels: `status` (valid/invalid)
  - Description: Size table validation results

### Scraper Metrics

- **`scraper_page_load_total`** (Counter)
  - Labels: `status` (success/failure)
  - Description: Total page load attempts
  
- **`scraper_page_load_duration_seconds`** (Histogram)
  - Labels: `status`
  - Description: Page load duration
  
- **`scraper_captcha_detected_total`** (Counter)
  - Description: CAPTCHA detection count
  
- **`scraper_rate_limit_delay_seconds`** (Histogram)
  - Description: Rate limiting delays

## Integration Example

```go
// Initialize metrics
extractionMetrics := metrics.NewExtractionMetrics()
scraperMetrics := metrics.NewScraperMetrics()

// Create instrumented extractor
extractor := scraper.NewInstrumentedProductExtractor(browser, logger, extractionMetrics)

// Start metrics server
http.Handle("/metrics", metrics.Handler())
go http.ListenAndServe(":2112", nil)
```

## Prometheus Configuration

Add this scrape config to your `prometheus.yml`:

```yaml
scrape_configs:
  - job_name: 'amazon-scraper'
    static_configs:
      - targets: ['localhost:2112']
```

## Grafana Dashboard Queries

### Field Extraction Success Rate
```promql
rate(scraper_field_extraction_total{field="gender",status="success"}[5m]) / 
rate(scraper_field_extraction_total{field="gender"}[5m]) * 100
```

### Average Extraction Duration by Field
```promql
histogram_quantile(0.95, 
  sum(rate(scraper_field_extraction_duration_seconds_bucket[5m])) by (field, le)
)
```

### Product Extraction Success Rate
```promql
sum(rate(scraper_product_extraction_total{status="complete"}[5m])) / 
sum(rate(scraper_product_extraction_total[5m])) * 100
```

### CAPTCHA Detection Rate
```promql
rate(scraper_captcha_detected_total[5m])
```

## Alerting Rules

Example Prometheus alerting rules:

```yaml
groups:
  - name: scraper_alerts
    rules:
      - alert: HighExtractionFailureRate
        expr: |
          (
            sum(rate(scraper_field_extraction_total{status="failure"}[5m])) /
            sum(rate(scraper_field_extraction_total[5m]))
          ) > 0.3
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "High extraction failure rate"
          description: "More than 30% of field extractions are failing"
      
      - alert: CaptchaDetected
        expr: rate(scraper_captcha_detected_total[5m]) > 0
        for: 1m
        labels:
          severity: critical
        annotations:
          summary: "CAPTCHA detected"
          description: "Amazon is showing CAPTCHA challenges"
```

## Best Practices

1. **Monitor Success Rates**: Track field extraction success rates to identify problematic selectors
2. **Watch for CAPTCHAs**: High CAPTCHA rates indicate you need to slow down
3. **Track Fallback Usage**: High fallback usage suggests primary selectors need updating
4. **Measure Performance**: Monitor extraction duration to identify performance bottlenecks
5. **Set Up Alerts**: Configure alerts for critical metrics like CAPTCHA detection

## Debugging with Metrics

When extraction fails for specific fields:

1. Check the success rate for that field:
   ```promql
   scraper_field_extraction_total{field="material",status="failure"}
   ```

2. Check if fallback methods are being used:
   ```promql
   scraper_fallback_usage_total{field="material"}
   ```

3. Monitor extraction duration to identify slow fields:
   ```promql
   scraper_field_extraction_duration_seconds{field="material"}
   ```

## Performance Optimization

Use metrics to identify optimization opportunities:

- Fields with high failure rates need selector updates
- Fields with long extraction times may need algorithm improvements
- High fallback usage indicates primary methods need fixing