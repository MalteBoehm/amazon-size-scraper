package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

// Event struct matches tall-affiliate-common Event structure
type Event struct {
	ID            string         `json:"id"`
	Type          string         `json:"type"`
	AggregateType string         `json:"aggregate_type"`
	AggregateID   string         `json:"aggregate_id"`
	Payload       any            `json:"payload"`
	Timestamp     time.Time      `json:"timestamp"`
	Metadata      map[string]any `json:"metadata,omitempty"`
}

// ProductCreatedPayload matches tall-affiliate-common ProductCreatedPayload
type ProductCreatedPayload struct {
	ASIN           string   `json:"asin"`
	Title          string   `json:"title"`
	Brand          string   `json:"brand,omitempty"`
	Category       string   `json:"category,omitempty"`
	Gender         string   `json:"gender,omitempty"`
	CurrentPrice   float64  `json:"current_price,omitempty"`
	Currency       string   `json:"currency,omitempty"`
	DetailPageURL  string   `json:"detail_page_url,omitempty"`
	ImageUrls      []string `json:"image_urls,omitempty"`
	Features       []string `json:"features,omitempty"`
	BrowseNodeID   string   `json:"browse_node_id,omitempty"`
	BrowseNodeTags []string `json:"browse_node_tags,omitempty"`
}

// Event constants from tall-affiliate-common
const (
	EVENT_02A_PRODUCT_VALIDATED = "02A_PRODUCT_VALIDATED"
)

// ParsePayload matches tall-affiliate-common helper function
func ParsePayload(payload interface{}, target interface{}) error {
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}
	if err := json.Unmarshal(jsonData, target); err != nil {
		return fmt.Errorf("failed to unmarshal payload: %w", err)
	}
	return nil
}

func main() {
	// Setup logger
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	// Redis connection
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}

	rdb := redis.NewClient(&redis.Options{
		Addr: redisAddr,
	})

	// Test Redis connection
	ctx := context.Background()
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	logger.Info("Connected to Redis", "addr", redisAddr)

	// Database connection
	dbURL := fmt.Sprintf("postgres://postgres:%s@localhost:%s/tall_affiliate?sslmode=disable",
		getEnv("DB_PASSWORD", "postgres"),
		getEnv("DB_PORT", "5433"),
	)
	
	db, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()
	
	if err := db.Ping(ctx); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}
	logger.Info("Connected to database")

	// Create consumer
	consumer := &Consumer{
		redis:      rdb,
		db:         db,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		scraperURL: getEnv("SCRAPER_URL", "http://localhost:8084"),
		logger:     logger,
	}

	// Setup graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start consuming in background
	ctx, cancel := context.WithCancel(ctx)
	go func() {
		<-sigChan
		logger.Info("Shutting down...")
		cancel()
	}()

	// Run consumer
	if err := consumer.Run(ctx); err != nil {
		log.Fatalf("Consumer error: %v", err)
	}
}

type Consumer struct {
	redis      *redis.Client
	db         *pgxpool.Pool
	httpClient *http.Client
	scraperURL string
	logger     *slog.Logger
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func (c *Consumer) Run(ctx context.Context) error {
	// Check for stream override from environment
	streamKey := getEnv("REDIS_STREAM", "stream:product_lifecycle")
	consumerGroup := "lifecycle-consumer-group"
	consumerName := "consumer-1"

	// Create consumer group (ignore error if already exists)
	c.redis.XGroupCreate(ctx, streamKey, consumerGroup, "0").Err()

	c.logger.Info("Starting consumer", "stream", streamKey, "group", consumerGroup)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			// Read from stream
			streams, err := c.redis.XReadGroup(ctx, &redis.XReadGroupArgs{
				Group:    consumerGroup,
				Consumer: consumerName,
				Streams:  []string{streamKey, ">"},
				Count:    1,
				Block:    5 * time.Second,
				NoAck:    false, // Auto-acknowledge for testing
			}).Result()

			if err != nil {
				if err == redis.Nil {
					continue // No new messages
				}
				c.logger.Error("Failed to read from stream", "error", err)
				time.Sleep(1 * time.Second)
				continue
			}

			// Process messages
			for _, stream := range streams {
				for _, message := range stream.Messages {
					if err := c.processMessage(ctx, message); err != nil {
						c.logger.Error("Failed to process message", "id", message.ID, "error", err)
						continue
					}

					// Acknowledge message
					if err := c.redis.XAck(ctx, streamKey, consumerGroup, message.ID).Err(); err != nil {
						c.logger.Error("Failed to acknowledge message", "id", message.ID, "error", err)
					}
				}
			}
		}
	}
}

func (c *Consumer) processMessage(ctx context.Context, msg redis.XMessage) error {
	// Parse the event using tall-affiliate-common structure
	var event Event

	// Method 1: Try to unmarshal the entire message as a single event JSON
	eventJSON, err := json.Marshal(msg.Values)
	if err != nil {
		return fmt.Errorf("failed to marshal message values: %w", err)
	}

	if err := json.Unmarshal(eventJSON, &event); err != nil {
		// Method 2: Try to find individual fields and build event manually
		event.Type, _ = msg.Values["type"].(string)
		event.AggregateID, _ = msg.Values["aggregate_id"].(string)
		event.AggregateType, _ = msg.Values["aggregate_type"].(string)
		event.Payload = msg.Values["payload"]

		// Parse timestamp if available
		if timestampStr, ok := msg.Values["timestamp"].(string); ok {
			if timestamp, err := time.Parse(time.RFC3339, timestampStr); err == nil {
				event.Timestamp = timestamp
			}
		}
	}

	// Check if this is a product event we should process
	if event.Type != EVENT_02A_PRODUCT_VALIDATED {
		c.logger.Info("Skipping non-product event",
			"event_type", event.Type,
			"aggregate_id", event.AggregateID,
		)
		return nil
	}

	// Get ASIN from aggregate_id (standard tall-affiliate-common pattern)
	asin := event.AggregateID
	if asin == "" {
		// Fallback: try to parse ASIN from payload
		var payload ProductCreatedPayload
		if err := ParsePayload(event.Payload, &payload); err == nil {
			asin = payload.ASIN
		}
	}

	if asin == "" {
		return fmt.Errorf("missing ASIN in event (aggregate_id or payload)")
	}

	c.logger.Info("Processing validated product",
		"message_id", msg.ID,
		"event_type", event.Type,
		"asin", asin,
		"aggregate_type", event.AggregateType,
	)

	// Parse payload to get product details
	var productPayload ProductCreatedPayload
	if err := ParsePayload(event.Payload, &productPayload); err != nil {
		c.logger.Warn("Failed to parse product payload, proceeding with ASIN only",
			"asin", asin,
			"error", err,
		)
		// Use minimal info if payload parsing fails
		productPayload = ProductCreatedPayload{
			ASIN:          asin,
			Title:         "Unknown Product",
			DetailPageURL: fmt.Sprintf("https://www.amazon.de/dp/%s", asin),
		}
	}

	// Check if product exists and is still pending
	var status string
	var dbErr error
	dbErr = c.db.QueryRow(ctx, "SELECT status FROM products WHERE asin = $1", asin).Scan(&status)
	if dbErr != nil {
		// Product doesn't exist, create it
		url := productPayload.DetailPageURL
		if url == "" {
			url = fmt.Sprintf("https://www.amazon.de/dp/%s", asin)
		}

		insertQuery := `INSERT INTO products (asin, title, url, brand, status)
		                VALUES ($1, $2, $3, $4, 'pending')
		                ON CONFLICT (asin) DO NOTHING`
		_, insertErr := c.db.Exec(ctx, insertQuery,
			productPayload.ASIN,
			productPayload.Title,
			url,
			productPayload.Brand,
		)
		if insertErr != nil {
			c.logger.Error("Failed to insert product", "asin", asin, "error", insertErr)
			return nil
		}
		c.logger.Info("Created new product", "asin", asin, "title", productPayload.Title)
		status = "pending"
	}

	if status != "pending" {
		c.logger.Info("Skipping non-pending product", "asin", asin, "status", status)
		return nil
	}

	// Extract size data
	dimensions, err := c.extractSizeData(ctx, asin)
	if err != nil {
		return fmt.Errorf("failed to extract size data: %w", err)
	}

	// Update database based on dimensions
	if err := c.updateProduct(ctx, asin, dimensions); err != nil {
		return fmt.Errorf("failed to update product: %w", err)
	}

	// Check if any size has length measurement
	hasLength := false
	if dimensions.SizeTable != nil {
		for _, measurements := range dimensions.SizeTable.Measurements {
			if length, ok := measurements["length"]; ok && length > 0 {
				hasLength = true
				break
			}
		}
	}

	// Publish PRODUCT_CREATED if has length
	if hasLength {
		if err := c.publishProductCreated(ctx, asin, dimensions); err != nil {
			c.logger.Error("Failed to publish PRODUCT_CREATED", "asin", asin, "error", err)
		}
	}

	return nil
}

// SizeChartResponse represents the API response
type SizeChartResponse struct {
	SizeChartFound bool           `json:"size_chart_found"`
	SizeTable      *SizeTableData `json:"size_table,omitempty"`
}

// SizeTableData represents the complete size table
type SizeTableData struct {
	Sizes        []string                       `json:"sizes"`
	Measurements map[string]map[string]float64  `json:"measurements"`
	Unit         string                        `json:"unit"`
}

func (c *Consumer) extractSizeData(ctx context.Context, asin string) (*SizeChartResponse, error) {
	url := fmt.Sprintf("%s/api/v1/scraper/size-chart", c.scraperURL)
	
	reqBody := map[string]string{"asin": asin}
	jsonData, _ := json.Marshal(reqBody)
	
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	
	// Retry logic
	var resp *http.Response
	for attempts := 0; attempts < 3; attempts++ {
		resp, err = c.httpClient.Do(req)
		if err == nil && resp.StatusCode == 200 {
			break
		}
		if resp != nil {
			resp.Body.Close()
		}
		if attempts < 2 {
			time.Sleep(time.Duration(attempts+1) * time.Second)
		}
	}
	
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}
	
	var dimensions SizeChartResponse
	if err := json.NewDecoder(resp.Body).Decode(&dimensions); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	
	c.logger.Info("Extracted dimensions", 
		"asin", asin,
		"found", dimensions.SizeChartFound,
		"hasSizeTable", dimensions.SizeTable != nil,
		"sizeCount", func() int {
			if dimensions.SizeTable != nil {
				return len(dimensions.SizeTable.Sizes)
			}
			return 0
		}(),
	)
	
	return &dimensions, nil
}

func (c *Consumer) updateProduct(ctx context.Context, asin string, dimensions *SizeChartResponse) error {
	var status string
	hasLength := false
	
	// Check if any size has length measurement
	if dimensions.SizeTable != nil {
		for _, measurements := range dimensions.SizeTable.Measurements {
			if length, ok := measurements["length"]; ok && length > 0 {
				hasLength = true
				break
			}
		}
	}
	
	if hasLength {
		status = "active"
	} else {
		status = "rejected"
	}
	
	// Convert SizeTableData to database.SizeTable if available
	var sizeTableJSON []byte
	if dimensions.SizeTable != nil {
		sizeTable := map[string]interface{}{
			"sizes":        dimensions.SizeTable.Sizes,
			"measurements": dimensions.SizeTable.Measurements,
			"unit":         dimensions.SizeTable.Unit,
		}
		var err error
		sizeTableJSON, err = json.Marshal(sizeTable)
		if err != nil {
			return fmt.Errorf("failed to marshal size table: %w", err)
		}
	}
	
	query := `
		UPDATE products 
		SET size_table = $2,
		    status = $3,
		    scraped_at = CURRENT_TIMESTAMP,
		    updated_at = CURRENT_TIMESTAMP
		WHERE asin = $1`
	
	_, err := c.db.Exec(ctx, query, asin, sizeTableJSON, status)
	if err != nil {
		return fmt.Errorf("failed to update product: %w", err)
	}
	
	c.logger.Info("Updated product", "asin", asin, "status", status, "hasSizeTable", dimensions.SizeTable != nil, "hasLength", hasLength)
	return nil
}

func (c *Consumer) publishProductCreated(ctx context.Context, asin string, dimensions *SizeChartResponse) error {
	// Get product details from database
	var title, url string
	var brand *string // Allow NULL
	err := c.db.QueryRow(ctx, 
		"SELECT title, brand, url FROM products WHERE asin = $1", 
		asin,
	).Scan(&title, &brand, &url)
	if err != nil {
		return fmt.Errorf("failed to get product details: %w", err)
	}
	
	// Create event payload
	eventPayload := map[string]interface{}{
		"event_id":    fmt.Sprintf("%d", time.Now().UnixNano()),
		"event_type":  "PRODUCT_CREATED",
		"timestamp":   time.Now().Format(time.RFC3339),
		"asin":        asin,
		"title":       title,
		"url":         url,
		"quality_score": 3.0, // Simple score if has length
	}
	
	// Add brand if not NULL
	if brand != nil {
		eventPayload["brand"] = *brand
	}
	
	// Add size table if available
	if dimensions.SizeTable != nil {
		eventPayload["size_table"] = map[string]interface{}{
			"sizes":        dimensions.SizeTable.Sizes,
			"measurements": dimensions.SizeTable.Measurements,
			"unit":         dimensions.SizeTable.Unit,
		}
	}
	
	payloadJSON, _ := json.Marshal(eventPayload)
	
	// Publish to Redis stream
	streamKey := "stream:product_lifecycle"
	err = c.redis.XAdd(ctx, &redis.XAddArgs{
		Stream: streamKey,
		Values: map[string]interface{}{
			"event_type": "PRODUCT_CREATED",
			"event_id":   eventPayload["event_id"],
			"asin":       asin,
			"payload":    string(payloadJSON),
		},
	}).Err()
	
	if err != nil {
		return fmt.Errorf("failed to publish event: %w", err)
	}
	
	c.logger.Info("Published PRODUCT_CREATED", "asin", asin)
	return nil
}