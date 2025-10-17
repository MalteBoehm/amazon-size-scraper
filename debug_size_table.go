package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/maltedev/amazon-size-scraper/internal/browser"
	"github.com/maltedev/amazon-size-scraper/internal/database"
	"github.com/playwright-community/playwright-go"
)

func main() {
	// Get first product
	ctx := context.Background()
	db, _ := database.New(ctx, database.Config{
		Host:     "localhost",
		Port:     5433,
		User:     "postgres",
		Password: "postgres",
		Database: "tall_affiliate",
		MaxConns: 2,
		MinConns: 1,
	})
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

	// Navigate to product
	productURL := "https://www.amazon.de/dp/B07B7ZQGSB"
	fmt.Printf("Going to product: %s\n", productURL)
	
	b.NavigateWithRetry(page, productURL, 2)
	time.Sleep(3 * time.Second)

	// Find size table button
	fmt.Println("\nLooking for size table button...")
	
	buttonSelectors := []string{
		`a:has-text("Größentabelle")`,
		`a[id*="size-chart"]`,
		`button:has-text("Größentabelle")`,
		`.a-button-text:has-text("Größentabelle")`,
		`a[class*="size-chart"]`,
		`[data-action*="size-chart"]`,
	}

	var foundButton playwright.Locator
	
	for _, selector := range buttonSelectors {
		btn := page.Locator(selector).First()
		count, _ := btn.Count()
		if count > 0 {
			foundButton = btn
			fmt.Printf("✓ Found with selector: %s\n", selector)
			
			// Get button details
			text, _ := btn.TextContent()
			fmt.Printf("  Text: %s\n", text)
			
			visible, _ := btn.IsVisible()
			fmt.Printf("  Visible: %v\n", visible)
			
			href, _ := btn.GetAttribute("href")
			if href != "" {
				fmt.Printf("  Href: %s\n", href)
			}
			
			break
		}
	}

	if foundButton == nil {
		fmt.Println("❌ No size table button found!")
		return
	}

	// Try to click
	fmt.Println("\nClicking button...")
	
	// Scroll into view first
	foundButton.ScrollIntoViewIfNeeded()
	time.Sleep(1 * time.Second)
	
	// Try click
	err = foundButton.Click()
	if err != nil {
		fmt.Printf("❌ Click failed: %v\n", err)
		
		// Try force click
		fmt.Println("Trying force click...")
		err = foundButton.Click(playwright.LocatorClickOptions{
			Force: playwright.Bool(true),
		})
		if err != nil {
			fmt.Printf("❌ Force click also failed: %v\n", err)
		}
	} else {
		fmt.Println("✓ Button clicked!")
	}

	// Wait and check for modal
	time.Sleep(3 * time.Second)
	
	fmt.Println("\nLooking for size table modal...")
	
	modalSelectors := []string{
		`.a-popover-content table`,
		`.a-modal-content table`,
		`[data-action="a-modal"] table`,
		`.size-chart-table`,
		`table[class*="size"]`,
		`#a-popover-content-1 table`,
		`.a-popover table`,
	}

	for _, selector := range modalSelectors {
		table := page.Locator(selector).First()
		count, _ := table.Count()
		if count > 0 {
			fmt.Printf("✓ Found table with selector: %s\n", selector)
			
			// Try to extract some data
			rows := table.Locator("tr")
			rowCount, _ := rows.Count()
			fmt.Printf("  Rows: %d\n", rowCount)
			
			if rowCount > 0 {
				firstRow := rows.First()
				text, _ := firstRow.TextContent()
				fmt.Printf("  First row: %s\n", text)
			}
			
			break
		}
	}

	fmt.Println("\nPress Enter to close...")
	fmt.Scanln()
}