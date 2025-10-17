package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/maltedev/amazon-size-scraper/internal/browser"
	"github.com/maltedev/amazon-size-scraper/internal/database"
)

func main() {
	ctx := context.Background()

	// Database connection
	db, err := database.New(ctx, database.Config{
		Host:     "localhost",
		Port:     5433,
		User:     "postgres",
		Password: "postgres",
		Database: "tall_affiliate",
		MaxConns: 2,
		MinConns: 1,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

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
	fmt.Println("1. Going to Amazon.de...")
	b.NavigateWithRetry(page, "https://www.amazon.de", 1)
	
	fmt.Println("2. Going to search...")
	searchURL := "https://www.amazon.de/s?k=t-shirt+größentabelle+länge&i=fashion"
	b.NavigateWithRetry(page, searchURL, 1)
	
	time.Sleep(3 * time.Second)

	// Extract products with debugging
	fmt.Println("\n3. Extracting products...")
	products := page.Locator(`[data-component-type="s-search-result"]`)
	count, _ := products.Count()
	fmt.Printf("Found %d product elements\n", count)

	saved := 0
	for i := 0; i < 5 && i < count; i++ {
		fmt.Printf("\n--- Product %d ---\n", i+1)
		productEl := products.Nth(i)
		
		// Get ASIN
		asin, _ := productEl.GetAttribute("data-asin")
		fmt.Printf("ASIN: %s\n", asin)
		
		// Try different selectors for title
		titleSelectors := []string{
			"h2 a span",
			"h2 span",
			"span.a-size-base-plus",
			"span.a-size-medium",
			"[data-cy='title-recipe'] span",
		}
		
		var title string
		for _, selector := range titleSelectors {
			el := productEl.Locator(selector).First()
			if c, _ := el.Count(); c > 0 {
				if t, err := el.TextContent(); err == nil && t != "" {
					title = strings.TrimSpace(t)
					fmt.Printf("Title found with selector '%s': %s\n", selector, title)
					break
				}
			}
		}
		
		if title == "" {
			title = "No title found"
		}
		
		// Save to database
		p := &database.Product{
			ASIN:   asin,
			Title:  title,
			URL:    fmt.Sprintf("https://www.amazon.de/dp/%s", asin),
			Status: database.StatusPending,
		}
		
		if err := db.InsertProduct(ctx, p); err != nil {
			fmt.Printf("Error: %v\n", err)
		} else {
			saved++
		}
	}
	
	fmt.Printf("\n✅ Saved %d products\n", saved)
	
	// Keep browser open
	fmt.Println("\nPress Enter to close...")
	fmt.Scanln()
}