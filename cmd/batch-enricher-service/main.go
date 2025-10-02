package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	_ "github.com/lib/pq"
	"github.com/maltedev/amazon-size-scraper/internal/redis"
	goredis "github.com/redis/go-redis/v9"
)

type Config struct {
	DatabaseURL       string
	CheckInterval     time.Duration
	EnrichmentEnabled bool
	BatchSize         int
	DryRun            bool
	StatsOnly         bool
	LogLevel          string
	LogFormat         string

	// PA-API credentials
	AccessKey  string
	SecretKey  string
	PartnerTag string
	Region     string

	// Redis configuration
	RedisHost     string
	RedisPort     string
	RedisPassword string
	RedisDB       int
	StreamName    string
}

func loadConfig() *Config {
	// Build DATABASE_URL from individual components if not provided
	databaseURL := getEnv("DATABASE_URL", "")
	if databaseURL == "" {
		// Construct from individual environment variables
		host := getEnv("DATABASE_HOST", getEnv("DB_HOST", "postgres"))
		port := getEnv("DATABASE_PORT", getEnv("DB_PORT", "5432"))
		user := getEnv("DATABASE_USER", getEnv("DB_USER", "postgres"))
		password := getEnv("DATABASE_PASSWORD", getEnv("DB_PASSWORD", "postgres"))
		dbname := getEnv("DATABASE_NAME", getEnv("DB_NAME", "tall_affiliate"))
		sslMode := getEnv("DATABASE_SSL_MODE", getEnv("DB_SSL_MODE", "disable"))

		databaseURL = fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
			user, password, host, port, dbname, sslMode)
	} else {
		// Ensure sslmode is in the URL if not present
		if !strings.Contains(databaseURL, "sslmode=") {
			if strings.Contains(databaseURL, "?") {
				databaseURL += "&sslmode=disable"
			} else {
				databaseURL += "?sslmode=disable"
			}
		}
	}

	config := &Config{
		DatabaseURL:       databaseURL,
		CheckInterval:     parseDuration(getEnv("ENRICHMENT_CHECK_INTERVAL", "3m"), 3*time.Minute),
		EnrichmentEnabled: parseBool(getEnv("ENRICHMENT_ENABLED", "true")),
		BatchSize:         parseInt(getEnv("ENRICHMENT_BATCH_SIZE", "10"), 10),
		DryRun:            parseBool(getEnv("ENRICHMENT_DRY_RUN", "false")),
		StatsOnly:         parseBool(getEnv("ENRICHMENT_STATS_ONLY", "false")),
		LogLevel:          getEnv("LOG_LEVEL", "info"),
		LogFormat:         getEnv("LOG_FORMAT", "json"),

		// PA-API
		AccessKey:  getEnv("AMAZON_ACCESS_KEY", ""),
		SecretKey:  getEnv("AMAZON_SECRET_KEY", ""),
		PartnerTag: getEnv("AMAZON_PARTNER_TAG", ""),
		Region:     getEnv("AMAZON_REGION", "de"),

		// Redis
		RedisHost:     getEnv("REDIS_HOST", "redis"),
		RedisPort:     getEnv("REDIS_PORT", "6379"),
		RedisPassword: getEnv("REDIS_PASSWORD", ""),
		RedisDB:       parseInt(getEnv("REDIS_DB", "0"), 0),
		StreamName:    getEnv("REDIS_STREAM_NAME", "stream:product_lifecycle"),
	}

	// Parse command line flags (override env vars)
	flag.DurationVar(&config.CheckInterval, "interval", config.CheckInterval, "Check interval for new products")
	flag.BoolVar(&config.DryRun, "dry-run", config.DryRun, "Dry run mode (no database updates)")
	flag.BoolVar(&config.StatsOnly, "stats", config.StatsOnly, "Show statistics only")
	flag.Parse()

	return config
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func parseDuration(s string, defaultValue time.Duration) time.Duration {
	if d, err := time.ParseDuration(s); err == nil {
		return d
	}
	return defaultValue
}

func parseInt(s string, defaultValue int) int {
	if i, err := strconv.Atoi(s); err == nil {
		return i
	}
	return defaultValue
}

func parseBool(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	return s == "true" || s == "1" || s == "yes" || s == "on"
}

func setupLogger(level, format string) *slog.Logger {
	var logLevel slog.Level
	switch strings.ToLower(level) {
	case "debug":
		logLevel = slog.LevelDebug
	case "info":
		logLevel = slog.LevelInfo
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{
		Level: logLevel,
	}

	var handler slog.Handler
	if format == "text" {
		handler = slog.NewTextHandler(os.Stdout, opts)
	} else {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	}

	return slog.New(handler)
}

type EnrichmentStats struct {
	TotalProducts             int
	ProductsWithoutColors     int
	ProductsWithScraperColors int
	ProductsNeedingEnrichment int
}

func getEnrichmentStats(db *sql.DB) (*EnrichmentStats, error) {
	stats := &EnrichmentStats{}

	// Total products
	err := db.QueryRow(`
		SELECT COUNT(*) FROM product WHERE status != 'error'
	`).Scan(&stats.TotalProducts)
	if err != nil {
		return nil, fmt.Errorf("failed to get total products: %w", err)
	}

	// Products without colors
	err = db.QueryRow(`
		SELECT COUNT(*) FROM product
		WHERE status != 'error'
		AND (color_variations IS NULL
			OR color_variations::text = '[]'
			OR (jsonb_typeof(color_variations) = 'array' AND jsonb_array_length(color_variations) = 0))
	`).Scan(&stats.ProductsWithoutColors)
	if err != nil {
		return nil, fmt.Errorf("failed to get products without colors: %w", err)
	}

	// Products with scraper colors
	err = db.QueryRow(`
		SELECT COUNT(*) FROM product
		WHERE status != 'error'
		AND enrichment_source = 'scraper'
		AND color_variations IS NOT NULL
		AND jsonb_typeof(color_variations) = 'array'
		AND jsonb_array_length(color_variations) > 0
	`).Scan(&stats.ProductsWithScraperColors)
	if err != nil {
		return nil, fmt.Errorf("failed to get products with scraper colors: %w", err)
	}

	stats.ProductsNeedingEnrichment = stats.ProductsWithoutColors + stats.ProductsWithScraperColors

	return stats, nil
}

func runEnrichmentCycle(ctx context.Context, config *Config, logger *slog.Logger, db *sql.DB, producer *redis.StreamProducer) error {
	// Get statistics
	stats, err := getEnrichmentStats(db)
	if err != nil {
		return fmt.Errorf("failed to get stats: %w", err)
	}

	logger.Info("enrichment cycle starting",
		"total_products", stats.TotalProducts,
		"without_colors", stats.ProductsWithoutColors,
		"scraper_colors", stats.ProductsWithScraperColors,
		"need_enrichment", stats.ProductsNeedingEnrichment)

	if config.StatsOnly {
		return nil
	}

	if stats.ProductsNeedingEnrichment == 0 {
		logger.Info("no products need enrichment")
		return nil
	}

	// Check if PA-API credentials are configured
	if config.AccessKey == "" || config.SecretKey == "" {
		if !config.DryRun {
			logger.Info("PA-API credentials not configured, running in dry-run mode")
			config.DryRun = true
		}
	}

	// Get products that need enrichment
	query := `
		SELECT asin FROM product
		WHERE status != 'error'
		AND (
			(color_variations IS NULL
				OR color_variations::text = '[]'
				OR (jsonb_typeof(color_variations) = 'array' AND jsonb_array_length(color_variations) = 0))
			OR enrichment_source = 'scraper'
		)
		LIMIT $1
	`

	rows, err := db.QueryContext(ctx, query, config.BatchSize)
	if err != nil {
		return fmt.Errorf("failed to query products: %w", err)
	}
	defer rows.Close()

	var asins []string
	for rows.Next() {
		var asin string
		if err := rows.Scan(&asin); err != nil {
			logger.Error("failed to scan asin", "error", err)
			continue
		}
		asins = append(asins, asin)
	}

	if len(asins) == 0 {
		logger.Info("no products found for enrichment")
		return nil
	}

	logger.Info("publishing enrichment requests",
		"count", len(asins),
		"dry_run", config.DryRun)

	if config.DryRun {
		logger.Info("dry run mode - would publish events for ASINs",
			"asins", asins)
		return nil
	}

	// Publish enrichment requests to Redis stream
	messageIDs, err := producer.ProduceBatchEnrichmentRequests(ctx, asins, config.Region, "normal")
	if err != nil {
		return fmt.Errorf("failed to publish enrichment requests: %w", err)
	}

	logger.Info("published enrichment requests",
		"count", len(messageIDs),
		"stream", config.StreamName)

	return nil
}

func main() {
	config := loadConfig()
	logger := setupLogger(config.LogLevel, config.LogFormat)

	logger.Info("batch enricher service starting",
		"interval", config.CheckInterval,
		"batch_size", config.BatchSize,
		"dry_run", config.DryRun,
		"stats_only", config.StatsOnly,
		"enrichment_enabled", config.EnrichmentEnabled)

	if !config.EnrichmentEnabled {
		logger.Info("enrichment is disabled via ENRICHMENT_ENABLED=false")
		os.Exit(0)
	}

	// Connect to database
	db, err := sql.Open("postgres", config.DatabaseURL)
	if err != nil {
		logger.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	// Test database connection
	if err := db.Ping(); err != nil {
		logger.Error("failed to ping database", "error", err)
		os.Exit(1)
	}

	logger.Info("connected to database")

	// Connect to Redis
	redisAddr := fmt.Sprintf("%s:%s", config.RedisHost, config.RedisPort)
	redisClient := goredis.NewClient(&goredis.Options{
		Addr:     redisAddr,
		Password: config.RedisPassword,
		DB:       config.RedisDB,
	})

	// Test Redis connection
	if err := redisClient.Ping(context.Background()).Err(); err != nil {
		logger.Error("failed to connect to Redis", "error", err)
		os.Exit(1)
	}

	logger.Info("connected to Redis", "addr", redisAddr, "stream", config.StreamName)

	// Create stream producer
	producer := redis.NewStreamProducer(redisClient, config.StreamName)

	// Create consumer group if it doesn't exist
	if err := producer.CreateConsumerGroup(context.Background(), "pa_api_enrichment"); err != nil {
		logger.Warn("consumer group creation", "error", err)
	}

	// Setup signal handling
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Create ticker for periodic checks
	ticker := time.NewTicker(config.CheckInterval)
	defer ticker.Stop()

	// Run initial check immediately
	if err := runEnrichmentCycle(ctx, config, logger, db, producer); err != nil {
		logger.Error("enrichment cycle failed", "error", err)
	}

	// Main loop
	for {
		select {
		case <-ctx.Done():
			logger.Info("shutting down")
			return
		case <-sigChan:
			logger.Info("received shutdown signal")
			cancel()
		case <-ticker.C:
			if err := runEnrichmentCycle(ctx, config, logger, db, producer); err != nil {
				logger.Error("enrichment cycle failed", "error", err)
			}
		}
	}
}
