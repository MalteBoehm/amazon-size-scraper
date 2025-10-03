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
	"strconv"
	"syscall"
	"time"

	"github.com/MalteBoehm/tall-affiliate-common/pkg/constants"
	"github.com/MalteBoehm/tall-affiliate-common/pkg/events"
	"github.com/maltedev/amazon-size-scraper/internal/database"
	"github.com/redis/go-redis/v9"
)

func main() {
	// Setup logger
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	// Redis connection - use individual host and port like the working services
	redisHost := os.Getenv("REDIS_HOST")
	redisPort := os.Getenv("REDIS_PORT")
	if redisHost == "" {
		redisHost = "localhost"
	}
	if redisPort == "" {
		redisPort = "6379"
	}

	redisAddr := fmt.Sprintf("%s:%s", redisHost, redisPort)
	redisPassword := os.Getenv("REDIS_PASSWORD")
	redisUsername := os.Getenv("REDIS_USERNAME")

	// Create Redis options
	options := &redis.Options{
		Addr:     redisAddr,
		Password: redisPassword,
	}

	// Only set username if it's not empty (Redis without username)
	if redisUsername != "" {
		options.Username = redisUsername
	}

	rdb := redis.NewClient(options)

	// Test Redis connection
	ctx := context.Background()
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	logger.Info("Connected to Redis", "addr", redisAddr)

	// Database connection - use same approach as size-scraper
	dbHost := getEnv("DB_HOST", "localhost")
	dbPortStr := getEnv("DB_PORT", "5432")
	dbUser := getEnv("DB_USER", "postgres")
	dbPassword := getEnv("DB_PASSWORD", "postgres")
	dbName := getEnv("DB_NAME", "amazon_scraper") // Default to amazon_scraper

	// Handle database name inconsistencies across the system
	// The actual database on the server is named "tall-affiliate" (with hyphen)
	// but different parts of the code use different defaults
	if dbName == "amazon_scraper" || dbName == "tall_affiliate" {
		dbName = "tall-affiliate" // Use the actual database name that exists
		logger.Info("Database name normalized to: tall-affiliate (actual database on server)")
	}

	// Convert port to int
	dbPort := 5432
	if port, err := strconv.Atoi(dbPortStr); err == nil {
		dbPort = port
	}

	// Log the final database name for debugging
	logger.Info("Final database configuration",
		"original_name", getEnv("DB_NAME", "amazon_scraper"),
		"final_name", dbName,
		"host", dbHost,
		"port", dbPort,
	)

	// DEBUG: Log all database configuration values
	logger.Info("DATABASE CONFIGURATION DEBUG",
		"DB_HOST", dbHost,
		"DB_PORT", dbPort,
		"DB_USER", dbUser,
		"DB_PASSWORD", "***MASKED***",
		"DB_NAME", dbName,
		"DB_SSL_MODE", getEnv("DB_SSL_MODE", "disable"),
	)

	// Use database.Config like size-scraper
	dbConfig := database.Config{
		Host:        dbHost,
		Port:        dbPort,
		User:        dbUser,
		Password:    dbPassword,
		Database:    dbName,
		SSLMode:     getEnv("DB_SSL_MODE", "disable"),
		MaxConns:    10,
		MinConns:    1,
		MaxConnLife: 5 * time.Minute,
		MaxConnIdle: 1 * time.Minute,
	}

	// Create database connection using same method as size-scraper
	db, err := database.New(ctx, dbConfig)
	if err != nil {
		logger.Error("FAILED TO CREATE DATABASE CONNECTION", "error", err)
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	logger.Info("Connected to database", "database", dbName)

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
	db         *database.DB
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
	streamKey := getEnv("REDIS_STREAM", constants.StreamProductLifecycle)
	consumerGroup := "lifecycle-consumer-group"
	consumerName := "consumer-1"

	// DEBUG: Log actual stream key and check if stream exists
	c.logger.Info("DEBUG: Consumer configuration",
		"env_stream_key", getEnv("REDIS_STREAM", "NOT_SET"),
		"constants_stream", constants.StreamProductLifecycle,
		"final_stream_key", streamKey)

	// Check if stream exists
	streamInfo, err := c.redis.XInfoStream(ctx, streamKey).Result()
	if err != nil {
		c.logger.Error("DEBUG: Stream does not exist", "stream", streamKey, "error", err)
		// Try alternative stream names
		alternativeStreams := []string{"stream:product_lifecycle", "product-lifecycle-stream"}
		for _, altStream := range alternativeStreams {
			if altInfo, altErr := c.redis.XInfoStream(ctx, altStream).Result(); altErr == nil {
				c.logger.Info("DEBUG: Found alternative stream", "stream", altStream, "length", altInfo.Length)
				streamKey = altStream
				break
			}
		}
	} else {
		c.logger.Info("DEBUG: Stream exists", "stream", streamKey, "length", streamInfo.Length)
	}

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
	// DEBUG: Log all incoming message details
	c.logger.Info("DEBUG: Processing message",
		"message_id", msg.ID,
		"keys", func() []string {
			keys := make([]string, 0, len(msg.Values))
			for k := range msg.Values {
				keys = append(keys, k)
			}
			return keys
		}(),
		"event_type", msg.Values["event_type"],
		"type", msg.Values["type"])

	// Skip initialization messages
	if initVal, ok := msg.Values["init"].(string); ok && initVal == "true" {
		c.logger.Info("skipping initialization message", "message_id", msg.ID)
		return nil
	}

	// Check event type - try both "event_type" and "type" fields
	eventType, hasEventType := msg.Values["event_type"].(string)
	typeField, hasTypeField := msg.Values["type"].(string)

	// DEBUG: Log event type detection
	c.logger.Info("DEBUG: Event type detection",
		"has_event_type", hasEventType,
		"event_type", eventType,
		"has_type_field", hasTypeField,
		"type_field", typeField,
		"expected_event_1", events.Event_01_ProductDetected,
		"expected_event_2", "NEW_PRODUCT_DETECTED")

	// Try both event type fields
	actualEventType := ""
	if hasEventType {
		actualEventType = eventType
	} else if hasTypeField {
		actualEventType = typeField
	}

	// Accept both official events and catalog.product_enrichment_requested.v1
	isValidEventType := actualEventType == events.Event_01_ProductDetected ||
		actualEventType == events.Event_02A_ProductValidated ||
		actualEventType == "catalog.product_enrichment_requested.v1"

	if actualEventType == "" || !isValidEventType {
		if actualEventType != "" {
			c.logger.Debug("Skipping non-matching event", "event_type", actualEventType, "message_id", msg.ID)
		} else {
			c.logger.Debug("Skipping message with no event type", "message_id", msg.ID)
		}
		return nil // Skip non-matching events
	}

	c.logger.Info("Processing valid event", "event_type", actualEventType, "message_id", msg.ID)

	// Get payload - try both "payload" and "data" fields
	var payloadStr string
	var ok bool

	// Try "payload" field first
	if payloadStr, ok = msg.Values["payload"].(string); !ok {
		// Try "data" field for catalog.product_enrichment_requested.v1 events
		if dataStr, dataOk := msg.Values["data"].(string); dataOk {
			payloadStr = dataStr
			ok = true
		}
	}

	if !ok {
		return fmt.Errorf("missing payload in event")
	}

	// Parse payload
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(payloadStr), &payload); err != nil {
		return fmt.Errorf("failed to parse payload: %w", err)
	}

	// For catalog.product_enrichment_requested.v1 events, extract ASIN from nested data
	var asin string
	if actualEventType == "catalog.product_enrichment_requested.v1" {
		// Try to get ASIN from nested structure
		if data, exists := payload["data"].(map[string]interface{}); exists {
			if nestedAsin, ok := data["asin"].(string); ok {
				asin = nestedAsin
			}
		}
		// Fallback to direct ASIN field
		if asin == "" {
			if nestedAsin, ok := payload["asin"].(string); ok {
				asin = nestedAsin
			}
		}
	} else {
		// For other event types, get ASIN directly
		if nestedAsin, ok := payload["asin"].(string); ok {
			asin = nestedAsin
		}
	}

	if asin == "" {
		return fmt.Errorf("missing ASIN in payload")
	}

	c.logger.Info("Extracted ASIN from payload", "asin", asin, "event_type", actualEventType)

	c.logger.Info("Processing product",
		"message_id", msg.ID,
		"asin", asin,
		"title", payload["title"],
	)

	// DEBUG: Check if product exists and is still pending
	var status string
	err := c.db.QueryRow(ctx, "SELECT status FROM product WHERE asin = $1", asin).Scan(&status)

	// DEBUG: Log product check
	c.logger.Info("DEBUG: Product existence check",
		"asin", asin,
		"query_error", err,
		"found_status", status)

	if err != nil {
		// Product doesn't exist, create it
		title, _ := payload["name"].(string)
		if title == "" {
			title, _ = payload["title"].(string)
		}
		url, _ := payload["detail_page_url"].(string)
		if url == "" {
			url = fmt.Sprintf("https://www.amazon.de/dp/%s", asin)
		}

		// DEBUG: Log product creation attempt
		c.logger.Info("DEBUG: Creating new product",
			"asin", asin,
			"title", title,
			"url", url)

		insertQuery := `INSERT INTO product (asin, title, detail_page_url, status)
		                VALUES ($1, $2, $3, 'pending')
		                ON CONFLICT (asin) DO NOTHING`
		_, insertErr := c.db.Exec(ctx, insertQuery, asin, title, url)
		if insertErr != nil {
			c.logger.Error("Failed to insert product", "asin", asin, "error", insertErr)
			return nil
		}
		c.logger.Info("Created new product", "asin", asin)
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
	// New fields from complete product extraction
	Title           string   `json:"title,omitempty"`
	Brand           string   `json:"brand,omitempty"`
	Category        string   `json:"category,omitempty"`
	ImageURLs       []string `json:"image_urls,omitempty"`
	Features        []string `json:"features,omitempty"`
	CurrentPrice    *float64 `json:"current_price,omitempty"`
	Currency        string   `json:"currency,omitempty"`
	Rating          *float64 `json:"rating,omitempty"`
	ReviewCount     *int     `json:"review_count,omitempty"`
	AvailableSizes  []string `json:"available_sizes,omitempty"`
	IsPrimeEligible bool     `json:"is_prime_eligible"`
	InStock         bool     `json:"in_stock"`
	Colors          []string `json:"colors,omitempty"`
	Material        string   `json:"material,omitempty"`
	Gender          string   `json:"gender,omitempty"`
	ProductGroups   []string `json:"product_groups,omitempty"`
}

// SizeTableData represents the complete size table
type SizeTableData struct {
	Sizes        []string                      `json:"sizes"`
	Measurements map[string]map[string]float64 `json:"measurements"`
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

	// Marshal arrays to JSON
	colorsJSON, _ := json.Marshal(dimensions.Colors)
	productGroupsJSON, _ := json.Marshal(dimensions.ProductGroups)

	query := `
		UPDATE product
		SET size_chart_data = $2,
		    status = $3,
		    title = COALESCE(NULLIF($4, ''), title),
		    brand = COALESCE(NULLIF($5, ''), brand),
		    category = COALESCE(NULLIF($6, ''), category),
		    is_prime_eligible = $7,
		    in_stock = $8,
		    material = COALESCE(NULLIF($9, ''), material),
		    gender = COALESCE(NULLIF($10, ''), gender),
		    colors = CASE WHEN $11::jsonb IS NOT NULL THEN $11::jsonb ELSE colors END,
		    product_groups = CASE WHEN $12::jsonb IS NOT NULL THEN $12::jsonb ELSE product_groups END,
		    updated_at = CURRENT_TIMESTAMP
		WHERE asin = $1`

	_, err := c.db.Exec(ctx, query,
		asin, sizeTableJSON, status,
		dimensions.Title, dimensions.Brand, dimensions.Category,
		dimensions.IsPrimeEligible, dimensions.InStock,
		dimensions.Material, dimensions.Gender,
		colorsJSON, productGroupsJSON,
	)
	if err != nil {
		return fmt.Errorf("failed to update product: %w", err)
	}

	c.logger.Info("Updated product with full data",
		"asin", asin,
		"status", status,
		"hasSizeTable", dimensions.SizeTable != nil,
		"hasLength", hasLength,
		"isPrime", dimensions.IsPrimeEligible,
		"inStock", dimensions.InStock,
		"gender", dimensions.Gender,
	)
	return nil
}

func (c *Consumer) publishProductCreated(ctx context.Context, asin string, dimensions *SizeChartResponse) error {
	// Get product details from database
	var title, url string
	var brand *string // Allow NULL
	err := c.db.QueryRow(ctx,
		"SELECT title, brand, detail_page_url FROM product WHERE asin = $1",
		asin,
	).Scan(&title, &brand, &url)
	if err != nil {
		return fmt.Errorf("failed to get product details: %w", err)
	}

	// Create event payload using new event type convention
	eventPayload := map[string]interface{}{
		"event_id":      fmt.Sprintf("%d", time.Now().UnixNano()),
		"event_type":    events.Event_01_ProductDetected, // Using event constant
		"timestamp":     time.Now().Format(time.RFC3339),
		"asin":          asin,
		"title":         title,
		"url":           url,
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
	streamKey := constants.StreamProductLifecycle
	err = c.redis.XAdd(ctx, &redis.XAddArgs{
		Stream: streamKey,
		Values: map[string]interface{}{
			"event_type": events.Event_02A_ProductValidated,
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
