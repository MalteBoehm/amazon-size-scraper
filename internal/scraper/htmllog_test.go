package scraper

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSaveHTML(t *testing.T) {
	// Create temp directory for testing
	tempDir := t.TempDir()
	
	// Create a test logger
	logger := slog.Default()
	
	testCases := []struct {
		name     string
		asin     string
		suffix   string
		html     string
		dir      string
		wantErr  bool
	}{
		{
			name:    "valid save",
			asin:    "B08N5WRWNW",
			suffix:  "page",
			html:    "<html><body>Test HTML content</body></html>",
			dir:     tempDir,
			wantErr: false,
		},
		{
			name:    "with size table suffix",
			asin:    "B08N5LGQNG",
			suffix:  "size_table",
			html:    "<div>Size table HTML</div>",
			dir:     tempDir,
			wantErr: false,
		},
		{
			name:    "empty HTML",
			asin:    "TESTASINN1",
			suffix:  "page",
			html:    "",
			dir:     tempDir,
			wantErr: false,
		},
		{
			name:    "special characters in ASIN",
			asin:    "TEST-ASIN",
			suffix:  "special",
			html:    "<html>Test</html>",
			dir:     tempDir,
			wantErr: false,
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			err := saveHTML(ctx, logger, tc.dir, tc.asin, tc.suffix, tc.html)
			
			if (err != nil) != tc.wantErr {
				t.Errorf("saveHTML() error = %v, wantErr %v", err, tc.wantErr)
				return
			}
			
			if !tc.wantErr {
				// Check if file was created
				files, err := os.ReadDir(tc.dir)
				if err != nil {
					t.Fatalf("Failed to read directory: %v", err)
				}
				
				// Find file with matching ASIN and suffix
				found := false
				for _, file := range files {
					if strings.HasPrefix(file.Name(), tc.asin+"_") && 
					   strings.Contains(file.Name(), "_"+tc.suffix+".html") {
						found = true
						
						// Read file content
						content, err := os.ReadFile(filepath.Join(tc.dir, file.Name()))
						if err != nil {
							t.Fatalf("Failed to read saved file: %v", err)
						}
						
						if string(content) != tc.html {
							t.Errorf("File content mismatch. Got %s, want %s", string(content), tc.html)
						}
						break
					}
				}
				
				if !found {
					t.Errorf("File with ASIN %s and suffix %s not found", tc.asin, tc.suffix)
				}
			}
		})
	}
}

func TestSaveHTMLDirectoryCreation(t *testing.T) {
	// Create temp base directory
	tempBase := t.TempDir()
	
	// Test directory that doesn't exist yet
	testDir := filepath.Join(tempBase, "nested", "log", "directory")
	
	ctx := context.Background()
	logger := slog.Default()
	
	// Save HTML to non-existent directory - should create it
	err := saveHTML(ctx, logger, testDir, "TESTASIN", "test", "<html>test</html>")
	if err != nil {
		t.Fatalf("saveHTML() should create directory, but got error: %v", err)
	}
	
	// Check if directory was created
	if _, err := os.Stat(testDir); os.IsNotExist(err) {
		t.Errorf("Directory %s was not created", testDir)
	}
	
	// Check if file exists in the created directory
	files, err := os.ReadDir(testDir)
	if err != nil {
		t.Fatalf("Failed to read directory: %v", err)
	}
	
	if len(files) != 1 {
		t.Errorf("Expected 1 file in directory, got %d", len(files))
	}
}

func TestSanitizeFilename(t *testing.T) {
	testCases := []struct {
		input    string
		expected string
	}{
		{"normal_text", "normal_text"},
		{"text with spaces", "text_with_spaces"},
		{"text/with/slashes", "text_with_slashes"},
		{"text\\with\\backslashes", "text_with_backslashes"},
		{"text:with:colons", "text_with_colons"},
		{"text*with*asterisks", "text_with_asterisks"},
		{"text?with?questions", "text_with_questions"},
		{"text\"with\"quotes", "text_with_quotes"},
		{"text<with>brackets", "text_with_brackets"},
		{"text|with|pipes", "text_with_pipes"},
	}
	
	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			result := sanitizeFilename(tc.input)
			if result != tc.expected {
				t.Errorf("sanitizeFilename(%s) = %s, want %s", tc.input, result, tc.expected)
			}
		})
	}
}

func TestWriteFileAtomic(t *testing.T) {
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.txt")
	testContent := []byte("test content")
	
	// Write file atomically
	err := writeFileAtomic(testFile, testContent)
	if err != nil {
		t.Fatalf("writeFileAtomic() error = %v", err)
	}
	
	// Check if file exists
	content, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}
	
	if string(content) != string(testContent) {
		t.Errorf("File content mismatch. Got %s, want %s", string(content), string(testContent))
	}
	
	// Check that temp file doesn't exist
	tempFile := testFile + ".tmp"
	if _, err := os.Stat(tempFile); !os.IsNotExist(err) {
		t.Errorf("Temp file %s should not exist after atomic write", tempFile)
	}
}