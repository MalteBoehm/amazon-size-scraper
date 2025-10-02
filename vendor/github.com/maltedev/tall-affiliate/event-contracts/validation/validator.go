package validation

import (
	"fmt"
	"net/url"
	"regexp"
)

var (
	// ASIN pattern - Amazon Standard Identification Number
	asinPattern = regexp.MustCompile(`^[A-Z0-9]{10}$`)
	
	// Region codes
	validRegions = map[string]bool{
		"de": true,
		"uk": true,
		"us": true,
		"fr": true,
		"it": true,
		"es": true,
		"jp": true,
		"ca": true,
	}
)

// ValidateASIN validates an Amazon ASIN
func ValidateASIN(asin string) error {
	if asin == "" {
		return fmt.Errorf("ASIN cannot be empty")
	}
	if !asinPattern.MatchString(asin) {
		return fmt.Errorf("invalid ASIN format: %s", asin)
	}
	return nil
}

// ValidateRegion validates a region code
func ValidateRegion(region string) error {
	if region == "" {
		return fmt.Errorf("region cannot be empty")
	}
	if !validRegions[region] {
		return fmt.Errorf("invalid region: %s", region)
	}
	return nil
}

// ValidateURL validates a URL string
func ValidateURL(urlStr string) error {
	if urlStr == "" {
		return nil // Empty URLs are allowed for optional fields
	}
	u, err := url.Parse(urlStr)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}
	// Check that it has a scheme and host
	if u.Scheme == "" || u.Host == "" {
		return fmt.Errorf("invalid URL: must have scheme and host")
	}
	return nil
}

// ValidateEventType validates event type format (domain.eventName.version)
func ValidateEventType(eventType string) error {
	pattern := regexp.MustCompile(`^[a-z]+\.[a-z_]+\.v\d+$`)
	if !pattern.MatchString(eventType) {
		return fmt.Errorf("invalid event type format: %s (expected domain.eventName.version)", eventType)
	}
	return nil
}