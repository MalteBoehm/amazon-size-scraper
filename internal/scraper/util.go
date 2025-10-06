package scraper

import (
	"regexp"

	"github.com/maltedev/amazon-size-scraper/internal/models"
)

var asinRegex = regexp.MustCompile(`(?i)(?:https?://)?(?:www\.)?amazon\.de/.*/dp/([A-Z0-9]{10})`)

func extractASINFromURL(url string) string {
	if url == "" {
		return ""
	}
	m := asinRegex.FindStringSubmatch(url)
	if len(m) >= 2 {
		return m[1]
	}
	return ""
}

// validateColorVariations filters and validates color variation items
func (ps *ProductScraper) validateColorVariations(items []*models.VariationItem) []*models.VariationItem {
	if items == nil {
		return []*models.VariationItem{}
	}

	seen := make(map[string]bool)
	var result []*models.VariationItem

	for _, item := range items {
		if item == nil {
			continue
		}

		// Skip empty values
		if item.Value == "" {
			continue
		}

		// Skip duplicates
		if seen[item.Value] {
			continue
		}
		seen[item.Value] = true

		// Set DisplayName to Value if empty
		if item.DisplayName == "" {
			item.DisplayName = item.Value
		}

		result = append(result, item)
	}

	return result
}

// getString safely extracts a string value from a map
func getString(m map[string]interface{}, key string) string {
	if m == nil {
		return ""
	}
	if val, ok := m[key].(string); ok {
		return val
	}
	return ""
}

// getBool safely extracts a boolean value from a map
func getBool(m map[string]interface{}, key string) bool {
	if m == nil {
		return false
	}
	if val, ok := m[key].(bool); ok {
		return val
	}
	return false
}
