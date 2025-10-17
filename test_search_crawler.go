package main

import (
	"fmt"
	"log"
	"log/slog"
	"os"

	"github.com/maltedev/amazon-size-scraper/internal/browser"
	"github.com/playwright-community/playwright-go"
)

func main() {
	// Setup logging
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
	slog.SetDefault(logger)

	// Browser setup
	browserOpts := browser.DefaultOptions()
	browserOpts.Headless = false // So we can see what's happening

	b, err := browser.New(browserOpts)
	if err != nil {
		log.Fatal("Failed to create browser:", err)
	}
	defer b.Close()

	page, err := b.NewPage()
	if err != nil {
		log.Fatal("Failed to create page:", err)
	}

	// Test URL
	searchURL := "https://www.amazon.de/s?k=t-shirt+größentabelle+länge&i=fashion"
	
	logger.Info("Navigating to search page", "url", searchURL)
	
	// Navigate with bot protection bypass
	if err := b.NavigateWithRetry(page, searchURL, 3); err != nil {
		log.Fatal("Failed to navigate:", err)
	}

	// Wait for products to load
	logger.Info("Waiting for products to load...")
	page.WaitForSelector(`[data-component-type="s-search-result"]`, playwright.PageWaitForSelectorOptions{
		Timeout: playwright.Float(10000),
	})

	// Extract ASINs
	productElements := page.Locator(`[data-component-type="s-search-result"]`)
	count, err := productElements.Count()
	if err != nil {
		log.Fatal("Failed to count products:", err)
	}

	logger.Info("Found products", "count", count)

	asins := []string{}
	for i := 0; i < count; i++ {
		productEl := productElements.Nth(i)
		asin, err := productEl.GetAttribute("data-asin")
		if err != nil || asin == "" {
			continue
		}
		
		// Get title
		titleEl := productEl.Locator("h2 a span").First()
		title := ""
		if titleEl != nil {
			title, _ = titleEl.TextContent()
			if len(title) > 50 {
				title = title[:50] + "..."
			}
		}
		
		asins = append(asins, asin)
		fmt.Printf("ASIN: %s - %s\n", asin, title)
	}

	fmt.Printf("\nTotal ASINs collected: %d\n", len(asins))

	// Check for next page
	nextButton := page.Locator(`a.s-pagination-next`).First()
	if nextButton != nil {
		isDisabled, _ := nextButton.GetAttribute("aria-disabled")
		if isDisabled != "true" {
			fmt.Println("\nNext page available!")
		} else {
			fmt.Println("\nNo more pages")
		}
	}

	// Keep browser open for inspection
	fmt.Println("\nPress Enter to close browser...")
	fmt.Scanln()
}