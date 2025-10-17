package browser

import (
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/playwright-community/playwright-go"
)

type Browser struct {
	pw      *playwright.Playwright
	browser playwright.Browser
	context playwright.BrowserContext
	logger  *slog.Logger
}

type Options struct {
	Headless        bool
	Timeout         time.Duration
	UserAgent       string
	ViewportWidth   int
	ViewportHeight  int
	AcceptLanguage  string
	TimezoneID      string
	Locale          string
	ProxyServer     string
	ExtraHeaders    map[string]string
}

func DefaultOptions() *Options {
	return &Options{
		Headless:       true,
		Timeout:        30 * time.Second,
		UserAgent:      "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		ViewportWidth:  1920,
		ViewportHeight: 1080,
		AcceptLanguage: "de-DE,de;q=0.9,en;q=0.8",
		TimezoneID:     "Europe/Berlin",
		Locale:         "de-DE",
		ExtraHeaders: map[string]string{
			"Accept":          "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8",
			"Accept-Encoding": "gzip, deflate, br",
			"DNT":             "1",
		},
	}
}

func New(opts *Options) (*Browser, error) {
	if opts == nil {
		opts = DefaultOptions()
	}

	pw, err := playwright.Run()
	if err != nil {
		return nil, fmt.Errorf("failed to start playwright: %w", err)
	}

	launchOpts := playwright.BrowserTypeLaunchOptions{
		Headless: &opts.Headless,
		Args: []string{
			"--disable-blink-features=AutomationControlled",
			"--disable-dev-shm-usage",
			"--no-sandbox",
			"--disable-setuid-sandbox",
			"--window-size=1920,1080",
			"--start-maximized",
			"--user-agent=" + opts.UserAgent,
		},
	}

	if opts.ProxyServer != "" {
		launchOpts.Proxy = &playwright.Proxy{
			Server: opts.ProxyServer,
		}
	}

	browser, err := pw.Chromium.Launch(launchOpts)
	if err != nil {
		pw.Stop()
		return nil, fmt.Errorf("failed to launch browser: %w", err)
	}

	contextOpts := playwright.BrowserNewContextOptions{
		UserAgent:      &opts.UserAgent,
		AcceptDownloads: playwright.Bool(false),
		JavaScriptEnabled: playwright.Bool(true), // Explicitly enable JavaScript
		Locale:         &opts.Locale,
		TimezoneId:     &opts.TimezoneID,
		Viewport: &playwright.Size{
			Width:  opts.ViewportWidth,
			Height: opts.ViewportHeight,
		},
		ExtraHttpHeaders: opts.ExtraHeaders,
	}

	context, err := browser.NewContext(contextOpts)
	if err != nil {
		browser.Close()
		pw.Stop()
		return nil, fmt.Errorf("failed to create browser context: %w", err)
	}

	return &Browser{
		pw:      pw,
		browser: browser,
		context: context,
		logger:  slog.Default().With("component", "browser"),
	}, nil
}

func (b *Browser) NewPage() (playwright.Page, error) {
	page, err := b.context.NewPage()
	if err != nil {
		return nil, fmt.Errorf("failed to create new page: %w", err)
	}

	page.SetDefaultTimeout(float64(DefaultOptions().Timeout.Milliseconds()))

	return page, nil
}

func (b *Browser) Context() playwright.BrowserContext {
	return b.context
}

func (b *Browser) Close() error {
	var errs []error

	if b.context != nil {
		if err := b.context.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close context: %w", err))
		}
	}

	if b.browser != nil {
		if err := b.browser.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close browser: %w", err))
		}
	}

	if b.pw != nil {
		if err := b.pw.Stop(); err != nil {
			errs = append(errs, fmt.Errorf("failed to stop playwright: %w", err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors during close: %v", errs)
	}

	return nil
}

func (b *Browser) NavigateWithRetry(page playwright.Page, url string, maxRetries int) error {
	var lastErr error
	
	for i := 0; i < maxRetries; i++ {
		if i > 0 {
			b.logger.Info("retrying navigation", "attempt", i+1, "url", url)
			time.Sleep(time.Duration(i+1) * time.Second)
		}
		
		_, err := page.Goto(url, playwright.PageGotoOptions{
			WaitUntil: playwright.WaitUntilStateDomcontentloaded,
			Timeout:   playwright.Float(30000),
		})
		
		if err == nil {
			// Check for bot protection after successful navigation
			protected, err := b.CheckAndBypassBotProtection(page)
			if err != nil {
				b.logger.Error("failed to check bot protection", "error", err)
				lastErr = err
				continue
			}
			if protected {
				b.logger.Info("bot protection bypassed")
			}
			return nil
		}
		
		lastErr = err
		b.logger.Error("navigation failed", "error", err, "attempt", i+1)
	}
	
	return fmt.Errorf("failed after %d retries: %w", maxRetries, lastErr)
}

// CheckAndBypassBotProtection checks for Amazon bot protection and attempts to bypass it
func (b *Browser) CheckAndBypassBotProtection(page playwright.Page) (bool, error) {
	// Wait a bit for page to fully load
	time.Sleep(2 * time.Second)
	
	// Check page title for bot check indicators
	title, err := page.Title()
	if err != nil {
		return false, fmt.Errorf("failed to get page title: %w", err)
	}
	
	b.logger.Debug("checking page", "title", title)
	
	// Check page content for bot protection
	content, err := page.Content()
	if err != nil {
		return false, fmt.Errorf("failed to get page content: %w", err)
	}
	
	// Look for German bot check indicators
	if strings.Contains(content, "Klicke auf die Schaltfläche unten") ||
	   strings.Contains(content, "Weiter shoppen") {
		b.logger.Info("bot protection detected, attempting bypass")
		
		// Try different button selectors
		buttonSelectors := []string{
			`button:has-text("Weiter shoppen")`,
			`input[type="submit"][value*="Weiter"]`,
			`.a-button-primary`,
			`button.a-button-text`,
		}
		
		for _, selector := range buttonSelectors {
			button := page.Locator(selector).First()
			
			// Check if button exists
			count, err := button.Count()
			if err != nil || count == 0 {
				continue
			}
			
			b.logger.Info("found bot check button", "selector", selector)
			
			// Click the button
			if err := button.Click(); err != nil {
				b.logger.Error("failed to click button", "error", err)
				continue
			}
			
			// Wait for navigation
			time.Sleep(3 * time.Second)
			
			// Verify we're past the check
			newContent, _ := page.Content()
			
			if !strings.Contains(newContent, "Klicke auf die Schaltfläche unten") {
				b.logger.Info("successfully bypassed bot protection")
				return true, nil
			}
		}
		
		return false, fmt.Errorf("could not find button to bypass bot protection")
	}
	
	// Check for "Tut uns Leid" error page
	if strings.Contains(title, "Tut uns Leid") || strings.Contains(content, "Tut uns Leid") {
		return false, fmt.Errorf("Amazon error page detected")
	}
	
	return false, nil
}

// HumanizeInteraction adds human-like behavior to page interactions
func (b *Browser) HumanizeInteraction(page playwright.Page) error {
	// Random mouse movements
	for i := 0; i < 3; i++ {
		x := float64(100 + i*200)
		y := float64(100 + i*150)
		page.Mouse().Move(x, y)
		time.Sleep(time.Millisecond * time.Duration(200+i*100))
	}
	
	// Random scroll
	page.Evaluate(`window.scrollBy(0, Math.random() * 300)`)
	time.Sleep(time.Second)
	
	return nil
}