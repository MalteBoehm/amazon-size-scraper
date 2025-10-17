package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/redis/go-redis/v9"
	"github.com/maltedev/amazon-size-scraper/internal/amazon-scraper/api"
	"github.com/maltedev/amazon-size-scraper/internal/amazon-scraper/config"
	"github.com/maltedev/amazon-size-scraper/internal/amazon-scraper/events"
	"github.com/maltedev/amazon-size-scraper/internal/amazon-scraper/jobs"
	"github.com/maltedev/amazon-size-scraper/internal/amazon-scraper/scraper"
	"github.com/maltedev/amazon-size-scraper/internal/browser"
	"github.com/maltedev/amazon-size-scraper/internal/database"
)

func main() {
	// Setup logging
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		logger.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	// Setup context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Database connection
	db, err := database.New(ctx, database.Config{
		Host:     cfg.Database.Host,
		Port:     cfg.Database.Port,
		User:     cfg.Database.User,
		Password: cfg.Database.Password,
		Database: cfg.Database.Name,
		MaxConns: cfg.Database.MaxConns,
	})
	if err != nil {
		logger.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	// Browser setup
	b, err := browser.New(&browser.Options{
		Headless: cfg.Scraper.Headless,
		Timeout:  time.Duration(cfg.Scraper.TimeoutSeconds) * time.Second,
	})
	if err != nil {
		logger.Error("failed to initialize browser", "error", err)
		os.Exit(1)
	}
	defer b.Close()

	// Initialize event publisher with database (for transactional outbox)
	publisher := events.NewPublisher(db, logger)

	// Initialize Redis client for Relay
	redisClient := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})
	defer redisClient.Close()

	// Test Redis connection
	if err := redisClient.Ping(ctx).Err(); err != nil {
		logger.Error("failed to connect to Redis", "error", err)
		os.Exit(1)
	}

	// Initialize and start Relay for outbox processing
	relay := database.NewRelay(db, redisClient, logger, database.RelayConfig{
		PollInterval: 5 * time.Second,
		BatchSize:    100,
	})
	go func() {
		if err := relay.Start(ctx); err != nil && err != context.Canceled {
			logger.Error("relay stopped with error", "error", err)
		}
	}()

	// Initialize services
	scraperService := scraper.NewService(b, db, logger)
	jobManager := jobs.NewManager(db, scraperService, publisher, logger)
	
	// Start job worker
	go jobManager.StartWorker(ctx)

	// Initialize API handlers
	handlers := api.NewHandlers(scraperService, jobManager, logger)

	// Setup Chi router
	r := chi.NewRouter()
	
	// Middleware
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))
	
	// CORS
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"http://localhost:*", "https://localhost:*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// Health check
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		
		// Check outbox status
		pendingCount, _ := relay.GetPendingCount(context.Background())
		deadLetterCount, _ := relay.GetDeadLetterCount(context.Background())
		
		health := map[string]interface{}{
			"status": "ok",
			"outbox": map[string]interface{}{
				"pending": pendingCount,
				"dead_letter": deadLetterCount,
			},
		}
		
		status := http.StatusOK
		if pendingCount > 1000 {
			health["status"] = "warning"
			health["message"] = "High number of pending outbox events"
		}
		if deadLetterCount > 100 {
			health["status"] = "error"
			health["message"] = "High number of dead letter events"
			status = http.StatusServiceUnavailable
		}
		
		w.WriteHeader(status)
		json.NewEncoder(w).Encode(health)
	})

	// API Routes
	r.Route("/api/v1", func(r chi.Router) {
		// Scraper endpoints (Oxylabs replacement)
		r.Route("/scraper", func(r chi.Router) {
			// Size chart endpoint - replaces Oxylabs size chart API
			r.Post("/size-chart", handlers.GetSizeChart)
			
			// Reviews endpoint - replaces Oxylabs reviews API
			r.Post("/reviews", handlers.GetReviews)
			
			// Job management endpoints
			r.Post("/jobs", handlers.CreateJob)
			r.Get("/jobs/{jobID}", handlers.GetJob)
			r.Get("/jobs", handlers.ListJobs)
			r.Get("/jobs/{jobID}/products", handlers.GetJobProducts)
		})
		
		// Stats endpoint
		r.Get("/stats", handlers.GetStats)
	})

	// Start server
	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Server.Port),
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
		<-sigChan

		logger.Info("shutting down server...")
		cancel()

		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer shutdownCancel()

		if err := server.Shutdown(shutdownCtx); err != nil {
			logger.Error("server shutdown failed", "error", err)
		}
	}()

	logger.Info("server starting", "port", cfg.Server.Port)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Error("server failed", "error", err)
		os.Exit(1)
	}

	logger.Info("server stopped")
}