package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

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
	consumer := &JobConsumer{
		redis:  rdb,
		db:     db,
		logger: logger,
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

type JobConsumer struct {
	redis  *redis.Client
	db     *database.DB
	logger *slog.Logger
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func (c *JobConsumer) Run(ctx context.Context) error {
	streamKey := "stream:scraper_jobs"    // This is where SCRAPER_JOB_REQUESTED events are stored
	consumerGroup := "group:scraper_jobs" // Create appropriate consumer group for scraper jobs
	consumerName := "redis-job-consumer-1"

	// DEBUG: Log actual stream key and check if stream exists
	c.logger.Info("DEBUG: Consumer configuration",
		"stream_key", streamKey,
		"consumer_group", consumerGroup)

	// Check if stream exists
	streamInfo, err := c.redis.XInfoStream(ctx, streamKey).Result()
	if err != nil {
		c.logger.Error("DEBUG: Stream does not exist", "stream", streamKey, "error", err)
		return fmt.Errorf("stream %s does not exist: %w", streamKey, err)
	} else {
		c.logger.Info("DEBUG: Stream exists", "stream", streamKey, "length", streamInfo.Length)
	}

	// Create consumer group (ignore error if already exists)
	c.redis.XGroupCreate(ctx, streamKey, consumerGroup, "0").Err()

	c.logger.Info("Starting job consumer", "stream", streamKey, "group", consumerGroup)

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
				NoAck:    false,
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

func (c *JobConsumer) processMessage(ctx context.Context, msg redis.XMessage) error {
	// DEBUG: Log all incoming message details
	c.logger.Info("DEBUG: Processing job message",
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
		"type_field", typeField)

	// Try both event type fields
	actualEventType := ""
	if hasEventType {
		actualEventType = eventType
	} else if hasTypeField {
		actualEventType = typeField
	}

	// Accept only scraper job requests
	isValidEventType := actualEventType == "SCRAPER_JOB_REQUESTED"

	if actualEventType == "" || !isValidEventType {
		if actualEventType != "" {
			c.logger.Debug("Skipping non-matching event", "event_type", actualEventType, "message_id", msg.ID)
		} else {
			c.logger.Debug("Skipping message with no event type", "message_id", msg.ID)
		}
		return nil // Skip non-matching events
	}

	c.logger.Info("Processing valid job event", "event_type", actualEventType, "message_id", msg.ID)

    // Get payload - accept string, []byte, or map under "payload" or "data"
    var payload map[string]interface{}
    if raw, ok := msg.Values["payload"]; ok {
        switch v := raw.(type) {
        case string:
            if err := json.Unmarshal([]byte(v), &payload); err != nil {
                return fmt.Errorf("failed to parse payload: %w", err)
            }
        case []byte:
            if err := json.Unmarshal(v, &payload); err != nil {
                return fmt.Errorf("failed to parse payload: %w", err)
            }
        case map[string]interface{}:
            payload = v
        default:
            // Unsupported type; continue to try data field
        }
    }
    if payload == nil {
        if raw, ok := msg.Values["data"]; ok {
            switch v := raw.(type) {
            case string:
                if err := json.Unmarshal([]byte(v), &payload); err != nil {
                    return fmt.Errorf("failed to parse data: %w", err)
                }
            case []byte:
                if err := json.Unmarshal(v, &payload); err != nil {
                    return fmt.Errorf("failed to parse data: %w", err)
                }
            case map[string]interface{}:
                payload = v
            }
        }
    }
    if payload == nil {
        return fmt.Errorf("missing payload/data in event")
    }

    // Some producers envelope under { data: {...} }
    if inner, ok := payload["data"].(map[string]interface{}); ok {
        payload = inner
    }
    // Some producers envelope under { payload: {...} } (event wrapper)
    if inner, ok := payload["payload"].(map[string]interface{}); ok {
        payload = inner
    }

    // Extract job details (support snake_case and camelCase)
    jobID, _ := payload["job_id"].(string)
    if jobID == "" {
        jobID, _ = payload["jobId"].(string)
    }
    searchQuery, _ := payload["search_query"].(string)
    if searchQuery == "" {
        if s, ok := payload["searchQuery"].(string); ok {
            searchQuery = s
        } else if s, ok := payload["query"].(string); ok {
            searchQuery = s
        }
    }
    category, _ := payload["category"].(string)
    if category == "" {
        if s, ok := payload["categoryName"].(string); ok {
            category = s
        }
    }
    var maxPages int
    if v, ok := payload["max_pages"].(float64); ok {
        maxPages = int(v)
    } else if v, ok := payload["maxPages"].(float64); ok {
        maxPages = int(v)
    } else if v, ok := payload["max_pages"].(int); ok {
        maxPages = v
    }

    // Final fallback: read fields directly from message if present
    if jobID == "" {
        if v, ok := msg.Values["job_id"].(string); ok { jobID = v }
        if v, ok := msg.Values["jobId"].(string); ok { jobID = v }
    }
    if searchQuery == "" {
        if v, ok := msg.Values["search_query"].(string); ok { searchQuery = v }
        if v, ok := msg.Values["searchQuery"].(string); ok { searchQuery = v }
        if v, ok := msg.Values["query"].(string); ok { searchQuery = v }
    }

	if jobID == "" || searchQuery == "" {
        return fmt.Errorf("missing required job fields: job_id or search_query")
	}

	c.logger.Info("Extracted job from payload",
		"job_id", jobID,
		"search_query", searchQuery,
		"category", category,
		"max_pages", maxPages)

	// Create job in database
	return c.createJobInDatabase(ctx, jobID, searchQuery, category, maxPages)
}

func (c *JobConsumer) createJobInDatabase(ctx context.Context, jobID, searchQuery, category string, maxPages int) error {
	// Check if job already exists
	var existingID string
	checkQuery := `SELECT id FROM scraper_jobs WHERE id = $1`
	err := c.db.QueryRow(ctx, checkQuery, jobID).Scan(&existingID)

	if err == nil {
		c.logger.Info("Job already exists in database", "job_id", jobID)
		return nil
	}

	// Insert new job
	insertQuery := `
		INSERT INTO scraper_jobs (id, search_query, category, max_pages, status, created_at)
		VALUES ($1, $2, $3, $4, 'pending', CURRENT_TIMESTAMP)
		ON CONFLICT (id) DO NOTHING
	`

	_, err = c.db.Exec(ctx, insertQuery, jobID, searchQuery, category, maxPages)
	if err != nil {
		c.logger.Error("Failed to insert job", "job_id", jobID, "error", err)
		return err
	}

	c.logger.Info("Created new job in database",
		"job_id", jobID,
		"search_query", searchQuery,
		"category", category,
		"max_pages", maxPages)

	return nil
}
