package main

import (
	"fmt"
	"log"
	"time"

	"github.com/maltedev/amazon-size-scraper/internal/browser"
)

func main() {
	// Browser mit sichtbarem Fenster
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

	// Gehe zu Amazon.de erst
	fmt.Println("1. Going to Amazon.de...")
	b.NavigateWithRetry(page, "https://www.amazon.de", 1)
	
	// Dann zum Produkt
	productURL := "https://www.amazon.de/dp/B07B7ZQGSB"
	fmt.Println("2. Going to product...")
	b.NavigateWithRetry(page, productURL, 1)
	
	// Warte auf Seite
	time.Sleep(5 * time.Second)
	
	// Prüfe JavaScript
	jsEnabled, _ := page.Evaluate(`() => typeof window !== 'undefined'`)
	fmt.Printf("\n3. JavaScript enabled: %v\n", jsEnabled)
	
	// Suche nach Größentabelle Link
	fmt.Println("\n4. Looking for size table link...")
	
	// Versuche verschiedene Wege
	// Methode 1: Direct click on visible link
	link := page.Locator(`a:visible:has-text("Größentabelle")`).First()
	count, _ := link.Count()
	
	if count > 0 {
		fmt.Println("✓ Found size table link!")
		
		// Scroll to element
		link.ScrollIntoViewIfNeeded()
		time.Sleep(1 * time.Second)
		
		// Get link info
		href, _ := link.GetAttribute("href")
		text, _ := link.TextContent()
		fmt.Printf("  Text: %s\n", text)
		fmt.Printf("  Href: %s\n", href)
		
		// Try JavaScript click
		fmt.Println("\n5. Clicking with JavaScript...")
		_, err := page.Evaluate(`(selector) => {
			const el = document.querySelector(selector);
			if (el) {
				el.click();
				return true;
			}
			return false;
		}`, `a:has-text("Größentabelle")`)
		
		if err != nil {
			fmt.Printf("JS click error: %v\n", err)
		} else {
			fmt.Println("✓ JavaScript click executed!")
		}
	} else {
		// Try to find any element with "Größentabelle"
		elements := page.Locator(`*:has-text("Größentabelle")`)
		count, _ := elements.Count()
		fmt.Printf("Found %d elements containing 'Größentabelle'\n", count)
		
		for i := 0; i < count && i < 3; i++ {
			el := elements.Nth(i)
			tag, _ := el.Evaluate(`el => el.tagName`, nil)
			text, _ := el.TextContent()
			fmt.Printf("  Element %d: <%s> %s\n", i, tag, text)
		}
	}
	
	// Wait for modal
	time.Sleep(3 * time.Second)
	
	// Check for modal/popup
	fmt.Println("\n6. Checking for modal...")
	modal := page.Locator(`.a-popover-content, .a-modal-content`).First()
	modalCount, _ := modal.Count()
	if modalCount > 0 {
		fmt.Println("✓ Modal found!")
		content, _ := modal.TextContent()
		fmt.Printf("Modal content preview: %.100s...\n", content)
	} else {
		fmt.Println("❌ No modal found")
	}
	
	fmt.Println("\nPress Enter to close...")
	fmt.Scanln()
}