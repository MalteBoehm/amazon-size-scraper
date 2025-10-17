package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"

	"github.com/maltedev/amazon-size-scraper/internal/config"
	"github.com/maltedev/amazon-size-scraper/internal/storage"
	"github.com/maltedev/amazon-size-scraper/pkg/logger"
	"log/slog"
	"strings"
)

func main() {
	var (
		mode       = flag.String("mode", "collect", "Mode: collect, process, or test")
		url        = flag.String("url", "", "URL to scrape")
		asin       = flag.String("asin", "", "ASIN to scrape")
		storageFile = flag.String("storage", "camoufox-products.json", "Storage file")
		headless   = flag.Bool("headless", false, "Run in headless mode")
	)
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	logger := logger.New(cfg.Logging.Level, cfg.Logging.Format)
	logger.Info("Starting Camoufox Scraper", "mode", *mode)

	// First, check if Camoufox is available
	if err := checkCamoufox(); err != nil {
		logger.Error("Camoufox not found. Please install it first", "error", err)
		logger.Info("Installation instructions: pip install camoufox[playwright]")
		os.Exit(1)
	}

	ctx := context.Background()

	switch *mode {
	case "test":
		testCamoufox(ctx, logger, *url, *headless)
	case "collect":
		if *url == "" {
			fmt.Println("Please provide URL with -url")
			os.Exit(1)
		}
		collectWithCamoufox(ctx, logger, *url, *storageFile, *headless)
	case "process":
		processWithCamoufox(ctx, logger, *asin, *headless)
	default:
		fmt.Printf("Unknown mode: %s\n", *mode)
		os.Exit(1)
	}
}

func pythonBool(b bool) string {
	if b {
		return "True"
	}
	return "False"
}

func checkCamoufox() error {
	// Check if Python and camoufox are installed
	cmd := exec.Command("python3", "-c", "import camoufox; print('Camoufox version:', camoufox.__version__)")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("camoufox not available: %v", err)
	}
	fmt.Printf("Camoufox check: %s\n", output)
	return nil
}

func testCamoufox(ctx context.Context, logger *slog.Logger, url string, headless bool) {
	if url == "" {
		url = "https://www.amazon.de"
	}

	logger.Info("Testing Camoufox connection", "url", url)

	// Create Python script to launch Camoufox
	pythonScript := `
import asyncio
from camoufox.sync_api import Camoufox

def test_camoufox():
    with Camoufox(
        headless=%v,
        block_images=False,
        block_webrtc=True,
        humanize=True,
        screen={'width': 1920, 'height': 1080},
        viewport={'width': 1920, 'height': 1080},
        locale='de-DE',
        timezone='Europe/Berlin',
    ) as browser:
        page = browser.new_page()
        
        # Navigate to URL
        page.goto('%s')
        
        # Wait a bit
        page.wait_for_timeout(5000)
        
        # Take screenshot
        page.screenshot(path='camoufox-test.png')
        
        # Get page title
        title = page.title()
        print(f"Page title: {title}")
        
        # Check for product elements
        products = page.query_selector_all('[data-asin]')
        print(f"Found {len(products)} products")
        
        # Keep browser open if not headless
        if not %v:
            input("Press Enter to close browser...")
        
        browser.close()

if __name__ == '__main__':
    test_camoufox()
`

	// Write Python script to temp file
	tmpFile, err := os.CreateTemp("", "camoufox-test-*.py")
	if err != nil {
		logger.Error("Failed to create temp file", "error", err)
		return
	}
	defer os.Remove(tmpFile.Name())

	script := fmt.Sprintf(pythonScript, pythonBool(headless), url, pythonBool(headless))
	if _, err := tmpFile.WriteString(script); err != nil {
		logger.Error("Failed to write script", "error", err)
		return
	}
	tmpFile.Close()

	// Execute Python script
	cmd := exec.Command("python3", tmpFile.Name())
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	
	logger.Info("Launching Camoufox...")
	if err := cmd.Run(); err != nil {
		logger.Error("Failed to run Camoufox", "error", err)
		return
	}

	logger.Info("Camoufox test completed")
}

func collectWithCamoufox(ctx context.Context, logger *slog.Logger, searchURL string, storageFile string, headless bool) {
	// Python script for collecting search results
	pythonScript := `
import asyncio
import json
from camoufox.sync_api import Camoufox

def collect_products(search_url, headless=False):
    results = []
    
    with Camoufox(
        headless=headless,
        humanize=True,
        screen={'width': 1920, 'height': 1080},
        viewport={'width': 1920, 'height': 1080},
        locale='de-DE',
        timezone='Europe/Berlin',
    ) as browser:
        page = browser.new_page()
        
        print(f"Navigating to: {search_url}")
        page.goto(search_url, wait_until='networkidle')
        
        # Wait for products to load
        page.wait_for_timeout(3000)
        
        # Take screenshot
        page.screenshot(path='camoufox-search.png')
        
        # Check page title
        title = page.title()
        print(f"Page title: {title}")
        
        # Try multiple selectors
        selectors = [
            '[data-component-type="s-search-result"]',
            'div[data-asin]:not([data-asin=""])',
            '[data-index]',
            '.s-result-item[data-asin]',
        ]
        
        products_found = []
        for selector in selectors:
            elements = page.query_selector_all(selector)
            if elements:
                print(f"Found {len(elements)} products with selector: {selector}")
                products_found = elements
                break
        
        # Extract product data
        for product in products_found:
            try:
                asin = product.get_attribute('data-asin')
                if not asin:
                    continue
                
                # Try to get title
                title_elem = product.query_selector('h2 a span') or product.query_selector('h2')
                title = title_elem.text_content() if title_elem else ''
                
                # Try to get price
                price_elem = product.query_selector('.a-price-whole')
                price = price_elem.text_content() if price_elem else ''
                
                result = {
                    'asin': asin,
                    'title': title.strip(),
                    'price': price.strip(),
                    'url': f'https://www.amazon.de/dp/{asin}'
                }
                
                results.append(result)
                print(f"Found: {asin} - {title[:50]}...")
                
            except Exception as e:
                print(f"Error extracting product: {e}")
                continue
        
        browser.close()
    
    return results

if __name__ == '__main__':
    import sys
    search_url = sys.argv[1]
    headless = sys.argv[2].lower() == 'true' if len(sys.argv) > 2 else False
    
    results = collect_products(search_url, headless)
    
    # Output as JSON
    print("\nJSON_OUTPUT_START")
    print(json.dumps(results))
    print("JSON_OUTPUT_END")
`

	// Create temp Python script
	tmpFile, err := os.CreateTemp("", "camoufox-collect-*.py")
	if err != nil {
		logger.Error("Failed to create temp file", "error", err)
		return
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(pythonScript); err != nil {
		logger.Error("Failed to write script", "error", err)
		return
	}
	tmpFile.Close()

	// Execute Python script
	cmd := exec.Command("python3", tmpFile.Name(), searchURL, fmt.Sprintf("%v", headless))
	output, err := cmd.CombinedOutput()
	if err != nil {
		logger.Error("Failed to run Camoufox", "error", err)
		logger.Error("Output", "output", string(output))
		return
	}

	// Parse output
	outputStr := string(output)
	fmt.Println(outputStr)

	// Extract JSON from output
	startIdx := strings.Index(outputStr, "JSON_OUTPUT_START")
	endIdx := strings.Index(outputStr, "JSON_OUTPUT_END")
	
	if startIdx != -1 && endIdx != -1 {
		jsonStr := outputStr[startIdx+len("JSON_OUTPUT_START"):endIdx]
		jsonStr = strings.TrimSpace(jsonStr)
		
		var results []map[string]string
		if err := json.Unmarshal([]byte(jsonStr), &results); err != nil {
			logger.Error("Failed to parse results", "error", err)
			return
		}

		// Save to storage
		linkStorage, err := storage.NewLinkStorage(storageFile)
		if err != nil {
			logger.Error("Failed to init storage", "error", err)
			return
		}

		var links []*storage.ProductLink
		for _, r := range results {
			link := &storage.ProductLink{
				ASIN:   r["asin"],
				Title:  r["title"],
				URL:    r["url"],
				Price:  r["price"],
				Status: "pending",
			}
			links = append(links, link)
		}

		if err := linkStorage.AddBatch(links); err != nil {
			logger.Error("Failed to save links", "error", err)
		}

		logger.Info("Collection completed", "products", len(results))
	}
}

func processWithCamoufox(ctx context.Context, logger *slog.Logger, asin string, headless bool) {
	if asin == "" {
		logger.Error("Please provide ASIN with -asin")
		return
	}

	// Python script for processing single product
	pythonScript := `
import asyncio
import json
import re
from camoufox.sync_api import Camoufox

def scrape_product(asin, headless=False):
    url = f'https://www.amazon.de/dp/{asin}'
    
    with Camoufox(
        headless=headless,
        humanize=True,
        screen={'width': 1920, 'height': 1080},
        viewport={'width': 1920, 'height': 1080},
        locale='de-DE',
        timezone='Europe/Berlin',
    ) as browser:
        page = browser.new_page()
        
        print(f"Navigating to: {url}")
        page.goto(url, wait_until='networkidle')
        
        # Human-like behavior
        page.wait_for_timeout(2000)
        page.mouse.move(500, 300)
        page.wait_for_timeout(1000)
        
        # Scroll down slowly
        for i in range(3):
            page.evaluate('window.scrollBy(0, 300)')
            page.wait_for_timeout(500)
        
        # Take screenshot
        page.screenshot(path=f'camoufox-{asin}.png', full_page=True)
        
        # Extract product details
        title = page.title()
        print(f"Title: {title}")
        
        # Get all text content for dimension extraction
        content = page.content()
        
        # Look for dimensions in various places
        dimension_patterns = [
            r'(\d+(?:[,.]\d+)?)\s*x\s*(\d+(?:[,.]\d+)?)\s*x\s*(\d+(?:[,.]\d+)?)\s*(cm|mm|m)',
            r'Abmessungen.*?:\s*(\d+(?:[,.]\d+)?)\s*x\s*(\d+(?:[,.]\d+)?)\s*x\s*(\d+(?:[,.]\d+)?)\s*(cm|mm|m)',
        ]
        
        dimensions = None
        for pattern in dimension_patterns:
            match = re.search(pattern, content, re.IGNORECASE)
            if match:
                dimensions = {
                    'length': float(match.group(1).replace(',', '.')),
                    'width': float(match.group(2).replace(',', '.')),
                    'height': float(match.group(3).replace(',', '.')),
                    'unit': match.group(4).lower()
                }
                break
        
        result = {
            'asin': asin,
            'title': title,
            'dimensions': dimensions,
            'url': url
        }
        
        browser.close()
        
        return result

if __name__ == '__main__':
    import sys
    asin = sys.argv[1]
    headless = sys.argv[2].lower() == 'true' if len(sys.argv) > 2 else False
    
    result = scrape_product(asin, headless)
    
    print("\nJSON_OUTPUT_START")
    print(json.dumps(result))
    print("JSON_OUTPUT_END")
`

	// Create and execute script
	tmpFile, err := os.CreateTemp("", "camoufox-process-*.py")
	if err != nil {
		logger.Error("Failed to create temp file", "error", err)
		return
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(pythonScript); err != nil {
		logger.Error("Failed to write script", "error", err)
		return
	}
	tmpFile.Close()

	cmd := exec.Command("python3", tmpFile.Name(), asin, fmt.Sprintf("%v", headless))
	output, err := cmd.CombinedOutput()
	if err != nil {
		logger.Error("Failed to run Camoufox", "error", err)
		logger.Error("Output", "output", string(output))
		return
	}

	// Parse and display results
	outputStr := string(output)
	fmt.Println(outputStr)

	startIdx := strings.Index(outputStr, "JSON_OUTPUT_START")
	endIdx := strings.Index(outputStr, "JSON_OUTPUT_END")
	
	if startIdx != -1 && endIdx != -1 {
		jsonStr := outputStr[startIdx+len("JSON_OUTPUT_START"):endIdx]
		jsonStr = strings.TrimSpace(jsonStr)
		
		var result map[string]interface{}
		if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
			logger.Error("Failed to parse result", "error", err)
			return
		}

		logger.Info("Product scraped", "result", result)
	}
}