package parser

import (
    "fmt"
    "regexp"
    "strconv"
    "strings"

    "github.com/PuerkitoBio/goquery"
    constants "github.com/maltedev/amazon-size-scraper/internal/scraper/constants"
    "github.com/maltedev/amazon-size-scraper/internal/models"
)

type AmazonParser struct {
	dimensionPatterns []*regexp.Regexp
	weightPatterns    []*regexp.Regexp
}

// percentMaterialRe matches percentage-material pairs anywhere in text, e.g. "75% Baumwolle"
var percentMaterialRe = regexp.MustCompile(`(?i)(\d{1,3}(?:[.,]\d{1,2})?)\s*[%％]\s*([a-zäöüß][a-zäöüß\-\/\s]+)`) // compiled once

// allowed canonical material keys
var allowedMaterials = map[string]struct{}{
    "cotton": {}, "polyester": {}, "elastane": {}, "spandex": {}, "lycra": {},
    "polyamide": {}, "nylon": {}, "viscose": {}, "rayon": {}, "acrylic": {},
    "wool": {}, "silk": {}, "linen": {}, "bamboo": {}, "cashmere": {},
    "merino": {}, "modal": {}, "lyocell": {},
    // Additional materials
    "polyurethane": {}, "acetate": {}, "copper": {}, "microfiber": {},
    "fleece": {}, "denim": {}, "corduroy": {},
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
	product.Features = p.extractFeatures(doc)
	product.ProductGroups = p.extractProductGroups(doc)

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

	// Variations: size/color and available values
	product.Size = p.extractCurrentSize(doc)
	product.Color = p.extractCurrentColor(doc)
	product.AvailableSizes = p.extractAvailableSizes(doc)
	availableColors := p.extractAvailableColors(doc)

	if len(product.AvailableSizes) > 0 {
		product.VariationAttributes = append(product.VariationAttributes, models.VariationAttribute{Type: "size", Values: product.AvailableSizes})
	}
	if len(availableColors) > 0 {
		product.VariationAttributes = append(product.VariationAttributes, models.VariationAttribute{Type: "color", Values: availableColors})
	}

	// TODO: extract model/material/care from detail sections if present
	model := p.extractModel(doc)
	if model != "" {
		product.Model = model
	}
	comp, care, materialInfo := p.extractMaterialAndCare(doc)
	if len(comp) > 0 {
		product.MaterialComposition = comp
	}
	if materialInfo != nil {
		product.MaterialInfo = materialInfo
	}

	// Extract MaterialFullText from care instructions (if any)
	var realCare []string
	for _, item := range care {
		if strings.HasPrefix(item, "MATERIAL_FULLTEXT:") {
			product.MaterialFullText = strings.TrimPrefix(item, "MATERIAL_FULLTEXT:")
		} else {
			realCare = append(realCare, item)
		}
	}

	if len(realCare) > 0 {
		product.CareInstructions = realCare
	}

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

func (p *AmazonParser) extractTitle(doc *goquery.Document) string {
	return strings.TrimSpace(doc.Find("#productTitle").Text())
}

func (p *AmazonParser) extractBrand(doc *goquery.Document) string {
	brand := doc.Find("#bylineInfo").Text()
	brand = strings.TrimPrefix(brand, "Marke: ")
	brand = strings.TrimPrefix(brand, "Brand: ")
	brand = strings.TrimPrefix(brand, "Besuchen Sie den ")
	brand = strings.TrimPrefix(brand, "Besuche den ")
	brand = strings.TrimPrefix(brand, "Besuchen den ")
	brand = strings.TrimSuffix(brand, "-Store")
	return strings.TrimSpace(brand)
}

func (p *AmazonParser) extractCurrentSize(doc *goquery.Document) string {
	if s := strings.TrimSpace(doc.Find("#variation_size_name .selection").Text()); s != "" {
		return s
	}
	return ""
}

func (p *AmazonParser) extractCurrentColor(doc *goquery.Document) string {
	// 1) Dropdown prompt
	if s := strings.TrimSpace(doc.Find("#dropdown_selected_color_name .a-dropdown-prompt").Text()); s != "" {
		return s
	}
	// 2) Selected option in native dropdown
	if sel := doc.Find("#native_dropdown_selected_color_name option[selected]"); sel.Length() > 0 {
		if t := strings.TrimSpace(sel.Text()); t != "" {
			return t
		}
	}
	// 3) Inline Twister (radiogroup with image swatches)
	// Find any input with aria-checked=true under the inline twister container and read the img alt in the same li
	if btn := doc.Find("#tp-inline-twister-dim-values-container ul.dimension-values-list [aria-checked='true']"); btn.Length() > 0 {
		li := btn.Closest("li")
		if li.Length() > 0 {
			if alt, ok := li.Find("img[alt]").Attr("alt"); ok {
				alt = strings.TrimSpace(alt)
				if alt != "" {
					return alt
				}
			}
			// fallback within inline twister: use visible text inside swatch-text if available
			if t := strings.TrimSpace(li.Find(".swatch-text").Text()); t != "" {
				return t
			}
		}
	}
	// 3b) Inline Twister via data-initiallySelected (case-sensitive)
	if li := doc.Find("#tp-inline-twister-dim-values-container ul.dimension-values-list li[data-initiallySelected='true']"); li.Length() > 0 {
		if alt, ok := li.Find("img[alt]").Attr("alt"); ok {
			alt = strings.TrimSpace(alt)
			if alt != "" {
				return alt
			}
		}
	}
	// 4) Legacy selection container
	if c := strings.TrimSpace(doc.Find("#variation_color_name .selection").Text()); c != "" {
		return c
	}
	return ""
}

func (p *AmazonParser) extractAvailableSizes(doc *goquery.Document) []string {
	var sizes []string
	doc.Find("#variation_size_name li, #native_dropdown_selected_size_name option").Each(func(_ int, s *goquery.Selection) {
		val := strings.TrimSpace(s.Text())
		val = strings.TrimPrefix(val, "Größe:")
		val = strings.TrimPrefix(val, "Size:")
		val = strings.TrimSpace(val)
		if val != "" && !strings.Contains(strings.ToLower(val), "auswählen") {
			sizes = append(sizes, val)
		}
	})
	return uniqueStrings(sizes)
}

func (p *AmazonParser) extractAvailableColors(doc *goquery.Document) []string {
	var colors []string

	// 1) Inline Twister swatches (only available ones)
	doc.Find("#tp-inline-twister-dim-values-container ul.dimension-values-list li[data-initiallyunavailable='false'], #tp-inline-twister-dim-values-container ul.dimension-values-list li[data-initiallyUnavailable='false']").Each(func(_ int, li *goquery.Selection) {
		if alt, ok := li.Find("img[alt]").Attr("alt"); ok {
			alt = strings.TrimSpace(alt)
			if alt != "" {
				colors = append(colors, alt)
			}
			return
		}
		if t := strings.TrimSpace(li.Find(".swatch-text").Text()); t != "" {
			colors = append(colors, t)
		}
	})

	// 2) Dropdown options
	doc.Find("#native_dropdown_selected_color_name option").Each(func(_ int, s *goquery.Selection) {
		if v, ok := s.Attr("data-a-html-content"); ok {
			v = strings.TrimSpace(v)
			if v != "" && !strings.Contains(strings.ToLower(v), "auswählen") {
				colors = append(colors, v)
			}
			return
		}
		if t := strings.TrimSpace(s.Text()); t != "" && !strings.Contains(strings.ToLower(t), "auswählen") {
			colors = append(colors, t)
		}
	})

	// 3) Legacy swatches/selection
	if len(colors) == 0 {
		doc.Find("#variation_color_name li img, #variation_color_name .swatchAvailable").Each(func(_ int, s *goquery.Selection) {
			if alt, exists := s.Attr("alt"); exists {
				alt = strings.TrimSpace(alt)
				if alt != "" {
					colors = append(colors, alt)
				}
			}
		})
		if len(colors) == 0 {
			doc.Find("#variation_color_name .selection").Each(func(_ int, s *goquery.Selection) {
				val := strings.TrimSpace(s.Text())
				if val != "" {
					colors = append(colors, val)
				}
			})
		}
	}
	return uniqueStrings(colors)
}

func (p *AmazonParser) extractModel(doc *goquery.Document) string {
	var model string
	doc.Find("#detailBullets_feature_div li, #productDetails_techSpec_section_1 tr, #productDetails_detailBullets_sections1 tr").Each(func(_ int, s *goquery.Selection) {
		label := strings.TrimSpace(s.Find("span.a-text-bold").Text())
		if label == "" {
			label = strings.TrimSpace(s.Find("th").Text())
		}
		val := strings.TrimSpace(s.Find("span, td").Last().Text())
		if strings.Contains(strings.ToLower(label), "modell") {
			model = val
		}
	})
	return model
}

func (p *AmazonParser) extractMaterialAndCare(doc *goquery.Document) (map[string]float64, []string, *models.MaterialInfo) {
	comp := map[string]float64{}
	var care []string
	var materialInfo *models.MaterialInfo

	// Track extraction sources and confidence
	sources := map[string][]string{}
	alternativeTexts := []string{}
	highestConfidence := 0.0
	bestSource := ""

	// Track processed texts to avoid duplicates
	processedTexts := make(map[string]bool)

	// 1. Try bullets first
	doc.Find("#feature-bullets li").Each(func(_ int, s *goquery.Selection) {
		text := strings.TrimSpace(s.Text())
		if text == "" || processedTexts[text] {
			return
		}
		processedTexts[text] = true
		lower := strings.ToLower(text)
        if strings.Contains(lower, "material") || strings.Contains(lower, "%") {
            if !parseMaterialPart(text, comp) {
                care = append(care, "MATERIAL_FULLTEXT:"+text)
            }
            sources["bullets"] = append(sources["bullets"], text)
            alternativeTexts = append(alternativeTexts, text)
		} else if strings.Contains(lower, "pflege") || strings.Contains(lower, "wasch") {
			care = append(care, text)
		}
	})

	// 2. Extract from productFactsDesktop section (Materialzusammensetzung)
	doc.Find(".product-facts-detail").Each(func(_ int, s *goquery.Selection) {
		labelText := strings.TrimSpace(s.Find(".a-col-left span").Eq(1).Text())
		valueText := strings.TrimSpace(s.Find(".a-col-right span").Eq(1).Text())

        if strings.Contains(strings.ToLower(labelText), "materialzusammensetzung") ||
           strings.Contains(strings.ToLower(labelText), "material") {
            if !processedTexts[valueText] {
                processedTexts[valueText] = true
                successfulParse := parseMaterialPart(valueText, comp)
                if !successfulParse && valueText != "" {
                    care = append(care, "MATERIAL_FULLTEXT:"+valueText)
                }
                sources["product_facts"] = append(sources["product_facts"], valueText)
                alternativeTexts = append(alternativeTexts, valueText)
                if successfulParse {
                	highestConfidence = 0.9 // High confidence for structured product facts
                	bestSource = "product_facts"
                }
            }
		}
		// Also check for care instructions in this section
		lowerValue := strings.ToLower(valueText)
		if strings.Contains(lowerValue, "pflege") || strings.Contains(lowerValue, "wasch") ||
		   strings.Contains(lowerValue, "care") || strings.Contains(lowerValue, "machine wash") {
			care = append(care, valueText)
		}
	})

	// 2b. Extract from a-fixed-left-grid-inner layout (the pattern you provided)
	doc.Find(".a-fixed-left-grid-inner").Each(func(_ int, s *goquery.Selection) {
		// Look for the left column containing the label
		leftCol := s.Find(".a-col-left")
		rightCol := s.Find(".a-col-right")

		if leftCol.Length() > 0 && rightCol.Length() > 0 {
			labelText := strings.TrimSpace(leftCol.Text())
			valueText := strings.TrimSpace(rightCol.Text())

			// Check if this is a material composition section
			if strings.Contains(strings.ToLower(labelText), "materialzusammensetzung") ||
			   strings.Contains(strings.ToLower(labelText), "material") ||
			   strings.Contains(strings.ToLower(labelText), "stoff") ||
			   strings.Contains(strings.ToLower(labelText), "obermaterial") {

				successfulParse := parseMaterialPart(valueText, comp)
				if !successfulParse && valueText != "" {
					care = append(care, "MATERIAL_FULLTEXT:"+valueText)
				}
				sources["fixed_grid"] = append(sources["fixed_grid"], valueText)
				alternativeTexts = append(alternativeTexts, valueText)
				if successfulParse {
					highestConfidence = 0.95 // Very high confidence for the specific pattern you provided
					bestSource = "fixed_grid"
				}
			}

			// Also check for care instructions
			lowerValue := strings.ToLower(valueText)
			if strings.Contains(lowerValue, "pflege") || strings.Contains(lowerValue, "wasch") ||
			   strings.Contains(lowerValue, "care") || strings.Contains(lowerValue, "machine wash") {
				care = append(care, valueText)
			}
		}
	})

	// 2c. Extract from other common Amazon material section patterns
	doc.Find("div.a-section.a-spacing-medium, div.a-section.a-spacing-small").Each(func(_ int, s *goquery.Selection) {
		// Try to find label-value pairs within this section
		// Look for spans containing material-related labels
		s.Find("span").Each(func(_ int, elem *goquery.Selection) {
			elemText := strings.TrimSpace(elem.Text())
			lowerElemText := strings.ToLower(elemText)

			// Check if this span contains a material label
			if strings.Contains(lowerElemText, "material") ||
			   strings.Contains(lowerElemText, "stoff") ||
			   strings.Contains(lowerElemText, "obermaterial") ||
			   strings.Contains(lowerElemText, "zusammensetzung") ||
			   strings.Contains(lowerElemText, "composition") {

				processedTexts := make(map[string]bool) // Prevent duplicate processing

				// Look for the value in the next sibling element
				next := elem.Next()
				if next.Length() > 0 {
					valueText := strings.TrimSpace(next.Text())
					if valueText != "" && !processedTexts[valueText] {
						processedTexts[valueText] = true
						successfulParse := parseMaterialPart(valueText, comp)
						if !successfulParse {
							care = append(care, "MATERIAL_FULLTEXT:"+valueText)
						}
						sources["alternative_sections"] = append(sources["alternative_sections"], valueText)
						alternativeTexts = append(alternativeTexts, valueText)
						if successfulParse && highestConfidence < 0.8 {
							highestConfidence = 0.8 // Medium confidence for alternative sections
							bestSource = "alternative_sections"
						}
					}
				}

				// Also look for the value in the parent's other children (but be more selective)
				parent := elem.Parent()
				if parent.Length() > 0 {
					parent.Find("div, p").Each(func(_ int, sibling *goquery.Selection) {
						if sibling.Get(0) != elem.Get(0) { // Not the same element
							valueText := strings.TrimSpace(sibling.Text())
							if valueText != "" && valueText != elemText && !processedTexts[valueText] {
								processedTexts[valueText] = true
								successfulParse := parseMaterialPart(valueText, comp)
								if !successfulParse {
									care = append(care, "MATERIAL_FULLTEXT:"+valueText)
								}
								sources["alternative_sections"] = append(sources["alternative_sections"], valueText)
								alternativeTexts = append(alternativeTexts, valueText)
								if successfulParse && highestConfidence < 0.8 {
									highestConfidence = 0.8 // Medium confidence for alternative sections
									bestSource = "alternative_sections"
								}
							}
						}
					})
				}
			}
		})
	})

	// 3. Fallback: details tables
	doc.Find("#detailBullets_feature_div li, #productDetails_techSpec_section_1 tr, #productDetails_detailBullets_sections1 tr").Each(func(_ int, s *goquery.Selection) {
		label := strings.TrimSpace(s.Find("span.a-text-bold").Text())
		if label == "" {
			label = strings.TrimSpace(s.Find("th").Text())
		}
		val := strings.TrimSpace(s.Find("span, td").Last().Text())
		ll := strings.ToLower(label)
        if strings.Contains(ll, "material") {
            if !parseMaterialPart(val, comp) && val != "" {
                care = append(care, "MATERIAL_FULLTEXT:"+val)
            }
            sources["details_tables"] = append(sources["details_tables"], val)
            alternativeTexts = append(alternativeTexts, val)
            if highestConfidence < 0.7 {
            	highestConfidence = 0.7 // Lower confidence for details tables fallback
            	bestSource = "details_tables"
            }
		}
		if strings.Contains(ll, "pflege") || strings.Contains(ll, "wasch") {
			care = append(care, val)
		}
	})

	// Normalize material percentages to ensure they don't exceed 100%
	// This handles cases where the same material is extracted from multiple sources
	var totalPercentage float64
	for _, percentage := range comp {
		if percentage > 0 {
			totalPercentage += percentage
		}
	}

	// If total exceeds realistic values (>200%), normalize to just the material presence
	if totalPercentage > 200 {
		normalizedComp := make(map[string]float64)
		for material := range comp {
			normalizedComp[material] = 0 // Set to 0 to indicate presence but no percentage
		}
		comp = normalizedComp
		highestConfidence = 0.5 // Lower confidence for deduplication
	}

	// Create MaterialInfo if we found any material composition
	if len(comp) > 0 || len(alternativeTexts) > 0 {
		// Normalize percentages to realistic values
		totalPercentage := 0.0
		for _, percentage := range comp {
			if percentage > 0 {
				totalPercentage += percentage
			}
		}

		// If we have unrealistic percentages (over 120%), fix them
		if totalPercentage > 120 {
			// Just show material presence without percentages for reliability
			normalizedComp := make(map[string]float64)
			for material := range comp {
				normalizedComp[material] = 0
			}
			comp = normalizedComp
			highestConfidence = 0.4 // Lower confidence for normalization
		}

		// Determine extraction method
		extractionMethod := "structured"
		if highestConfidence < 0.8 {
			extractionMethod = "regex"
		}

		// Get the best raw text (from the highest confidence source)
		var rawText string
		if bestSource != "" && len(sources[bestSource]) > 0 {
			rawText = strings.Join(sources[bestSource], "; ")
		} else if len(alternativeTexts) > 0 {
			rawText = strings.Join(alternativeTexts, "; ")
		}

		// If no materials found but we have alternative texts, set low confidence
		if len(comp) == 0 && len(alternativeTexts) > 0 {
			highestConfidence = 0.3
			extractionMethod = "text_only"
		}

		materialInfo = &models.MaterialInfo{
			Materials:        comp,
			RawText:          rawText,
			Confidence:       highestConfidence,
			Source:           bestSource,
			ExtractionMethod: extractionMethod,
			AlternativeTexts: uniqueStrings(alternativeTexts),
		}
	}

	return comp, uniqueStrings(care), materialInfo
}

// normalizeMaterialName maps DE/EN variants to canonical keys
func normalizeMaterialName(s string) string {
    s = strings.ToLower(strings.TrimSpace(s))
    // Handle common misspellings and variations first
    s = strings.ReplaceAll(s, "polyster", "polyester") // Handle the specific misspelling you provided
    s = strings.ReplaceAll(s, "polyestor", "polyester") // Another common misspelling
    s = strings.ReplaceAll(s, "poliester", "polyester") // Another variation
    s = strings.ReplaceAll(s, "polyster", "polyester") // Ensure replacement

    repl := map[string]string{
        "baumwolle": "cotton", "cotton": "cotton",
        "polyester": "polyester",
        "elasthan": "elastane", "elastan": "elastane", "elastane": "elastane", "spandex": "elastane", "lycra": "elastane",
        "polyamid": "polyamide", "polyamide": "polyamide", "nylon": "nylon",
        "viskose": "viscose", "viscose": "viscose", "rayon": "rayon",
        "acryl": "acrylic", "acrylic": "acrylic",
        "wolle": "wool", "wool": "wool",
        "seide": "silk", "silk": "silk",
        "leinen": "linen", "linen": "linen",
        "bambus": "bamboo", "bamboo": "bamboo",
        "kaschmir": "cashmere", "cashmere": "cashmere",
        "merinowolle": "merino", "merino": "merino",
        "modal": "modal",
        "lyocell": "lyocell",
        // Additional materials
        "polyurethan": "polyurethane",
        "acetat": "acetate",
        "kupfer": "copper",
        "tencel": "lyocell", // Tencel is a brand name for lyocell
        "microfaser": "microfiber",
        "fleece": "fleece",
        "denim": "denim",
        "jeans": "denim", // jeans is made of denim
        "cord": "corduroy",
    }
    if v, ok := repl[s]; ok {
        return v
    }
    return s
}

func isAllowedMaterial(name string) bool {
    _, ok := allowedMaterials[name]
    return ok
}

func detectMaterialsNoPercent(s string) []string {
    s = strings.ToLower(s)
    found := map[string]struct{}{}
    for _, kw := range constants.GetMaterialKeywords() {
        if strings.Contains(s, kw) {
            norm := normalizeMaterialName(kw)
            if isAllowedMaterial(norm) {
                found[norm] = struct{}{}
            }
        }
    }
    out := make([]string, 0, len(found))
    for k := range found {
        out = append(out, k)
    }
    return out
}

// parseMaterialPart extracts materials from any text fragment; returns true if something was added
func parseMaterialPart(part string, comp map[string]float64) bool {
    part = strings.ReplaceAll(part, "·", " ")
    part = strings.ReplaceAll(part, "•", " ")
    part = strings.TrimSpace(part)
    if part == "" {
        return false
    }

    txt := strings.ToLower(part)
    // Handle various separators including Chinese comma (，)
    txt = strings.ReplaceAll(txt, ";", ",")
    txt = strings.ReplaceAll(txt, "/", ",")
    txt = strings.ReplaceAll(txt, "，", ",") // Chinese comma
    txt = strings.ReplaceAll(txt, " und ", ",") // German "and"
    txt = strings.ReplaceAll(txt, " and ", ",") // English "and"
    txt = strings.ReplaceAll(txt, " & ", ",") // Ampersand

    matched := false

    // 1) Percentage-based extraction anywhere in text
    for _, m := range percentMaterialRe.FindAllStringSubmatch(txt, -1) {
        pctStr := strings.ReplaceAll(m[1], ",", ".")
        if v, err := strconv.ParseFloat(pctStr, 64); err == nil {
            name := strings.TrimSpace(m[2])
            // keep only first word for cases like "elastane spandex"
            name = strings.Fields(name)[0]
            name = strings.Trim(name, ",.;:()[]{}")
            name = normalizeMaterialName(name)
            if name != "" && isAllowedMaterial(name) {
                // Use the highest percentage found, don't accumulate
                if existing, exists := comp[name]; !exists || v > existing {
                    comp[name] = v
                }
                matched = true
            }
        }
    }
    if matched {
        return true
    }

    // 2) Enhanced fallback keyword detection without percentages
    // Split by common separators and process each part
    parts := strings.Split(txt, ",")
    for _, part := range parts {
        part = strings.TrimSpace(part)
        if part == "" {
            continue
        }

        // First, try to normalize the part directly (to handle misspellings like "Polyster")
        normalized := normalizeMaterialName(part)
        if normalized != "" && isAllowedMaterial(normalized) {
            if _, ok := comp[normalized]; !ok {
                comp[normalized] = 0
                matched = true
            }
            continue // Skip further processing if we already matched
        }

        // Check if this part contains a material keyword
        names := detectMaterialsNoPercent(part)
        for _, n := range names {
            if _, ok := comp[n]; !ok {
                comp[n] = 0 // Set to 0 to indicate presence but no percentage
                matched = true
            }
        }

        // If no materials found in this part, try to extract from compound materials
        // like "baumwollpolyester" or "cottonblend"
        if len(names) == 0 && len(part) > 4 {
            // Try to find material substrings within compound words
            for _, keyword := range constants.GetMaterialKeywords() {
                if strings.Contains(part, keyword) {
                    norm := normalizeMaterialName(keyword)
                    if norm != "" && isAllowedMaterial(norm) {
                        if _, ok := comp[norm]; !ok {
                            comp[norm] = 0
                            matched = true
                        }
                    }
                }
            }
        }
    }

    return matched
}

func uniqueStrings(in []string) []string {
	m := map[string]struct{}{}
	var out []string
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if _, ok := m[s]; ok {
			continue
		}
		m[s] = struct{}{}
		out = append(out, s)
	}
	return out
}

// extractTwisterFromScripts tries to parse sizes/colors from inline Twister JSON blocks.
// It heuristically scans <script> contents for keys like 'dimensionValuesDisplayData'
// and extracts arrays for 'size_name' and 'color_name'.
func (p *AmazonParser) extractTwisterFromScripts(doc *goquery.Document) (sizes []string, colors []string) {
	var (
		sizeRe  = regexp.MustCompile(`(?i)\"size_name\"\s*:\s*\[(.*?)\]`)
		colorRe = regexp.MustCompile(`(?i)\"color_name\"\s*:\s*\[(.*?)\]`)
		itemRe  = regexp.MustCompile(`\"([^\"]+)\"`)
	)
	doc.Find("script").Each(func(_ int, s *goquery.Selection) {
		text := s.Text()
		if text == "" {
			return
		}
		if m := sizeRe.FindStringSubmatch(text); len(m) == 2 {
			inner := m[1]
			for _, it := range itemRe.FindAllStringSubmatch(inner, -1) {
				if len(it) == 2 {
					val := strings.TrimSpace(it[1])
					if val != "" {
						sizes = append(sizes, val)
					}
				}
			}

		}
		if m := colorRe.FindStringSubmatch(text); len(m) == 2 {
			inner := m[1]
			for _, it := range itemRe.FindAllStringSubmatch(inner, -1) {
				if len(it) == 2 {
					val := strings.TrimSpace(it[1])
					if val != "" {
						colors = append(colors, val)
					}
				}
			}
		}
	})
	return uniqueStrings(sizes), uniqueStrings(colors)
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

// extractFeatures extrahiert Features aus "Info zu diesem Artikel" <ul>
func (p *AmazonParser) extractFeatures(doc *goquery.Document) []string {
	var features []string

	// Suche nach "Info zu diesem Artikel" H3 gefolgt von <ul>
	doc.Find("h3").Each(func(i int, h3 *goquery.Selection) {
		text := strings.TrimSpace(h3.Text())
		if strings.Contains(strings.ToLower(text), "info zu diesem artikel") {
			// Finde die nächste <ul> nach diesem H3
			var ul *goquery.Selection
			// Suche in nachfolgenden Geschwistern nach <ul>
			h3.NextAll().EachWithBreak(func(j int, s *goquery.Selection) bool {
				if s.Is("ul") {
					ul = s
					return false // Break the loop
				}
				return true // Continue
			})

			// Extrahiere alle <li> Elemente aus der <ul>
			if ul != nil {
				ul.Find("li").Each(func(k int, li *goquery.Selection) {
					feature := strings.TrimSpace(li.Text())
					if feature != "" {
						features = append(features, feature)
					}
				})
			}
		}
	})

	// Fallback: Suche auch in feature-bullets div
	if len(features) == 0 {
		doc.Find("#feature-bullets ul li").Each(func(i int, li *goquery.Selection) {
			// Überspringe das erste Element (oft nur "Beschreibung")
			if i == 0 {
				return
			}

			feature := strings.TrimSpace(li.Text())
			// Bereinige common Amazon text patterns
			feature = strings.TrimPrefix(feature, "Beschreibung")
			feature = strings.TrimSpace(feature)

			if feature != "" && !strings.Contains(feature, "Weitere Informationen") {
				features = append(features, feature)
			}
		})
	}

	return features
}

// extractProductGroups extrahiert alle Breadcrumb-Kategorien in umgekehrter Reihenfolge
func (p *AmazonParser) extractProductGroups(doc *goquery.Document) []string {
	var productGroups []string

	// Mehrere Selektoren versuchen, da Amazon verschiedene Strukturen verwendet
	selectors := []string{
		// Dein spezifisches HTML Format
		"#wayfinding-breadcrumbs_feature_div .a-unordered-list.a-horizontal li",
		// Alternative Formate
		"#wayfinding-breadcrumbs_feature_div ul li",
		"#wayfinding-breadcrumbs_feature_div li",
		".a-breadcrumb ul li",
		".a-breadcrumb li",
		// Noch mehr Fallbacks
		"[data-feature-name='wayfinding-breadcrumbs'] li",
		".a-subheader.a-breadcrumb li",
	}

	for _, selector := range selectors {
		tempGroups := []string{}

		doc.Find(selector).Each(func(i int, li *goquery.Selection) {
			// Debug: Log was gefunden wird
			// fmt.Printf("DEBUG: Li %d - Classes: %s, HTML: %s\n", i, li.AttrOr("class", ""), li.Text())

			// Überspringe Divider-Elemente (›)
			if li.HasClass("a-breadcrumb-divider") ||
				strings.Contains(li.AttrOr("class", ""), "divider") ||
				strings.Contains(li.AttrOr("aria-hidden", ""), "true") {
				return
			}

			// Extrahiere Text - probiere verschiedene Wege
			var categoryText string

			// 1. Versuche zuerst Links innerhalb von spans
			spanLink := li.Find("span a")
			if spanLink.Length() > 0 {
				categoryText = strings.TrimSpace(spanLink.Text())
			}

			// 2. Dann direkte Links
			if categoryText == "" {
				link := li.Find("a")
				if link.Length() > 0 {
					categoryText = strings.TrimSpace(link.Text())
				}
			}

			// 3. Dann spans ohne Links
			if categoryText == "" {
				span := li.Find("span")
				if span.Length() > 0 {
					categoryText = strings.TrimSpace(span.Text())
				}
			}

			// 4. Als letztes den ganzen li Text
			if categoryText == "" {
				categoryText = strings.TrimSpace(li.Text())
			}

			// Bereinige den Text
			categoryText = strings.TrimSpace(categoryText)
			// Entferne common Amazon patterns
			categoryText = strings.TrimSuffix(categoryText, "›")
			categoryText = strings.TrimSpace(categoryText)

			if categoryText != "" && categoryText != "›" {
				tempGroups = append(tempGroups, categoryText)
			}
		})

		// Wenn wir Ergebnisse haben, verwende sie
		if len(tempGroups) > 0 {
			productGroups = tempGroups
			break
		}
	}

	// Wenn immer noch leer, versuche noch aggressivere Selektoren
	if len(productGroups) == 0 {
		// Alle Links in breadcrumb-ähnlichen Containern
		fallbackSelectors := []string{
			"#wayfinding-breadcrumbs_feature_div a",
			".a-breadcrumb a",
			".breadcrumb a",
			"[data-feature-name='wayfinding-breadcrumbs'] a",
		}

		for _, selector := range fallbackSelectors {
			doc.Find(selector).Each(func(i int, a *goquery.Selection) {
				categoryText := strings.TrimSpace(a.Text())
				if categoryText != "" {
					productGroups = append(productGroups, categoryText)
				}
			})

			if len(productGroups) > 0 {
				break
			}
		}
	}

	// Umkehren der Reihenfolge: Hauptkategorie zuerst, oberste zuletzt
	// Beispiel: ["Fashion", "Herren", "Bekleidung", "Poloshirts"]
	// wird zu: ["Poloshirts", "Bekleidung", "Herren", "Fashion"]
	if len(productGroups) > 0 {
		reversed := make([]string, len(productGroups))
		for i, group := range productGroups {
			reversed[len(productGroups)-1-i] = group
		}
		return reversed
	}

	return productGroups
}
