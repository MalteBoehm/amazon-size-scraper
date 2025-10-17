package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Server   ServerConfig
	Scraper  ScraperConfig
	Browser  BrowserConfig
	Database DatabaseConfig
	Queue    QueueConfig
	Logging  LoggingConfig
}

type ServerConfig struct {
	Port            string
	Host            string
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	ShutdownTimeout time.Duration
}

type ScraperConfig struct {
	RateLimitMin    time.Duration
	RateLimitMax    time.Duration
	MaxRetries      int
	RetryDelay      time.Duration
	ConcurrentLimit int
	UserAgents      []string
	Proxies         []string
}

type BrowserConfig struct {
	Headless       bool
	Timeout        time.Duration
	ViewportWidth  int
	ViewportHeight int
	AcceptLanguage string
	TimezoneID     string
	Locale         string
}

type DatabaseConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	DBName   string
	SSLMode  string
}

type QueueConfig struct {
	Type      string
	BatchSize int
	MaxSize   int
}

type LoggingConfig struct {
	Level  string
	Format string
}

func Load() (*Config, error) {
	cfg := &Config{
		Server: ServerConfig{
			Port:            getEnvOrDefault("SERVER_PORT", "8080"),
			Host:            getEnvOrDefault("SERVER_HOST", "0.0.0.0"),
			ReadTimeout:     getDurationOrDefault("SERVER_READ_TIMEOUT", 30*time.Second),
			WriteTimeout:    getDurationOrDefault("SERVER_WRITE_TIMEOUT", 30*time.Second),
			ShutdownTimeout: getDurationOrDefault("SERVER_SHUTDOWN_TIMEOUT", 10*time.Second),
		},
		Scraper: ScraperConfig{
			RateLimitMin:    getDurationOrDefault("SCRAPER_RATE_LIMIT_MIN", 5*time.Second),
			RateLimitMax:    getDurationOrDefault("SCRAPER_RATE_LIMIT_MAX", 30*time.Second),
			MaxRetries:      getIntOrDefault("SCRAPER_MAX_RETRIES", 3),
			RetryDelay:      getDurationOrDefault("SCRAPER_RETRY_DELAY", 5*time.Second),
			ConcurrentLimit: getIntOrDefault("SCRAPER_CONCURRENT_LIMIT", 5),
			UserAgents:      getStringSliceOrDefault("SCRAPER_USER_AGENTS", defaultUserAgents()),
			Proxies:         getStringSliceOrDefault("SCRAPER_PROXIES", []string{}),
		},
		Browser: BrowserConfig{
			Headless:       getBoolOrDefault("BROWSER_HEADLESS", true),
			Timeout:        getDurationOrDefault("BROWSER_TIMEOUT", 30*time.Second),
			ViewportWidth:  getIntOrDefault("BROWSER_VIEWPORT_WIDTH", 1920),
			ViewportHeight: getIntOrDefault("BROWSER_VIEWPORT_HEIGHT", 1080),
			AcceptLanguage: getEnvOrDefault("BROWSER_ACCEPT_LANGUAGE", "de-DE,de;q=0.9,en;q=0.8"),
			TimezoneID:     getEnvOrDefault("BROWSER_TIMEZONE", "Europe/Berlin"),
			Locale:         getEnvOrDefault("BROWSER_LOCALE", "de-DE"),
		},
		Database: DatabaseConfig{
			Host:     getEnvOrDefault("DB_HOST", "localhost"),
			Port:     getIntOrDefault("DB_PORT", 5432),
			User:     getEnvOrDefault("DB_USER", "postgres"),
			Password: getEnvOrDefault("DB_PASSWORD", ""),
			DBName:   getEnvOrDefault("DB_NAME", "amazon_scraper"),
			SSLMode:  getEnvOrDefault("DB_SSL_MODE", "disable"),
		},
		Queue: QueueConfig{
			Type:      getEnvOrDefault("QUEUE_TYPE", "memory"),
			BatchSize: getIntOrDefault("QUEUE_BATCH_SIZE", 10),
			MaxSize:   getIntOrDefault("QUEUE_MAX_SIZE", 1000),
		},
		Logging: LoggingConfig{
			Level:  getEnvOrDefault("LOG_LEVEL", "info"),
			Format: getEnvOrDefault("LOG_FORMAT", "json"),
		},
	}
	
	return cfg, nil
}

func (c *Config) Validate() error {
	if c.Scraper.ConcurrentLimit < 1 {
		return fmt.Errorf("SCRAPER_CONCURRENT_LIMIT must be at least 1")
	}
	
	if c.Scraper.RateLimitMin > c.Scraper.RateLimitMax {
		return fmt.Errorf("SCRAPER_RATE_LIMIT_MIN cannot be greater than SCRAPER_RATE_LIMIT_MAX")
	}
	
	if c.Queue.BatchSize < 1 {
		return fmt.Errorf("QUEUE_BATCH_SIZE must be at least 1")
	}
	
	return nil
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getIntOrDefault(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if i, err := strconv.Atoi(value); err == nil {
			return i
		}
	}
	return defaultValue
}

func getBoolOrDefault(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if b, err := strconv.ParseBool(value); err == nil {
			return b
		}
	}
	return defaultValue
}

func getDurationOrDefault(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if d, err := time.ParseDuration(value); err == nil {
			return d
		}
	}
	return defaultValue
}

func getStringSliceOrDefault(key string, defaultValue []string) []string {
	if value := os.Getenv(key); value != "" {
		return strings.Split(value, ",")
	}
	return defaultValue
}

func defaultUserAgents() []string {
	return []string{
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Safari/537.36",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
	}
}