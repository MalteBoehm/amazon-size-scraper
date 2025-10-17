package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	Redis    RedisConfig
	Scraper  ScraperConfig
}

type ServerConfig struct {
	Port int
}

type DatabaseConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	Name     string
	MaxConns int32
}

type RedisConfig struct {
	Addr     string
	Password string
	DB       int
}

type ScraperConfig struct {
	Headless           bool
	TimeoutSeconds     int
	ConcurrentWorkers  int
	RateLimitSeconds   int
	MaxRetries         int
}

func Load() (*Config, error) {
	cfg := &Config{
		Server: ServerConfig{
			Port: getEnvInt("PORT", 8084),
		},
		Database: DatabaseConfig{
			Host:     getEnv("DB_HOST", "localhost"),
			Port:     getEnvInt("DB_PORT", 5432),
			User:     getEnv("DB_USER", "postgres"),
			Password: getEnv("DB_PASSWORD", ""),
			Name:     getEnv("DB_NAME", "tall_affiliate"),
			MaxConns: int32(getEnvInt("DB_MAX_CONNS", 20)),
		},
		Redis: RedisConfig{
			Addr:     getEnv("REDIS_ADDR", "localhost:6379"),
			Password: getEnv("REDIS_PASSWORD", ""),
			DB:       getEnvInt("REDIS_DB", 0),
		},
		Scraper: ScraperConfig{
			Headless:          getEnvBool("SCRAPER_HEADLESS", true),
			TimeoutSeconds:    getEnvInt("SCRAPER_TIMEOUT", 30),
			ConcurrentWorkers: getEnvInt("SCRAPER_WORKERS", 2),
			RateLimitSeconds:  getEnvInt("SCRAPER_RATE_LIMIT", 3),
			MaxRetries:        getEnvInt("SCRAPER_MAX_RETRIES", 3),
		},
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func (c *Config) Validate() error {
	if c.Server.Port <= 0 || c.Server.Port > 65535 {
		return fmt.Errorf("invalid server port: %d", c.Server.Port)
	}

	if c.Database.Host == "" {
		return fmt.Errorf("database host is required")
	}

	if c.Database.Name == "" {
		return fmt.Errorf("database name is required")
	}

	if c.Scraper.ConcurrentWorkers < 1 {
		return fmt.Errorf("at least 1 concurrent worker is required")
	}

	return nil
}

func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value, exists := os.LookupEnv(key); exists {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value, exists := os.LookupEnv(key); exists {
		if boolVal, err := strconv.ParseBool(value); err == nil {
			return boolVal
		}
	}
	return defaultValue
}