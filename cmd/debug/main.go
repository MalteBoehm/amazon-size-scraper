package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/maltedev/amazon-size-scraper/internal/browser"
	"github.com/maltedev/amazon-size-scraper/internal/config"
	"github.com/maltedev/amazon-size-scraper/pkg/logger"
	"github.com/playwright-community/playwright-go"
)

func main() {
	var (
		url        = flag.String("url", "", "URL to debug")
		screenshot = flag.String("screenshot", "debug.png", "Screenshot filename")
		html       = flag.String("html", "debug.html", "HTML output filename")
	)
	flag.Parse()

	if *url == "" {
		fmt.Println("Please provide a URL with -url")
		os.Exit(1)
	}

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	logger := logger.New(cfg.Logging.Level, cfg.Logging.Format)
	logger.Info("Starting Debug Mode")

	browserOpts := &browser.Options{
		Headless:       false,
		Timeout:        30 * time.Second,
		ViewportWidth:  1920,
		ViewportHeight: 1080,
		AcceptLanguage: "de-DE,de;q=0.9,en;q=0.8",
		TimezoneID:     "Europe/Berlin",
		Locale:         "de-DE",
	}

	b, err := browser.New(browserOpts)
	if err != nil {
		logger.Error("Failed to initialize browser", "error", err)
		os.Exit(1)
	}
	defer b.Close()

	page, err := b.NewPage()
	if err != nil {
		logger.Error("Failed to create page", "error", err)
		os.Exit(1)
	}
	defer page.Close()

	logger.Info("Navigating to URL", "url", *url)
	
	if err := b.NavigateWithRetry(page, *url, 3); err != nil {
		logger.Error("Failed to navigate", "error", err)
		os.Exit(1)
	}

	// Wait for page to load
	time.Sleep(5 * time.Second)

	// Take screenshot
	if _, err := page.Screenshot(playwright.PageScreenshotOptions{
		Path: playwright.String(*screenshot),
		FullPage: playwright.Bool(true),
	}); err != nil {
		logger.Error("Failed to take screenshot", "error", err)
	} else {
		logger.Info("Screenshot saved", "file", *screenshot)
	}

	// Save HTML
	content, err := page.Content()
	if err != nil {
		logger.Error("Failed to get content", "error", err)
	} else {
		if err := os.WriteFile(*html, []byte(content), 0644); err != nil {
			logger.Error("Failed to save HTML", "error", err)
		} else {
			logger.Info("HTML saved", "file", *html)
		}
	}

	// Try to find products with different selectors
	selectors := []string{
		"[data-component-type='s-search-result']",
		"[data-asin]",
		".s-result-item",
		".sg-col-inner",
		"div[data-index]",
		".s-card-container",
		".s-search-results",
	}

	for _, selector := range selectors {
		count, _ := page.Locator(selector).Count()
		if count > 0 {
			logger.Info("Found elements", "selector", selector, "count", count)
			
			// Get first few data-asin values
			elements, _ := page.Locator(selector).All()
			for i, elem := range elements {
				if i >= 3 {
					break
				}
				
				asin, _ := elem.GetAttribute("data-asin")
				text, _ := elem.TextContent()
				if asin != "" {
					logger.Info("Sample element", "index", i, "asin", asin, "text_length", len(text))
				}
			}
		}
	}

	// Check for captcha or blocks
	captchaSelectors := []string{
		"#captchacharacters",
		"form[action*='Captcha']",
		".a-box-inner:has-text('Robot')",
		"img[src*='captcha']",
	}

	for _, selector := range captchaSelectors {
		if count, _ := page.Locator(selector).Count(); count > 0 {
			logger.Warn("Captcha detected!", "selector", selector)
		}
	}

	// Check page title
	title, _ := page.Title()
	logger.Info("Page title", "title", title)

	// Wait for user to close
	fmt.Println("\nPress Ctrl+C to exit...")
	select {}
}