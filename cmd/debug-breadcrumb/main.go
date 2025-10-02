package main

import (
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/maltedev/amazon-size-scraper/internal/browser"
	"github.com/maltedev/amazon-size-scraper/internal/config"
	"github.com/maltedev/amazon-size-scraper/internal/parser"
	"github.com/maltedev/amazon-size-scraper/internal/scraper"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run main.go <ASIN>")
		fmt.Println("Example: go run main.go B08N5WRWNW")
		os.Exit(1)
	}

	asin := os.Args[1]

	// Load config
	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Setup logging
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	// Create browser
	browserOpts := browser.DefaultOptions()
	browserOpts.Headless = true

	b, err := browser.New(browserOpts)
	if err != nil {
		logger.Error("failed to create browser", "error", err)
		os.Exit(1)
	}
	defer b.Close()

	// Create scraper to get HTML
	parser := parser.NewAmazonParser()
	amazonScraper := scraper.NewAmazonScraperLegacy(b, parser, logger, cfg)
	defer amazonScraper.Close()

	// Navigate and get HTML
	url := fmt.Sprintf("https://www.amazon.de/dp/%s", asin)
	
	page, err := b.NewPage()
	if err != nil {
		logger.Error("failed to create page", "error", err)
		os.Exit(1)
	}
	defer page.Close()

	err = b.NavigateWithRetry(page, url, 3)
	if err != nil {
		logger.Error("failed to navigate", "error", err)
		os.Exit(1)
	}

	html, err := page.Content()
	if err != nil {
		logger.Error("failed to get content", "error", err)
		os.Exit(1)
	}

	// Parse HTML
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		logger.Error("failed to parse HTML", "error", err)
		os.Exit(1)
	}

	fmt.Printf("=== BREADCRUMB DEBUG für ASIN: %s ===\n\n", asin)

	// Debug verschiedene Selektoren
	selectors := []string{
		"#wayfinding-breadcrumbs_feature_div",
		"#wayfinding-breadcrumbs_feature_div .a-unordered-list",
		"#wayfinding-breadcrumbs_feature_div .a-unordered-list.a-horizontal",
		"#wayfinding-breadcrumbs_feature_div .a-unordered-list.a-horizontal li",
		"#wayfinding-breadcrumbs_feature_div ul",
		"#wayfinding-breadcrumbs_feature_div ul li",
		".a-breadcrumb",
		".a-subheader.a-breadcrumb",
		"[data-feature-name='wayfinding-breadcrumbs']",
	}

	for _, selector := range selectors {
		found := doc.Find(selector)
		fmt.Printf("Selector '%s': %d Elemente gefunden\n", selector, found.Length())
		
		if found.Length() > 0 {
			found.Each(func(i int, s *goquery.Selection) {
				text := strings.TrimSpace(s.Text())
				classes := s.AttrOr("class", "")
				tagName := goquery.NodeName(s)
				
				fmt.Printf("  [%d] <%s> class='%s' text='%s'\n", i, tagName, classes, text[:min(50, len(text))])
			})
		}
		fmt.Println()
	}

	// Speziell für Li-Elemente
	fmt.Println("=== LI ELEMENTE DETAIL ===")
	doc.Find("#wayfinding-breadcrumbs_feature_div li").Each(func(i int, li *goquery.Selection) {
		classes := li.AttrOr("class", "")
		ariaHidden := li.AttrOr("aria-hidden", "")
		role := li.AttrOr("role", "")
		text := strings.TrimSpace(li.Text())
		
		fmt.Printf("Li[%d]:\n", i)
		fmt.Printf("  Classes: '%s'\n", classes)
		fmt.Printf("  Aria-hidden: '%s'\n", ariaHidden)
		fmt.Printf("  Role: '%s'\n", role)
		fmt.Printf("  Text: '%s'\n", text)
		
		// Check for links and spans
		links := li.Find("a")
		spans := li.Find("span")
		fmt.Printf("  Links: %d, Spans: %d\n", links.Length(), spans.Length())
		
		if links.Length() > 0 {
			links.Each(func(j int, a *goquery.Selection) {
				fmt.Printf("    Link[%d]: '%s'\n", j, strings.TrimSpace(a.Text()))
			})
		}
		if spans.Length() > 0 {
			spans.Each(func(j int, span *goquery.Selection) {
				fmt.Printf("    Span[%d]: '%s'\n", j, strings.TrimSpace(span.Text()))
			})
		}
		fmt.Println()
	})
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}