# Size‑Chart Parser: Robustheits‑Konzept und Implementierungsleitfaden

Ziel: Zuverlässige Extraktion der Größen‑ und Maßtabellen (Size Charts) aus Amazon‑Produktseiten, robust gegen HTML‑Varianten, Sprachen und unvollständige Daten. Ergänzend: robuste Varianten‑Ermittlung (available_sizes, colors) über DOM und Twister‑Script‑Fallback.

## Anforderungen
- Erkennen und Parsen verschiedener Tabellen‑Layouts (Popover/Modal und In‑Page)
- Normalisierung von Headern auf kanonische Keys
- Tolerantes Zahlen‑Parsing (Komma/Punkt, Bereiche, Einheiten)
- Einheitliche Maßeinheit: cm
- Mehrsprachigkeit: DE/EN (erweiterbar)
- Resilienz bei Rauschen ("ca.", Sonderzeichen, Zierstriche)
- Testbarkeit mit HTML‑Fixtures

## Architektur und Hauptschritte

1) Kandidatenfindung (Table Discovery)
- Selektoren (gewichtetes Scoring):
  - `.a-popover-wrapper`, `#poSizeChart`, `#chart-table`, `#sizeCharts`, `.aplus-module`, `.size-chart-table`, `.a-table`
- Scoring‑Kriterien:
  - Header enthält mind. einen Size‑Header und mind. einen Messkopf
  - Zeilenanzahl ≥ 3
  - Anteil valider Größenwerte (S, M, L, XL, XXL, 3XL… bzw. numerische EU‑Größen)
- Auswahl: Tabelle mit höchstem Score; Fallback: größte valide Tabelle

2) Header‑Normalisierung (Canonical Keys)
- Mapping auf: `size, length, chest, shoulder, waist, hips, collar`
- Heuristik (Kleinschreibung, Diakritika entfernen, Substring‑Matches):
  - `größe|size` → `size`
  - `länge|gesamtlänge|rückenlänge|length` → `length`
  - `brust|brustumfang|chest|bust` → `chest`
  - `schulter|schulterbreite|shoulder` → `shoulder`
  - `taille|bund|waist` → `waist`
  - `hüfte|hip|hips` → `hips`
  - `kragen|collar|neck` → `collar`
- Unbekannte Spalten: ignorieren oder im Debug loggen

3) Größen‑Spalte bestimmen
- Primär: Header == `size`
- Sekundär: Spalte mit höchster Übereinstimmung typischer Größenmuster (S…3XL, 44/46/48)
- Normalisierung: Trimmen, Varianten vereinheitlichen (z. B. `EU L` → `L`)

4) Wert‑Parsing (Zahlen, Bereiche, Einheiten)
- Entfernen: `cm`, `in`, `″`, `"`, `ca.`, Zierzeichen (`—`, `–`, `·`, `•`)
- Dezimal: `,` → `.`
- Bereiche: `73–77`, `73-77`, `73 bis 77` → [73, 77] (oder Mittelwert, je nach Schema)
- Einheiten: alles in `cm` (1 inch = 2.54 cm)
- Ergebnisstruktur pro Größe: Map[KanonischerMessKey] → float64 oder [min,max]

5) Ausschluss nicht relevanter Messungen
- Arm/Ärmellänge (Sleeve) ignorieren für T‑Shirts, Fokus auf `length` (Gesamtlänge)
- Falls `back length` und `length` existieren: priorisiere `length`

6) Variationen (Größen/Farben)
- DOM‑Extraktion: `#variation_size_name`, `#variation_color_name`
- Twister‑Fallback: suche Script mit `dimensionValuesDisplayData`/`twister` und extrahiere `size_name`/`color_name`
- Ergebnis: `available_sizes`, `variation_attributes` (type=size|color)

7) Logging & Debug
- Strukturierte Logs pro ASIN:
  - gewählter Table‑Selector, Header‑Mapping, Zeilenanzahl
  - Mess‑Keys, Einheiten, Anzahl gültiger Werte
  - Warnungen zu unbekannten Headern/Einheiten
- Optional: Debug‑Dump des finalen `size_chart_data` vor Persistierung

## Öffentliche API (Vorschlag)

Package: `internal/parser`

- `func ParseSizeChartFromHTML(doc *goquery.Document) (*database.SizeTable, error)`
  - Liest die beste Tabelle, normalisiert, konvertiert nach cm
  - Füllt `database.SizeTable{ Sizes []string, Measurements map[string]map[string]float64, Unit string }`
  - Für Bereiche optional: zusätzlicher Schlüssel `length_min/length_max` – oder Speicherung als Durchschnitt (Designentscheidung, siehe unten)

- `func ExtractVariations(doc *goquery.Document) (sizes []string, colors []string, attrs []models.VariationAttribute)`
  - Kombiniert DOM + Twister‑Fallback

Hilfsfunktionen (nicht‑exportiert):
- `findBestSizeTable(doc *goquery.Document) *goquery.Selection`
- `normHeader(s string) string`
- `isSizeLabel(s string) bool`
- `parseValue(s string) (min, max float64, ok bool)`
- `toCM(val float64, unit string) float64`

## Datenmodell‑Details

- Kanonische Keys: `size, length, chest, shoulder, waist, hips, collar`
- Einheit: `cm`
- Bereiche: Zwei Optionen – bitte festlegen
  1) Mittelwert speichern (einfach, kompatibel zu vorhandenem `float64`‑Schema)
  2) Min/Max als separate Keys (z. B. `length_min`, `length_max`)

Empfehlung: Mittelwert, plus in `size_chart_data` optional die Rohwerte/Range als Kommentar/Meta.

## Tests

- Fixtures: abgelegte HTML‑Dumps (`debug/html/*_size_table.html`, `*_product_page.html`)
- Unit‑Tests:
  - DE/EN Header‑Varianten
  - Bereichswerte (73–77)
  - Inch‑Werte (Konvertierung)
  - Mehrere Tabellen – Scoring wählt die richtige
  - Größen‑Spalte wird korrekt erkannt
- Integration‑lite:
  - Gegen reale Dumps sicherstellen, dass kein Panic/Fehler entsteht und `SizeTable` sinnvolle Werte enthält

## Observability

- Log‑Felder: `asin`, `selector`, `row_count`, `headers`, `keys`, `unit`, `valid_values`, `warnings`
- Optional Metriken: Anzahl geparster Tabellen, Verteilungs‑Histogramme zu Wertebereichen

## Rollout‑Plan (Schritte A–F)

- A: Table‑Scoring + Header‑Normalization
- B: Value‑Parser (Range/Einheiten)
- C: Size‑Spalte robust erkennen + Größen normalisieren
- D: Twister‑Fallback (JSON isolieren, Unescape & parse)
- E: Tests mit 4+ Layouts und euren Dumps
- F: psql‑Verifikation (Stichprobe)

## Bezüge zu tall‑affiliate‑common (falls verfügbar)

Wenn das Repo `tall-affiliate-common` genutzt wird, empfehle ich die Auslagerung gemeinsamer Konstanten/Contracts:

- `packages/measurement/headers.go`
  - Exportiert kanonische Keys und Header‑Synonyms (DE/EN)
- `packages/measurement/units.go`
  - Einheitenkonvertierung (inch→cm), erlaubte Einheiten, Normalisierung
- `packages/measurement/size_norm.go`
  - Normalisierung von Größenlabels (S/M/L/XL/XXL/3XL, numerische EU)
- `docs/scraping/size-chart-parser-spec.md`
  - Kurze Spezifikation (diese Datei konsolidiert)

Namenskonventionen: snake_case für Events/Streams, wie in eurer Architektur.

## Risiken & Fallbacks

- HTML‑Layout ändert sich → Table‑Scoring + Header‑Map abfedern
- Rauschen in Zellen → Parser ignoriert nicht parsebare Felder statt hart zu scheitern
- Inches dominieren → Konvertierung sicherstellen, Tests hinzufügen

## Offene Punkte

- Speicherung von Bereichen (Min/Max vs. Mittelwert) – bitte Entscheidung
- Erweiterung auf weitere Messarten (z. B. `sleeve` ausschließlich für Langarm)

## Quick‑Start (Entwickler)

- Tests: `go test ./internal/parser -v`
- Debug Run (HTML‑Dump → DB):
  - `DB_HOST=... DB_PORT=... DB_USER=... DB_PASSWORD=... DB_NAME=... DB_SSL_MODE=disable go run amzon-size-scraper/cmd/fill-from-html --dir ./amzon-size-scraper/debug/html --match "*_product_page.html"`
- psql‑Verifikation (Beispiel):
  - `SELECT asin, size, color, jsonb_array_length(available_sizes) sizes_len, jsonb_array_length(variation_attributes) var_len FROM product ORDER BY updated_at DESC LIMIT 20;`

