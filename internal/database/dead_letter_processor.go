package database

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
)

// DeadLetterProcessor handles dead letter events with analysis and recovery
type DeadLetterProcessor struct {
	db            *DB
	outboxRepo    *OutboxRepository
	logger        *slog.Logger
	config        DeadLetterConfig
	analyzers     []EventAnalyzer
	recoveryStrategies map[string]RecoveryStrategy
}

// DeadLetterConfig defines dead letter processing configuration
type DeadLetterConfig struct {
	ProcessingInterval   time.Duration
	BatchSize           int
	MaxRetries          int
	RetryDelay          time.Duration
	EnableAutoRecovery  bool
	AlertOnDeadLetter   bool
	RetentionPeriod     time.Duration
	MaxProcessingTime   time.Duration
}

// EventAnalyzer analyzes dead letter events to determine root cause
type EventAnalyzer interface {
	Analyze(ctx context.Context, event *OutboxEvent) (*AnalysisResult, error)
	SupportedEventTypes() []string
}

// AnalysisResult contains the analysis of a dead letter event
type AnalysisResult struct {
	CauseType        string                 `json:"cause_type"`
	CauseDescription string                 `json:"cause_description"`
	IsRecoverable    bool                   `json:"is_recoverable"`
	SuggestedAction  string                 `json:"suggested_action"`
	Metadata         map[string]interface{} `json:"metadata"`
	Confidence       float64                `json:"confidence"`
}

// RecoveryStrategy defines how to recover from specific types of failures
type RecoveryStrategy interface {
	CanRecover(event *OutboxEvent, analysis *AnalysisResult) bool
	Recover(ctx context.Context, event *OutboxEvent) error
	GetStrategyName() string
}

// DefaultDeadLetterConfig returns sensible defaults
func DefaultDeadLetterConfig() DeadLetterConfig {
	return DeadLetterConfig{
		ProcessingInterval:  5 * time.Minute,
		BatchSize:          10,
		MaxRetries:         3,
		RetryDelay:         30 * time.Second,
		EnableAutoRecovery: true,
		AlertOnDeadLetter:  true,
		RetentionPeriod:    30 * 24 * time.Hour, // 30 days
		MaxProcessingTime:  10 * time.Minute,
	}
}

// NewDeadLetterProcessor creates a new dead letter processor
func NewDeadLetterProcessor(
	db *DB,
	logger *slog.Logger,
	config DeadLetterConfig,
) *DeadLetterProcessor {
	dlp := &DeadLetterProcessor{
		db:                db,
		outboxRepo:        NewOutboxRepository(db),
		logger:            logger,
		config:            config,
		analyzers:         []EventAnalyzer{},
		recoveryStrategies: make(map[string]RecoveryStrategy),
	}

	// Register default analyzers
	dlp.registerDefaultAnalyzers()

	// Register default recovery strategies
	dlp.registerDefaultRecoveryStrategies()

	return dlp
}

// Start starts the dead letter processor
func (dlp *DeadLetterProcessor) Start(ctx context.Context) error {
	dlp.logger.Info("Starting dead letter processor",
		"processing_interval", dlp.config.ProcessingInterval,
		"batch_size", dlp.config.BatchSize,
		"auto_recovery", dlp.config.EnableAutoRecovery,
	)

	ticker := time.NewTicker(dlp.config.ProcessingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			dlp.logger.Info("Stopping dead letter processor")
			return ctx.Err()
		case <-ticker.C:
			if err := dlp.processBatch(ctx); err != nil {
				dlp.logger.Error("Failed to process dead letter batch", "error", err)
			}
		}
	}
}

// processBatch processes a batch of dead letter events
func (dlp *DeadLetterProcessor) processBatch(ctx context.Context) error {
	// Get dead letter events
	events, err := dlp.getDeadLetterEvents(ctx, dlp.config.BatchSize)
	if err != nil {
		return fmt.Errorf("failed to get dead letter events: %w", err)
	}

	if len(events) == 0 {
		return nil
	}

	dlp.logger.Info("Processing dead letter batch",
		"event_count", len(events),
	)

	// Create processing timeout
	processCtx, cancel := context.WithTimeout(ctx, dlp.config.MaxProcessingTime)
	defer cancel()

	// Process each event
	for _, event := range events {
		if err := dlp.processEvent(processCtx, event); err != nil {
			dlp.logger.Error("Failed to process dead letter event",
				"event_id", event.ID,
				"event_type", event.EventType,
				"error", err,
			)
		}
	}

	return nil
}

// getDeadLetterEvents retrieves dead letter events for processing
func (dlp *DeadLetterProcessor) getDeadLetterEvents(ctx context.Context, limit int) ([]*OutboxEvent, error) {
	query := `
		SELECT
			id, aggregate_type, aggregate_id, event_type,
			payload, target_stream, status, retry_count,
			error_message, created_at, processed_at, next_retry_at
		FROM outbox_event
		WHERE status = $1
			AND created_at > $2
		ORDER BY created_at ASC
		LIMIT $3`

	rows, err := dlp.db.Query(ctx, query,
		OutboxStatusDeadLetter,
		time.Now().Add(-dlp.config.RetentionPeriod),
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query dead letter events: %w", err)
	}
	defer rows.Close()

	var events []*OutboxEvent
	for rows.Next() {
		event := &OutboxEvent{}
		err := rows.Scan(
			&event.ID, &event.AggregateType, &event.AggregateID, &event.EventType,
			&event.Payload, &event.TargetStream, &event.Status, &event.RetryCount,
			&event.ErrorMessage, &event.CreatedAt, &event.ProcessedAt, &event.NextRetryAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan dead letter event: %w", err)
		}
		events = append(events, event)
	}

	return events, rows.Err()
}

// processEvent processes a single dead letter event
func (dlp *DeadLetterProcessor) processEvent(ctx context.Context, event *OutboxEvent) error {
	dlp.logger.Info("Processing dead letter event",
		"event_id", event.ID,
		"event_type", event.EventType,
		"aggregate_id", event.AggregateID,
		"retry_count", event.RetryCount,
	)

	// Analyze the event
	analysis, err := dlp.analyzeEvent(ctx, event)
	if err != nil {
		return fmt.Errorf("failed to analyze event: %w", err)
	}

	dlp.logger.Info("Event analysis completed",
		"event_id", event.ID,
		"cause_type", analysis.CauseType,
		"is_recoverable", analysis.IsRecoverable,
		"confidence", analysis.Confidence,
	)

	// Alert if configured
	if dlp.config.AlertOnDeadLetter {
		dlp.sendAlert(ctx, event, analysis)
	}

	// Attempt recovery if enabled and recoverable
	if dlp.config.EnableAutoRecovery && analysis.IsRecoverable {
		if err := dlp.attemptRecovery(ctx, event, analysis); err != nil {
			dlp.logger.Error("Recovery attempt failed",
				"event_id", event.ID,
				"error", err,
			)
			return err
		}
	}

	// Update event with analysis results
	if err := dlp.updateEventWithAnalysis(ctx, event, analysis); err != nil {
		return fmt.Errorf("failed to update event with analysis: %w", err)
	}

	return nil
}

// analyzeEvent analyzes a dead letter event using registered analyzers
func (dlp *DeadLetterProcessor) analyzeEvent(ctx context.Context, event *OutboxEvent) (*AnalysisResult, error) {
	for _, analyzer := range dlp.analyzers {
		if dlp.isEventTypeSupported(analyzer, event.EventType) {
			result, err := analyzer.Analyze(ctx, event)
			if err != nil {
				dlp.logger.Warn("Event analyzer failed",
					"analyzer", analyzer,
					"event_id", event.ID,
					"error", err,
				)
				continue
			}

			if result != nil && result.Confidence > 0.5 {
				return result, nil
			}
		}
	}

	// Default analysis if no specific analyzer succeeded
	return dlp.defaultAnalysis(event), nil
}

// isEventTypeSupported checks if an analyzer supports the given event type
func (dlp *DeadLetterProcessor) isEventTypeSupported(analyzer EventAnalyzer, eventType string) bool {
	supportedTypes := analyzer.SupportedEventTypes()
	for _, supportedType := range supportedTypes {
		if supportedType == eventType {
			return true
		}
	}
	return false
}

// defaultAnalysis provides a basic analysis when specific analyzers fail
func (dlp *DeadLetterProcessor) defaultAnalysis(event *OutboxEvent) *AnalysisResult {
	causeType := "unknown"
	isRecoverable := false
	suggestedAction := "manual_review"

	// Analyze error message if available
	if event.ErrorMessage != nil {
		errorMsg := *event.ErrorMessage
		if contains(errorMsg, "timeout") {
			causeType = "timeout"
			isRecoverable = true
			suggestedAction = "retry_with_longer_timeout"
		} else if contains(errorMsg, "connection") {
			causeType = "connection_error"
			isRecoverable = true
			suggestedAction = "retry_after_backoff"
		} else if contains(errorMsg, "serialization") || contains(errorMsg, "deadlock") {
			causeType = "concurrency_conflict"
			isRecoverable = true
			suggestedAction = "retry_with_exponential_backoff"
		} else if contains(errorMsg, "validation") || contains(errorMsg, "constraint") {
			causeType = "data_validation_error"
			isRecoverable = false
			suggestedAction = "fix_data_and_retry"
		}
	}

	return &AnalysisResult{
		CauseType:        causeType,
		CauseDescription: fmt.Sprintf("Event failed due to %s", causeType),
		IsRecoverable:    isRecoverable,
		SuggestedAction:  suggestedAction,
		Metadata:         map[string]interface{}{
			"original_error": event.ErrorMessage,
			"retry_count":    event.RetryCount,
			"created_at":     event.CreatedAt,
		},
		Confidence: 0.6, // Moderate confidence for default analysis
	}
}

// attemptRecovery attempts to recover a dead letter event
func (dlp *DeadLetterProcessor) attemptRecovery(ctx context.Context, event *OutboxEvent, analysis *AnalysisResult) error {
	// Find applicable recovery strategy
	var strategy RecoveryStrategy
	for _, s := range dlp.recoveryStrategies {
		if s.CanRecover(event, analysis) {
			strategy = s
			break
		}
	}

	if strategy == nil {
		dlp.logger.Info("No recovery strategy available",
			"event_id", event.ID,
			"cause_type", analysis.CauseType,
		)
		return nil
	}

	dlp.logger.Info("Attempting recovery",
		"event_id", event.ID,
		"strategy", strategy.GetStrategyName(),
		"cause_type", analysis.CauseType,
	)

	// Attempt recovery
	if err := strategy.Recover(ctx, event); err != nil {
		return fmt.Errorf("recovery strategy %s failed: %w", strategy.GetStrategyName(), err)
	}

	// Mark event as recovered (move back to pending)
	if err := dlp.markEventAsRecovered(ctx, event.ID); err != nil {
		return fmt.Errorf("failed to mark event as recovered: %w", err)
	}

	dlp.logger.Info("Event successfully recovered",
		"event_id", event.ID,
		"strategy", strategy.GetStrategyName(),
	)

	return nil
}

// markEventAsRecovered moves a dead letter event back to pending status
func (dlp *DeadLetterProcessor) markEventAsRecovered(ctx context.Context, eventID uuid.UUID) error {
	query := `
		UPDATE outbox_event
		SET status = $1, retry_count = 0, error_message = NULL,
			next_retry_at = $2, processed_at = NULL
		WHERE id = $3`

	_, err := dlp.db.Exec(ctx, query,
		OutboxStatusPending,
		time.Now(),
		eventID,
	)

	if err != nil {
		return fmt.Errorf("failed to mark event as recovered: %w", err)
	}

	return nil
}

// updateEventWithAnalysis updates an event with analysis results
func (dlp *DeadLetterProcessor) updateEventWithAnalysis(ctx context.Context, event *OutboxEvent, analysis *AnalysisResult) error {
	analysisJSON, err := json.Marshal(analysis)
	if err != nil {
		return fmt.Errorf("failed to marshal analysis: %w", err)
	}

	query := `
		UPDATE outbox_event
		SET payload = payload || $2::jsonb
		WHERE id = $1`

	_, err = dlp.db.Exec(ctx, query, event.ID, analysisJSON)
	if err != nil {
		return fmt.Errorf("failed to update event with analysis: %w", err)
	}

	return nil
}

// sendAlert sends an alert for a dead letter event
func (dlp *DeadLetterProcessor) sendAlert(ctx context.Context, event *OutboxEvent, analysis *AnalysisResult) {
	dlp.logger.Warn("Dead letter event alert",
		"event_id", event.ID,
		"event_type", event.EventType,
		"aggregate_id", event.AggregateID,
		"cause_type", analysis.CauseType,
		"is_recoverable", analysis.IsRecoverable,
		"retry_count", event.RetryCount,
		"created_at", event.CreatedAt,
		"error_message", event.ErrorMessage,
		"suggested_action", analysis.SuggestedAction,
	)

	// TODO: Integrate with external alerting system (e.g., PagerDuty, Slack, etc.)
}

// registerDefaultAnalyzers registers the built-in event analyzers
func (dlp *DeadLetterProcessor) registerDefaultAnalyzers() {
	dlp.analyzers = append(dlp.analyzers,
		&ProductEventAnalyzer{},
		&TransactionAnalyzer{},
		&NetworkErrorAnalyzer{},
	)
}

// registerDefaultRecoveryStrategies registers the built-in recovery strategies
func (dlp *DeadLetterProcessor) registerDefaultRecoveryStrategies() {
	strategies := []RecoveryStrategy{
		&RetryStrategy{},
		&DataFixStrategy{},
		&BackoffStrategy{},
	}

	for _, strategy := range strategies {
		dlp.recoveryStrategies[strategy.GetStrategyName()] = strategy
	}
}

// ProductEventAnalyzer analyzes product-related events
type ProductEventAnalyzer struct{}

func (a *ProductEventAnalyzer) SupportedEventTypes() []string {
	return []string{
		"PRODUCT_SIZE_TABLE_SCRAPED",
		"PRODUCT_IGNORED",
		"NEW_PRODUCT_DETECTED",
		"01B_PRODUCT_SIZE_TABLE_SCRAPED",
		"02B_PRODUCT_IGNORED",
	}
}

func (a *ProductEventAnalyzer) Analyze(ctx context.Context, event *OutboxEvent) (*AnalysisResult, error) {
	// Parse payload to extract product-specific information
	var payload map[string]interface{}
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return nil, fmt.Errorf("failed to parse payload: %w", err)
	}

	causeType := "product_processing_error"
	isRecoverable := false
	suggestedAction := "manual_review"
	confidence := 0.8

	// Analyze specific product event patterns
	if event.EventType == "PRODUCT_IGNORED" || event.EventType == "02B_PRODUCT_IGNORED" {
		if reason, ok := payload["reason"].(string); ok {
			if reason == "no_valid_size_table" || reason == "no_size_information" {
				causeType = "no_size_table"
				isRecoverable = false
				suggestedAction = "ignore_product"
				confidence = 0.95
			}
		}
	}

	// Check for data validation errors
	if event.ErrorMessage != nil && contains(*event.ErrorMessage, "validation") {
		causeType = "product_data_validation_error"
		isRecoverable = true
		suggestedAction = "fix_product_data"
		confidence = 0.9
	}

	return &AnalysisResult{
		CauseType:        causeType,
		CauseDescription: fmt.Sprintf("Product event failed due to %s", causeType),
		IsRecoverable:    isRecoverable,
		SuggestedAction:  suggestedAction,
		Metadata: map[string]interface{}{
			"event_type": event.EventType,
			"asin":       event.AggregateID,
			"payload":    payload,
		},
		Confidence: confidence,
	}, nil
}

// TransactionAnalyzer analyzes transaction-related failures
type TransactionAnalyzer struct{}

func (a *TransactionAnalyzer) SupportedEventTypes() []string {
	return []string{
		"TRANSACTION_DEAD_LETTER",
	}
}

func (a *TransactionAnalyzer) Analyze(ctx context.Context, event *OutboxEvent) (*AnalysisResult, error) {
	var payload map[string]interface{}
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return nil, fmt.Errorf("failed to parse payload: %w", err)
	}

	causeType := "transaction_error"
	isRecoverable := true
	suggestedAction := "retry_transaction"
	confidence := 0.85

	// Analyze error details
	if errorType, ok := payload["error_type"].(string); ok {
		if contains(errorType, "timeout") {
			causeType = "transaction_timeout"
			suggestedAction = "retry_with_longer_timeout"
		} else if contains(errorType, "connection") {
			causeType = "connection_error"
			suggestedAction = "retry_after_backoff"
		} else if contains(errorType, "serialization") {
			causeType = "serialization_error"
			suggestedAction = "retry_with_isolation_level_change"
		}
	}

	return &AnalysisResult{
		CauseType:        causeType,
		CauseDescription: fmt.Sprintf("Transaction failed due to %s", causeType),
		IsRecoverable:    isRecoverable,
		SuggestedAction:  suggestedAction,
		Metadata:         payload,
		Confidence:       confidence,
	}, nil
}

// NetworkErrorAnalyzer analyzes network-related errors
type NetworkErrorAnalyzer struct{}

func (a *NetworkErrorAnalyzer) SupportedEventTypes() []string {
	return []string{
		"*", // Analyzes all event types for network-related errors
	}
}

func (a *NetworkErrorAnalyzer) Analyze(ctx context.Context, event *OutboxEvent) (*AnalysisResult, error) {
	if event.ErrorMessage == nil {
		return nil, nil // No error message to analyze
	}

	errorMsg := *event.ErrorMessage
	causeType := "unknown"
	isRecoverable := true
	suggestedAction := "retry"
	confidence := 0.7

	// Identify network-specific error patterns
	if contains(errorMsg, "connection refused") {
		causeType = "connection_refused"
		suggestedAction = "retry_after_backoff"
		confidence = 0.9
	} else if contains(errorMsg, "timeout") {
		causeType = "network_timeout"
		suggestedAction = "retry_with_longer_timeout"
		confidence = 0.85
	} else if contains(errorMsg, "network is unreachable") {
		causeType = "network_unreachable"
		suggestedAction = "retry_after_extended_backoff"
		confidence = 0.9
	}

	return &AnalysisResult{
		CauseType:        causeType,
		CauseDescription: fmt.Sprintf("Network error: %s", causeType),
		IsRecoverable:    isRecoverable,
		SuggestedAction:  suggestedAction,
		Metadata: map[string]interface{}{
			"error_message": errorMsg,
			"event_type":    event.EventType,
		},
		Confidence: confidence,
	}, nil
}

// Recovery strategies implementation

// RetryStrategy implements basic retry recovery
type RetryStrategy struct{}

func (s *RetryStrategy) GetStrategyName() string {
	return "retry"
}

func (s *RetryStrategy) CanRecover(event *OutboxEvent, analysis *AnalysisResult) bool {
	return analysis.IsRecoverable &&
		(analysis.CauseType == "timeout" ||
			analysis.CauseType == "connection_error" ||
			analysis.CauseType == "network_timeout")
}

func (s *RetryStrategy) Recover(ctx context.Context, event *OutboxEvent) error {
	// This would typically be handled by the main outbox processor
	// Here we just implement the interface
	return fmt.Errorf("retry recovery should be handled by outbox processor")
}

// DataFixStrategy implements data correction recovery
type DataFixStrategy struct{}

func (s *DataFixStrategy) GetStrategyName() string {
	return "data_fix"
}

func (s *DataFixStrategy) CanRecover(event *OutboxEvent, analysis *AnalysisResult) bool {
	return analysis.IsRecoverable &&
		(analysis.CauseType == "product_data_validation_error" ||
			analysis.CauseType == "data_validation_error")
}

func (s *DataFixStrategy) Recover(ctx context.Context, event *OutboxEvent) error {
	// TODO: Implement data correction logic
	// This would involve parsing the payload, fixing invalid data,
	// and updating the event with corrected data
	return fmt.Errorf("data fix recovery not yet implemented")
}

// BackoffStrategy implements exponential backoff recovery
type BackoffStrategy struct{}

func (s *BackoffStrategy) GetStrategyName() string {
	return "exponential_backoff"
}

func (s *BackoffStrategy) CanRecover(event *OutboxEvent, analysis *AnalysisResult) bool {
	return analysis.IsRecoverable &&
		(analysis.CauseType == "concurrency_conflict" ||
			analysis.CauseType == "serialization_error")
}

func (s *BackoffStrategy) Recover(ctx context.Context, event *OutboxEvent) error {
	// This strategy would need access to the database
	// For now, return not implemented
	return fmt.Errorf("backoff recovery needs database access - not implemented")
}