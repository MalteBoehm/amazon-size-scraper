package events

import (
	"fmt"
	"strings"
	"unicode"
)

// String utility functions for event processing

// contains checks if a string contains a substring (case-insensitive)
func contains(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

// replaceAll replaces all occurrences of old with new in string s
func replaceAll(s, old, new string) string {
	return strings.ReplaceAll(s, old, new)
}

// trimSpace trims whitespace from a string
func trimSpace(s string) string {
	return strings.TrimSpace(s)
}

// toUpper converts string to uppercase
func toUpper(s string) string {
	return strings.ToUpper(s)
}

// split splits a string by separator
func split(s, sep string) []string {
	return strings.Split(s, sep)
}

// findSubstring checks if substring exists in string (case-insensitive)
func findSubstring(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

// extractFirstWord extracts the first word from a string
func extractFirstWord(s string) string {
	words := split(trimSpace(s), " ")
	if len(words) > 0 {
		return words[0]
	}
	return ""
}

// extractNumeric extracts numeric value from string
func extractNumeric(s string) float64 {
	var result float64
	found := false

	for _, r := range s {
		if unicode.IsDigit(r) || r == '.' || r == ',' {
			if !found {
				found = true
			}
		} else if found {
			break
		}
	}

	if found {
		// Extract the numeric part
		var numStr string
		for _, r := range s {
			if unicode.IsDigit(r) || r == '.' || r == ',' {
				numStr += string(r)
			} else if len(numStr) > 0 {
				break
			}
		}
		// Convert comma to dot for decimal
		numStr = replaceAll(numStr, ",", ".")
		fmt.Sscanf(numStr, "%f", &result)
	}

	return result
}

// isValidASIN checks if ASIN format is valid
func isValidASIN(asin string) bool {
	if len(asin) != 10 {
		return false
	}

	for _, r := range asin {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			return false
		}
	}

	return true
}

// sanitizeURL sanitizes and validates URL
func sanitizeURL(url string) string {
	// Basic URL validation and sanitization
	url = trimSpace(url)
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		return ""
	}
	return url
}