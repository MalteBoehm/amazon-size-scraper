package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/maltedev/amazon-size-scraper/internal/browser"
	"github.com/maltedev/amazon-size-scraper/internal/database"
)

func main() {
	// Logging
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	ctx := context.Background()

	// Database
	db, err := database.New(ctx, database.Config{
		Host:     "localhost",
		Port:     5433,
		User:     "postgres",
		Password: "postgres",
		Database: "tall_affiliate",
		MaxConns: 4,
		MinConns: 1,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// Count before
	var before int
	db.Pool().QueryRow(ctx, "SELECT COUNT(*) FROM products").Scan(&before)
	logger.Info("products before crawl", "count", before)

	// Browser
	b, err := browser.New(&browser.Options{
		Headless: true,
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

	// Navigate to Amazon first
	logger.Info("navigating to Amazon.de")
	if err := b.NavigateWithRetry(page, "https://www.amazon.de", 2); err != nil {
		logger.Warn("failed to navigate to homepage", "error", err)
	}

	// Now search
	searchURL := "https://www.amazon.de/s?k=t-shirt+größentabelle+länge&i=fashion"
	logger.Info("navigating to search", "url", searchURL)
	if err := b.NavigateWithRetry(page, searchURL, 2); err != nil {
		log.Fatal("failed to navigate to search:", err)
	}

	totalProducts := 0
	pageNum := 1

	for {
		logger.Info("processing page", "page", pageNum)
		
		// Wait for products
		page.WaitForSelector(`[data-component-type="s-search-result"]`)
		time.Sleep(2 * time.Second)
		
		// Extract products
		products := page.Locator(`[data-component-type="s-search-result"]`)
		count, _ := products.Count()
		logger.Info("found products on page", "page", pageNum, "count", count)
		
		pageProducts := 0
		for i := 0; i < count; i++ {
			productEl := products.Nth(i)
			asin, err := productEl.GetAttribute("data-asin")
			if err != nil || asin == "" {
				continue
			}
			
			// Get title  
			title := "Unknown"
			titleEl := productEl.Locator("h2 span").First()
			if t, err := titleEl.TextContent(); err == nil && t != "" {
				title = strings.TrimSpace(t)
			}
			
			// Save to DB
			p := &database.Product{
				ASIN:   asin,
				Title:  title,
				URL:    fmt.Sprintf("https://www.amazon.de/dp/%s", asin),
				Status: database.StatusPending,
			}
			
			if err := db.InsertProduct(ctx, p); err == nil {
				pageProducts++
			}
		}
		
		totalProducts += pageProducts
		logger.Info("saved products from page", "page", pageNum, "saved", pageProducts, "total", totalProducts)
		
		// Check for next page
		nextBtn := page.Locator(`a.s-pagination-next`).First()
		hasNext := false
		
		if c, _ := nextBtn.Count(); c > 0 {
			disabled, _ := nextBtn.GetAttribute("aria-disabled") 
			if disabled != "true" {
				logger.Info("clicking next page button")
				if err := nextBtn.Click(); err == nil {
					hasNext = true
					time.Sleep(5 * time.Second) // Wait for page load
				}
			}
		}
		
		if !hasNext {
			logger.Info("no more pages")
			break
		}
		
		pageNum++
		
		// Safety limit
		if pageNum > 10 {
			logger.Info("reached page limit")
			break
		}
	}
	
	// Final count
	var after int
	db.Pool().QueryRow(ctx, "SELECT COUNT(*) FROM products").Scan(&after)
	
	logger.Info("crawl completed", 
		"pages", pageNum,
		"total_products", totalProducts,
		"products_before", before,
		"products_after", after,
		"new_products", after-before)
}