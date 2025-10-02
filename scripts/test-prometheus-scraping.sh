#!/bin/bash

echo "=== Testing Prometheus Metrics Endpoint ==="
echo "Endpoint: http://localhost:2112/metrics"
echo ""

# Check if the endpoint is responding
if curl -s -o /dev/null -w "%{http_code}" http://localhost:2112/metrics | grep -q 200; then
    echo "✅ Metrics endpoint is responding with HTTP 200"
else
    echo "❌ Metrics endpoint is not responding"
    exit 1
fi

echo ""
echo "=== Field Extraction Success Rates ==="
# Calculate success rates for each field
for field in gender material colors prime_eligibility stock_status; do
    success=$(curl -s http://localhost:2112/metrics | grep "scraper_field_extraction_total{field=\"$field\",status=\"success\"}" | awk '{print $2}')
    failure=$(curl -s http://localhost:2112/metrics | grep "scraper_field_extraction_total{field=\"$field\",status=\"failure\"}" | awk '{print $2}')
    
    if [ -n "$success" ] && [ -n "$failure" ]; then
        total=$((success + failure))
        if [ $total -gt 0 ]; then
            rate=$(echo "scale=2; $success * 100 / $total" | bc)
            echo "$field: ${rate}% success rate ($success/$total)"
        fi
    fi
done

echo ""
echo "=== Product Extraction Status ==="
curl -s http://localhost:2112/metrics | grep "scraper_product_extraction_total" | grep -v "HELP" | grep -v "TYPE"

echo ""
echo "=== Size Table Validation ==="
curl -s http://localhost:2112/metrics | grep "scraper_size_table_validation_total" | grep -v "HELP" | grep -v "TYPE"

echo ""
echo "=== Anti-Bot Detection ==="
captcha_count=$(curl -s http://localhost:2112/metrics | grep "scraper_captcha_detected_total" | grep -v "HELP" | grep -v "TYPE" | awk '{print $2}')
echo "CAPTCHA detections: $captcha_count"

echo ""
echo "=== Sample Prometheus Query Examples ==="
echo "1. Field extraction success rate:"
echo "   rate(scraper_field_extraction_total{field=\"gender\",status=\"success\"}[5m]) / rate(scraper_field_extraction_total{field=\"gender\"}[5m]) * 100"
echo ""
echo "2. Average extraction duration:"
echo "   histogram_quantile(0.95, sum(rate(scraper_field_extraction_duration_seconds_bucket[5m])) by (field, le))"
echo ""
echo "3. CAPTCHA detection rate:"
echo "   rate(scraper_captcha_detected_total[5m])"
echo ""
echo "✅ Metrics endpoint is working correctly and ready for Prometheus!"