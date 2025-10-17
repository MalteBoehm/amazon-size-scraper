package main

import (
	"fmt"
	"log"
	"time"

	"github.com/maltedev/amazon-size-scraper/internal/browser"
)

func main() {
	// Browser
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

	// Navigate
	fmt.Println("1. Going to Amazon.de...")
	b.NavigateWithRetry(page, "https://www.amazon.de", 1)
	
	productURL := "https://www.amazon.de/dp/B07B7ZQGSB"
	fmt.Println("2. Going to product...")
	b.NavigateWithRetry(page, productURL, 1)
	
	time.Sleep(5 * time.Second)
	
	// Find and click size table
	fmt.Println("\n3. Looking for size table link...")
	
	// Use JavaScript to find and click
	clicked, err := page.Evaluate(`() => {
		// Find all links
		const links = document.querySelectorAll('a');
		for (let link of links) {
			if (link.textContent && link.textContent.includes('Größentabelle')) {
				console.log('Found size table link:', link);
				link.scrollIntoView();
				link.click();
				return true;
			}
		}
		return false;
	}`)
	
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else if clicked.(bool) {
		fmt.Println("✓ Size table link clicked!")
	} else {
		fmt.Println("❌ Size table link not found")
	}
	
	// Wait for modal
	time.Sleep(3 * time.Second)
	
	// Check for size table
	fmt.Println("\n4. Looking for size table...")
	
	// Try to find table in modal
	tableData, err := page.Evaluate(`() => {
		// Look for tables in modals
		const tables = document.querySelectorAll('.a-popover-content table, .a-modal-content table, [id*="popover"] table');
		if (tables.length > 0) {
			const table = tables[0];
			const data = {
				found: true,
				rows: table.rows.length,
				headers: [],
				firstRow: []
			};
			
			// Get headers
			if (table.rows.length > 0) {
				const headerRow = table.rows[0];
				for (let cell of headerRow.cells) {
					data.headers.push(cell.textContent.trim());
				}
			}
			
			// Get first data row
			if (table.rows.length > 1) {
				const dataRow = table.rows[1];
				for (let cell of dataRow.cells) {
					data.firstRow.push(cell.textContent.trim());
				}
			}
			
			return data;
		}
		return { found: false };
	}`)
	
	if err != nil {
		fmt.Printf("Error checking table: %v\n", err)
	} else {
		tableMap := tableData.(map[string]interface{})
		if tableMap["found"].(bool) {
			fmt.Println("✓ Size table found!")
			fmt.Printf("  Rows: %v\n", tableMap["rows"])
			fmt.Printf("  Headers: %v\n", tableMap["headers"])
			fmt.Printf("  First row: %v\n", tableMap["firstRow"])
		} else {
			fmt.Println("❌ No size table found in modal")
			
			// Debug: check what's visible
			fmt.Println("\n5. Debugging - checking visible modals...")
			visible, _ := page.Evaluate(`() => {
				const popovers = document.querySelectorAll('.a-popover, .a-modal, [id*="popover"]');
				const result = [];
				for (let el of popovers) {
					if (el.offsetParent !== null) { // visible check
						result.push({
							class: el.className,
							id: el.id,
							text: el.textContent.substring(0, 100)
						});
					}
				}
				return result;
			}`)
			fmt.Printf("Visible modals: %v\n", visible)
		}
	}
	
	fmt.Println("\nPress Enter to close...")
	fmt.Scanln()
}