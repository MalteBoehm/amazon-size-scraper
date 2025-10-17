package browser

import (
	"testing"
	"time"
)

func TestDefaultOptions(t *testing.T) {
	opts := DefaultOptions()
	
	if !opts.Headless {
		t.Error("Expected headless to be true by default")
	}
	
	if opts.Timeout != 30*time.Second {
		t.Errorf("Expected timeout to be 30s, got %v", opts.Timeout)
	}
	
	if opts.ViewportWidth != 1920 || opts.ViewportHeight != 1080 {
		t.Errorf("Expected viewport to be 1920x1080, got %dx%d", opts.ViewportWidth, opts.ViewportHeight)
	}
	
	if opts.Locale != "de-DE" {
		t.Errorf("Expected locale to be de-DE, got %s", opts.Locale)
	}
}