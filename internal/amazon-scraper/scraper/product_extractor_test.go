package scraper

import (
	"testing"

	"github.com/maltedev/amazon-size-scraper/internal/database"
	"github.com/stretchr/testify/assert"
)

func TestExtractCompleteProductData(t *testing.T) {
	tests := []struct {
		name        string
		asin        string
		expectValid bool
		checkFunc   func(t *testing.T, product *CompleteProduct)
	}{
		{
			name:        "Valid product with size table containing length and width",
			asin:        "B08N5WRWNW",
			expectValid: true,
			checkFunc: func(t *testing.T, product *CompleteProduct) {
				assert.NotEmpty(t, product.Title)
				assert.NotEmpty(t, product.DetailPageURL)
				assert.NotNil(t, product.SizeTable)
				assert.True(t, len(product.SizeTable.Sizes) > 0)
				
				// Check that at least one size has length and width
				hasLengthAndWidth := false
				for _, measurements := range product.SizeTable.Measurements {
					if _, hasLength := measurements["length"]; hasLength {
						if _, hasWidth := measurements["width"]; hasWidth {
							hasLengthAndWidth = true
							break
						}
					}
				}
				assert.True(t, hasLengthAndWidth, "Size table must have length and width measurements")
			},
		},
		{
			name:        "Product without size table",
			asin:        "B0INVALIDNO",
			expectValid: false,
			checkFunc: func(t *testing.T, product *CompleteProduct) {
				assert.Nil(t, product)
			},
		},
		{
			name:        "Product with size table but missing length/width",
			asin:        "B0SIZENOLEN",
			expectValid: false,
			checkFunc: func(t *testing.T, product *CompleteProduct) {
				assert.Nil(t, product)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This would need a mock browser or test mode
			t.Skip("Requires browser implementation")
		})
	}
}

func TestValidateSizeTableForProduct(t *testing.T) {
	tests := []struct {
		name      string
		sizeTable *database.SizeTable
		expected  bool
	}{
		{
			name: "Valid size table with length and width",
			sizeTable: &database.SizeTable{
				Sizes: []string{"S", "M", "L"},
				Measurements: map[string]map[string]float64{
					"S": {"chest": 96, "length": 70, "width": 52},
					"M": {"chest": 100, "length": 72, "width": 54},
					"L": {"chest": 104, "length": 74, "width": 56},
				},
				Unit: "cm",
			},
			expected: true,
		},
		{
			name: "Invalid - no length",
			sizeTable: &database.SizeTable{
				Sizes: []string{"S", "M"},
				Measurements: map[string]map[string]float64{
					"S": {"chest": 96, "width": 52},
					"M": {"chest": 100, "width": 54},
				},
				Unit: "cm",
			},
			expected: false,
		},
		{
			name: "Invalid - no width",
			sizeTable: &database.SizeTable{
				Sizes: []string{"S", "M"},
				Measurements: map[string]map[string]float64{
					"S": {"chest": 96, "length": 70},
					"M": {"chest": 100, "length": 72},
				},
				Unit: "cm",
			},
			expected: false,
		},
		{
			name: "Invalid - empty measurements",
			sizeTable: &database.SizeTable{
				Sizes:        []string{"S", "M"},
				Measurements: map[string]map[string]float64{},
				Unit:         "cm",
			},
			expected: false,
		},
		{
			name:      "Invalid - nil table",
			sizeTable: nil,
			expected:  false,
		},
		{
			name: "Valid - at least one size has length and width",
			sizeTable: &database.SizeTable{
				Sizes: []string{"S", "M", "L"},
				Measurements: map[string]map[string]float64{
					"S": {"chest": 96}, // Missing length and width
					"M": {"chest": 100, "length": 72, "width": 54}, // Has both
					"L": {"chest": 104, "length": 74}, // Missing width
				},
				Unit: "cm",
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateSizeTableForProduct(tt.sizeTable)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParseProductDetails(t *testing.T) {
	t.Run("Parse price from German format", func(t *testing.T) {
		testCases := []struct {
			input    string
			expected float64
		}{
			{"19,99 €", 19.99},
			{"1.299,00 €", 1299.00},
			{"€ 45,50", 45.50},
			{"29,99", 29.99},
		}

		for _, tc := range testCases {
			result := parsePrice(tc.input)
			assert.Equal(t, tc.expected, result)
		}
	})

	t.Run("Parse rating", func(t *testing.T) {
		testCases := []struct {
			input    string
			expected float64
		}{
			{"4,5 von 5 Sternen", 4.5},
			{"3,0 von 5", 3.0},
			{"4.5 out of 5 stars", 4.5},
		}

		for _, tc := range testCases {
			result := parseRating(tc.input)
			assert.Equal(t, tc.expected, result)
		}
	})

	t.Run("Parse review count", func(t *testing.T) {
		testCases := []struct {
			input    string
			expected int
		}{
			{"1.234 Bewertungen", 1234},
			{"567 Sternebewertungen", 567},
			{"89 Kundenrezensionen", 89},
		}

		for _, tc := range testCases {
			result := parseReviewCount(tc.input)
			assert.Equal(t, tc.expected, result)
		}
	})
}

// Mock helper functions that would be implemented in the actual code
func parsePrice(input string) float64 {
	// This is a placeholder - actual implementation would parse German price format
	return 0.0
}

func parseRating(input string) float64 {
	// This is a placeholder - actual implementation would parse rating
	return 0.0
}

func parseReviewCount(input string) int {
	// This is a placeholder - actual implementation would parse review count
	return 0
}

func ValidateSizeTableForProduct(st *database.SizeTable) bool {
	return database.ValidateSizeTable(st)
}