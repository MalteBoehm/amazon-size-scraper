package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// ScraperMetrics holds general scraper metrics
type ScraperMetrics struct {
	// Page load metrics
	PageLoadTotal      *prometheus.CounterVec
	PageLoadDuration   *prometheus.HistogramVec
	
	// Anti-bot detection
	CaptchaDetectedTotal prometheus.Counter
	
	// Rate limiting
	RateLimitDelay prometheus.Histogram
	
	// Browser metrics
	BrowserRestartTotal prometheus.Counter
	ActivePages         prometheus.Gauge
}

// NewScraperMetrics creates new scraper metrics
func NewScraperMetrics() *ScraperMetrics {
	return NewScraperMetricsWithRegistry(prometheus.DefaultRegisterer)
}

// NewScraperMetricsWithRegistry creates new scraper metrics with a specific registry
func NewScraperMetricsWithRegistry(reg prometheus.Registerer) *ScraperMetrics {
	factory := promauto.With(reg)
	
	return &ScraperMetrics{
		PageLoadTotal: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "scraper_page_load_total",
				Help: "Total number of page load attempts",
			},
			[]string{"status"},
		),
		PageLoadDuration: factory.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "scraper_page_load_duration_seconds",
				Help:    "Duration of page loads in seconds",
				Buckets: []float64{0.5, 1, 2.5, 5, 10, 20, 30},
			},
			[]string{"status"},
		),
		CaptchaDetectedTotal: factory.NewCounter(
			prometheus.CounterOpts{
				Name: "scraper_captcha_detected_total",
				Help: "Total number of times CAPTCHA was detected",
			},
		),
		RateLimitDelay: factory.NewHistogram(
			prometheus.HistogramOpts{
				Name:    "scraper_rate_limit_delay_seconds",
				Help:    "Rate limit delay duration in seconds",
				Buckets: []float64{1, 5, 10, 15, 20, 30, 60},
			},
		),
		BrowserRestartTotal: factory.NewCounter(
			prometheus.CounterOpts{
				Name: "scraper_browser_restart_total",
				Help: "Total number of browser restarts",
			},
		),
		ActivePages: factory.NewGauge(
			prometheus.GaugeOpts{
				Name: "scraper_active_pages",
				Help: "Number of currently active browser pages",
			},
		),
	}
}

// RecordPageLoad records a page load attempt
func (m *ScraperMetrics) RecordPageLoad(url string, success bool, duration time.Duration) {
	status := "success"
	if !success {
		status = "failure"
	}
	
	m.PageLoadTotal.WithLabelValues(status).Inc()
	m.PageLoadDuration.WithLabelValues(status).Observe(duration.Seconds())
}

// RecordCaptchaDetection records CAPTCHA detection
func (m *ScraperMetrics) RecordCaptchaDetection() {
	m.CaptchaDetectedTotal.Inc()
}

// RecordRateLimitDelay records rate limiting delay
func (m *ScraperMetrics) RecordRateLimitDelay(delay time.Duration) {
	m.RateLimitDelay.Observe(delay.Seconds())
}

// RecordBrowserRestart records browser restart
func (m *ScraperMetrics) RecordBrowserRestart() {
	m.BrowserRestartTotal.Inc()
}

// SetActivePages sets the number of active pages
func (m *ScraperMetrics) SetActivePages(count float64) {
	m.ActivePages.Set(count)
}

// IncrementActivePages increments active page count
func (m *ScraperMetrics) IncrementActivePages() {
	m.ActivePages.Inc()
}

// DecrementActivePages decrements active page count
func (m *ScraperMetrics) DecrementActivePages() {
	m.ActivePages.Dec()
}