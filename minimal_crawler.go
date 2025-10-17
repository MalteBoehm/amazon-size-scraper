package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/maltedev/amazon-size-scraper/internal/browser"
	"github.com/maltedev/amazon-size-scraper/internal/database"
)

func main() {
	ctx := context.Background()

	// Database
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
	fmt.Println("Going to Amazon.de...")
	b.NavigateWithRetry(page, "https://www.amazon.de", 1)
	
	searchURL := "https://www.amazon.de/s?k=t-shirt+größentabelle+länge&i=fashion"
	fmt.Println("Going to search...")
	b.NavigateWithRetry(page, searchURL, 1)
	
	time.Sleep(3 * time.Second)

	// Extract directly
	fmt.Println("\nExtracting products...")
	
	// Wait for products
	page.WaitForSelector(`[data-component-type="s-search-result"]`)
	
	// Get all products
	products := page.Locator(`[data-component-type="s-search-result"]`)
	count, _ := products.Count()
	fmt.Printf("Found %d products\n", count)
	
	// Extract and save
	saved := 0
	for i := 0; i < count; i++ {
		productEl := products.Nth(i)
		asin, err := productEl.GetAttribute("data-asin")
		if err != nil || asin == "" {
			continue
		}
		
		// Try to get title
		titleEl := productEl.Locator("h2 span").First()
		title := "Unknown"
		if t, err := titleEl.TextContent(); err == nil && t != "" {
			title = t
		}
		
		// Save
		p := &database.Product{
			ASIN:   asin,
			Title:  title,
			URL:    fmt.Sprintf("https://www.amazon.de/dp/%s", asin),
			Status: database.StatusPending,
		}
		
		if err := db.InsertProduct(ctx, p); err == nil {
			saved++
			if saved <= 5 {
				fmt.Printf("Saved: %s - %.50s\n", asin, title)
			}
		}
	}
	
	fmt.Printf("\n✅ Total saved: %d products\n", saved)
	
	// Check next page
	nextBtn := page.Locator(`a.s-pagination-next`).First()
	if c, _ := nextBtn.Count(); c > 0 {
		disabled, _ := nextBtn.GetAttribute("aria-disabled")
		if disabled != "true" {
			fmt.Println("Next page available!")
		}
	}
}