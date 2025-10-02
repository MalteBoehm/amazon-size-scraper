package metrics

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestExtractionMetrics(t *testing.T) {
	// Reset metrics for testing
	prometheus.DefaultRegisterer = prometheus.NewRegistry()
	
	// Initialize metrics
	m := NewExtractionMetrics()
	
	t.Run("RecordFieldExtraction", func(t *testing.T) {
		// Record successful extraction
		m.RecordFieldExtraction("gender", true, 100*time.Millisecond)
		
		// Verify counter incremented
		counter := testutil.ToFloat64(m.FieldExtractionTotal.WithLabelValues("gender", "success"))
		if counter != 1 {
			t.Errorf("expected 1 successful extraction, got %f", counter)
		}
		
		// Record failed extraction
		m.RecordFieldExtraction("material", false, 50*time.Millisecond)
		
		counter = testutil.ToFloat64(m.FieldExtractionTotal.WithLabelValues("material", "failure"))
		if counter != 1 {
			t.Errorf("expected 1 failed extraction, got %f", counter)
		}
	})
	
	t.Run("RecordFallbackUsage", func(t *testing.T) {
		// Record fallback usage
		m.RecordFallbackUsage("colors", 2)
		
		counter := testutil.ToFloat64(m.FallbackUsageTotal.WithLabelValues("colors", "2"))
		if counter != 1 {
			t.Errorf("expected 1 fallback usage at level 2, got %f", counter)
		}
	})
	
	t.Run("RecordProductExtraction", func(t *testing.T) {
		// Record complete product extraction
		m.RecordProductExtraction("complete", 5*time.Second)
		
		counter := testutil.ToFloat64(m.ProductExtractionTotal.WithLabelValues("complete"))
		if counter != 1 {
			t.Errorf("expected 1 complete extraction, got %f", counter)
		}
		
		// For histograms, we can't use testutil.ToFloat64
		// In production, you would verify via Prometheus queries
	})
	
	t.Run("RecordSizeTableValidation", func(t *testing.T) {
		// Record valid size table
		m.RecordSizeTableValidation(true)
		
		counter := testutil.ToFloat64(m.SizeTableValidationTotal.WithLabelValues("valid"))
		if counter != 1 {
			t.Errorf("expected 1 valid size table, got %f", counter)
		}
		
		// Record invalid size table
		m.RecordSizeTableValidation(false)
		
		counter = testutil.ToFloat64(m.SizeTableValidationTotal.WithLabelValues("invalid"))
		if counter != 1 {
			t.Errorf("expected 1 invalid size table, got %f", counter)
		}
	})
}

func TestScraperMetrics(t *testing.T) {
	// Reset metrics
	prometheus.DefaultRegisterer = prometheus.NewRegistry()
	
	m := NewScraperMetrics()
	
	t.Run("RecordPageLoad", func(t *testing.T) {
		// Record successful page load
		m.RecordPageLoad("https://amazon.de/dp/test", true, 2*time.Second)
		
		counter := testutil.ToFloat64(m.PageLoadTotal.WithLabelValues("success"))
		if counter != 1 {
			t.Errorf("expected 1 successful page load, got %f", counter)
		}
	})
	
	t.Run("RecordCaptchaDetection", func(t *testing.T) {
		// Record captcha detection
		m.RecordCaptchaDetection()
		
		counter := testutil.ToFloat64(m.CaptchaDetectedTotal)
		if counter != 1 {
			t.Errorf("expected 1 captcha detection, got %f", counter)
		}
	})
	
	t.Run("RecordRateLimitDelay", func(t *testing.T) {
		// Record rate limit delay
		m.RecordRateLimitDelay(5 * time.Second)
		
		// For histograms, we can't use testutil.ToFloat64 directly
		// In production, you would verify histogram metrics via Prometheus queries
	})
}

func TestMetricsRegistry(t *testing.T) {
	// Test that all metrics can be created with a custom registry
	registry := prometheus.NewRegistry()
	
	// Create metrics with custom registry
	extractionMetrics := NewExtractionMetricsWithRegistry(registry)
	scraperMetrics := NewScraperMetricsWithRegistry(registry)
	
	// Verify they were created (not nil)
	if extractionMetrics == nil {
		t.Fatal("extraction metrics should not be nil")
	}
	if scraperMetrics == nil {
		t.Fatal("scraper metrics should not be nil")
	}
	
	// Use the metrics to ensure they're initialized
	extractionMetrics.RecordFieldExtraction("test", true, 1*time.Millisecond)
	extractionMetrics.RecordFallbackUsage("test", 1)
	extractionMetrics.RecordProductExtraction("complete", 1*time.Second)
	extractionMetrics.RecordSizeTableValidation(true)
	
	scraperMetrics.RecordPageLoad("test-url", true, 1*time.Second)
	scraperMetrics.RecordCaptchaDetection()
	scraperMetrics.RecordRateLimitDelay(1*time.Second)
	
	// Verify metrics are registered
	metricFamilies, err := registry.Gather()
	if err != nil {
		t.Fatalf("failed to gather metrics: %v", err)
	}
	
	expectedMetrics := []string{
		"scraper_field_extraction_total",
		"scraper_field_extraction_duration_seconds",
		"scraper_fallback_usage_total",
		"scraper_product_extraction_total",
		"scraper_product_extraction_duration_seconds",
		"scraper_size_table_validation_total",
		"scraper_page_load_total",
		"scraper_page_load_duration_seconds",
		"scraper_captcha_detected_total",
		"scraper_rate_limit_delay_seconds",
	}
	
	foundMetrics := make(map[string]bool)
	for _, mf := range metricFamilies {
		foundMetrics[*mf.Name] = true
	}
	
	for _, expected := range expectedMetrics {
		if !foundMetrics[expected] {
			t.Errorf("expected metric %s not found in registry", expected)
		}
	}
}

func TestFieldExtractionSuccessRate(t *testing.T) {
	// Reset metrics
	prometheus.DefaultRegisterer = prometheus.NewRegistry()
	
	m := NewExtractionMetrics()
	
	// Record some extractions
	m.RecordFieldExtraction("gender", true, 100*time.Millisecond)
	m.RecordFieldExtraction("gender", true, 100*time.Millisecond)
	m.RecordFieldExtraction("gender", false, 100*time.Millisecond)
	
	// Verify metrics using testutil
	successCount := testutil.ToFloat64(m.FieldExtractionTotal.WithLabelValues("gender", "success"))
	failureCount := testutil.ToFloat64(m.FieldExtractionTotal.WithLabelValues("gender", "failure"))
	
	if successCount != 2 {
		t.Errorf("expected 2 successful extractions, got %f", successCount)
	}
	
	if failureCount != 1 {
		t.Errorf("expected 1 failed extraction, got %f", failureCount)
	}
	
	// Calculate success rate manually for test
	total := successCount + failureCount
	if total > 0 {
		rate := (successCount / total) * 100
		if rate < 66 || rate > 67 {
			t.Errorf("expected success rate around 66.67%%, got %.2f%%", rate)
		}
	}
}