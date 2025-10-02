package browser

import (
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/maltedev/amazon-size-scraper/internal/proxy"
	"github.com/playwright-community/playwright-go"
)

// BrowserPool manages a pool of browsers with proxy rotation
type BrowserPool struct {
	browsers      []*Browser
	proxies       *proxy.ProxyManager
	baseOptions   *Options
	poolSize      int
	logger        *slog.Logger
	mu            sync.RWMutex
	requestCounts []int // Track requests per browser for rotation
	maxRequests   int   // Max requests before recreating browser
}

// PoolConfig configures the browser pool
type PoolConfig struct {
	PoolSize         int           // Number of browsers in pool
	MaxRequests      int           // Max requests per browser before recreation
	RecreationWindow time.Duration // Time window for recreation
}

// DefaultPoolConfig returns default pool configuration
func DefaultPoolConfig() *PoolConfig {
	return &PoolConfig{
		PoolSize:         3, // 3 browsers with different proxies
		MaxRequests:      20, // Recreate after 20 requests
		RecreationWindow: 10 * time.Minute,
	}
}

// NewBrowserPool creates a new browser pool with proxy rotation
func NewBrowserPool(proxyManager *proxy.ProxyManager, baseOptions *Options, config *PoolConfig, logger *slog.Logger) (*BrowserPool, error) {
	if config == nil {
		config = DefaultPoolConfig()
	}

	pool := &BrowserPool{
		browsers:      make([]*Browser, config.PoolSize),
		proxies:       proxyManager,
		baseOptions:   baseOptions,
		poolSize:      config.PoolSize,
		logger:        logger.With("component", "browser-pool"),
		requestCounts: make([]int, config.PoolSize),
		maxRequests:   config.MaxRequests,
	}

	// Initialize browsers with different proxies
	for i := 0; i < config.PoolSize; i++ {
		browser, err := pool.createBrowserWithProxy(i)
		if err != nil {
			// Clean up already created browsers
			for j := 0; j < i; j++ {
				if pool.browsers[j] != nil {
					pool.browsers[j].Close()
				}
			}
			return nil, fmt.Errorf("failed to create browser %d: %w", i, err)
		}
		pool.browsers[i] = browser
	}

	pool.logger.Info("browser pool initialized", "size", config.PoolSize, "available_proxies", proxyManager.GetProxyCount())
	return pool, nil
}

// GetBrowser returns a browser from the pool using round-robin
func (bp *BrowserPool) GetBrowser() (*Browser, error) {
	bp.mu.Lock()
	defer bp.mu.Unlock()

	// Find browser with lowest request count
	minIdx := 0
	minCount := bp.requestCounts[0]
	for i := 1; i < bp.poolSize; i++ {
		if bp.requestCounts[i] < minCount {
			minIdx = i
			minCount = bp.requestCounts[i]
		}
	}

	// Check if browser needs recreation
	if bp.requestCounts[minIdx] >= bp.maxRequests {
		bp.logger.Info("recreating browser due to request limit", "index", minIdx, "requests", bp.requestCounts[minIdx])
		if err := bp.recreateBrowser(minIdx); err != nil {
			bp.logger.Error("failed to recreate browser", "index", minIdx, "error", err)
			// Continue with existing browser as fallback
		}
	}

	// Increment request count
	bp.requestCounts[minIdx]++
	
	return bp.browsers[minIdx], nil
}

// GetBrowserWithNewProxy returns a browser with a fresh proxy (forces recreation)
func (bp *BrowserPool) GetBrowserWithNewProxy() (*Browser, error) {
	bp.mu.Lock()
	defer bp.mu.Unlock()

	// Find browser with highest request count (most used)
	maxIdx := 0
	maxCount := bp.requestCounts[0]
	for i := 1; i < bp.poolSize; i++ {
		if bp.requestCounts[i] > maxCount {
			maxIdx = i
			maxCount = bp.requestCounts[i]
		}
	}

	bp.logger.Info("forcing browser recreation for new proxy", "index", maxIdx)
	if err := bp.recreateBrowser(maxIdx); err != nil {
		return nil, fmt.Errorf("failed to recreate browser with new proxy: %w", err)
	}

	bp.requestCounts[maxIdx]++
	return bp.browsers[maxIdx], nil
}

// MarkBrowserFailed marks a browser as failed and recreates it
func (bp *BrowserPool) MarkBrowserFailed(browser *Browser, reason string) error {
	bp.mu.Lock()
	defer bp.mu.Unlock()

	// Find browser index
	var browserIdx = -1
	for i, b := range bp.browsers {
		if b == browser {
			browserIdx = i
			break
		}
	}

	if browserIdx == -1 {
		return fmt.Errorf("browser not found in pool")
	}

	bp.logger.Warn("marking browser as failed", "index", browserIdx, "reason", reason)
	
	// Mark proxy as failed if the failure might be proxy-related
	if reason == "proxy_error" || reason == "connection_error" || reason == "bot_detected" {
		// Extract proxy info from browser options (we'll need to store this)
		// For now, we'll recreate with a new proxy
	}

	return bp.recreateBrowser(browserIdx)
}

// createBrowserWithProxy creates a new browser with a proxy from the manager
func (bp *BrowserPool) createBrowserWithProxy(index int) (*Browser, error) {
	proxy, err := bp.proxies.GetNextProxy()
	if err != nil {
		return nil, fmt.Errorf("failed to get proxy: %w", err)
	}

	// Create options with proxy configuration
	opts := *bp.baseOptions // Copy base options
	opts.ProxyServer = fmt.Sprintf("http://%s:%s", proxy.Host, proxy.Port)
	opts.ProxyUsername = proxy.Username
	opts.ProxyPassword = proxy.Password

	bp.logger.Debug("creating browser with proxy", "index", index, "proxy_host", proxy.Host)

	browser, err := New(&opts)
	if err != nil {
		// Mark proxy as failed
		bp.proxies.MarkProxyFailed(proxy)
		return nil, fmt.Errorf("failed to create browser with proxy %s:%s: %w", proxy.Host, proxy.Port, err)
	}

	return browser, nil
}

// recreateBrowser recreates a browser at the given index with a new proxy
func (bp *BrowserPool) recreateBrowser(index int) error {
	// Close existing browser
	if bp.browsers[index] != nil {
		bp.browsers[index].Close()
	}

	// Create new browser with new proxy
	browser, err := bp.createBrowserWithProxy(index)
	if err != nil {
		return fmt.Errorf("failed to recreate browser: %w", err)
	}

	bp.browsers[index] = browser
	bp.requestCounts[index] = 0 // Reset request count

	bp.logger.Info("browser recreated", "index", index)
	return nil
}

// GetPoolStats returns statistics about the browser pool
func (bp *BrowserPool) GetPoolStats() map[string]interface{} {
	bp.mu.RLock()
	defer bp.mu.RUnlock()

	stats := map[string]interface{}{
		"pool_size":          bp.poolSize,
		"max_requests":       bp.maxRequests,
		"request_counts":     make([]int, len(bp.requestCounts)),
		"healthy_proxies":    bp.proxies.GetHealthyProxyCount(),
		"total_proxies":      bp.proxies.GetProxyCount(),
	}

	copy(stats["request_counts"].([]int), bp.requestCounts)
	return stats
}

// Close closes all browsers in the pool
func (bp *BrowserPool) Close() error {
	bp.mu.Lock()
	defer bp.mu.Unlock()

	var errors []error
	for i, browser := range bp.browsers {
		if browser != nil {
			if err := browser.Close(); err != nil {
				errors = append(errors, fmt.Errorf("failed to close browser %d: %w", i, err))
			}
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("errors closing browsers: %v", errors)
	}

	bp.logger.Info("browser pool closed")
	return nil
}

// RecreateAllBrowsers recreates all browsers with new proxies (useful for periodic refresh)
func (bp *BrowserPool) RecreateAllBrowsers() error {
	bp.mu.Lock()
	defer bp.mu.Unlock()

	bp.logger.Info("recreating all browsers with new proxies")
	
	var errors []error
	for i := 0; i < bp.poolSize; i++ {
		if err := bp.recreateBrowser(i); err != nil {
			errors = append(errors, fmt.Errorf("failed to recreate browser %d: %w", i, err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("errors recreating browsers: %v", errors)
	}

	return nil
}

// NewPageFromPool creates a new page from a browser in the pool
func (bp *BrowserPool) NewPage() (playwright.Page, *Browser, error) {
	browser, err := bp.GetBrowser()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get browser from pool: %w", err)
	}

	page, err := browser.NewPage()
	if err != nil {
		// Try to get a different browser if page creation fails
		bp.logger.Warn("failed to create page, trying different browser", "error", err)
		browser, err2 := bp.GetBrowserWithNewProxy()
		if err2 != nil {
			return nil, nil, fmt.Errorf("failed to get alternative browser: %w", err2)
		}
		
		page, err = browser.NewPage()
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create page with alternative browser: %w", err)
		}
	}

	return page, browser, nil
}