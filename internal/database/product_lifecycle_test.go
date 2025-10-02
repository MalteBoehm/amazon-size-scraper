package database

import (
	"testing"
)

func TestValidateSizeTable(t *testing.T) {
	tests := []struct {
		name     string
		table    *SizeTable
		expected bool
	}{
		{
			name:     "nil table",
			table:    nil,
			expected: false,
		},
		{
			name: "empty table",
			table: &SizeTable{
				Sizes:        []string{},
				Measurements: map[string]map[string]float64{},
			},
			expected: false,
		},
		{
			name: "table with only chest measurements",
			table: &SizeTable{
				Unit:  "cm",
				Sizes: []string{"S", "M", "L"},
				Measurements: map[string]map[string]float64{
					"S": {"chest": 92},
					"M": {"chest": 98},
					"L": {"chest": 104},
				},
			},
			expected: false,
		},
		{
			name: "table with only length measurements",
			table: &SizeTable{
				Unit:  "cm",
				Sizes: []string{"S", "M", "L"},
				Measurements: map[string]map[string]float64{
					"S": {"length": 73},
					"M": {"length": 75},
					"L": {"length": 77},
				},
			},
			expected: false,
		},
		{
			name: "valid table with chest and length",
			table: &SizeTable{
				Unit:  "cm",
				Sizes: []string{"S", "M", "L"},
				Measurements: map[string]map[string]float64{
					"S": {"chest": 92, "length": 73},
					"M": {"chest": 98, "length": 75},
					"L": {"chest": 104, "length": 77},
				},
			},
			expected: true,
		},
		{
			name: "table with bust instead of chest",
			table: &SizeTable{
				Unit:  "cm",
				Sizes: []string{"S", "M", "L"},
				Measurements: map[string]map[string]float64{
					"S": {"bust": 92, "length": 73},
					"M": {"bust": 98, "length": 75},
					"L": {"bust": 104, "length": 77},
				},
			},
			expected: true,
		},
		{
			name: "table with missing measurements for one size",
			table: &SizeTable{
				Unit:  "cm",
				Sizes: []string{"S", "M", "L"},
				Measurements: map[string]map[string]float64{
					"S": {"chest": 92, "length": 73},
					"M": {"chest": 98}, // missing length
					"L": {"chest": 104, "length": 77},
				},
			},
			expected: false,
		},
		{
			name: "table with zero values",
			table: &SizeTable{
				Unit:  "cm",
				Sizes: []string{"S", "M"},
				Measurements: map[string]map[string]float64{
					"S": {"chest": 92, "length": 0}, // zero length
					"M": {"chest": 98, "length": 75},
				},
			},
			expected: false,
		},
		{
			name: "real example from user - missing length",
			table: &SizeTable{
				Unit:  "cm",
				Sizes: []string{"S", "M", "L", "XL", "3XL", "4XL", "5XL", "6XL"},
				Measurements: map[string]map[string]float64{
					"S":   {"chest": 92},
					"M":   {"chest": 98},
					"L":   {"chest": 104},
					"XL":  {"chest": 110},
					"3XL": {"chest": 125},
					"4XL": {"chest": 134},
					"5XL": {"chest": 143},
					"6XL": {"chest": 152},
				},
			},
			expected: false,
		},
		{
			name: "table with arm length instead of t-shirt length",
			table: &SizeTable{
				Unit:  "cm",
				Sizes: []string{"S", "M", "L", "XL"},
				Measurements: map[string]map[string]float64{
					"S":  {"chest": 102, "sleeve": 20},    // sleeve/arm length, NOT t-shirt length
					"M":  {"chest": 108, "sleeve": 20.5},
					"L":  {"chest": 114, "sleeve": 21.5},
					"XL": {"chest": 120, "sleeve": 22.5},
				},
			},
			expected: false, // Should fail because it has no actual "length" measurement
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateSizeTable(tt.table)
			if result != tt.expected {
				t.Errorf("ValidateSizeTable() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestValidateSizeTableWithDetails(t *testing.T) {
	tests := []struct {
		name           string
		table          *SizeTable
		expectedValid  bool
		expectedFields []string
	}{
		{
			name:           "nil table",
			table:          nil,
			expectedValid:  false,
			expectedFields: []string{"no_size_table"},
		},
		{
			name: "missing chest measurements",
			table: &SizeTable{
				Unit:  "cm",
				Sizes: []string{"S", "M", "L"},
				Measurements: map[string]map[string]float64{
					"S": {"length": 73},
					"M": {"length": 75},
					"L": {"length": 77},
				},
			},
			expectedValid:  false,
			expectedFields: []string{"chest_measurements"},
		},
		{
			name: "missing length measurements",
			table: &SizeTable{
				Unit:  "cm",
				Sizes: []string{"S", "M", "L"},
				Measurements: map[string]map[string]float64{
					"S": {"chest": 92},
					"M": {"chest": 98},
					"L": {"chest": 104},
				},
			},
			expectedValid:  false,
			expectedFields: []string{"length_measurements"},
		},
		{
			name: "missing both measurements",
			table: &SizeTable{
				Unit:  "cm",
				Sizes: []string{"S", "M", "L"},
				Measurements: map[string]map[string]float64{
					"S": {"shoulder": 45},
					"M": {"shoulder": 47},
					"L": {"shoulder": 49},
				},
			},
			expectedValid:  false,
			expectedFields: []string{"chest_measurements", "length_measurements"},
		},
		{
			name: "valid complete table",
			table: &SizeTable{
				Unit:  "cm",
				Sizes: []string{"S", "M", "L"},
				Measurements: map[string]map[string]float64{
					"S": {"chest": 92, "length": 73, "shoulder": 45},
					"M": {"chest": 98, "length": 75, "shoulder": 47},
					"L": {"chest": 104, "length": 77, "shoulder": 49},
				},
			},
			expectedValid:  true,
			expectedFields: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			valid, fields := ValidateSizeTableWithDetails(tt.table)
			if valid != tt.expectedValid {
				t.Errorf("ValidateSizeTableWithDetails() valid = %v, want %v", valid, tt.expectedValid)
			}
			
			// Check missing fields
			if len(fields) != len(tt.expectedFields) {
				t.Errorf("ValidateSizeTableWithDetails() fields = %v, want %v", fields, tt.expectedFields)
			} else {
				// For some tests, we expect general missing fields, not size-specific
				if len(tt.expectedFields) > 0 && (tt.expectedFields[0] == "chest_measurements" || 
					tt.expectedFields[0] == "length_measurements" || 
					tt.expectedFields[0] == "no_size_table") {
					// Just check if the fields contain expected keywords
					for i, expectedField := range tt.expectedFields {
						if i < len(fields) && fields[i] != expectedField {
							t.Errorf("ValidateSizeTableWithDetails() field[%d] = %v, want %v", i, fields[i], expectedField)
						}
					}
				}
			}
		})
	}
}