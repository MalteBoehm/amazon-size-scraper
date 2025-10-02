package scraper

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// saveHTML saves HTML content to a file for debugging purposes
// The filename pattern is: ASIN_YYYY-MM-DDThh-mm-ssZ_suffix.html
func saveHTML(ctx context.Context, logger *slog.Logger, dir, asin, suffix, html string) error {
	// Create timestamp without special characters
	timestamp := time.Now().UTC().Format("2006-01-02T15-04-05Z")
	
	// Build filename
	filename := fmt.Sprintf("%s_%s_%s.html", asin, timestamp, suffix)
	
	// Ensure directory exists
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create HTML log directory %s: %w", dir, err)
	}
	
	// Build full path
	fullPath := filepath.Join(dir, filename)
	
	// Write file atomically
	if err := writeFileAtomic(fullPath, []byte(html)); err != nil {
		return fmt.Errorf("failed to write HTML file: %w", err)
	}
	
	// Log the saved file path
	logger.Info("HTML content saved for debugging",
		"asin", asin,
		"suffix", suffix,
		"path", fullPath,
		"size_bytes", len(html))
	
	return nil
}

// writeFileAtomic writes data to a file atomically by writing to a temp file first
func writeFileAtomic(path string, data []byte) error {
	// Write to temp file first
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return err
	}
	
	// Rename atomically
	return os.Rename(tmpPath, path)
}

// sanitizeFilename removes or replaces characters that might be problematic in filenames
func sanitizeFilename(s string) string {
	// Replace problematic characters with underscore
	replacer := strings.NewReplacer(
		"/", "_",
		"\\", "_",
		":", "_",
		"*", "_",
		"?", "_",
		"\"", "_",
		"<", "_",
		">", "_",
		"|", "_",
		" ", "_",
	)
	return replacer.Replace(s)
}