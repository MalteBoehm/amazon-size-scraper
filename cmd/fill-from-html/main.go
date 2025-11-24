package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/maltedev/amazon-size-scraper/internal/database"
	"github.com/maltedev/amazon-size-scraper/internal/parser"
)

func main() {
	var (
		dir    = flag.String("dir", "amzon-size-scraper/debug/html", "Directory with HTML dumps")
		match  = flag.String("match", "*_product_page.html", "Glob to match product page dumps")
		dbHost = flag.String("db_host", os.Getenv("DB_HOST"), "DB host")
		dbPort = flag.Int("db_port", envInt("DB_PORT", 5432), "DB port")
		dbUser = flag.String("db_user", os.Getenv("DB_USER"), "DB user")
		dbPass = flag.String("db_password", os.Getenv("DB_PASSWORD"), "DB password")
		dbName = flag.String("db_name", os.Getenv("DB_NAME"), "DB name")
		dbSSL  = flag.String("db_sslmode", getenvDefault("DB_SSL_MODE", "disable"), "DB sslmode")
	)
	flag.Parse()

	ctx := context.Background()
	p := parser.NewAmazonParser()

	// Connect DB (database/sql based adapter)
	db, err := database.New(ctx, database.Config{
		Host:     *dbHost,
		Port:     *dbPort,
		User:     *dbUser,
		Password: *dbPass,
		Database: *dbName,
		SSLMode:  *dbSSL,
	})
	if err != nil {
		log.Fatalf("db connect failed: %v", err)
	}
	defer db.Close()

	glob := filepath.Join(*dir, *match)
	files, err := filepath.Glob(glob)
	if err != nil {
		log.Fatalf("glob failed: %v", err)
	}
	if len(files) == 0 {
		log.Printf("no files matched: %s", glob)
		return
	}

	asinRe := regexp.MustCompile(`([A-Z0-9]{10})_`) // prefix in filename

	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			log.Printf("read %s failed: %v", f, err)
			continue
		}

		asin := extractFromFilename(asinRe, filepath.Base(f))
		if asin == "" {
			asin = guessASINFromHTML(string(b))
		}
		if asin == "" {
			log.Printf("skip %s: cannot detect ASIN", f)
			continue
		}

		prod, err := p.ParseProductPage(string(b), asin)
		if err != nil {
			log.Printf("parse failed for %s: %v", f, err)
			continue
		}

		// Prepare payloads
		imgsJSON, _ := json.Marshal(prod.Images)
		featuresJSON, _ := json.Marshal(prod.Features)
		sizesJSON, _ := json.Marshal(prod.AvailableSizes)
		varAttrsJSON, _ := json.Marshal(prod.VariationAttributes)
		matCompJSON, _ := json.Marshal(prod.MaterialComposition)
		careJSON, _ := json.Marshal(prod.CareInstructions)

		// Upsert product from HTML only (no PA-API)
		// Fallback detail URL to canonical dp-link
		detailURL := fmt.Sprintf("https://www.amazon.de/dp/%s", asin)
		if strings.TrimSpace(prod.URL) != "" {
			detailURL = strings.TrimSpace(prod.URL)
		}

		// Ensure JSON arrays are [] instead of null
		if string(imgsJSON) == "null" {
			imgsJSON = []byte("[]")
		}
		if string(featuresJSON) == "null" {
			featuresJSON = []byte("[]")
		}
		if string(sizesJSON) == "null" {
			sizesJSON = []byte("[]")
		}
		if string(varAttrsJSON) == "null" {
			varAttrsJSON = []byte("[]")
		}
		if string(matCompJSON) == "null" {
			matCompJSON = []byte("{}")
		}
		if string(careJSON) == "null" {
			careJSON = []byte("[]")
		}

		q := `INSERT INTO products (
			asin, title, brand, detail_page_url, category,
			image_urls, features, available_sizes, variation_attributes,
			size, color, material_composition, material_full_text, care_instructions
		) VALUES (
			$1, $2, $3, $4, $5,
			$6::jsonb, $7::jsonb, $8::jsonb, $9::jsonb,
			$10, $11, $12::jsonb, $13, $14::jsonb
		)
		ON CONFLICT (asin) DO UPDATE SET
			title = COALESCE(NULLIF(EXCLUDED.title,''), products.title),
			brand = COALESCE(NULLIF(EXCLUDED.brand,''), products.brand),
			detail_page_url = COALESCE(NULLIF(EXCLUDED.detail_page_url,''), products.detail_page_url),
			category = COALESCE(NULLIF(EXCLUDED.category,''), products.category),
			image_urls = COALESCE(EXCLUDED.image_urls, products.image_urls),
			features = COALESCE(EXCLUDED.features, products.features),
			available_sizes = COALESCE(EXCLUDED.available_sizes, products.available_sizes),
			variation_attributes = COALESCE(EXCLUDED.variation_attributes, products.variation_attributes),
			size = COALESCE(NULLIF(EXCLUDED.size,''), products.size),
			color = COALESCE(NULLIF(EXCLUDED.color,''), products.color),
			material_composition = COALESCE(EXCLUDED.material_composition, products.material_composition),
			material_full_text = COALESCE(EXCLUDED.material_full_text, products.material_full_text),
			care_instructions = COALESCE(EXCLUDED.care_instructions, products.care_instructions),
			updated_at = NOW()`

		_, err = db.Exec(ctx, q,
			asin,
			strings.TrimSpace(prod.Title),
			strings.TrimSpace(prod.Brand),
			detailURL,
			strings.TrimSpace(prod.Category),
			json.RawMessage(imgsJSON),
			json.RawMessage(featuresJSON),
			json.RawMessage(sizesJSON),
			json.RawMessage(varAttrsJSON),
			strings.TrimSpace(prod.Size),
			strings.TrimSpace(prod.Color),
			json.RawMessage(matCompJSON),
			prod.MaterialFullText,
			json.RawMessage(careJSON),
		)
		if err != nil {
			log.Printf("update failed asin=%s file=%s: %v", asin, f, err)
			continue
		}
		fmt.Printf("updated asin=%s from %s\n", asin, filepath.Base(f))
	}
}

func extractFromFilename(re *regexp.Regexp, name string) string {
	m := re.FindStringSubmatch(name)
	if len(m) >= 2 {
		return m[1]
	}
	return ""
}

func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		var n int
		fmt.Sscanf(v, "%d", &n)
		if n > 0 {
			return n
		}
	}
	return def
}

func getenvDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func guessASINFromHTML(html string) string {
	re := regexp.MustCompile(`(?i)amazon\.de/.*/dp/([A-Z0-9]{10})`)
	m := re.FindStringSubmatch(html)
	if len(m) >= 2 {
		return m[1]
	}
	return ""
}
