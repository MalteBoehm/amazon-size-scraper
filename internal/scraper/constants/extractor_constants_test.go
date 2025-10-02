package constants

import (
	"testing"
)

func TestGenderConstants(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
		valid    bool
	}{
		{
			name:     "valid gender damen",
			input:    "Damen",
			expected: GenderDamen,
			valid:    true,
		},
		{
			name:     "valid gender herren",
			input:    "Herren",
			expected: GenderHerren,
			valid:    true,
		},
		{
			name:     "valid gender unisex",
			input:    "Unisex",
			expected: GenderUnisex,
			valid:    true,
		},
		{
			name:     "valid gender jungen",
			input:    "Jungen",
			expected: GenderJungen,
			valid:    true,
		},
		{
			name:     "valid gender mädchen",
			input:    "Mädchen",
			expected: GenderMädchen,
			valid:    true,
		},
		{
			name:     "invalid gender",
			input:    "Invalid",
			expected: "",
			valid:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.valid {
				if !IsValidGender(tt.input) {
					t.Errorf("expected %s to be a valid gender", tt.input)
				}
			} else {
				if IsValidGender(tt.input) {
					t.Errorf("expected %s to be an invalid gender", tt.input)
				}
			}
		})
	}
}

func TestPrimeEligibilitySelectors(t *testing.T) {
	selectors := GetPrimeEligibilitySelectors()

	expectedSelectors := []string{
		"i.a-icon-prime",
		"span.a-icon-alt:has-text('Prime')",
		"div#apex_desktop_delivery_feature_div i.a-icon-prime",
		"span.a-color-base:has-text('Prime')",
	}

	if len(selectors) != len(expectedSelectors) {
		t.Errorf("expected %d selectors, got %d", len(expectedSelectors), len(selectors))
	}

	for i, selector := range expectedSelectors {
		if i < len(selectors) && selectors[i] != selector {
			t.Errorf("selector mismatch at index %d: expected %s, got %s", i, selector, selectors[i])
		}
	}
}

func TestStockStatusSelectors(t *testing.T) {
	selectors := GetStockStatusSelectors()

	if len(selectors) == 0 {
		t.Error("expected stock status selectors to be defined")
	}

	// Verify specific selectors exist
	expectedSelectors := []string{
		"div#availability span",
		"div.a-section.a-spacing-base span.a-size-medium",
		"span.a-declarative[data-action='availability-message-show-text']",
	}

	for _, expected := range expectedSelectors {
		found := false
		for _, selector := range selectors {
			if selector == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected selector %s not found", expected)
		}
	}
}

func TestOutOfStockKeywords(t *testing.T) {
	keywords := GetOutOfStockKeywords()

	expectedKeywords := []string{
		"nicht auf lager",
		"nicht verfügbar",
		"derzeit nicht verfügbar",
		"out of stock",
		"unavailable",
	}

	if len(keywords) != len(expectedKeywords) {
		t.Errorf("expected %d keywords, got %d", len(expectedKeywords), len(keywords))
	}

	for _, expected := range expectedKeywords {
		found := false
		for _, keyword := range keywords {
			if keyword == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected keyword '%s' not found", expected)
		}
	}
}

func TestMaterialKeywords(t *testing.T) {
	keywords := GetMaterialKeywords()

	// Test that we have material keywords
	if len(keywords) == 0 {
		t.Error("expected material keywords to be defined")
	}

	// Check for essential material keywords
	essentialKeywords := []string{
		"baumwolle", "cotton", "polyester", "elasthan", "spandex",
		"wolle", "wool", "seide", "silk", "leinen", "linen",
	}

	for _, essential := range essentialKeywords {
		found := false
		for _, keyword := range keywords {
			if keyword == essential {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("essential material keyword '%s' not found", essential)
		}
	}
}

func TestColorSwatchSelectors(t *testing.T) {
	selectors := GetColorSwatchSelectors()

	if len(selectors) == 0 {
		t.Error("expected color swatch selectors to be defined")
	}

	// Verify primary selector exists (data-asin)
	primary := "div#variation_color_name li[data-asin]"
	legacy := "div#variation_color_name li[data-defaultasin]"
	hasPrimary := false
	hasLegacy := false
	for _, selector := range selectors {
		if selector == primary {
			hasPrimary = true
		}
		if selector == legacy {
			hasLegacy = true
		}
	}
	if !hasPrimary {
		t.Errorf("primary color selector %s not found", primary)
	}
	if !hasLegacy {
		t.Errorf("legacy color selector %s not found", legacy)
	}
}

func TestSizeDropdownSelectors(t *testing.T) {
	selectors := GetSizeDropdownSelectors()

	expectedSelectors := []string{
		"select#native_dropdown_selected_size_name option",
		"select[name='dropdown_selected_size_name'] option",
	}

	for _, expected := range expectedSelectors {
		found := false
		for _, selector := range selectors {
			if selector == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected size dropdown selector %s not found", expected)
		}
	}
}

func TestGenderIndicatorPatterns(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		expected string
	}{
		{
			name:     "detect damen in title",
			text:     "Schönes Damen T-Shirt",
			expected: GenderDamen,
		},
		{
			name:     "detect herren in category",
			text:     "Herren Bekleidung",
			expected: GenderHerren,
		},
		{
			name:     "detect women in english",
			text:     "Women's Fashion Top",
			expected: GenderDamen,
		},
		{
			name:     "detect unisex",
			text:     "Unisex Hoodie für Erwachsene",
			expected: GenderUnisex,
		},
		{
			name:     "no gender indicator",
			text:     "Normale Jacke",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DetectGender(tt.text)
			if result != tt.expected {
				t.Errorf("expected gender %s, got %s for text: %s", tt.expected, result, tt.text)
			}
		})
	}
}

func TestDetectGenderStandardized(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		expected string
	}{
		{
			name:     "detect damen as female",
			text:     "Schönes Damen T-Shirt",
			expected: "female",
		},
		{
			name:     "detect herren as male",
			text:     "Herren Bekleidung",
			expected: "male",
		},
		{
			name:     "detect women as female",
			text:     "Women's Fashion Top",
			expected: "female",
		},
		{
			name:     "detect men as male",
			text:     "Men's Casual Shirt",
			expected: "male",
		},
		{
			name:     "detect unisex",
			text:     "Unisex Hoodie für Erwachsene",
			expected: "unisex",
		},
		{
			name:     "detect jungen as male",
			text:     "Jungen Sportshirt",
			expected: "male",
		},
		{
			name:     "detect mädchen as female",
			text:     "Mädchen Kleid",
			expected: "female",
		},
		{
			name:     "no gender indicator defaults to unisex",
			text:     "Normale Jacke",
			expected: "unisex",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DetectGenderStandardized(tt.text)
			if result != tt.expected {
				t.Errorf("expected standardized gender %s, got %s for text: %s", tt.expected, result, tt.text)
			}
		})
	}
}

func TestMapGenderToStandardized(t *testing.T) {
	tests := []struct {
		name           string
		germanGender   string
		expectedResult string
	}{
		{
			name:           "map Damen to female",
			germanGender:   GenderDamen,
			expectedResult: "female",
		},
		{
			name:           "map Herren to male",
			germanGender:   GenderHerren,
			expectedResult: "male",
		},
		{
			name:           "map Unisex to unisex",
			germanGender:   GenderUnisex,
			expectedResult: "unisex",
		},
		{
			name:           "map Jungen to male",
			germanGender:   GenderJungen,
			expectedResult: "male",
		},
		{
			name:           "map Mädchen to female",
			germanGender:   GenderMädchen,
			expectedResult: "female",
		},
		{
			name:           "map unknown to unisex",
			germanGender:   "Unknown",
			expectedResult: "unisex",
		},
		{
			name:           "map empty to unisex",
			germanGender:   "",
			expectedResult: "unisex",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MapGenderToStandardized(tt.germanGender)
			if result != tt.expectedResult {
				t.Errorf("expected %s, got %s for german gender: %s", tt.expectedResult, result, tt.germanGender)
			}
		})
	}
}
