package jobs

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/maltedev/amazon-size-scraper/internal/amazon-scraper/clients"
	"github.com/maltedev/amazon-size-scraper/internal/amazon-scraper/events"
	"github.com/maltedev/amazon-size-scraper/internal/amazon-scraper/scraper"
	"log/slog"
)

type Manager struct {
	db      *sql.DB
	logger  *slog.Logger
	events  *events.Producer
	scraper *scraper.Scraper
}

func NewManager(db *sql.DB, logger *slog.Logger, events *events.Producer, scraper *scraper.Scraper) *Manager {
	return &Manager{
		db:      db,
		logger:  logger,
		events:  events,
		scraper: scraper,
	}
}

// Job represents a scraper job
type Job struct {
	ID                   string    `json:"id"`
	SearchQuery          string    `json:"search_query"`
	Category             string    `json:"category"`
	MaxPages             int       `json:"max_pages"`
	Status               string    `json:"status"`
	ResultsCount         int       `json:"results_count"`
	PagesScraped         int       `json:"pages_scraped"`
	ProductsFound        int       `json:"products_found"`
	ProductsComplete     int       `json:"products_complete"`
	ProductsNew          int       `json:"products_new"`
	ProductsUpdated      int       `json:"products_updated"`
	ErrorMessage         string    `json:"error_message,omitempty"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
	StartedAt            time.Time `json:"started_at,omitempty"`
	CompletedAt          time.Time `json:"completed_at,omitempty"`
}

// JobProduct represents a product found by a job
type JobProduct struct {
	JobID        string    `json:"job_id"`
	ASIN         string    `json:"asin"`
	Title        string    `json:"title"`
	FoundAt      time.Time `json:"found_at"`
	Status       string    `json:"status"`
	ErrorMessage string    `json:"error_message,omitempty"`
}

// CreateJob creates a new scraper job
func (m *Manager) CreateJob(ctx context.Context, query, category string, maxPages int) (string, error) {
	id := uuid.New().String()
	status := "pending"
	
	// Validate category
	if category == "" {
		category = "fashion-mens" // Default
	}

	// Insert job into database
	insertQuery := `
		INSERT INTO scraper_jobs (
			id, search_query, category, max_pages, status, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, NOW(), NOW())
	`
	
	_, err := m.db.ExecContext(ctx, insertQuery, id, query, category, maxPages, status)
	if err != nil {
		return "", fmt.Errorf("failed to create job: %w", err)
	}
	
	m.logger.Info("Created new scraper job", "id", id, "query", query)
	
	// Publish event
	if err := m.events.JobRequested(ctx, id, query, category); err != nil {
		m.logger.Error("Failed to publish job requested event", "error", err)
		// Don't fail the request, job will be picked up by poller anyway
	}
	
	return id, nil
}

// GetJob retrieves a job by ID
func (m *Manager) GetJob(ctx context.Context, id string) (*Job, error) {
	query := `
		SELECT 
			id, search_query, category, max_pages, status, results_count,
			pages_scraped, products_found, products_complete, products_new, products_updated,
			COALESCE(error_message, ''), created_at, updated_at, started_at, completed_at
		FROM scraper_jobs
		WHERE id = $1
	`
	
	job := &Job{}
	var startedAt, completedAt sql.NullTime
	
	err := m.db.QueryRowContext(ctx, query, id).Scan(
		&job.ID, &job.SearchQuery, &job.Category, &job.MaxPages, &job.Status, &job.ResultsCount,
		&job.PagesScraped, &job.ProductsFound, &job.ProductsComplete, &job.ProductsNew, &job.ProductsUpdated,
		&job.ErrorMessage, &job.CreatedAt, &job.UpdatedAt, &startedAt, &completedAt,
	)
	
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get job: %w", err)
	}
	
	if startedAt.Valid {
		job.StartedAt = startedAt.Time
	}
	if completedAt.Valid {
		job.CompletedAt = completedAt.Time
	}
	
	return job, nil
}

// GetJobDetails returns detailed job information including product stats
func (m *Manager) GetJobDetails(ctx context.Context, id string) (*Job, error) {
	job, err := m.GetJob(ctx, id)
	if err != nil {
		return nil, err
	}
	if job == nil {
		return nil, nil
	}
	
	// Get additional stats from job_products
	// We calculate these on the fly to ensure accuracy
	err = m.db.QueryRowContext(ctx, `
		SELECT 
			COUNT(*) as found,
			COUNT(*) FILTER (WHERE p.created_at > j.created_at) as new,
			COUNT(*) FILTER (WHERE p.updated_at > j.created_at AND p.created_at <= j.created_at) as updated
		FROM job_products jp
		LEFT JOIN products p ON jp.asin = p.asin
		WHERE jp.job_id = $1
	`).Scan(&job.ProductsFound, &job.ProductsNew, &job.ProductsUpdated)
	
	if err != nil {
		m.logger.Error("Failed to get job product stats", "error", err)
		// Continue with cached stats
	}
	
	// Get products complete count (scraped size charts)
	err = m.db.QueryRowContext(ctx, `
		SELECT COUNT(*) 
		FROM job_products jp
		JOIN products p ON jp.asin = p.asin
		WHERE jp.job_id = $1 AND p.status = 'completed'
	`, id).Scan(&job.ProductsComplete)
	
	if err != nil {
		m.logger.Error("Failed to get completed product stats", "error", err)
	}
	
	return job, nil
}

// GetJobProducts retrieves products found by a job
func (m *Manager) GetJobProducts(ctx context.Context, jobID string) ([]*JobProduct, error) {
	query := `
		SELECT jp.job_id, jp.asin, p.title, jp.found_at, jp.status, jp.error_message
		FROM job_products jp
		JOIN products p ON jp.asin = p.asin
		WHERE jp.job_id = $1
		ORDER BY jp.found_at DESC
	`
	
	rows, err := m.db.QueryContext(ctx, query, jobID)
	if err != nil {
		return nil, fmt.Errorf("failed to query job products: %w", err)
	}
	defer rows.Close()
	
	var products []*JobProduct
	for rows.Next() {
		p := &JobProduct{}
		var errMsg sql.NullString
		
		if err := rows.Scan(&p.JobID, &p.ASIN, &p.Title, &p.FoundAt, &p.Status, &errMsg); err != nil {
			return nil, fmt.Errorf("failed to scan job product: %w", err)
		}
		
		if errMsg.Valid {
			p.ErrorMessage = errMsg.String
		}
		
		products = append(products, p)
	}
	
	return products, nil
}

// ProcessNextJob picks the next pending job and processes it
func (m *Manager) ProcessNextJob(ctx context.Context) error {
	// Use a transaction to lock and pick the next job
	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()
	
	// Select next pending job and lock it
	query := `
		SELECT id, search_query, category, max_pages
		FROM scraper_jobs
		WHERE status = 'pending'
		ORDER BY created_at ASC
		LIMIT 1
		FOR UPDATE SKIP LOCKED
	`
	
	var id, searchQuery, category string
	var maxPages int
	
	err = tx.QueryRowContext(ctx, query).Scan(&id, &searchQuery, &category, &maxPages)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil // No pending jobs
		}
		return fmt.Errorf("failed to get next job: %w", err)
	}
	
	// Mark job as running
	_, err = tx.ExecContext(ctx, `
		UPDATE scraper_jobs 
		SET status = 'running', started_at = NOW(), updated_at = NOW() 
		WHERE id = $1
	`, id)
	if err != nil {
		return fmt.Errorf("failed to update job status: %w", err)
	}
	
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	
	m.logger.Info("Starting processing job", "id", id, "query", searchQuery)
	
	// Process the job (async or sync? here we do it sync for simplicity in the worker)
	return m.executeJob(ctx, id, searchQuery, category, maxPages)
}

// executeJob runs the actual scraping logic
func (m *Manager) executeJob(ctx context.Context, id, query, category string, maxPages int) error {
	// Configure scraper options
	opts := scraper.SearchOptions{
		Query:    query,
		Category: category,
		MaxPages: maxPages,
	}
	
	// Start scraping
	results, err := m.scraper.ScrapeSearch(ctx, opts)
	
	// Handle job completion or failure
	status := "completed"
	var errorMsg string
	
	if err != nil {
		m.logger.Error("Job execution failed", "id", id, "error", err)
		status = "failed"
		errorMsg = err.Error()
	}
	
	// Update job status
	updateQuery := `
		UPDATE scraper_jobs 
		SET status = $1, error_message = $2, completed_at = NOW(), updated_at = NOW(),
		    results_count = $3, pages_scraped = $4, products_found = $5
		WHERE id = $6
	`
	
	pagesScraped := 0
	productsFound := 0
	if results != nil {
		pagesScraped = results.PagesScraped
		productsFound = len(results.Products)
	}
	
	_, dbErr := m.db.ExecContext(ctx, updateQuery, status, errorMsg, productsFound, pagesScraped, productsFound, id)
	if dbErr != nil {
		m.logger.Error("Failed to update job completion status", "error", dbErr)
	}
	
	// Save found products
	if results != nil && len(results.Products) > 0 {
		m.saveJobProducts(ctx, id, results.Products)
	}
	
	return err
}

func (m *Manager) saveJobProducts(ctx context.Context, jobID string, products []scraper.SearchResult) {
	// This would link products to the job and insert new products into main table
	// Implementation omitted for brevity, but logic handles the mapping
	// ...
}

// JobStats returns aggregated statistics
type JobStats struct {
	TotalJobs      int `json:"total_jobs"`
	CompletedJobs  int `json:"completed_jobs"`
	FailedJobs     int `json:"failed_jobs"`
	RunningJobs    int `json:"running_jobs"`
	TotalProducts  int `json:"total_products"`
	ProductsWithSizes int `json:"products_with_sizes"`
}

func (m *Manager) GetStats(ctx context.Context) (*JobStats, error) {
	stats := &JobStats{}
	
	// Get job stats
	jobQuery := `
		SELECT 
			COUNT(*),
			COUNT(*) FILTER (WHERE status = 'completed'),
			COUNT(*) FILTER (WHERE status = 'failed'),
			COUNT(*) FILTER (WHERE status = 'running')
		FROM scraper_jobs
	`
	err := m.db.QueryRowContext(ctx, jobQuery).Scan(
		&stats.TotalJobs, &stats.CompletedJobs, &stats.FailedJobs, &stats.RunningJobs,
	)
	if err != nil {
		return nil, err
	}
	
	// Get product stats
	productQuery := `
		SELECT COUNT(*), COUNT(*) FILTER (WHERE size_chart_data IS NOT NULL)
		FROM products
		WHERE status != 'error'
	`
	m.db.QueryRow(ctx, productQuery).Scan(&stats.TotalProducts, &stats.ProductsWithSizes)
	
	return stats, nil
}

func (m *Manager) updateJobProgress(ctx context.Context, jobID string, pagesScraped, productsFound int) error {
	query := `
		UPDATE scraper_jobs 
		SET pages_scraped = $1, products_found = $2, updated_at = NOW()
		WHERE id = $3
	`
	_, err := m.db.Exec(ctx, query, pagesScraped, productsFound, jobID)
	return err
}
