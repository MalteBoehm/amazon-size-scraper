package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// PA-API request metrics
	PAAPIRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "paapi_requests_total",
			Help: "Total number of PA-API requests",
		},
		[]string{"operation", "status"},
	)

	// PA-API request duration
	PAAPIRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "paapi_request_duration_seconds",
			Help:    "Duration of PA-API requests in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"operation"},
	)

	// Color enrichment metrics
	PAAPIColorEnrichmentTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "paapi_color_enrichment_total",
			Help: "Total number of color enrichment operations",
		},
		[]string{"status", "reason"},
	)

	// Variations found metrics
	PAAPIVariationsFound = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "paapi_variations_found",
			Help:    "Number of color variations found per product",
			Buckets: []float64{0, 1, 2, 3, 5, 10, 15, 20, 30, 50},
		},
	)

	// Rate limit metrics
	PAAPIRateLimitHits = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "paapi_rate_limit_hits_total",
			Help: "Total number of rate limit hits",
		},
	)

	// Retry metrics
	PAAPIRetries = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "paapi_retries_total",
			Help: "Total number of API call retries",
		},
		[]string{"reason"},
	)

	// Batch processing metrics
	PAAPIBatchSize = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "paapi_batch_size",
			Help:    "Size of batches processed",
			Buckets: []float64{1, 2, 3, 5, 10, 15, 20},
		},
	)

	// GetVariations pagination metrics
	PAAPIGetVariationsPages = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "paapi_get_variations_pages",
			Help:    "Number of pages retrieved per GetVariations call",
			Buckets: []float64{1, 2, 3, 5, 10, 15, 20},
		},
	)

	// Error metrics by error type
	PAAPIErrors = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "paapi_errors_total",
			Help: "Total number of PA-API errors by type",
		},
		[]string{"error_code", "operation"},
	)

	// Cache metrics (for future implementation)
	PAAPICacheHits = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "paapi_cache_hits_total",
			Help: "Total number of cache hits for PA-API responses",
		},
	)

	PAAPICacheMisses = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "paapi_cache_misses_total",
			Help: "Total number of cache misses for PA-API responses",
		},
	)
)

// RecordPAAPIRequest records metrics for a PA-API request
func RecordPAAPIRequest(operation string, success bool, duration float64) {
	status := "success"
	if !success {
		status = "failure"
	}
	PAAPIRequestsTotal.WithLabelValues(operation, status).Inc()
	PAAPIRequestDuration.WithLabelValues(operation).Observe(duration)
}

// RecordColorEnrichment records color enrichment metrics
func RecordColorEnrichment(success bool, reason string, variationCount int) {
	status := "success"
	if !success {
		status = "failure"
	}
	PAAPIColorEnrichmentTotal.WithLabelValues(status, reason).Inc()
	if success && variationCount > 0 {
		PAAPIVariationsFound.Observe(float64(variationCount))
	}
}

// RecordRateLimit records a rate limit hit
func RecordRateLimit() {
	PAAPIRateLimitHits.Inc()
}

// RecordRetry records an API retry
func RecordRetry(reason string) {
	PAAPIRetries.WithLabelValues(reason).Inc()
}

// RecordBatchSize records the size of a batch being processed
func RecordBatchSize(size int) {
	PAAPIBatchSize.Observe(float64(size))
}

// RecordGetVariationsPages records the number of pages retrieved
func RecordGetVariationsPages(pages int) {
	PAAPIGetVariationsPages.Observe(float64(pages))
}

// RecordPAAPIError records a PA-API error
func RecordPAAPIError(errorCode, operation string) {
	PAAPIErrors.WithLabelValues(errorCode, operation).Inc()
}

// RecordCacheHit records a cache hit
func RecordCacheHit() {
	PAAPICacheHits.Inc()
}

// RecordCacheMiss records a cache miss
func RecordCacheMiss() {
	PAAPICacheMisses.Inc()
}