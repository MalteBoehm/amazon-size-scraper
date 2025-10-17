package main

import (
	"fmt"
	"log"
	"time"

	"github.com/maltedev/amazon-size-scraper/internal/browser"
	"github.com/playwright-community/playwright-go"
)

func main() {
	// Browser
	b, err := browser.New(&browser.Options{Headless: false})
	if err != nil {
		log.Fatal(err)
	}
	defer b.Close()

	page, err := b.NewPage()
	if err != nil {
		log.Fatal(err)
	}

	// Navigate
	fmt.Println("Navigating to Amazon...")
	b.NavigateWithRetry(page, "https://www.amazon.de", 1)
	
	searchURL := "https://www.amazon.de/s?k=t-shirt+größentabelle+länge&i=fashion"
	fmt.Println("Going to search...")
	b.NavigateWithRetry(page, searchURL, 1)
	
	time.Sleep(3 * time.Second)

	// Test different selectors
	fmt.Println("\nTesting selectors:")
	
	selectors := []string{
		`[data-component-type="s-search-result"]`,
		`div[data-asin]`,
		`[data-asin]:not([data-asin=""])`,
		`.s-result-item[data-asin]`,
		`div.s-card-container`,
	}
	
	for _, selector := range selectors {
		elements := page.Locator(selector)
		count, _ := elements.Count()
		fmt.Printf("\n'%s': %d elements\n", selector, count)
		
		if count > 0 {
			// Get first ASIN
			first := elements.First()
			asin, _ := first.GetAttribute("data-asin")
			fmt.Printf("  First ASIN: %s\n", asin)
		}
	}
	
	// Wait for selector specifically
	fmt.Println("\nWaiting for search results...")
	_, err = page.WaitForSelector(`[data-component-type="s-search-result"]`, playwright.PageWaitForSelectorOptions{
		Timeout: playwright.Float(10000),
		State:   playwright.WaitForSelectorStateVisible,
	})
	if err != nil {
		fmt.Printf("Wait error: %v\n", err)
	}
	
	// Check page content
	content, _ := page.Content()
	if len(content) > 0 {
		fmt.Printf("\nPage content length: %d bytes\n", len(content))
	}
	
	fmt.Println("\nPress Enter to close...")
	fmt.Scanln()
}