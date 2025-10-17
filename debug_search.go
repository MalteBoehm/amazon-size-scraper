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
	// Browser with visible window
	b, err := browser.New(&browser.Options{
		Headless: false,
		Timeout:  30 * time.Second,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer b.Close()

	page, err := b.NewPage()
	if err != nil {
		log.Fatal(err)
	}

	// Navigate to Amazon.de first
	fmt.Println("1. Going to Amazon.de...")
	b.NavigateWithRetry(page, "https://www.amazon.de", 1)
	
	time.Sleep(2 * time.Second)
	
	// Navigate to search
	searchURL := "https://www.amazon.de/s?k=t+shirt+%2B+%22gr%C3%B6%C3%9Fentabelle%22+%2B+%22l%C3%A4nge%22&i=fashion"
	fmt.Println("2. Going to search...")
	b.NavigateWithRetry(page, searchURL, 1)
	
	time.Sleep(3 * time.Second)
	
	// Find products
	fmt.Println("3. Looking for products...")
	
	// Check for products
	productCount, _ := page.Locator(`[data-component-type="s-search-result"]`).Count()
	fmt.Printf("Found %d products\n", productCount)
	
	// Get first 5 ASINs
	for i := 0; i < 5 && i < productCount; i++ {
		productEl := page.Locator(`[data-component-type="s-search-result"]`).Nth(i)
		asin, err := productEl.GetAttribute("data-asin")
		if err == nil && asin != "" {
			titleEl := productEl.Locator("h2 a span").First()
			title, _ := titleEl.TextContent()
			fmt.Printf("ASIN %d: %s - %s\n", i+1, asin, title)
		}
	}
	
	// Database connection to save these
	db, err := database.New(context.Background(), database.Config{
		Host:     "localhost",
		Port:     5433,
		User:     "postgres",
		Password: "postgres",
		Database: "amazon_scraper",
		MaxConns: 10,
	})
	if err != nil {
		log.Fatal("failed to connect to database:", err)
	}
	defer db.Close()
	
	// Save ASINs
	fmt.Println("\n4. Saving ASINs to database...")
	ctx := context.Background()
	
	for i := 0; i < productCount && i < 50; i++ {
		productEl := page.Locator(`[data-component-type="s-search-result"]`).Nth(i)
		asin, err := productEl.GetAttribute("data-asin")
		if err != nil || asin == "" {
			continue
		}
		
		titleEl := productEl.Locator("h2 a span").First()
		title, _ := titleEl.TextContent()
		
		product := &database.Product{
			ASIN:  asin,
			URL:   fmt.Sprintf("https://www.amazon.de/dp/%s", asin),
			Title: title,
		}
		
		if err := db.InsertProduct(ctx, product); err != nil {
			fmt.Printf("Error saving %s: %v\n", asin, err)
		} else {
			fmt.Printf("Saved: %s\n", asin)
		}
	}
	
	// Check for next page
	nextButton := page.Locator(`a:has-text("Weiter")`)
	if count, _ := nextButton.Count(); count > 0 {
		fmt.Println("\n5. Found 'Weiter' button, clicking...")
		nextButton.Click()
		time.Sleep(3 * time.Second)
		
		// Get more products
		productCount2, _ := page.Locator(`[data-component-type="s-search-result"]`).Count()
		fmt.Printf("Page 2 has %d products\n", productCount2)
	}
	
	fmt.Println("\nPress Enter to close...")
	fmt.Scanln()
}