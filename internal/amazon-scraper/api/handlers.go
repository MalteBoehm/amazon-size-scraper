package api

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/maltedev/amazon-size-scraper/internal/amazon-scraper/jobs"
	"github.com/maltedev/amazon-size-scraper/internal/amazon-scraper/scraper"
)

type Handlers struct {
	scraper *scraper.Service
	jobs    *jobs.Manager
	logger  *slog.Logger
}

func NewHandlers(scraper *scraper.Service, jobs *jobs.Manager, logger *slog.Logger) *Handlers {
	return &Handlers{
		scraper: scraper,
		jobs:    jobs,
		logger:  logger,
	}
}

// SizeChartRequest represents the request for size chart data
type SizeChartRequest struct {
	ASIN string `json:"asin"`
	URL  string `json:"url"`
}

// SizeChartResponse represents the size chart data response
type SizeChartResponse struct {
	SizeChartFound bool           `json:"size_chart_found"`
	SizeTable      *SizeTableData `json:"size_table,omitempty"`
	Error          string         `json:"error,omitempty"`
}

// SizeTableData represents the complete size table
type SizeTableData struct {
	Sizes        []string                       `json:"sizes"`
	Measurements map[string]map[string]float64  `json:"measurements"`
	Unit         string                        `json:"unit"`
}

// GetSizeChart handles size chart extraction requests (Oxylabs replacement)
func (h *Handlers) GetSizeChart(w http.ResponseWriter, r *http.Request) {
	var req SizeChartRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.ASIN == "" && req.URL == "" {
		h.respondError(w, http.StatusBadRequest, "either asin or url is required")
		return
	}

	// Extract size chart data
	dimensions, err := h.scraper.ExtractSizeChart(r.Context(), req.ASIN, req.URL)
	if err != nil {
		h.logger.Error("failed to extract size chart", "error", err, "asin", req.ASIN)
		h.respondJSON(w, http.StatusOK, SizeChartResponse{
			SizeChartFound: false,
			Error:          err.Error(),
		})
		return
	}

	resp := SizeChartResponse{
		SizeChartFound: dimensions.Found,
	}

	// Include complete size table if available
	if dimensions.SizeTable != nil {
		resp.SizeTable = &SizeTableData{
			Sizes:        dimensions.SizeTable.Sizes,
			Measurements: dimensions.SizeTable.Measurements,
			Unit:         dimensions.SizeTable.Unit,
		}
	}

	h.respondJSON(w, http.StatusOK, resp)
}

// ReviewsRequest represents the request for product reviews
type ReviewsRequest struct {
	ASIN string `json:"asin"`
	URL  string `json:"url"`
}

// ReviewsResponse represents the reviews data response
type ReviewsResponse struct {
	Reviews       []Review `json:"reviews"`
	AverageRating float64  `json:"average_rating"`
	TotalReviews  int      `json:"total_reviews"`
	Error         string   `json:"error,omitempty"`
}

type Review struct {
	Rating         int    `json:"rating"`
	Title          string `json:"title"`
	Text           string `json:"text"`
	VerifiedBuyer  bool   `json:"verified_buyer"`
	Date           string `json:"date"`
	MentionsSize   bool   `json:"mentions_size"`
	MentionsLength bool   `json:"mentions_length"`
}

// GetReviews handles product reviews extraction requests (Oxylabs replacement)
func (h *Handlers) GetReviews(w http.ResponseWriter, r *http.Request) {
	var req ReviewsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.ASIN == "" && req.URL == "" {
		h.respondError(w, http.StatusBadRequest, "either asin or url is required")
		return
	}

	// Extract reviews data
	reviewData, err := h.scraper.ExtractReviews(r.Context(), req.ASIN, req.URL)
	if err != nil {
		h.logger.Error("failed to extract reviews", "error", err, "asin", req.ASIN)
		h.respondJSON(w, http.StatusOK, ReviewsResponse{
			Error: err.Error(),
		})
		return
	}

	// Convert to API response format
	reviews := make([]Review, len(reviewData.Reviews))
	for i, r := range reviewData.Reviews {
		reviews[i] = Review{
			Rating:         r.Rating,
			Title:          r.Title,
			Text:           r.Text,
			VerifiedBuyer:  r.VerifiedBuyer,
			Date:           r.Date,
			MentionsSize:   r.MentionsSize,
			MentionsLength: r.MentionsLength,
		}
	}

	h.respondJSON(w, http.StatusOK, ReviewsResponse{
		Reviews:       reviews,
		AverageRating: reviewData.AverageRating,
		TotalReviews:  reviewData.TotalReviews,
	})
}

// CreateJobRequest represents a new scraping job request
type CreateJobRequest struct {
	SearchQuery string `json:"search_query"`
	Category    string `json:"category"`
	MaxPages    int    `json:"max_pages"`
}

// CreateJobResponse represents the job creation response
type CreateJobResponse struct {
	JobID   string `json:"job_id"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

// CreateJob handles new scraping job creation
func (h *Handlers) CreateJob(w http.ResponseWriter, r *http.Request) {
	var req CreateJobRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.SearchQuery == "" {
		h.respondError(w, http.StatusBadRequest, "search_query is required")
		return
	}

	if req.MaxPages <= 0 {
		req.MaxPages = 10
	}

	// Create job
	job, err := h.jobs.CreateJob(r.Context(), req.SearchQuery, req.Category, req.MaxPages)
	if err != nil {
		h.logger.Error("failed to create job", "error", err)
		h.respondError(w, http.StatusInternalServerError, "failed to create job")
		return
	}

	h.respondJSON(w, http.StatusCreated, CreateJobResponse{
		JobID:   job.ID,
		Status:  job.Status,
		Message: "Job created successfully",
	})
}

// GetJob handles job status retrieval
func (h *Handlers) GetJob(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "jobID")
	if jobID == "" {
		h.respondError(w, http.StatusBadRequest, "job ID is required")
		return
	}

	job, err := h.jobs.GetJob(r.Context(), jobID)
	if err != nil {
		h.respondError(w, http.StatusNotFound, "job not found")
		return
	}

	h.respondJSON(w, http.StatusOK, job)
}

// ListJobs handles listing all jobs
func (h *Handlers) ListJobs(w http.ResponseWriter, r *http.Request) {
	// TODO: Add pagination
	jobs, err := h.jobs.ListJobs(r.Context())
	if err != nil {
		h.logger.Error("failed to list jobs", "error", err)
		h.respondError(w, http.StatusInternalServerError, "failed to list jobs")
		return
	}

	h.respondJSON(w, http.StatusOK, jobs)
}

// GetJobProducts handles retrieving products found by a job
func (h *Handlers) GetJobProducts(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "jobID")
	if jobID == "" {
		h.respondError(w, http.StatusBadRequest, "job ID is required")
		return
	}

	products, err := h.jobs.GetJobProducts(r.Context(), jobID)
	if err != nil {
		h.logger.Error("failed to get job products", "error", err)
		h.respondError(w, http.StatusInternalServerError, "failed to get products")
		return
	}

	h.respondJSON(w, http.StatusOK, products)
}

// GetStats handles statistics retrieval
func (h *Handlers) GetStats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.jobs.GetStats(r.Context())
	if err != nil {
		h.logger.Error("failed to get stats", "error", err)
		h.respondError(w, http.StatusInternalServerError, "failed to get stats")
		return
	}

	h.respondJSON(w, http.StatusOK, stats)
}

// Helper methods
func (h *Handlers) respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		h.logger.Error("failed to encode response", "error", err)
	}
}

func (h *Handlers) respondError(w http.ResponseWriter, status int, message string) {
	h.respondJSON(w, status, map[string]string{"error": message})
}