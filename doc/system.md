⏺ Architektur-Beschreibung: Tall Affiliate Amazon Product 
  Scraping System

  System-Übersicht

  Das Tall Affiliate System ist eine event-getriebene
  Microservices-Architektur zur automatischen Erfassung und
   Anreicherung von Amazon-Produktdaten, speziell optimiert
   für große/lange Kleidungsstücke. Das System extrahiert
  physische Dimensionen (Länge, Breite) von T-Shirts und
  berechnet einen Fit Score für große Menschen.

  Kernfunktionalität

  1. Produkterkennung: Automatische Erfassung neuer
  Amazon-Produkte via PA-API
  2. Dimensionsextraktion: Scraping von Größentabellen
  mittels Oxylabs zur Ermittlung von T-Shirt-Länge und
  -Breite
  3. Qualitätsbewertung: Berechnung eines Fit Scores
  (0-5.5) unter Berücksichtigung der physischen Dimensionen
  4. Content-Generierung: LLM-basierte Erstellung von
  produktspezifischen Beschreibungen

  Microservices-Architektur

  1. API Gateway (Port 8080)

  - Integration mit Amazon Product Advertising API
  - Initiiert den Event-Flow mit NEW_PRODUCT_DETECTED
  - REST und gRPC Schnittstellen

  2. Amazon Scraper Service (Port 8084)

  - REST API für Produktsuche und Dimensionsextraktion
  - Ersetzt Oxylabs durch direktes Browser-Scraping mit Playwright
  - API Endpunkte:
    - POST /api/v1/scraper/jobs - Erstellt neuen Such-Job
    - GET /api/v1/scraper/jobs - Listet alle Jobs
    - GET /api/v1/scraper/jobs/{id} - Job-Status
    - GET /api/v1/scraper/jobs/{id}/products - Gefundene Produkte
    - POST /api/v1/scraper/size-chart - Extrahiert Größentabelle
    - POST /api/v1/scraper/reviews - Extrahiert Reviews
  - Extrahiert komplette Größentabellen:
    - Alle verfügbaren Größen (S, M, L, XL, etc.)
    - Alle Messungen pro Größe (length, chest, shoulder, sleeve)
    - Speichert als JSONB in PostgreSQL
  - Event-basierte Integration via Redis Streams

3. Product Lifecycle Service (Port 8082)

  - Konsumiert NEW_PRODUCT_DETECTED Events
  - Ruft Amazon Scraper Service auf für Größentabellen
  - Speichert komplette size_table als JSONB
  - Berechnet Quality/Fit Score
  - Entscheidet über Produktaktivierung (Score ≥ 3.0)

  3. Content Generation Service (Port 8083)

  - Generiert Produktbeschreibungen via OpenRouter LLM
  - Nutzt Oxylabs für Review-Analyse
  - Arbeitet nur mit bereits validierten Produkten

  Event-Flow für Dimensionsextraktion

  1. Produktsuche via Amazon Scraper REST API
     - POST /api/v1/scraper/jobs mit Suchbegriff
     - Durchsucht Amazon Kategorie "fashion"
     - Erstellt NEW_PRODUCT_DETECTED Events für gefundene Produkte

  2. Product Lifecycle Service empfängt Event
     - Ruft Amazon Scraper API für Größentabellen-Extraktion auf
     - Scraper klickt "Größentabelle" Button mit Playwright
     - Parst komplette Größentabelle (alle Größen und Messungen)
     - Speichert als JSONB in PostgreSQL

  3. Dimension-Speicherung und Score-Berechnung
     - Komplette size_table in PostgreSQL gespeichert
     - Quality Score berechnet basierend auf Verfügbarkeit von Längenmaßen
     - Penalty (-1.0) wenn keine Längenmessung gefunden
     - Bonus (+0.5 bis +1.0) für tall-friendly Dimensionen

  4. Bei Score ≥ 3.0 → Content-Generierung
     - Produkt wird als "active" markiert
     - Content Generation nutzt gespeicherte Größentabellen

  Technologie-Stack

  - Event-System: Redis Streams mit Transactional Outbox
  Pattern
  - Datenbank: PostgreSQL (Hauptdaten), Redis
  (Events/Cache)
  - Web-Scraping: Playwright-go für direktes Browser-Scraping
  - LLM: OpenRouter für Content-Generierung
  - Monitoring: OpenTelemetry, Prometheus, Grafana

  Besonderheiten für T-Shirt Dimensionen

  - Komplette Größentabellen-Speicherung:
    - Alle verfügbaren Größen (XS bis 6XL)
    - Alle Messungen pro Größe (Länge, Brust, Schulter, Ärmel)
    - JSONB-Format ermöglicht flexible Abfragen
  - Fit Score Algorithmus: Prüft Verfügbarkeit von
  Längenmaßen in beliebiger Größe
  - Fallback-Strategie: Bei fehlenden Längenmaßen
  Score-Penalty, aber Produkt kann trotzdem aktiv werden
  - REST API für manuelle Größentabellen-Abfragen

  Fehlerbehandlung

  - Exponential Backoff für API-Calls
  - Dead Letter Queue für nicht-verarbeitbare Events
  - Circuit Breaker für externe APIs
  - Strukturiertes Logging aller
  Dimensionsextraktions-Versuche

  Diese Architektur ermöglicht die zuverlässige Extraktion
  von T-Shirt-Dimensionen aus Amazon-Produktseiten und
  deren Verwendung zur Bewertung der Eignung für große
  Menschen.