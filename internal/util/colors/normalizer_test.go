package colors

import (
	"testing"
	"github.com/maltedev/amazon-size-scraper/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestColorNormalizer_NormalizeValue(t *testing.T) {
	cn := NewColorNormalizer()

	tests := []struct {
		name     string
		input    string
		maxLen   int
		expected string
	}{
		{
			name:     "Remove technical prefix",
			input:    "X123 Navy Blue",
			maxLen:   100,
			expected: "navy blue",
		},
		{
			name:     "Remove year suffix",
			input:    "Navy Blue (2024 Edition)",
			maxLen:   100,
			expected: "navy blue",
		},
		{
			name:     "Remove parentheses content",
			input:    "Red (Bright Red)",
			maxLen:   100,
			expected: "red",
		},
		{
			name:     "Normalize whitespace",
			input:    "  Dark   Gray  ",
			maxLen:   100,
			expected: "dark gray",
		},
		{
			name:     "Convert to lowercase",
			input:    "NAVY BLUE",
			maxLen:   100,
			expected: "navy blue",
		},
		{
			name:     "Respect max length",
			input:    "Very Long Color Name That Exceeds Maximum",
			maxLen:   10,
			expected: "very long ",
		},
		{
			name:     "Handle German characters",
			input:    "Königsblau",
			maxLen:   100,
			expected: "königsblau",
		},
		{
			name:     "Complex pattern",
			input:    "X999 Royal Blue (2024 Limited Edition)",
			maxLen:   100,
			expected: "royal blue",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cn.NormalizeValue(tt.input, tt.maxLen)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestColorNormalizer_NormalizeDisplayName(t *testing.T) {
	cn := NewColorNormalizer()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Preserve casing",
			input:    "Navy Blue (2024)",
			expected: "Navy Blue (2024)",
		},
		{
			name:     "Trim whitespace only",
			input:    "  Red  ",
			expected: "Red",
		},
		{
			name:     "Keep decorative elements",
			input:    "Royal Blue (Limited Edition)",
			expected: "Royal Blue (Limited Edition)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cn.NormalizeDisplayName(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestColorNormalizer_DeduplicateVariations(t *testing.T) {
	cn := NewColorNormalizer()

	variations := []*models.VariationItem{
		{Value: "Navy Blue", DisplayName: "Navy Blue", ASIN: "ASIN1", Available: true},
		{Value: "NAVY BLUE", DisplayName: "NAVY BLUE", ASIN: "ASIN2", Available: true},
		{Value: "X123 Navy Blue", DisplayName: "X123 Navy Blue", ASIN: "ASIN3", Available: false},
		{Value: "Red", DisplayName: "Red", ASIN: "ASIN4", Available: true},
		{Value: "RED", DisplayName: "RED", ASIN: "ASIN5", Available: true},
	}

	result := cn.DeduplicateVariations(variations, 100)

	// Should have only 2 unique colors: navy blue and red
	require.Len(t, result, 2)
	
	// Check that we have the expected normalized values
	values := make(map[string]bool)
	for _, v := range result {
		values[v.Value] = true
	}
	assert.True(t, values["navy blue"])
	assert.True(t, values["red"])
}

func TestColorNormalizer_ValidateVariation(t *testing.T) {
	cn := NewColorNormalizer()

	tests := []struct {
		name      string
		variation *models.VariationItem
		expected  bool
	}{
		{
			name:      "Valid color",
			variation: &models.VariationItem{Value: "Navy Blue", Available: true},
			expected:  true,
		},
		{
			name:      "Empty value",
			variation: &models.VariationItem{Value: "", Available: true},
			expected:  false,
		},
		{
			name:      "Too short",
			variation: &models.VariationItem{Value: "X", Available: true},
			expected:  false,
		},
		{
			name:      "Too long",
			variation: &models.VariationItem{Value: string(make([]byte, 101)), Available: true},
			expected:  false,
		},
		{
			name:      "Contains size keyword",
			variation: &models.VariationItem{Value: "Size Large", Available: true},
			expected:  false,
		},
		{
			name:      "Contains material keyword",
			variation: &models.VariationItem{Value: "Cotton Material", Available: true},
			expected:  false,
		},
		{
			name:      "Nil variation",
			variation: nil,
			expected:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cn.ValidateVariation(tt.variation)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestColorNormalizer_ProcessVariations(t *testing.T) {
	cn := NewColorNormalizer()

	variations := []*models.VariationItem{
		{Value: "X123 Navy Blue (2024)", DisplayName: "X123 Navy Blue (2024)", ASIN: "ASIN1", Available: true},
		{Value: "NAVY BLUE", DisplayName: "NAVY BLUE", ASIN: "ASIN2", Available: true},
		{Value: "Size Large", DisplayName: "Size Large", ASIN: "ASIN3", Available: true}, // Should be filtered
		{Value: "", DisplayName: "", ASIN: "ASIN4", Available: true},                    // Should be filtered
		{Value: "Red", DisplayName: "Red", ASIN: "ASIN5", Available: true},
		nil, // Should be filtered
	}

	result := cn.ProcessVariations(variations, 100)

	// Should have only 2 valid, unique colors
	require.Len(t, result, 2)
	
	// Check normalized values
	values := make(map[string]bool)
	for _, v := range result {
		values[v.Value] = true
		// DisplayName should be preserved (with trim)
		assert.NotEmpty(t, v.DisplayName)
	}
	assert.True(t, values["navy blue"])
	assert.True(t, values["red"])
}

func TestColorNormalizer_ExtractCommonPatterns(t *testing.T) {
	cn := NewColorNormalizer()

	text := "This product is available in Schwarz, Navy Blue, and Königsblau colors."
	patterns := cn.ExtractCommonPatterns(text)

	assert.Contains(t, patterns, "schwarz")
	assert.Contains(t, patterns, "navy")
	assert.Contains(t, patterns, "blue")
}