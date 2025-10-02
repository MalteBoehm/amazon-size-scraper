package parser

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
)

func TestExtractAvailableSizesFromDropdown(t *testing.T) {
	html := `
    <div id='variation_size_name'>
      <select id='native_dropdown_selected_size_name'>
        <option>Auswählen</option>
        <option> Größe: S </option>
        <option> Size: M </option>
        <option> L </option>
        <option> S </option> <!-- duplicate to test dedup -->
      </select>
    </div>`

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		t.Fatalf("failed to parse html: %v", err)
	}

	p := &AmazonParser{}
	sizes := p.extractAvailableSizes(doc)

	// Expect deduped, trimmed values, ignoring 'Auswählen' and prefixes
	want := []string{"S", "M", "L"}
	if len(sizes) != len(want) {
		t.Fatalf("expected %d sizes, got %d: %v", len(want), len(sizes), sizes)
	}
	for i, v := range want {
		if sizes[i] != v {
			t.Fatalf("at %d expected %q, got %q (all: %v)", i, v, sizes[i], sizes)
		}
	}
}

func TestBrandCleanupVariants(t *testing.T) {
	cases := []struct{ in, out string }{
		{"Marke: ACME", "ACME"},
		{"Brand: ACME", "ACME"},
		{"Besuche den FULL TIME SPORTS-Store", "FULL TIME SPORTS"},
		{"Besuchen Sie den Foo-Store", "Foo"},
	}

	for _, c := range cases {
		html := `<a id='bylineInfo'>` + c.in + `</a>`
		doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
		if err != nil {
			t.Fatalf("failed to parse html: %v", err)
		}
		p := &AmazonParser{}
		got := p.extractBrand(doc)
		if got != c.out {
			t.Fatalf("brand cleanup failed: in=%q want=%q got=%q", c.in, c.out, got)
		}
	}
}

// Integration-lite: ensure ExtractProduct runs on the local dump without crashing.
// This test is lenient to avoid flakiness; it only checks that no error occurs and
// that some basic fields are parsed if present.
func TestExtractProductOnLocalDump_NoPanic(t *testing.T) {
	// Adjust to a known existing dump path in the repo; skip if missing.
	rel := filepath.Join("..", "..", "debug", "html", "B0D3M4HXHM_2025-08-10T17-56-06Z_product_page.html")
	// The path above expects the test to run from amzon-size-scraper/internal/parser
	// If file does not exist, skip.
	if _, err := os.Stat(rel); err != nil {
		t.Skipf("local dump not found: %s", rel)
	}

	b, err := os.ReadFile(rel)
	if err != nil {
		t.Fatalf("failed to read dump: %v", err)
	}

	p := &AmazonParser{}
	// For integration-lite: we don’t assert fields, only ensure parsing does not panic
	_, err = p.ParseProductPage(string(b), "B0D3M4HXHM")
	if err != nil {
		t.Fatalf("ParseProductPage returned error: %v", err)
	}
}
