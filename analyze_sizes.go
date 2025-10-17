package main

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/maltedev/amazon-size-scraper/internal/browser"
	"github.com/playwright-community/playwright-go"
)

type ProductResult struct {
	ASIN       string
	Title      string
	HasTable   bool
	HasLength  bool
	HasWidth   bool
	Status     string // "complete", "missing_length", "missing_width", "missing_both", "no_table", "error"
	Details    string
}

type Statistics struct {
	Total         int
	Complete      int // Has both length and width
	MissingLength int
	MissingWidth  int
	MissingBoth   int
	NoTable       int
	Errors        int
}

func main() {
	fmt.Println("=== Amazon Size Table Analyzer ===\n")

	// Browser setup
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

	// Step 1: Go to category page
	fmt.Println("Step 1: Navigating to category page...")
	
	// First go to Amazon.de
	if err := b.NavigateWithRetry(page, "https://www.amazon.de", 1); err != nil {
		log.Fatal("Failed to navigate to Amazon.de:", err)
	}
	
	time.Sleep(2 * time.Second)
	
	// Then go to search
	searchURL := "https://www.amazon.de/s?k=t+shirt+%2B+%22größentabelle%22+%2B+%22länge%22&i=fashion"
	if err := b.NavigateWithRetry(page, searchURL, 1); err != nil {
		log.Fatal("Failed to navigate to search:", err)
	}
	
	time.Sleep(3 * time.Second)
	
	// Step 2: Collect ASINs
	fmt.Println("\nStep 2: Collecting product ASINs...")
	asins := collectASINs(page, 50) // Collect up to 50 ASINs
	fmt.Printf("Collected %d ASINs\n", len(asins))
	
	if len(asins) == 0 {
		log.Fatal("No ASINs found!")
	}
	
	// Step 3: Analyze each product
	fmt.Println("\nStep 3: Analyzing products...")
	fmt.Println("----------------------------------------")
	
	results := make([]ProductResult, 0)
	stats := &Statistics{}
	
	for i, asin := range asins {
		fmt.Printf("\n[%d/%d] Processing ASIN: %s\n", i+1, len(asins), asin)
		
		result := analyzeProduct(b, asin)
		results = append(results, result)
		
		// Update statistics
		stats.Total++
		switch result.Status {
		case "complete":
			stats.Complete++
			fmt.Printf("✅ Has both length and width measurements\n")
		case "missing_length":
			stats.MissingLength++
			fmt.Printf("⚠️  Missing length measurement (ignored)\n")
		case "missing_width":
			stats.MissingWidth++
			fmt.Printf("⚠️  Missing width measurement (ignored)\n")
		case "missing_both":
			stats.MissingBoth++
			fmt.Printf("⚠️  Missing both measurements (ignored)\n")
		case "no_table":
			stats.NoTable++
			fmt.Printf("❌ No size table found\n")
		case "error":
			stats.Errors++
			fmt.Printf("❗ Error: %s\n", result.Details)
		}
		
		// Rate limiting
		time.Sleep(3 * time.Second)
	}
	
	// Step 4: Summary
	fmt.Println("\n\n=== SUMMARY ===")
	fmt.Println("----------------------------------------")
	fmt.Printf("Total products analyzed: %d\n", stats.Total)
	fmt.Printf("Complete (length + width): %d (%.1f%%)\n", stats.Complete, float64(stats.Complete)*100/float64(stats.Total))
	fmt.Printf("Missing length only: %d (%.1f%%)\n", stats.MissingLength, float64(stats.MissingLength)*100/float64(stats.Total))
	fmt.Printf("Missing width only: %d (%.1f%%)\n", stats.MissingWidth, float64(stats.MissingWidth)*100/float64(stats.Total))
	fmt.Printf("Missing both: %d (%.1f%%)\n", stats.MissingBoth, float64(stats.MissingBoth)*100/float64(stats.Total))
	fmt.Printf("No size table: %d (%.1f%%)\n", stats.NoTable, float64(stats.NoTable)*100/float64(stats.Total))
	fmt.Printf("Errors: %d\n", stats.Errors)
	
	// List products with complete measurements
	fmt.Println("\n=== Products WITH Complete Measurements (Length + Width) ===")
	for _, result := range results {
		if result.Status == "complete" {
			fmt.Printf("- %s: %s\n", result.ASIN, result.Title)
		}
	}
}

func collectASINs(page playwright.Page, maxCount int) []string {
	asins := make([]string, 0)
	
	productElements := page.Locator(`[data-component-type="s-search-result"]`)
	count, _ := productElements.Count()
	
	for i := 0; i < count && len(asins) < maxCount; i++ {
		productEl := productElements.Nth(i)
		asin, err := productEl.GetAttribute("data-asin")
		if err == nil && asin != "" {
			asins = append(asins, asin)
		}
	}
	
	return asins
}

func analyzeProduct(b *browser.Browser, asin string) ProductResult {
	result := ProductResult{
		ASIN:   asin,
		Status: "error",
	}
	
	page, err := b.NewPage()
	if err != nil {
		result.Details = "Failed to create page"
		return result
	}
	defer page.Close()
	
	// Navigate to product
	productURL := fmt.Sprintf("https://www.amazon.de/dp/%s", asin)
	if err := b.NavigateWithRetry(page, productURL, 2); err != nil {
		result.Details = "Failed to navigate"
		return result
	}
	
	// Add human-like behavior
	b.HumanizeInteraction(page)
	
	// Get product title
	titleEl := page.Locator("#productTitle").First()
	if title, err := titleEl.TextContent(); err == nil {
		result.Title = strings.TrimSpace(title)
	}
	
	// Look for size table button
	clicked, err := page.Evaluate(`() => {
		const links = document.querySelectorAll('a');
		for (let link of links) {
			if (link.textContent && link.textContent.includes('Größentabelle')) {
				link.scrollIntoView();
				link.click();
				return true;
			}
		}
		return false;
	}`)
	
	if err != nil || !clicked.(bool) {
		result.Status = "no_table"
		result.HasTable = false
		return result
	}
	
	result.HasTable = true
	
	// Wait for modal
	time.Sleep(3 * time.Second)
	
	// Extract table data
	tableData, err := page.Evaluate(`() => {
		const tables = document.querySelectorAll('.a-popover-content table, .a-modal-content table, [id*="popover"] table');
		if (tables.length === 0) return null;
		
		const table = tables[0];
		const data = {
			headers: [],
			rows: [],
			hasLength: false,
			hasWidth: false
		};
		
		// Get all text content to check for measurements
		const allText = table.textContent.toLowerCase();
		
		// Check for length
		if (allText.includes('länge') || allText.includes('laenge') || allText.includes('length')) {
			data.hasLength = true;
		}
		
		// Check for width/chest
		if (allText.includes('breite') || allText.includes('brustumfang') || 
		    allText.includes('brust') || allText.includes('chest') || 
		    allText.includes('width')) {
			data.hasWidth = true;
		}
		
		// Get headers
		const headerRow = table.rows[0];
		for (let cell of headerRow.cells) {
			const text = cell.textContent.trim().toLowerCase();
			data.headers.push(text);
			
			if (text.includes('länge') || text.includes('laenge')) {
				data.hasLength = true;
			}
			if (text.includes('brustumfang') || text.includes('breite') || text.includes('brust')) {
				data.hasWidth = true;
			}
		}
		
		// Check first column for measurements
		for (let i = 1; i < table.rows.length; i++) {
			const firstCell = table.rows[i].cells[0];
			if (firstCell) {
				const text = firstCell.textContent.toLowerCase();
				if (text.includes('länge') || text.includes('laenge') || text.includes('length')) {
					data.hasLength = true;
				}
				if (text.includes('brustumfang') || text.includes('breite') || text.includes('brust')) {
					data.hasWidth = true;
				}
			}
		}
		
		return data;
	}`)
	
	if err != nil || tableData == nil {
		result.Status = "error"
		result.Details = "Failed to extract table"
		return result
	}
	
	tableMap := tableData.(map[string]interface{})
	hasLength := tableMap["hasLength"].(bool)
	hasWidth := tableMap["hasWidth"].(bool)
	
	result.HasLength = hasLength
	result.HasWidth = hasWidth
	
	if hasLength && hasWidth {
		result.Status = "complete"
	} else if !hasLength && !hasWidth {
		result.Status = "missing_both"
	} else if !hasLength {
		result.Status = "missing_length"
	} else {
		result.Status = "missing_width"
	}
	
	return result
}