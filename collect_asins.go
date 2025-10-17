package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/maltedev/amazon-size-scraper/internal/browser"
	"github.com/maltedev/amazon-size-scraper/internal/database"
	"github.com/maltedev/amazon-size-scraper/internal/scraper"
)

func main() {
	// Browser setup
	b, err := browser.New(&browser.Options{
		Headless: true,
		Timeout:  30 * time.Second,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer b.Close()

	// Database connection
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

	// Create search crawler
	crawler := scraper.NewSearchCrawler(b, db)

	// Crawl search results
	searchURL := "https://www.amazon.de/s?k=t+shirt+%2B+%22größentabelle%22+%2B+%22länge%22&i=fashion"
	
	fmt.Println("Starting search crawl...")
	ctx := context.Background()
	
	// Let it run for a limited time to collect ASINs
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	
	if err := crawler.CrawlSearch(ctx, searchURL); err != nil {
		log.Printf("Crawl error: %v", err)
	}

	fmt.Println("Crawl completed!")
}