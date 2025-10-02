package constants

import "strings"

// Gender constants for product categorization
const (
	GenderDamen   = "Damen"
	GenderHerren  = "Herren"
	GenderUnisex  = "Unisex"
	GenderJungen  = "Jungen"
	GenderMädchen = "Mädchen"
)

// ValidGenders contains all valid gender values
var ValidGenders = []string{
	GenderDamen,
	GenderHerren,
	GenderUnisex,
	GenderJungen,
	GenderMädchen,
}

// IsValidGender checks if a gender value is valid
func IsValidGender(gender string) bool {
	for _, valid := range ValidGenders {
		if gender == valid {
			return true
		}
	}
	return false
}

// DetectGender analyzes text to determine gender
func DetectGender(text string) string {
	lowerText := strings.ToLower(text)

	// Check for women's indicators first (more specific)
	if strings.Contains(lowerText, "damen") || strings.Contains(lowerText, "frauen") ||
		strings.Contains(lowerText, "women") || strings.Contains(lowerText, "women's") ||
		strings.Contains(lowerText, "mädchen") || strings.Contains(lowerText, "girls") {
		if strings.Contains(lowerText, "mädchen") || strings.Contains(lowerText, "girls") {
			return GenderMädchen
		}
		return GenderDamen
	}

	// Check for men's indicators
	if strings.Contains(lowerText, "herren") || strings.Contains(lowerText, "männer") ||
		strings.Contains(lowerText, "men") || strings.Contains(lowerText, "men's") ||
		strings.Contains(lowerText, "jungen") || strings.Contains(lowerText, "boys") {
		if strings.Contains(lowerText, "jungen") || strings.Contains(lowerText, "boys") {
			return GenderJungen
		}
		return GenderHerren
	}

	// Check for unisex
	if strings.Contains(lowerText, "unisex") || strings.Contains(lowerText, "erwachsene") {
		return GenderUnisex
	}

	return ""
}

// MapGenderToStandardized converts German gender values to standardized English values
func MapGenderToStandardized(germanGender string) string {
	switch germanGender {
	case GenderDamen, GenderMädchen:
		return "female"
	case GenderHerren, GenderJungen:
		return "male"
	case GenderUnisex:
		return "unisex"
	default:
		return "unisex" // Default fallback
	}
}

// DetectGenderStandardized analyzes text and returns standardized gender values
func DetectGenderStandardized(text string) string {
	germanGender := DetectGender(text)
	return MapGenderToStandardized(germanGender)
}
