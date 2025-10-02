package scraper

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"github.com/maltedev/amazon-size-scraper/internal/browser"
	"github.com/maltedev/amazon-size-scraper/internal/config"
	"github.com/maltedev/amazon-size-scraper/internal/models"
	"github.com/maltedev/amazon-size-scraper/internal/parser"
	"github.com/playwright-community/playwright-go"
)

const (
	amazonDEBaseURL   = "https://www.amazon.de"
	productURLPattern = `(?i)(?:https?://)?(?:www\.)?amazon\.de/.*?/dp/([A-Z0-9]{10})`
)

type AmazonScraper struct {
	browserPool   *browser.BrowserPool
	singleBrowser *browser.Browser
	parser        parser.Parser
	logger        *slog.Logger
	rateLimit     time.Duration
	lastScrape    time.Time
	config        *config.Config
}

func NewAmazonScraper(pool *browser.BrowserPool, p parser.Parser, logger *slog.Logger, cfg *config.Config) *AmazonScraper {
	return &AmazonScraper{
		browserPool: pool,
		parser:      p,
		logger:      logger,
		rateLimit:   5 * time.Second,
		config:      cfg,
	}
}

// NewAmazonScraperLegacy creates a scraper with a single browser (for backward compatibility)
func NewAmazonScraperLegacy(b *browser.Browser, p parser.Parser, logger *slog.Logger, cfg *config.Config) *AmazonScraper {
	return &AmazonScraper{
		browserPool:   nil, // Fall back to single browser mode
		singleBrowser: b,
		parser:        p,
		logger:        logger,
		rateLimit:     5 * time.Second,
		config:        cfg,
	}
}

func (s *AmazonScraper) ScrapeProduct(ctx context.Context, url string) (*models.Product, error) {
	asin, err := s.ExtractASIN(url)
	if err != nil {
		return nil, fmt.Errorf("failed to extract ASIN: %w", err)
	}

	return s.ScrapeByASIN(ctx, asin)
}

func (s *AmazonScraper) ScrapeByASIN(ctx context.Context, asin string) (*models.Product, error) {
	s.enforceRateLimit()

	url := fmt.Sprintf("%s/dp/%s", amazonDEBaseURL, asin)
	s.logger.Info("scraping product", "asin", asin, "url", url)

	var page playwright.Page
	var currentBrowser *browser.Browser
	var err error

	// Use browser pool if available, otherwise fall back to legacy mode
	if s.browserPool != nil {
		page, currentBrowser, err = s.browserPool.NewPage()
		if err != nil {
			return nil, fmt.Errorf("failed to create page from pool: %w", err)
		}
	} else if s.singleBrowser != nil {
		page, err = s.singleBrowser.NewPage()
		if err != nil {
			return nil, fmt.Errorf("failed to create page: %w", err)
		}
		currentBrowser = s.singleBrowser
	} else {
		return nil, fmt.Errorf("no browser available (neither pool nor single browser)")
	}
	defer page.Close()

	if err := s.humanizeInteraction(page); err != nil {
		s.logger.Warn("failed to humanize interaction", "error", err)
	}

	// Use browser pool's navigation with retry
	if err := currentBrowser.NavigateWithRetry(page, url, 3); err != nil {
		// Mark browser as failed if navigation consistently fails
		s.browserPool.MarkBrowserFailed(currentBrowser, "navigation_failed")
		return nil, fmt.Errorf("failed to navigate: %w", err)
	}

	if blocked := s.checkIfBlocked(page); blocked {
		// Mark browser as failed due to bot detection
		s.browserPool.MarkBrowserFailed(currentBrowser, "bot_detected")
		return nil, ErrBlocked
	}

	time.Sleep(2 * time.Second)

	html, err := page.Content()
	if err != nil {
		return nil, fmt.Errorf("failed to get page content: %w", err)
	}

	// Save HTML for debugging if configured
	if s.config != nil && s.config.Logging.LogHTML {
		if err := saveHTML(ctx, s.logger, s.config.Logging.HTMLDir, asin, "product_page", html); err != nil {
			s.logger.Warn("failed to save HTML for debugging", "error", err, "asin", asin)
		}
	}

	product, err := s.parser.ParseProductPage(html, asin)
	if err != nil {
		return nil, fmt.Errorf("failed to parse product: %w", err)
	}

	product.URL = url
	product.ASIN = asin

	return product, nil
}

func (s *AmazonScraper) ExtractASIN(url string) (string, error) {
	re := regexp.MustCompile(productURLPattern)
	matches := re.FindStringSubmatch(url)

	if len(matches) < 2 {
		return "", ErrInvalidURL
	}

	return matches[1], nil
}

func (s *AmazonScraper) Close() error {
	if s.browserPool != nil {
		return s.browserPool.Close()
	}
	return nil
}

// GetPoolStats returns browser pool statistics
func (s *AmazonScraper) GetPoolStats() map[string]interface{} {
	if s.browserPool != nil {
		return s.browserPool.GetPoolStats()
	}
	return map[string]interface{}{
		"pool_enabled": false,
	}
}

// RecreateAllBrowsers recreates all browsers with new proxies
func (s *AmazonScraper) RecreateAllBrowsers() error {
	if s.browserPool != nil {
		return s.browserPool.RecreateAllBrowsers()
	}
	return fmt.Errorf("browser pool not enabled")
}

func (s *AmazonScraper) enforceRateLimit() {
	elapsed := time.Since(s.lastScrape)
	if elapsed < s.rateLimit {
		time.Sleep(s.rateLimit - elapsed)
	}
	s.lastScrape = time.Now()
}

func (s *AmazonScraper) humanizeInteraction(page playwright.Page) error {
	viewport := page.ViewportSize()
	if viewport == nil {
		return nil
	}

	x := float64(viewport.Width / 2)
	y := float64(viewport.Height / 2)

	page.Mouse().Move(x, y, playwright.MouseMoveOptions{
		Steps: playwright.Int(10),
	})

	time.Sleep(100 * time.Millisecond)

	page.Mouse().Move(x+50, y+50, playwright.MouseMoveOptions{
		Steps: playwright.Int(5),
	})

	return nil
}

func (s *AmazonScraper) checkIfBlocked(page playwright.Page) bool {
	captchaSelectors := []string{
		"#captchacharacters",
		".a-box-inner h4:has-text('Robot')",
		"form[action*='Captcha']",
	}

	for _, selector := range captchaSelectors {
		if count, _ := page.Locator(selector).Count(); count > 0 {
			s.logger.Warn("detected captcha/block", "selector", selector)
			return true
		}
	}

	title, _ := page.Title()
	if strings.Contains(strings.ToLower(title), "robot") {
		s.logger.Warn("detected robot check in title", "title", title)
		return true
	}

	return false
}
