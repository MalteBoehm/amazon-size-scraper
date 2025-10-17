package scraper

import (
	"testing"
)

func TestIsSizeLabel(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"Valid S", "S", true},
		{"Valid M", "M", true},
		{"Valid L", "L", true},
		{"Valid XL", "XL", true},
		{"Valid XXL", "XXL", true},
		{"Valid 3XL", "3XL", true},
		{"Lowercase", "xl", true},
		{"With spaces", " XL ", true},
		{"Invalid", "ABC", false},
		{"Number", "42", false},
		{"Empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isSizeLabel(tt.input)
			if result != tt.expected {
				t.Errorf("isSizeLabel(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestParseValue(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected float64
	}{
		{"Simple number", "42", 42.0},
		{"With cm", "42 cm", 42.0},
		{"With decimal", "42.5", 42.5},
		{"German decimal", "42,5", 42.5},
		{"Range max", "84 - 94", 94.0},
		{"Range with units", "84 - 94 cm", 94.0},
		{"Text only", "abc", 0.0},
		{"Empty", "", 0.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseValue(tt.input)
			if result != tt.expected {
				t.Errorf("parseValue(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestParseTableData(t *testing.T) {
	s := &Service{}

	// Test horizontal layout (sizes in first row)
	tableData := map[string]interface{}{
		"headers": []interface{}{"Größe", "Brustumfang", "Länge"},
		"rows": []interface{}{
			[]interface{}{"S", "84-94", "70"},
			[]interface{}{"M", "95-98", "72"},
			[]interface{}{"L", "99-102", "74"},
		},
	}

	dimensions := s.parseTableData(tableData)

	if dimensions.WidthCM == 0 {
		t.Error("Expected width to be calculated from chest measurement")
	}

	if dimensions.LengthCM == 0 {
		t.Error("Expected length to be extracted")
	}
}

func TestDimensionExtraction(t *testing.T) {
	// Test with mock table data
	tableData := map[string]interface{}{
		"headers": []interface{}{"Größe", "Brustumfang (cm)", "Länge (cm)"},
		"rows": []interface{}{
			[]interface{}{"S", "84-94", "70"},
			[]interface{}{"M", "95-98", "72"},
			[]interface{}{"L", "99-102", "74"},
			[]interface{}{"XL", "103-106", "76"},
		},
	}

	s := &Service{}
	dimensions := s.parseTableData(tableData)

	// Should use XL measurements (largest common size)
	expectedWidth := 106.0 / 2 // Chest / 2
	expectedLength := 76.0

	if dimensions.WidthCM != expectedWidth {
		t.Errorf("Expected width %v, got %v", expectedWidth, dimensions.WidthCM)
	}

	if dimensions.LengthCM != expectedLength {
		t.Errorf("Expected length %v, got %v", expectedLength, dimensions.LengthCM)
	}
}