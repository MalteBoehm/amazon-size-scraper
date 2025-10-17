package parser

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/maltedev/amazon-size-scraper/internal/models"
)

type AmazonParser struct {
	dimensionPatterns []*regexp.Regexp
	weightPatterns    []*regexp.Regexp
	materialPatterns  []*regexp.Regexp
}

func NewAmazonParser() *AmazonParser {
	return &AmazonParser{
		dimensionPatterns: []*regexp.Regexp{
			regexp.MustCompile(`(\d+(?:[,.]\d+)?)\s*x\s*(\d+(?:[,.]\d+)?)\s*x\s*(\d+(?:[,.]\d+)?)\s*(cm|mm|m|zoll|inch|")`),
			regexp.MustCompile(`(?i)abmessungen.*?:\s*(\d+(?:[,.]\d+)?)\s*x\s*(\d+(?:[,.]\d+)?)\s*x\s*(\d+(?:[,.]\d+)?)\s*(cm|mm|m)`),
			regexp.MustCompile(`(?i)produktabmessungen.*?:\s*(\d+(?:[,.]\d+)?)\s*x\s*(\d+(?:[,.]\d+)?)\s*x\s*(\d+(?:[,.]\d+)?)\s*(cm|mm|m)`),
		},
		weightPatterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)gewicht.*?:\s*(\d+(?:[,.]\d+)?)\s*(kg|g|mg|pound|lb|oz)`),
			regexp.MustCompile(`(?i)artikelgewicht.*?:\s*(\d+(?:[,.]\d+)?)\s*(kg|g|mg)`),
			regexp.MustCompile(`(\d+(?:[,.]\d+)?)\s*(kilogramm|gramm|kg|g)`),
		},
		materialPatterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)materialzusammensetzung.*?([\d%]+\s*[^,]+(?:,\s*[\d%]+\s*[^,]+)*)`),
			regexp.MustCompile(`(?i)material.*?([\d%]+\s*[^,]+(?:,\s*[\d%]+\s*[^,]+)*)`),
			regexp.MustCompile(`(?i)stoff.*?([\d%]+\s*[^,]+(?:,\s*[\d%]+\s*[^,]+)*)`),
			regexp.MustCompile(`(?i)gewebe.*?([\d%]+\s*[^,]+(?:,\s*[\d%]+\s*[^,]+)*)`),
		},
	}
}

func (p *AmazonParser) ParseProductPage(html string, asin string) (*models.Product, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}

	product := models.NewProduct(asin)

	product.Title = p.extractTitle(doc)
	product.Brand = p.extractBrand(doc)
	product.Category = p.extractCategory(doc)

	if material, err := p.ExtractMaterial(html); err == nil {
		product.Material = material
	}

	if dimensions, err := p.ExtractDimensions(html); err == nil {
		product.Dimensions = *dimensions
	}

	if weight, err := p.ExtractWeight(html); err == nil {
		product.Weight = *weight
	}

	if price, err := p.ExtractPrice(html); err == nil {
		product.Price = *price
	}

	product.Images = p.extractImages(doc)

	return product, nil
}

func (p *AmazonParser) ExtractDimensions(html string) (*models.Dimension, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil, err
	}
	
	productDetails := p.extractProductDetails(doc)
	
	for _, pattern := range p.dimensionPatterns {
		matches := pattern.FindStringSubmatch(productDetails)
		if len(matches) >= 5 {
			dim := &models.Dimension{
				Unit: p.normalizeUnit(matches[4]),
			}
			
			dim.Length = p.parseFloat(matches[1])
			dim.Width = p.parseFloat(matches[2])
			dim.Height = p.parseFloat(matches[3])
			
			if dim.Length > 0 && dim.Width > 0 && dim.Height > 0 {
				return dim, nil
			}
		}
	}
	
	technicalDetails := doc.Find("#productDetails_techSpec_section_1, #productDetails_detailBullets_sections1").Text()
	for _, pattern := range p.dimensionPatterns {
		matches := pattern.FindStringSubmatch(technicalDetails)
		if len(matches) >= 5 {
			dim := &models.Dimension{
				Unit: p.normalizeUnit(matches[4]),
			}
			
			dim.Length = p.parseFloat(matches[1])
			dim.Width = p.parseFloat(matches[2])
			dim.Height = p.parseFloat(matches[3])
			
			if dim.Length > 0 && dim.Width > 0 && dim.Height > 0 {
				return dim, nil
			}
		}
	}
	
	return nil, fmt.Errorf("dimensions not found")
}

func (p *AmazonParser) ExtractWeight(html string) (*models.Weight, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil, err
	}
	
	productDetails := p.extractProductDetails(doc)
	
	for _, pattern := range p.weightPatterns {
		matches := pattern.FindStringSubmatch(productDetails)
		if len(matches) >= 3 {
			weight := &models.Weight{
				Value: p.parseFloat(matches[1]),
				Unit:  p.normalizeWeightUnit(matches[2]),
			}
			
			if weight.Value > 0 {
				return weight, nil
			}
		}
	}
	
	return nil, fmt.Errorf("weight not found")
}

func (p *AmazonParser) ExtractPrice(html string) (*models.Price, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil, err
	}

	priceSelectors := []string{
		".a-price-whole",
		"span.a-price.a-text-price.a-size-medium.apexPriceToPay",
		".a-price-range",
		"#priceblock_dealprice",
		"#priceblock_ourprice",
		".a-price.a-text-price.header-price",
	}

	for _, selector := range priceSelectors {
		priceText := strings.TrimSpace(doc.Find(selector).First().Text())
		if priceText != "" {
			price := p.parsePrice(priceText)
			if price != nil && price.Amount > 0 {
				return price, nil
			}
		}
	}

	return nil, fmt.Errorf("price not found")
}

func (p *AmazonParser) ExtractMaterial(html string) (string, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return "", err
	}

	// First try structured extraction from the specific HTML pattern you provided
	var foundMaterial string
	doc.Find(".a-fixed-left-grid-inner").Each(func(i int, s *goquery.Selection) {
		leftCol := s.Find(".a-col-left .a-color-base")
		rightCol := s.Find(".a-col-right .a-color-base")

		leftText := strings.TrimSpace(leftCol.Text())
		rightText := strings.TrimSpace(rightCol.Text())

		// Check if this is the material composition row
		if strings.Contains(strings.ToLower(leftText), "materialzusammensetzung") ||
		   strings.Contains(strings.ToLower(leftText), "material") {
			if rightText != "" && foundMaterial == "" {
				foundMaterial = rightText
			}
		}
	})

	if foundMaterial != "" {
		return foundMaterial, nil
	}

	// Try regex patterns on the full HTML text
	fullText := strings.TrimSpace(doc.Text())

	for _, pattern := range p.materialPatterns {
		matches := pattern.FindStringSubmatch(fullText)
		if len(matches) > 1 {
			material := strings.TrimSpace(matches[1])
			if material != "" && !strings.Contains(strings.ToLower(material), "nicht angegeben") {
				return material, nil
			}
		}
	}

	// Look specifically for the pattern you provided in product details
	productDetails := p.extractProductDetails(doc)
	for _, pattern := range p.materialPatterns {
		matches := pattern.FindStringSubmatch(productDetails)
		if len(matches) > 1 {
			material := strings.TrimSpace(matches[1])
			if material != "" && !strings.Contains(strings.ToLower(material), "nicht angegeben") {
				return material, nil
			}
		}
	}

	// Search in technical details section
	technicalDetails := doc.Find("#productDetails_techSpec_section_1, #productDetails_detailBullets_sections1").Text()
	for _, pattern := range p.materialPatterns {
		matches := pattern.FindStringSubmatch(technicalDetails)
		if len(matches) > 1 {
			material := strings.TrimSpace(matches[1])
			if material != "" && !strings.Contains(strings.ToLower(material), "nicht angegeben") {
				return material, nil
			}
		}
	}

	return "", fmt.Errorf("material not found")
}

// ExtractMaterialComposition extracts structured material data and fallback text
func (p *AmazonParser) ExtractMaterialComposition(html string) (*models.MaterialComposition, string, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil, "", err
	}

	var fullTextParts []string
	var materialSources []struct {
		text   string
		source string
	}

	// 1. Try structured extraction first
	var foundMaterial string
	doc.Find(".a-fixed-left-grid-inner").Each(func(i int, s *goquery.Selection) {
		leftCol := s.Find(".a-col-left .a-color-base")
		rightCol := s.Find(".a-col-right .a-color-base")

		leftText := strings.TrimSpace(leftCol.Text())
		rightText := strings.TrimSpace(rightCol.Text())

		// Check if this is the material composition row
		if strings.Contains(strings.ToLower(leftText), "materialzusammensetzung") ||
		   strings.Contains(strings.ToLower(leftText), "material") {
			if rightText != "" {
				if foundMaterial == "" {
					foundMaterial = rightText
				}
				materialSources = append(materialSources, struct {
					text   string
					source string
				}{leftText + ": " + rightText, "structured"})
				fullTextParts = append(fullTextParts, leftText+": "+rightText)
			}
		}
	})

	// If we found structured material data, parse it
	if foundMaterial != "" {
		if composition := p.parseMaterialComposition(foundMaterial); composition != nil {
			composition.Source = "structured"
			return composition, strings.Join(fullTextParts, "; "), nil
		}
	}

	// 2. Try regex patterns on full text
	fullText := strings.TrimSpace(doc.Text())

	for _, pattern := range p.materialPatterns {
		matches := pattern.FindStringSubmatch(fullText)
		if len(matches) > 1 {
			material := strings.TrimSpace(matches[1])
			if material != "" && !strings.Contains(strings.ToLower(material), "nicht angegeben") {
				if composition := p.parseMaterialComposition(material); composition != nil {
					composition.Source = "regex"
					materialSources = append(materialSources, struct {
						text   string
						source string
					}{material, "regex"})
					fullTextParts = append(fullTextParts, material)
					return composition, strings.Join(fullTextParts, "; "), nil
				}
			}
		}
	}

	// 3. Look in product details
	productDetails := p.extractProductDetails(doc)
	for _, pattern := range p.materialPatterns {
		matches := pattern.FindStringSubmatch(productDetails)
		if len(matches) > 1 {
			material := strings.TrimSpace(matches[1])
			if material != "" && !strings.Contains(strings.ToLower(material), "nicht angegeben") {
				if composition := p.parseMaterialComposition(material); composition != nil {
					composition.Source = "product_details"
					materialSources = append(materialSources, struct {
						text   string
						source string
					}{material, "product_details"})
					fullTextParts = append(fullTextParts, material)
					return composition, strings.Join(fullTextParts, "; "), nil
				}
			}
		}
	}

	// 4. Search in technical details as fallback
	technicalDetails := doc.Find("#productDetails_techSpec_section_1, #productDetails_detailBullets_sections1").Text()
	for _, pattern := range p.materialPatterns {
		matches := pattern.FindStringSubmatch(technicalDetails)
		if len(matches) > 1 {
			material := strings.TrimSpace(matches[1])
			if material != "" && !strings.Contains(strings.ToLower(material), "nicht angegeben") {
				if composition := p.parseMaterialComposition(material); composition != nil {
					composition.Source = "technical_details"
					materialSources = append(materialSources, struct {
						text   string
						source string
					}{material, "technical_details"})
					fullTextParts = append(fullTextParts, material)
					return composition, strings.Join(fullTextParts, "; "), nil
				}
			}
		}
	}

	// 5. Collect all material-related text as fallback
	p.collectAllMaterialText(doc, &fullTextParts)

	if len(fullTextParts) > 0 {
		// Return fallback with empty composition but full text
		return nil, strings.Join(fullTextParts, "; "), nil
	}

	return nil, "", fmt.Errorf("material not found")
}

// parseMaterialComposition parses material text into structured data
func (p *AmazonParser) parseMaterialComposition(text string) *models.MaterialComposition {
	// Pattern for "80% Baumwolle, 20% Elasthan"
	percentPattern := regexp.MustCompile(`(\d+(?:[,.]\d+)?)\s*%?\s*([^,]+)(?:,\s*|$)`)

	// Pattern for "80% Baumwolle, 5% Elasthan, 15% Polyester" (multiple materials)
	matches := percentPattern.FindAllStringSubmatch(text, -1)

	if len(matches) == 0 {
		// Try alternative patterns
		if strings.Contains(strings.ToLower(text), "100%") {
			material := strings.TrimSpace(strings.Replace(text, "100%", "", -1))
			if material != "" {
				return &models.MaterialComposition{
					Materials: []models.MaterialItem{
						{Name: material, Percent: 100.0},
					},
					Confidence: 0.9,
				}
			}
		}
		return nil
	}

	var materials []models.MaterialItem
	totalPercent := 0.0

	for _, match := range matches {
		if len(match) >= 3 {
			percentStr := strings.Replace(match[1], ",", ".", -1)
			if percent, err := strconv.ParseFloat(percentStr, 64); err == nil {
				material := strings.TrimSpace(match[2])
				if material != "" {
					materials = append(materials, models.MaterialItem{
						Name:    material,
						Percent: percent,
					})
					totalPercent += percent
				}
			}
		}
	}

	if len(materials) == 0 {
		return nil
	}

	// Calculate confidence based on total percentage and completeness
	confidence := 0.7 // Base confidence
	if totalPercent >= 95 && totalPercent <= 105 {
		confidence = 0.95 // High confidence for complete compositions
	} else if totalPercent >= 80 && totalPercent <= 120 {
		confidence = 0.85 // Good confidence
	}

	return &models.MaterialComposition{
		Materials:  materials,
		Confidence: confidence,
	}
}

// collectAllMaterialText collects all material-related text for learning purposes
func (p *AmazonParser) collectAllMaterialText(doc *goquery.Document, textParts *[]string) {
	keywords := []string{
		"material", "stoff", "gewebe", "textil", "faser", "zusammensetzung",
		"baumwolle", "cotton", "polyester", "elasthan", "spandex", "viskose",
		"leinen", "linen", "seide", "silk", "wolle", "wool", "nylon",
	}

	fullText := strings.TrimSpace(doc.Text())

	// Find sentences containing material keywords
	sentences := regexp.MustCompile(`[^.!?]+[.!?]?`).FindAllString(fullText, -1)

	for _, sentence := range sentences {
		sentenceLower := strings.ToLower(sentence)
		for _, keyword := range keywords {
			if strings.Contains(sentenceLower, keyword) {
				*textParts = append(*textParts, strings.TrimSpace(sentence))
				break // Only add once per sentence
			}
		}
	}
}

func (p *AmazonParser) extractTitle(doc *goquery.Document) string {
	return strings.TrimSpace(doc.Find("#productTitle").Text())
}

func (p *AmazonParser) extractBrand(doc *goquery.Document) string {
	brand := doc.Find("#bylineInfo").Text()
	brand = strings.TrimPrefix(brand, "Marke: ")
	brand = strings.TrimPrefix(brand, "Brand: ")
	brand = strings.TrimPrefix(brand, "Besuchen Sie den ")
	brand = strings.TrimSuffix(brand, "-Store")
	return strings.TrimSpace(brand)
}

func (p *AmazonParser) extractCategory(doc *goquery.Document) string {
	breadcrumb := doc.Find("#wayfinding-breadcrumbs_feature_div .a-list-item").Last().Text()
	return strings.TrimSpace(breadcrumb)
}

func (p *AmazonParser) extractImages(doc *goquery.Document) []string {
	var images []string
	
	doc.Find("#altImages ul li img").Each(func(i int, s *goquery.Selection) {
		if src, exists := s.Attr("src"); exists {
			fullSrc := strings.Replace(src, "_AC_US40_", "_AC_SL1500_", 1)
			images = append(images, fullSrc)
		}
	})
	
	if mainImage, exists := doc.Find("#landingImage").Attr("src"); exists && len(images) == 0 {
		images = append(images, mainImage)
	}
	
	return images
}

func (p *AmazonParser) extractProductDetails(doc *goquery.Document) string {
	selectors := []string{
		"#feature-bullets",
		"#productDetails_detailBullets_sections1",
		"#detailBullets_feature_div",
		".detail-bullet-list",
	}
	
	var details strings.Builder
	for _, selector := range selectors {
		doc.Find(selector).Each(func(i int, s *goquery.Selection) {
			details.WriteString(s.Text())
			details.WriteString(" ")
		})
	}
	
	return details.String()
}

func (p *AmazonParser) parseFloat(s string) float64 {
	s = strings.Replace(s, ",", ".", -1)
	s = strings.TrimSpace(s)
	val, _ := strconv.ParseFloat(s, 64)
	return val
}

func (p *AmazonParser) parsePrice(s string) *models.Price {
	re := regexp.MustCompile(`(\d+(?:[,.]\d+)?)`)
	matches := re.FindStringSubmatch(s)
	
	if len(matches) > 1 {
		amount := p.parseFloat(matches[1])
		if amount > 0 {
			return &models.Price{
				Amount:   amount,
				Currency: "EUR",
			}
		}
	}
	
	return nil
}

func (p *AmazonParser) normalizeUnit(unit string) string {
	unit = strings.ToLower(strings.TrimSpace(unit))
	switch unit {
	case "cm", "centimeter", "zentimeter":
		return "cm"
	case "mm", "millimeter":
		return "mm"
	case "m", "meter":
		return "m"
	case "inch", "zoll", "\"":
		return "inch"
	default:
		return unit
	}
}

func (p *AmazonParser) normalizeWeightUnit(unit string) string {
	unit = strings.ToLower(strings.TrimSpace(unit))
	switch unit {
	case "kg", "kilogramm", "kilo":
		return "kg"
	case "g", "gramm", "gram":
		return "g"
	case "mg", "milligramm":
		return "mg"
	case "lb", "pound", "pounds":
		return "lb"
	case "oz", "ounce", "ounces":
		return "oz"
	default:
		return unit
	}
}