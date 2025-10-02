package database

import (
	"fmt"
	"testing"
)

func TestValidateSizeTable_UnrealisticMeasurements(t *testing.T) {
	tests := []struct {
		name           string
		sizeTable      *SizeTable
		expectedValid  bool
		expectedReason []string
	}{
		{
			name: "Unrealistic chest measurements from user example",
			sizeTable: &SizeTable{
				Sizes: []string{"S", "M", "L", "XL"},
				Measurements: map[string]map[string]float64{
					"S": {
						"chest":  256.5, // Unrealistic!
						"length": 80.3,  // Normal
					},
					"M": {
						"chest":  276.9, // Unrealistic!
						"length": 85.4,  // Normal
					},
					"L": {
						"chest":  297.2, // Unrealistic!
						"length": 90.5,  // Normal
					},
					"XL": {
						"chest":  317.5, // Unrealistic!
						"length": 95.6,  // Normal
					},
				},
				Unit: "cm",
			},
			expectedValid: false,
			expectedReason: []string{
				"unrealistic_chest_256.5cm_for_size_S",
				"unrealistic_chest_276.9cm_for_size_M",
				"unrealistic_chest_297.2cm_for_size_L",
				"unrealistic_chest_317.5cm_for_size_XL",
			},
		},
		{
			name: "Unrealistic length measurements from user example",
			sizeTable: &SizeTable{
				Sizes: []string{"S", "M", "L", "XL"},
				Measurements: map[string]map[string]float64{
					"S": {
						"chest":  96.5,  // Normal
						"length": 180.3, // Unrealistic!
					},
					"M": {
						"chest":  101.5, // Normal
						"length": 185.4, // Unrealistic!
					},
					"L": {
						"chest":  106.5, // Normal
						"length": 190.5, // Unrealistic!
					},
					"XL": {
						"chest":  111.5, // Normal
						"length": 195.6, // Unrealistic!
					},
				},
				Unit: "cm",
			},
			expectedValid: false,
			expectedReason: []string{
				"unrealistic_length_180.3cm_for_size_S",
				"unrealistic_length_185.4cm_for_size_M",
				"unrealistic_length_190.5cm_for_size_L",
				"unrealistic_length_195.6cm_for_size_XL",
			},
		},
		{
			name: "Both chest and length unrealistic (user's actual data)",
			sizeTable: &SizeTable{
				Sizes: []string{"S", "M", "L", "XL", "2XL", "3XL", "4XL"},
				Measurements: map[string]map[string]float64{
					"S": {
						"chest":    256.5,
						"length":   180.3,
						"sleeve":   157.5,
						"shoulder": 116.8,
					},
					"M": {
						"chest":    276.9,
						"length":   185.4,
						"sleeve":   160.0,
						"shoulder": 121.9,
					},
					"L": {
						"chest":    297.2,
						"length":   190.5,
						"sleeve":   162.6,
						"shoulder": 127.0,
					},
					"XL": {
						"chest":    317.5,
						"length":   195.6,
						"sleeve":   165.1,
						"shoulder": 132.1,
					},
				},
				Unit: "cm",
			},
			expectedValid:  false,
			expectedReason: []string{"unrealistic"}, // Will contain multiple unrealistic measurements
		},
		{
			name: "Normal measurements should pass",
			sizeTable: &SizeTable{
				Sizes: []string{"S", "M", "L", "XL"},
				Measurements: map[string]map[string]float64{
					"S": {
						"chest":  92.0,
						"length": 68.0,
					},
					"M": {
						"chest":  98.0,
						"length": 70.0,
					},
					"L": {
						"chest":  104.0,
						"length": 72.0,
					},
					"XL": {
						"chest":  110.0,
						"length": 74.0,
					},
				},
				Unit: "cm",
			},
			expectedValid:  true,
			expectedReason: []string{},
		},
		{
			name: "Edge case - just within realistic range",
			sizeTable: &SizeTable{
				Sizes: []string{"4XL", "5XL"},
				Measurements: map[string]map[string]float64{
					"4XL": {
						"chest":  149.0, // Just under 150cm limit
						"length": 99.0,  // Just under 100cm limit
					},
					"5XL": {
						"chest":  150.0, // At the limit
						"length": 100.0, // At the limit
					},
				},
				Unit: "cm",
			},
			expectedValid:  true,
			expectedReason: []string{},
		},
		{
			name: "Edge case - just outside realistic range",
			sizeTable: &SizeTable{
				Sizes: []string{"XL"},
				Measurements: map[string]map[string]float64{
					"XL": {
						"chest":  151.0, // Just over 150cm limit
						"length": 75.0,
					},
				},
				Unit: "cm",
			},
			expectedValid:  false,
			expectedReason: []string{"unrealistic_chest_151.0cm_for_size_XL"},
		},
		{
			name: "Sleeve length mistaken for T-shirt length",
			sizeTable: &SizeTable{
				Sizes: []string{"S", "M", "L"},
				Measurements: map[string]map[string]float64{
					"S": {
						"chest":  92.0,
						"sleeve": 58.4, // This is arm length, not T-shirt length
						// Missing "length" measurement
					},
					"M": {
						"chest":  98.0,
						"sleeve": 61.0,
					},
					"L": {
						"chest":  104.0,
						"sleeve": 63.5,
					},
				},
				Unit: "cm",
			},
			expectedValid:  false,
			expectedReason: []string{"length_measurements"}, // Missing length
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test ValidateSizeTable
			valid := ValidateSizeTable(tt.sizeTable)
			if valid != tt.expectedValid {
				t.Errorf("ValidateSizeTable() = %v, want %v", valid, tt.expectedValid)
			}

			// Test ValidateSizeTableWithDetails
			validWithDetails, reasons := ValidateSizeTableWithDetails(tt.sizeTable)
			if validWithDetails != tt.expectedValid {
				t.Errorf("ValidateSizeTableWithDetails() valid = %v, want %v", validWithDetails, tt.expectedValid)
			}

			// For invalid cases, check that we get appropriate reasons
			if !tt.expectedValid && len(reasons) == 0 {
				t.Errorf("ValidateSizeTableWithDetails() returned no reasons for invalid table")
			}

			// Check for specific expected reasons
			if len(tt.expectedReason) > 0 {
				for _, expectedReason := range tt.expectedReason {
					found := false
					for _, reason := range reasons {
						if contains(reason, expectedReason) {
							found = true
							break
						}
					}
					if !found && expectedReason != "unrealistic" {
						t.Errorf("Expected reason containing '%s' not found in %v", expectedReason, reasons)
					}
				}
			}
		})
	}
}

func TestIsRealisticMeasurement(t *testing.T) {
	tests := []struct {
		measurementType string
		value          float64
		expectedValid  bool
	}{
		// Chest measurements
		{"chest", 90.0, true},   // Normal
		{"chest", 150.0, true},  // At upper limit
		{"chest", 151.0, false}, // Over limit
		{"chest", 256.5, false}, // User's unrealistic example
		{"chest", 69.0, false},  // Below minimum

		// Length measurements
		{"length", 70.0, true},   // Normal
		{"length", 100.0, true},  // At upper limit
		{"length", 101.0, false}, // Over limit
		{"length", 180.3, false}, // User's unrealistic example
		{"length", 49.0, false},  // Below minimum

		// Shoulder measurements
		{"shoulder", 45.0, true},   // Normal
		{"shoulder", 60.0, true},   // At upper limit
		{"shoulder", 116.8, false}, // User's unrealistic example
		{"shoulder", 34.0, false},  // Below minimum

		// Sleeve measurements
		{"sleeve", 60.0, true},   // Normal
		{"sleeve", 90.0, true},   // At upper limit
		{"sleeve", 157.5, false}, // User's unrealistic example
		{"sleeve", 14.0, false},  // Below minimum

		// Special case for German terms
		{"brustumfang", 100.0, true},  // German for chest
		{"brustumfang", 256.5, false}, // Unrealistic
		{"länge", 75.0, true},         // German for length
		{"länge", 180.3, false},       // Unrealistic
	}

	for _, tt := range tests {
		t.Run(tt.measurementType+"_"+floatToString(tt.value), func(t *testing.T) {
			result := isRealisticMeasurement(tt.measurementType, tt.value)
			if result != tt.expectedValid {
				t.Errorf("isRealisticMeasurement(%s, %.1f) = %v, want %v",
					tt.measurementType, tt.value, result, tt.expectedValid)
			}
		})
	}
}

// Helper functions for tests
func contains(str, substr string) bool {
	return len(str) >= len(substr) && str[:len(substr)] == substr || 
		   len(str) >= len(substr) && containsSubstring(str, substr)
}

func containsSubstring(str, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	if len(str) < len(substr) {
		return false
	}
	for i := 0; i <= len(str)-len(substr); i++ {
		if str[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func floatToString(f float64) string {
	return fmt.Sprintf("%.1f", f)
}