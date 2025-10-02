package scraper

import (
	"testing"

	"github.com/maltedev/amazon-size-scraper/internal/models"
	"github.com/stretchr/testify/assert"
	"log/slog"
)

func TestValidateColorVariations(t *testing.T) {
	ps := &ProductScraper{
		logger: slog.Default(),
	}

	tests := []struct {
		name     string
		input    []*models.VariationItem
		expected int
		desc     string
	}{
		{
			name: "valid_colors",
			input: []*models.VariationItem{
				{Value: "Red", ASIN: "B001", Available: true},
				{Value: "Blue", ASIN: "B002", Available: true},
				{Value: "Green", ASIN: "B003", Available: false},
			},
			expected: 3,
			desc:     "All valid colors should be returned",
		},
		{
			name: "duplicate_colors",
			input: []*models.VariationItem{
				{Value: "Red", ASIN: "B001", Available: true},
				{Value: "Red", ASIN: "B001", Available: true},
				{Value: "Blue", ASIN: "B002", Available: true},
			},
			expected: 2,
			desc:     "Duplicate colors should be filtered out",
		},
		{
			name: "empty_values",
			input: []*models.VariationItem{
				{Value: "", ASIN: "B001", Available: true},
				{Value: "Blue", ASIN: "B002", Available: true},
				{Value: "", ASIN: "B003", Available: false},
			},
			expected: 1,
			desc:     "Empty values should be filtered out",
		},
		{
			name: "nil_items",
			input: []*models.VariationItem{
				nil,
				{Value: "Blue", ASIN: "B002", Available: true},
				nil,
			},
			expected: 1,
			desc:     "Nil items should be filtered out",
		},
		{
			name: "empty_display_name",
			input: []*models.VariationItem{
				{Value: "Red", ASIN: "B001", Available: true, DisplayName: ""},
				{Value: "Blue", ASIN: "B002", Available: true, DisplayName: "Dark Blue"},
			},
			expected: 2,
			desc:     "Empty DisplayName should default to Value",
		},
		{
			name:     "empty_input",
			input:    []*models.VariationItem{},
			expected: 0,
			desc:     "Empty input should return empty result",
		},
		{
			name:     "nil_input",
			input:    nil,
			expected: 0,
			desc:     "Nil input should return empty result",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ps.validateColorVariations(tt.input)
			assert.Equal(t, tt.expected, len(result), tt.desc)

			// Additional checks for specific test cases
			if tt.name == "empty_display_name" && len(result) > 0 {
				// Check that empty DisplayName was set to Value
				assert.Equal(t, "Red", result[0].DisplayName, "DisplayName should default to Value")
			}
		})
	}
}

func TestGetString(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]interface{}
		key      string
		expected string
	}{
		{
			name:     "existing_string",
			input:    map[string]interface{}{"color": "red"},
			key:      "color",
			expected: "red",
		},
		{
			name:     "missing_key",
			input:    map[string]interface{}{"size": "large"},
			key:      "color",
			expected: "",
		},
		{
			name:     "non_string_value",
			input:    map[string]interface{}{"count": 42},
			key:      "count",
			expected: "",
		},
		{
			name:     "nil_map",
			input:    nil,
			key:      "color",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getString(tt.input, tt.key)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetBool(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]interface{}
		key      string
		expected bool
	}{
		{
			name:     "existing_true",
			input:    map[string]interface{}{"available": true},
			key:      "available",
			expected: true,
		},
		{
			name:     "existing_false",
			input:    map[string]interface{}{"available": false},
			key:      "available",
			expected: false,
		},
		{
			name:     "missing_key",
			input:    map[string]interface{}{"size": "large"},
			key:      "available",
			expected: false,
		},
		{
			name:     "non_bool_value",
			input:    map[string]interface{}{"available": "yes"},
			key:      "available",
			expected: false,
		},
		{
			name:     "nil_map",
			input:    nil,
			key:      "available",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getBool(tt.input, tt.key)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNormalizeLabel(t *testing.T) {
	ps := &ProductScraper{
		logger: slog.Default(),
	}

	tests := []struct {
		input    string
		expected string
	}{
		{"länge", "length"},
		{"Länge", "length"},
		{"LÄNGE", "length"},
		{"breite", "width"},
		{"Breite", "width"},
		{"brustumfang", "chest"},
		{"Brustumfang", "chest"},
		{"brust", "chest"},
		{"schulter", "shoulder"},
		{"ärmel", "sleeve"},
		{"höhe", "height"},
		{"taille", "waist"},
		{"hüfte", "hip"},
		{"Gesamtlänge", "length"},
		{"laenge", "length"},
		{"unknown_label", "unknown_label"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := ps.normalizeLabel(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParseValue(t *testing.T) {
	ps := &ProductScraper{
		logger: slog.Default(),
	}

	tests := []struct {
		input    string
		expected float64
		desc     string
	}{
		{"42", 42.0, "Simple integer"},
		{"42.5", 42.5, "Decimal with dot"},
		{"42,5", 42.5, "German decimal with comma"},
		{"84 - 94", 89.0, "Range should return average"},
		{"100-110", 105.0, "Range without spaces"},
		{"42 cm", 42.0, "Value with unit"},
		{"ca. 42", 42.0, "Value with text prefix"},
		{"", 0.0, "Empty string"},
		{"invalid", 0.0, "Invalid text"},
		{"42.5 - 47.5", 45.0, "Decimal range"},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			result := ps.parseValue(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsSizeLabel(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"S", true},
		{"M", true},
		{"L", true},
		{"XL", true},
		{"XXL", true},
		{"XXXL", true},
		{"3XL", true},
		{"4XL", true},
		{"5XL", true},
		{"6XL", true},
		{"s", true},  // Should match lowercase
		{"xl", true}, // Should match lowercase
		{"XS", true},
		{"Small", false},
		{"Medium", false},
		{"Large", false},
		{"", false},
		{"42", false},
		{"A", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := isSizeLabel(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}