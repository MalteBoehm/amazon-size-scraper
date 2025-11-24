package jobs

import (
	"context"
    "encoding/json"
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/maltedev/amazon-size-scraper/internal/amazon-scraper/events"
	"github.com/maltedev/amazon-size-scraper/internal/amazon-scraper/scraper"
	"github.com/maltedev/amazon-size-scraper/internal/database"
)

type Manager struct {
	db        *database.DB
	scraper   *scraper.Service
	logger    *slog.Logger
	publisher *events.Publisher
}

func NewManager(db *database.DB, scraper *scraper.Service, publisher *events.Publisher, logger *slog.Logger) *Manager {
	return &Manager{
		db:        db,
		scraper:   scraper,
		logger:    logger.With("component", "job_manager"),
		publisher: publisher,
	}
}

// Job represents a scraping job
type Job struct {
	ID               string    `json:"id"`
	SearchQuery      string    `json:"search_query"`
	Category         string    `json:"category"`
	MaxPages         int       `json:"max_pages"`
	Status           string    `json:"status"`
	PagesScraped     int       `json:"pages_scraped"`
	ProductsFound    int       `json:"products_found"`
	ProductsComplete int       `json:"products_complete"`
	ProductsNew      int       `json:"products_new"`
	ProductsUpdated  int       `json:"products_updated"`
	CreatedAt        time.Time `json:"created_at"`
	StartedAt        *time.Time `json:"started_at,omitempty"`
	CompletedAt      *time.Time `json:"completed_at,omitempty"`
	Error            string    `json:"error,omitempty"`
}

// JobProduct represents a product found by a job
type JobProduct struct {
	JobID      string `json:"job_id"`
	ASIN       string `json:"asin"`
	PageNumber int    `json:"page_number"`
	Title      string `json:"title"`
	HasSizes   bool   `json:"has_sizes"`
}

// Stats represents scraper statistics
type Stats struct {
	TotalJobs         int     `json:"total_jobs"`
	PendingJobs       int     `json:"pending_jobs"`
	RunningJobs       int     `json:"running_jobs"`
	CompletedJobs     int     `json:"completed_jobs"`
	FailedJobs        int     `json:"failed_jobs"`
	TotalProducts     int     `json:"total_products"`
	ProductsWithSizes int     `json:"products_with_sizes"`
	SuccessRate       float64 `json:"success_rate"`
}

// CreateJob creates a new scraping job
func (m *Manager) CreateJob(ctx context.Context, searchQuery, category string, maxPages int) (*Job, error) {
	job := &Job{
		ID:          uuid.New().String(),
		SearchQuery: searchQuery,
		Category:    category,
		MaxPages:    maxPages,
		Status:      "pending",
		CreatedAt:   time.Now(),
	}

    outbox := database.NewOutboxRepository(m.db)

    if err := m.db.Transaction(ctx, func(tx database.Tx) error {
        // 1) Insert job
        if _, err := tx.Exec(ctx, `
            INSERT INTO scraper_jobs (id, search_query, category, max_pages, status, created_at)
            VALUES ($1, $2, $3, $4, $5, $6)
        `, job.ID, job.SearchQuery, job.Category, job.MaxPages, job.Status, job.CreatedAt); err != nil {
            return fmt.Errorf("failed to create job: %w", err)
        }

        // 2) Insert outbox event for SCRAPER_JOB_REQUESTED
        payload := map[string]any{
            "job_id":       job.ID,
            "search_query": job.SearchQuery,
            "category":     job.Category,
            "max_pages":    job.MaxPages,
            "requested_at": job.CreatedAt,
        }
        data, err := json.Marshal(payload)
        if err != nil {
            return fmt.Errorf("marshal outbox payload: %w", err)
        }

        evt := &database.OutboxEvent{
            AggregateType: "scraper",
            AggregateID:   job.ID,
            EventType:     "SCRAPER_JOB_REQUESTED",
            Payload:       data,
            TargetStream:  "stream:scraper_jobs",
        }

        if err := outbox.InsertWithTx(ctx, tx, evt); err != nil {
            return fmt.Errorf("insert outbox event: %w", err)
        }
        return nil
    }); err != nil {
        return nil, err
    }

    m.logger.Info("job created", "id", job.ID, "query", searchQuery)
    return job, nil
}

// GetJob retrieves a job by ID
func (m *Manager) GetJob(ctx context.Context, jobID string) (*Job, error) {
	query := `
		SELECT id, search_query, category, max_pages, status,
		       pages_scraped, products_found, products_complete,
		       created_at, started_at, completed_at, COALESCE(error_message, '')
		FROM scraper_jobs
		WHERE id = $1
	`

	job := &Job{}
	err := m.db.QueryRow(ctx, query, jobID).Scan(
		&job.ID, &job.SearchQuery, &job.Category, &job.MaxPages, &job.Status,
		&job.PagesScraped, &job.ProductsFound, &job.ProductsComplete,
		&job.CreatedAt, &job.StartedAt, &job.CompletedAt, &job.Error,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("job not found")
	}
	if err != nil {
		m.logger.Error("failed to scan job", "jobID", jobID, "error", err)
		return nil, fmt.Errorf("failed to get job: %w", err)
	}

	// Get additional stats
	countQuery := `
		        SELECT 
					COUNT(*) as found,
					COUNT(*) FILTER (WHERE p.created_at > j.created_at) as new,
					COUNT(*) FILTER (WHERE p.updated_at > j.created_at AND p.created_at <= j.created_at) as updated
				FROM job_products jp
				LEFT JOIN products p ON jp.asin = p.asin
				WHERE jp.job_id = $1
			`).Scan(&job.ProductsFound, &job.ProductsNew, &job.ProductsUpdated)
	return job, nil
}

// ListJobs lists all jobs
func (m *Manager) ListJobs(ctx context.Context) ([]*Job, error) {
	query := `
		SELECT id, search_query, category, max_pages, status,
		       pages_scraped, products_found, products_complete,
		       created_at, started_at, completed_at
		FROM scraper_jobs
		ORDER BY created_at DESC
		LIMIT 100
	`

	rows, err := m.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list jobs: %w", err)
	}
	defer rows.Close()

	var jobs []*Job
	for rows.Next() {
		job := &Job{}
		err := rows.Scan(
			&job.ID, &job.SearchQuery, &job.Category, &job.MaxPages, &job.Status,
			&job.PagesScraped, &job.ProductsFound, &job.ProductsComplete,
			&job.CreatedAt, &job.StartedAt, &job.CompletedAt,
		)
		if err != nil {
			continue
		}
		jobs = append(jobs, job)
	}

	return jobs, nil
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

	rows, err := m.db.Query(ctx, query, jobID)
	if err != nil {
		return nil, fmt.Errorf("failed to get job products: %w", err)
	}
	defer rows.Close()

	var products []*JobProduct
	for rows.Next() {
		p := &JobProduct{}
		err := rows.Scan(&p.JobID, &p.ASIN, &p.PageNumber, &p.Title, &p.HasSizes)
		if err != nil {
			continue
		}
		products = append(products, p)
	}

	return products, nil
}

// GetStats retrieves scraper statistics
func (m *Manager) GetStats(ctx context.Context) (*Stats, error) {
	stats := &Stats{}

	query := `
		SELECT 
			COUNT(*) as total_jobs,
			COUNT(CASE WHEN status = 'pending' THEN 1 END) as pending_jobs,
			COUNT(CASE WHEN status = 'running' THEN 1 END) as running_jobs,
			COUNT(CASE WHEN status = 'completed' THEN 1 END) as completed_jobs,
			COUNT(CASE WHEN status = 'failed' THEN 1 END) as failed_jobs
		FROM scraper_jobs
	`

	err := m.db.QueryRow(ctx, query).Scan(
		&stats.TotalJobs, &stats.PendingJobs, &stats.RunningJobs,
		&stats.CompletedJobs, &stats.FailedJobs,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get stats: %w", err)
	}

	// Calculate success rate
	if stats.TotalJobs > 0 {
		stats.SuccessRate = float64(stats.CompletedJobs) / float64(stats.TotalJobs) * 100
	}

	// Get product stats
	productQuery := `
		SELECT 
			COUNT(*) as total,
			COUNT(CASE WHEN size_chart_data IS NOT NULL THEN 1 END) as with_sizes
		FROM products
	`

	m.db.QueryRow(ctx, productQuery).Scan(&stats.TotalProducts, &stats.ProductsWithSizes)

	return stats, nil
}

// updateJobStatus updates the status of a job
func (m *Manager) updateJobStatus(ctx context.Context, jobID, status string, err error) error {
	var query string
	var args []interface{}

	if status == "running" {
		now := time.Now()
		query = `UPDATE scraper_jobs SET status = $1, started_at = $2 WHERE id = $3`
		args = []interface{}{status, now, jobID}
	} else if status == "completed" {
		now := time.Now()
		query = `UPDATE scraper_jobs SET status = $1, completed_at = $2 WHERE id = $3`
		args = []interface{}{status, now, jobID}
	} else if status == "failed" && err != nil {
		now := time.Now()
		query = `UPDATE scraper_jobs SET status = $1, completed_at = $2, error_message = $3 WHERE id = $4`
		args = []interface{}{status, now, err.Error(), jobID}
	} else {
		query = `UPDATE scraper_jobs SET status = $1 WHERE id = $2`
		args = []interface{}{status, jobID}
	}

	_, execErr := m.db.Exec(ctx, query, args...)
	return execErr
}

// updateJobProgress updates job progress
func (m *Manager) updateJobProgress(ctx context.Context, jobID string, pagesScraped, productsFound int) error {
	query := `
		UPDATE scraper_jobs 
		SET pages_scraped = $1, products_found = $2 
		WHERE id = $3
	`
	_, err := m.db.Exec(ctx, query, pagesScraped, productsFound, jobID)
	return err
}