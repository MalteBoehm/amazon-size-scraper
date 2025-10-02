package parser

import (
	"errors"
	"regexp"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/maltedev/amazon-size-scraper/internal/database"
)

// ParseSizeChartFromHTML finds the most likely size chart table in the document and
// parses it into database.SizeTable. Returns (nil, nil) if nothing suitable found.
func ParseSizeChartFromHTML(doc *goquery.Document) (*database.SizeTable, error) {
	table := findBestSizeTable(doc)
	if table == nil || table.Length() == 0 {
		return nil, nil
	}

	// Extract headers
	headers := extractHeaders(table)
	if len(headers) == 0 {
		return nil, errors.New("size chart: no headers")
	}
	keys := make([]string, len(headers))
	sizeCol := -1
	for i, h := range headers {
		key := normHeader(h)
		keys[i] = key
		if key == "size" {
			sizeCol = i
		}
	}
	if sizeCol == -1 {
		// Try to guess size col by value heuristics
		sizeCol = guessSizeColumn(table)
	}
	if sizeCol == -1 {
		return nil, errors.New("size chart: no size column")
	}

	st := &database.SizeTable{
		Sizes:        []string{},
		Measurements: map[string]map[string]float64{},
		Unit:         "cm",
	}

	// Parse rows
	table.Find("tr").Each(func(i int, tr *goquery.Selection) {
		if i == 0 {
			return // skip header
		}
		cells := tr.Find("th,td")
		if cells.Length() == 0 {
			return
		}
		// Size value
		sz := strings.TrimSpace(cells.Eq(sizeCol).Text())
		sz = normalizeSizeLabel(sz)
		if !isSizeLabel(sz) {
			return
		}
		if _, ok := st.Measurements[sz]; !ok {
			st.Measurements[sz] = make(map[string]float64)
			st.Sizes = append(st.Sizes, sz)
		}
		// Measurements
		cells.Each(func(ci int, c *goquery.Selection) {
			if ci == sizeCol || ci >= len(keys) {
				return
			}
			k := keys[ci]
			if k == "" {
				return
			}
			valText := strings.TrimSpace(c.Text())
			if valText == "" {
				return
			}
			min, max, ok := parseValue(valText)
			if !ok {
				return
			}
			v := min
			if max > 0 && max != min {
				v = (min + max) / 2.0 // store midpoint when range
			}
			// Values are converted to cm inside parseValue
			st.Measurements[sz][k] = v
		})
	})

	if len(st.Sizes) == 0 || len(st.Measurements) == 0 {
		return nil, nil
	}
	return st, nil
}

// findBestSizeTable picks a <table> with the best score
func findBestSizeTable(doc *goquery.Document) *goquery.Selection {
	best := 0
	var bestTable *goquery.Selection
	// Prefer popover tables
	candidates := doc.Find(".a-popover-wrapper table, #poSizeChart table, #sizeCharts table, .size-chart-table table, table")
	candidates.Each(func(_ int, t *goquery.Selection) {
		h := extractHeaders(t)
		if len(h) == 0 {
			return
		}
		keys := 0
		hasSize := false
		for _, x := range h {
			k := normHeader(x)
			if k == "size" {
				hasSize = true
			}
			if k != "" {
				keys++
			}
		}
		rows := t.Find("tr").Length()
		score := rows + keys*3
		if hasSize {
			score += 5
		}
		if score > best {
			best = score
			bestTable = t
		}
	})
	return bestTable
}

func extractHeaders(table *goquery.Selection) []string {
	var headers []string
	headerRow := table.Find("tr").First()
	if headerRow.Length() == 0 {
		return nil
	}
	headerRow.Find("th,td").Each(func(_ int, cell *goquery.Selection) {
		text := strings.TrimSpace(cell.Text())
		if text != "" {
			headers = append(headers, text)
		}
	})
	return headers
}

func normHeader(s string) string {
	t := strings.ToLower(strings.TrimSpace(s))
	t = strings.ReplaceAll(t, "ä", "a")
	t = strings.ReplaceAll(t, "ö", "o")
	t = strings.ReplaceAll(t, "ü", "u")
	switch {
	case strings.Contains(t, "gro") || strings.Contains(t, "size"):
		return "size"
	case strings.Contains(t, "lange") || strings.Contains(t, "length"):
		return "length"
	case strings.Contains(t, "brust") || strings.Contains(t, "chest") || strings.Contains(t, "bust"):
		return "chest"
	case strings.Contains(t, "schulter") || strings.Contains(t, "shoulder"):
		return "shoulder"
	case strings.Contains(t, "taille") || strings.Contains(t, "bund") || strings.Contains(t, "waist"):
		return "waist"
	case strings.Contains(t, "huf") || strings.Contains(t, "hip"):
		return "hips"
	case strings.Contains(t, "kragen") || strings.Contains(t, "collar") || strings.Contains(t, "neck"):
		return "collar"
	}
	return ""
}

func isSizeLabel(s string) bool {
	if s == "" {
		return false
	}
	ss := strings.ToUpper(strings.TrimSpace(s))
	// common alpha sizes
	alpha := map[string]struct{}{"XS":{}, "S":{}, "M":{}, "L":{}, "XL":{}, "XXL":{}, "3XL":{}, "4XL":{}}
	if _, ok := alpha[ss]; ok {
		return true
	}
	// numeric EU sizes
	numRe := regexp.MustCompile(`^\d{2}(?:/\d{2})?$`)
	return numRe.MatchString(ss)
}

func normalizeSizeLabel(s string) string {
	x := strings.TrimSpace(s)
	x = strings.ReplaceAll(x, "EU", "")
	x = strings.ReplaceAll(x, "DE", "")
	x = strings.TrimSpace(x)
	return strings.ToUpper(x)
}

// parseValue parses strings like "73,5 cm", "73-77", "28 in" and returns (min,max,ok) in cm.
func parseValue(s string) (float64, float64, bool) {
	x := strings.ToLower(strings.TrimSpace(s))
	x = strings.ReplaceAll(x, "ca.", "")
	x = strings.ReplaceAll(x, "~", "")
	x = strings.ReplaceAll(x, "—", "-")
	x = strings.ReplaceAll(x, "–", "-")
	x = strings.ReplaceAll(x, "−", "-")
	x = strings.ReplaceAll(x, "·", " ")
	x = strings.ReplaceAll(x, "•", " ")
	
	// detect unit
	unit := "cm"
	if strings.Contains(x, "inch") || strings.Contains(x, " in ") || strings.HasSuffix(x, " in") || strings.Contains(x, "\"") || strings.Contains(x, "″") {
		unit = "in"
	}
	// remove unit markers
	x = strings.ReplaceAll(x, "cm", "")
	x = strings.ReplaceAll(x, "inch", "")
	x = strings.ReplaceAll(x, "\"", "")
	x = strings.ReplaceAll(x, "″", "")
	x = strings.TrimSpace(x)
	
	// Handle potential misinterpretation of decimal separators
	// Check if we have a pattern like "25.65" or "256.5" which might be misread
	// German format uses comma as decimal separator, dot as thousands separator
	// But sometimes the data is inconsistent
	
	// First, normalize comma to dot for decimal
	x = strings.ReplaceAll(x, ",", ".")
	
	// range?
	if strings.Contains(x, " bis ") {
		parts := strings.Split(x, " bis ")
		if len(parts) == 2 {
			min, e1 := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
			max, e2 := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
			if e1 == nil && e2 == nil {
				if unit == "in" {
					min *= 2.54
					max *= 2.54
				}
				// Validate parsed values
				if min > 200 || max > 200 {
					// Log suspicious value for debugging
					// This likely means we misinterpreted the format
					// Try dividing by 10 if the value seems off by a factor of 10
					if min > 200 && min < 2000 {
						min = min / 10
					}
					if max > 200 && max < 2000 {
						max = max / 10
					}
				}
				return min, max, true
			}
		}
	}
	if strings.Contains(x, "-") {
		parts := strings.SplitN(x, "-", 2)
		min, e1 := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
		max, e2 := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
		if e1 == nil && e2 == nil {
			if unit == "in" {
				min *= 2.54
				max *= 2.54
			}
			// Validate parsed values
			if min > 200 || max > 200 {
				// Try dividing by 10 if the value seems off by a factor of 10
				if min > 200 && min < 2000 {
					min = min / 10
				}
				if max > 200 && max < 2000 {
					max = max / 10
				}
			}
			return min, max, true
		}
	}
	v, err := strconv.ParseFloat(strings.TrimSpace(x), 64)
	if err != nil {
		return 0, 0, false
	}
	if unit == "in" {
		v *= 2.54
	}
	
	// Validate single value
	// If value is suspiciously large (>200cm for clothing), it might be a parsing error
	if v > 200 && v < 2000 {
		// Try dividing by 10 - common error where 76.5 becomes 765
		v = v / 10
	}
	
	return v, v, true
}

// guessSizeColumn tries to find the size column by scanning values in first data rows.
func guessSizeColumn(table *goquery.Selection) int {
	rows := table.Find("tr")
	if rows.Length() < 2 {
		return -1
	}
	cols := rows.First().Find("th,td").Length()
	best := -1
	bestScore := 0
	for c := 0; c < cols; c++ {
		score := 0
		rows.Each(func(i int, r *goquery.Selection) {
			if i == 0 { // header row
				return
			}
			cell := r.Find("th,td").Eq(c)
			val := normalizeSizeLabel(strings.TrimSpace(cell.Text()))
			if isSizeLabel(val) {
				score++
			}
		})
		if score > bestScore {
			bestScore = score
			best = c
		}
	}
	return best
}

