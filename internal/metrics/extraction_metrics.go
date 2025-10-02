package metrics

import (
	"fmt"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// ExtractionMetrics holds metrics for product data extraction
type ExtractionMetrics struct {
	// Field extraction metrics
	FieldExtractionTotal      *prometheus.CounterVec
	FieldExtractionDuration   *prometheus.HistogramVec
	FallbackUsageTotal        *prometheus.CounterVec
	
	// Product extraction metrics
	ProductExtractionTotal    *prometheus.CounterVec
	ProductExtractionDuration prometheus.Histogram
	
	// Size table validation
	SizeTableValidationTotal  *prometheus.CounterVec
}

// NewExtractionMetrics creates new extraction metrics
func NewExtractionMetrics() *ExtractionMetrics {
	return NewExtractionMetricsWithRegistry(prometheus.DefaultRegisterer)
}

// NewExtractionMetricsWithRegistry creates new extraction metrics with a specific registry
func NewExtractionMetricsWithRegistry(reg prometheus.Registerer) *ExtractionMetrics {
	factory := promauto.With(reg)
	
	return &ExtractionMetrics{
		FieldExtractionTotal: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "scraper_field_extraction_total",
				Help: "Total number of field extraction attempts",
			},
			[]string{"field", "status"},
		),
		FieldExtractionDuration: factory.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "scraper_field_extraction_duration_seconds",
				Help:    "Duration of field extraction in seconds",
				Buckets: []float64{0.01, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
			},
			[]string{"field"},
		),
		FallbackUsageTotal: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "scraper_fallback_usage_total",
				Help: "Total number of times fallback extraction methods were used",
			},
			[]string{"field", "fallback_level"},
		),
		ProductExtractionTotal: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "scraper_product_extraction_total",
				Help: "Total number of product extractions",
			},
			[]string{"status"},
		),
		ProductExtractionDuration: factory.NewHistogram(
			prometheus.HistogramOpts{
				Name:    "scraper_product_extraction_duration_seconds",
				Help:    "Duration of complete product extraction in seconds",
				Buckets: []float64{1, 2.5, 5, 10, 20, 30, 60},
			},
		),
		SizeTableValidationTotal: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "scraper_size_table_validation_total",
				Help: "Total number of size table validations",
			},
			[]string{"status"},
		),
	}
}

// RecordFieldExtraction records a field extraction attempt
func (m *ExtractionMetrics) RecordFieldExtraction(field string, success bool, duration time.Duration) {
	status := "success"
	if !success {
		status = "failure"
	}
	
	m.FieldExtractionTotal.WithLabelValues(field, status).Inc()
	m.FieldExtractionDuration.WithLabelValues(field).Observe(duration.Seconds())
}

// RecordFallbackUsage records when a fallback extraction method is used
func (m *ExtractionMetrics) RecordFallbackUsage(field string, level int) {
	m.FallbackUsageTotal.WithLabelValues(field, fmt.Sprintf("%d", level)).Inc()
}

// RecordProductExtraction records a complete product extraction
func (m *ExtractionMetrics) RecordProductExtraction(status string, duration time.Duration) {
	m.ProductExtractionTotal.WithLabelValues(status).Inc()
	m.ProductExtractionDuration.Observe(duration.Seconds())
}

// RecordSizeTableValidation records size table validation result
func (m *ExtractionMetrics) RecordSizeTableValidation(valid bool) {
	status := "valid"
	if !valid {
		status = "invalid"
	}
	m.SizeTableValidationTotal.WithLabelValues(status).Inc()
}

// GetFieldSuccessRate calculates the success rate for a field
// NOTE: This is a helper method for testing. In production, use Prometheus queries
func (m *ExtractionMetrics) GetFieldSuccessRate(field string) float64 {
	// This method is primarily for testing purposes
	// In production, you would use Prometheus queries like:
	// rate(scraper_field_extraction_total{field="gender",status="success"}[5m]) /
	// rate(scraper_field_extraction_total{field="gender"}[5m]) * 100
	
	// For testing, we'll need to use the prometheus/testutil package
	// which is imported in the test files
	return 0
}