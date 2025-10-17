package main

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/maltedev/amazon-size-scraper/internal/browser"
	"github.com/playwright-community/playwright-go"
)

func main() {
	// Browser setup
	browserOpts := browser.DefaultOptions()
	browserOpts.Headless = false // Need to see what's happening

	b, err := browser.New(browserOpts)
	if err != nil {
		log.Fatal("Failed to create browser:", err)
	}
	defer b.Close()

	page, err := b.NewPage()
	if err != nil {
		log.Fatal("Failed to create page:", err)
	}

	// First go to Amazon.de
	fmt.Println("1. Navigating to Amazon.de...")
	if _, err := page.Goto("https://www.amazon.de", playwright.PageGotoOptions{
		WaitUntil: playwright.WaitUntilStateDomcontentloaded,
	}); err != nil {
		log.Fatal("Failed to navigate:", err)
	}

	time.Sleep(2 * time.Second)

	// Check for bot protection
	content, _ := page.Content()
	if strings.Contains(content, "Klicke auf die Schaltfläche unten") {
		fmt.Println("2. Bot protection detected! Looking for button...")
		
		// Try to find and click the button
		buttonSelectors := []string{
			`button:has-text("Weiter shoppen")`,
			`input[type="submit"][value*="Weiter"]`,
			`.a-button-primary`,
			`button`,
		}

		for _, selector := range buttonSelectors {
			button := page.Locator(selector).First()
			count, _ := button.Count()
			if count > 0 {
				text, _ := button.TextContent()
				fmt.Printf("   Found button: %s (text: %s)\n", selector, strings.TrimSpace(text))
				
				if err := button.Click(); err == nil {
					fmt.Println("   ✓ Clicked button!")
					time.Sleep(3 * time.Second)
					break
				}
			}
		}
	}

	// Now try the search
	searchURL := "https://www.amazon.de/s?k=t-shirt+größentabelle+länge&i=fashion"
	fmt.Printf("\n3. Navigating to search: %s\n", searchURL)
	
	if _, err := page.Goto(searchURL); err != nil {
		log.Fatal("Failed to navigate to search:", err)
	}

	time.Sleep(3 * time.Second)

	// Check result
	title, _ := page.Title()
	fmt.Printf("\n4. Page title: %s\n", title)

	if strings.Contains(title, "Tut uns Leid") {
		fmt.Println("   ❌ Still getting error page")
	} else {
		// Count products
		products := page.Locator(`[data-component-type="s-search-result"]`)
		count, _ := products.Count()
		fmt.Printf("   ✅ Success! Found %d products\n", count)
		
		// Get first few ASINs
		for i := 0; i < 5 && i < count; i++ {
			asin, _ := products.Nth(i).GetAttribute("data-asin")
			fmt.Printf("   - ASIN: %s\n", asin)
		}
	}

	fmt.Println("\nPress Enter to close...")
	fmt.Scanln()
}