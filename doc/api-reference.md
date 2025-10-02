# Amazon Scraper API Reference

## Base URL
```
http://localhost:8084/api/v1/scraper
```

## Endpoints

### 1. Create Search Job
Erstellt einen neuen Job zur Produktsuche auf Amazon.

**Endpoint:** `POST /jobs`

**Request Body:**
```json
{
  "search_query": "lange rote t shirts männer",
  "category": "fashion",
  "max_pages": 5
}
```

**Response (201 Created):**
```json
{
  "job_id": "3113786c-00f9-43d8-97d4-1eccfded4794",
  "status": "pending",
  "message": "Job created successfully"
}
```

### 2. Get Job Status
Ruft den aktuellen Status eines Jobs ab.

**Endpoint:** `GET /jobs/{jobID}`

**Response (200 OK):**
```json
{
  "id": "3113786c-00f9-43d8-97d4-1eccfded4794",
  "search_query": "lange rote t shirts männer",
  "status": "completed",
  "products_found": 60,
  "created_at": "2025-07-04T22:05:01Z",
  "completed_at": "2025-07-04T22:05:38Z"
}
```

### 3. List All Jobs
Listet alle Jobs auf.

**Endpoint:** `GET /jobs`

**Response (200 OK):**
```json
[
  {
    "id": "3113786c-00f9-43d8-97d4-1eccfded4794",
    "search_query": "lange rote t shirts männer",
    "status": "completed",
    "products_found": 60
  }
]
```

### 4. Get Job Products
Ruft alle Produkte ab, die von einem Job gefunden wurden.

**Endpoint:** `GET /jobs/{jobID}/products`

**Response (200 OK):**
```json
[
  {
    "asin": "B01FEHAWC4",
    "title": "Fruit of the Loom Herren T-Shirt",
    "price": 12.99,
    "url": "https://www.amazon.de/dp/B01FEHAWC4",
    "image_url": "https://m.media-amazon.com/images/I/..."
  }
]
```

### 5. Extract Size Chart
Extrahiert die Größentabelle eines Produkts.

**Endpoint:** `POST /size-chart`

**Request Body:**
```json
{
  "asin": "B01FEHAWC4"
}
```
oder
```json
{
  "url": "https://www.amazon.de/dp/B01FEHAWC4"
}
```

**Response (200 OK):**
```json
{
  "size_chart_found": true,
  "size_table": {
    "sizes": ["S", "M", "L", "XL", "XXL", "3XL", "4XL", "5XL"],
    "measurements": {
      "S": {
        "chest": 96,
        "length": 71
      },
      "M": {
        "chest": 104,
        "length": 73
      },
      "L": {
        "chest": 112,
        "length": 75
      },
      "XL": {
        "chest": 124,
        "length": 77
      },
      "XXL": {
        "chest": 132,
        "length": 79
      },
      "3XL": {
        "chest": 140,
        "length": 81
      },
      "4XL": {
        "chest": 148,
        "length": 83
      },
      "5XL": {
        "chest": 156,
        "length": 85
      }
    },
    "unit": "cm"
  }
}
```

### 6. Extract Reviews
Extrahiert Produktbewertungen.

**Endpoint:** `POST /reviews`

**Request Body:**
```json
{
  "asin": "B01FEHAWC4"
}
```

**Response (200 OK):**
```json
{
  "reviews": [
    {
      "rating": 5,
      "title": "Super T-Shirt",
      "text": "Passt perfekt, gute Länge für große Menschen",
      "verified_buyer": true,
      "date": "4. Juli 2025",
      "mentions_size": true,
      "mentions_length": true
    }
  ],
  "average_rating": 4.3,
  "total_reviews": 1234
}
```

### 7. Get Statistics
Ruft Statistiken über alle Jobs ab.

**Endpoint:** `GET /stats`

**Response (200 OK):**
```json
{
  "total_jobs": 10,
  "completed_jobs": 8,
  "failed_jobs": 1,
  "pending_jobs": 1,
  "total_products": 480
}
```

## Error Responses

**400 Bad Request:**
```json
{
  "error": "search_query is required"
}
```

**404 Not Found:**
```json
{
  "error": "job not found"
}
```

**500 Internal Server Error:**
```json
{
  "error": "failed to create job"
}
```

## Beispiel-Workflow

1. **Produktsuche starten:**
```bash
curl -X POST http://localhost:8084/api/v1/scraper/jobs \
  -H "Content-Type: application/json" \
  -d '{"search_query": "lange t-shirts männer größe xl", "category": "fashion", "max_pages": 3}'
```

2. **Job-Status prüfen:**
```bash
curl http://localhost:8084/api/v1/scraper/jobs/3113786c-00f9-43d8-97d4-1eccfded4794
```

3. **Gefundene Produkte abrufen:**
```bash
curl http://localhost:8084/api/v1/scraper/jobs/3113786c-00f9-43d8-97d4-1eccfded4794/products
```

4. **Größentabelle für ein Produkt extrahieren:**
```bash
curl -X POST http://localhost:8084/api/v1/scraper/size-chart \
  -H "Content-Type: application/json" \
  -d '{"asin": "B01FEHAWC4"}'
```

## Hinweise

- Die API verwendet Playwright für das Browser-Scraping
- Es gibt automatische Bot-Detection-Umgehung
- Rate Limiting ist implementiert, um Blockierungen zu vermeiden
- Größentabellen werden als JSONB in PostgreSQL gespeichert
- Alle gefundenen Produkte lösen NEW_PRODUCT_DETECTED Events in Redis aus