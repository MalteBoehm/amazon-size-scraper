package colors

import (
	"regexp"
	"strings"
	"github.com/maltedev/amazon-size-scraper/internal/models"
)

// ColorNormalizer handles color name normalization
type ColorNormalizer struct {
	// Compiled regex patterns for efficiency
	technicalPrefixPattern   *regexp.Regexp
	parenthesesPattern       *regexp.Regexp
	yearPattern              *regexp.Regexp
	whitespacePattern        *regexp.Regexp
}

// NewColorNormalizer creates a new color normalizer with pre-compiled patterns
func NewColorNormalizer() *ColorNormalizer {
	return &ColorNormalizer{
		// Pattern to remove technical prefixes like "X1234 "
		technicalPrefixPattern: regexp.MustCompile(`^[A-Z]\d+\s+`),
		// Pattern to remove decorative parentheses content for value field
		parenthesesPattern: regexp.MustCompile(`\s*\([^)]+\)\s*$`),
		// Pattern to remove year suffixes like "(2024 Edition)"
		yearPattern: regexp.MustCompile(`\s*\(\d{4}.*?\)\s*$`),
		// Pattern to normalize whitespace
		whitespacePattern: regexp.MustCompile(`\s+`),
	}
}

// NormalizeValue normalizes a color name for the Value field
// - Applies lowercase
// - Removes technical prefixes
// - Removes decorative suffixes in parentheses
// - Normalizes whitespace
// - Limits length to maxLength characters
func (cn *ColorNormalizer) NormalizeValue(colorName string, maxLength int) string {
	if colorName == "" {
		return ""
	}

	// Trim whitespace
	normalized := strings.TrimSpace(colorName)

	// Remove technical prefixes (e.g., "X123 Navy Blue" -> "Navy Blue")
	normalized = cn.technicalPrefixPattern.ReplaceAllString(normalized, "")

	// Remove year/edition suffixes (e.g., "Navy Blue (2024 Edition)" -> "Navy Blue")
	normalized = cn.yearPattern.ReplaceAllString(normalized, "")

	// Remove other parenthetical content for value field
	normalized = cn.parenthesesPattern.ReplaceAllString(normalized, "")

	// Normalize whitespace
	normalized = cn.whitespacePattern.ReplaceAllString(normalized, " ")

	// Convert to lowercase for value field
	normalized = strings.ToLower(strings.TrimSpace(normalized))

	// Limit length
	if maxLength > 0 && len(normalized) > maxLength {
		normalized = normalized[:maxLength]
	}

	return normalized
}

// NormalizeDisplayName normalizes a color name for the DisplayName field
// - Preserves original casing
// - Only trims whitespace
// - Keeps decorative elements
func (cn *ColorNormalizer) NormalizeDisplayName(colorName string) string {
	return strings.TrimSpace(colorName)
}

// DeduplicateVariations removes duplicate color variations based on normalized value
func (cn *ColorNormalizer) DeduplicateVariations(variations []*models.VariationItem, maxLength int) []*models.VariationItem {
	seen := make(map[string]bool)
	var deduplicated []*models.VariationItem

	for _, variation := range variations {
		if variation == nil {
			continue
		}

		// Normalize the value for deduplication comparison
		normalizedValue := cn.NormalizeValue(variation.Value, maxLength)
		
		// Skip if we've already seen this normalized value
		if seen[normalizedValue] {
			continue
		}

		// Mark as seen
		seen[normalizedValue] = true

		// Create normalized variation
		normalizedVariation := &models.VariationItem{
			Value:       normalizedValue,
			DisplayName: cn.NormalizeDisplayName(variation.DisplayName),
			ASIN:        variation.ASIN,
			Available:   variation.Available,
		}

		// Ensure DisplayName is set
		if normalizedVariation.DisplayName == "" {
			normalizedVariation.DisplayName = variation.Value
		}

		deduplicated = append(deduplicated, normalizedVariation)
	}

	return deduplicated
}

// ExtractCommonPatterns extracts common color patterns from product text
// This can be used as a fallback when PA-API doesn't provide color data
func (cn *ColorNormalizer) ExtractCommonPatterns(text string) []string {
	var colors []string
	
	// Common color patterns in German/English
	colorPatterns := []string{
		"schwarz", "black",
		"weiß", "weiss", "white",
		"rot", "red",
		"blau", "blue",
		"grün", "gruen", "green",
		"gelb", "yellow",
		"grau", "grey", "gray",
		"braun", "brown",
		"navy", "marine",
		"beige", "khaki",
		"pink", "rosa",
		"lila", "purple", "violett",
		"orange",
		"türkis", "tuerkis", "turquoise",
	}

	textLower := strings.ToLower(text)
	for _, pattern := range colorPatterns {
		if strings.Contains(textLower, pattern) {
			colors = append(colors, pattern)
		}
	}

	return colors
}

// ValidateVariation validates a single variation item
func (cn *ColorNormalizer) ValidateVariation(variation *models.VariationItem) bool {
	if variation == nil {
		return false
	}

	// Validate using the model's built-in validation
	if err := variation.Validate(); err != nil {
		return false
	}

	// Additional validation: check for suspicious patterns
	value := strings.ToLower(variation.Value)
	
	// Skip variations that are clearly not colors
	skipPatterns := []string{
		"size", "größe",
		"länge", "length",
		"breite", "width",
		"material",
		"style", "stil",
	}

	for _, pattern := range skipPatterns {
		if strings.Contains(value, pattern) {
			return false
		}
	}

	// Check minimum length (colors should have at least 2 characters)
	if len(variation.Value) < 2 {
		return false
	}

	// Check maximum length (colors shouldn't be too long)
	if len(variation.Value) > 100 {
		return false
	}

	return true
}

// ProcessVariations processes and normalizes a list of variations
// This is the main entry point for normalizing color variations
func (cn *ColorNormalizer) ProcessVariations(variations []*models.VariationItem, maxLength int) []*models.VariationItem {
	var processed []*models.VariationItem

	// First pass: normalize and validate
	for _, variation := range variations {
		if !cn.ValidateVariation(variation) {
			continue
		}

		// Create normalized variation
		normalized := &models.VariationItem{
			Value:       cn.NormalizeValue(variation.Value, maxLength),
			DisplayName: cn.NormalizeDisplayName(variation.DisplayName),
			ASIN:        variation.ASIN,
			Available:   variation.Available,
		}

		// Ensure DisplayName is set
		if normalized.DisplayName == "" {
			normalized.DisplayName = variation.DisplayName
			if normalized.DisplayName == "" {
				normalized.DisplayName = variation.Value
			}
		}

		processed = append(processed, normalized)
	}

	// Second pass: deduplicate
	return cn.DeduplicateVariations(processed, maxLength)
}