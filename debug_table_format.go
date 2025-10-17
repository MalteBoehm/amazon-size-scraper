package main

import (
	"encoding/json"
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
	b.NavigateWithRetry(page, "https://www.amazon.de", 1)
	b.NavigateWithRetry(page, "https://www.amazon.de/dp/B07B7ZQGSB", 1)
	
	time.Sleep(3 * time.Second)
	
	// Click size table
	page.Evaluate(`() => {
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
	
	time.Sleep(3 * time.Second)
	
	// Get detailed table structure
	tableData, _ := page.Evaluate(`() => {
		const tables = document.querySelectorAll('.a-popover-content table, .a-modal-content table, [id*="popover"] table');
		if (tables.length === 0) return null;
		
		const table = tables[0];
		const result = {
			rowCount: table.rows.length,
			structure: []
		};
		
		// Analyze each row
		for (let i = 0; i < table.rows.length; i++) {
			const row = table.rows[i];
			const rowInfo = {
				index: i,
				cellCount: row.cells.length,
				cells: []
			};
			
			for (let j = 0; j < row.cells.length; j++) {
				const cell = row.cells[j];
				rowInfo.cells.push({
					text: cell.textContent.trim(),
					isHeader: cell.tagName === 'TH',
					colspan: cell.colSpan,
					rowspan: cell.rowSpan
				});
			}
			
			result.structure.push(rowInfo);
		}
		
		// Try to identify the actual structure
		result.analysis = {
			firstRowIsHeader: result.structure[0].cells.some(c => c.isHeader),
			firstColIsLabel: true // Usually true for size tables
		};
		
		return result;
	}`)
	
	// Pretty print
	jsonData, _ := json.MarshalIndent(tableData, "", "  ")
	fmt.Printf("Table Structure:\n%s\n", jsonData)
	
	fmt.Println("\nPress Enter to close...")
	fmt.Scanln()
}