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
	dbConfig := database.Config{
		Host:     "localhost",
		Port:     5433,
		User:     "postgres",
		Password: "postgres",
		Database: "tall_affiliate",
		MaxConns: 2,
		MinConns: 1,
	}

	db, err := database.New(ctx, dbConfig)
	if err != nil {
		log.Fatal("DB error:", err)
	}
	defer db.Close()

	// Browser
	browserOpts := browser.DefaultOptions()
	browserOpts.Headless = true

	b, err := browser.New(browserOpts)
	if err != nil {
		log.Fatal("Browser error:", err)
	}
	defer b.Close()

	page, err := b.NewPage()
	if err != nil {
		log.Fatal("Page error:", err)
	}

	// Navigate to Amazon.de first
	fmt.Println("1. Going to Amazon.de...")
	b.NavigateWithRetry(page, "https://www.amazon.de", 1)
	
	// Then search
	searchURL := "https://www.amazon.de/s?k=t-shirt+größentabelle+länge&i=fashion"
	fmt.Println("2. Going to search...")
	b.NavigateWithRetry(page, searchURL, 1)
	
	time.Sleep(2 * time.Second)

	// Extract products
	fmt.Println("3. Extracting products...")
	products := page.Locator(`[data-component-type="s-search-result"]`)
	count, _ := products.Count()
	
	fmt.Printf("Found %d products\n", count)
	
	// Save first 10 products
	saved := 0
	for i := 0; i < count && i < 10; i++ {
		productEl := products.Nth(i)
		asin, err := productEl.GetAttribute("data-asin")
		if err != nil || asin == "" {
			continue
		}
		
		titleEl := productEl.Locator("h2 a span").First()
		title := "Unknown"
		if t, err := titleEl.TextContent(); err == nil {
			title = t
		}
		
		// Save to DB
		p := &database.Product{
			ASIN:   asin,
			Title:  title,
			URL:    fmt.Sprintf("https://www.amazon.de/dp/%s", asin),
			Status: database.StatusPending,
		}
		
		if err := db.InsertProduct(ctx, p); err != nil {
			fmt.Printf("Error saving %s: %v\n", asin, err)
		} else {
			saved++
			fmt.Printf("Saved: %s - %.50s...\n", asin, title)
		}
	}
	
	fmt.Printf("\nTotal saved: %d\n", saved)
}