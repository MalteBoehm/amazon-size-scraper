package database

import (
	"context"
	"fmt"
	"log/slog"
	"runtime"
	"sync"
	"time"

	"github.com/google/uuid"
)

// TransactionMonitor monitors database transaction health and performance
type TransactionMonitor struct {
	db           *DB
	logger       *slog.Logger
	config       MonitorConfig
	metrics      *TransactionMetrics
	alertManager *AlertManager
	isRunning    bool
	mu           sync.RWMutex
}

// MonitorConfig defines monitoring configuration
type MonitorConfig struct {
	Enabled                bool
	CheckInterval          time.Duration
	MetricsRetentionPeriod time.Duration
	AlertThresholds        AlertThresholds
	PerformanceTracking    bool
	SlowQueryThreshold     time.Duration
	MaxActiveTransactions  int
}

// AlertThresholds defines when to trigger alerts
type AlertThresholds struct {
	FailedTransactionRate     float64 // Percentage
	SlowTransactionRate       float64 // Percentage
	AverageTransactionTime    time.Duration
	MaxRetryRate              float64 // Percentage
	DeadLetterRate            float64 // Percentage
	ConnectionPoolUtilization float64 // Percentage
	TransactionCountPerMinute int
}

// TransactionMetrics tracks transaction performance metrics
type TransactionMetrics struct {
	mu                     sync.RWMutex
	TotalTransactions      int64     `json:"total_transactions"`
	SuccessfulTransactions int64     `json:"successful_transactions"`
	FailedTransactions     int64     `json:"failed_transactions"`
	RetriedTransactions    int64     `json:"retried_transactions"`
	DeadLetterTransactions int64     `json:"dead_letter_transactions"`
	SlowTransactions       int64     `json:"slow_transactions"`
	AverageTransactionTime time.Duration `json:"average_transaction_time"`
	PeakTransactionTime    time.Duration `json:"peak_transaction_time"`
	LastTransactionTime    time.Time   `json:"last_transaction_time"`
	TransactionHistory     []TransactionRecord `json:"transaction_history"`
	ActiveTransactions     int64     `json:"active_transactions"`
	ErrorsByType           map[string]int64 `json:"errors_by_type"`
	PerformanceByType      map[string]TransactionTypeMetrics `json:"performance_by_type"`
}

// TransactionRecord represents a single transaction record
type TransactionRecord struct {
	ID          uuid.UUID `json:"id"`
	Type        string    `json:"type"`
	StartTime   time.Time `json:"start_time"`
	EndTime     time.Time `json:"end_time"`
	Duration    time.Duration `json:"duration"`
	Status      string    `json:"status"`
	RetryCount  int       `json:"retry_count"`
	Error       string    `json:"error,omitempty"`
	IsSlow      bool      `json:"is_slow"`
}

// TransactionTypeMetrics tracks metrics for specific transaction types
type TransactionTypeMetrics struct {
	Count           int64         `json:"count"`
	SuccessCount    int64         `json:"success_count"`
	FailureCount    int64         `json:"failure_count"`
	AverageTime     time.Duration `json:"average_time"`
	MinTime         time.Duration `json:"min_time"`
	MaxTime         time.Duration `json:"max_time"`
	LastUpdated     time.Time     `json:"last_updated"`
}

// AlertManager handles alerting for transaction issues
type AlertManager struct {
	logger   *slog.Logger
	channels []AlertChannel
	mu       sync.RWMutex
}

// AlertChannel defines an alert delivery channel
type AlertChannel interface {
	Send(ctx context.Context, alert Alert) error
	GetChannelName() string
}

// Alert represents a transaction alert
type Alert struct {
	ID          uuid.UUID              `json:"id"`
	Type        AlertType              `json:"type"`
	Severity    AlertSeverity          `json:"severity"`
	Title       string                 `json:"title"`
	Description string                 `json:"description"`
	Timestamp   time.Time              `json:"timestamp"`
	Metadata    map[string]interface{} `json:"metadata"`
	Source      string                 `json:"source"`
}

// AlertType defines the type of alert
type AlertType string

const (
	AlertTypeHighFailureRate      AlertType = "high_failure_rate"
	AlertTypeSlowTransactions     AlertType = "slow_transactions"
	AlertTypeConnectionPool       AlertType = "connection_pool_exhaustion"
	AlertTypeDeadLetterBuildup    AlertType = "dead_letter_buildup"
	AlertTypeTransactionTimeout   AlertType = "transaction_timeout"
	AlertTypeRetryThreshold       AlertType = "retry_threshold_exceeded"
	AlertTypeDatabaseUnreachable  AlertType = "database_unreachable"
)

// AlertSeverity defines alert severity levels
type AlertSeverity string

const (
	SeverityInfo    AlertSeverity = "info"
	SeverityWarning AlertSeverity = "warning"
	SeverityError   AlertSeverity = "error"
	SeverityCritical AlertSeverity = "critical"
)

// DefaultMonitorConfig returns sensible defaults
func DefaultMonitorConfig() MonitorConfig {
	return MonitorConfig{
		Enabled:                true,
		CheckInterval:          30 * time.Second,
		MetricsRetentionPeriod: 24 * time.Hour,
		PerformanceTracking:    true,
		SlowQueryThreshold:     1 * time.Second,
		MaxActiveTransactions:  100,
		AlertThresholds: AlertThresholds{
			FailedTransactionRate:     5.0,  // 5%
			SlowTransactionRate:       10.0, // 10%
			AverageTransactionTime:    5 * time.Second,
			MaxRetryRate:              20.0, // 20%
			DeadLetterRate:            2.0,  // 2%
			ConnectionPoolUtilization: 85.0, // 85%
			TransactionCountPerMinute: 1000,
		},
	}
}

// NewTransactionMonitor creates a new transaction monitor
func NewTransactionMonitor(
	db *DB,
	logger *slog.Logger,
	config MonitorConfig,
) *TransactionMonitor {
	tm := &TransactionMonitor{
		db:      db,
		logger:  logger,
		config:  config,
		metrics: &TransactionMetrics{
			TransactionHistory: make([]TransactionRecord, 0),
			ErrorsByType:       make(map[string]int64),
			PerformanceByType:  make(map[string]TransactionTypeMetrics),
		},
		alertManager: NewAlertManager(logger),
	}

	return tm
}

// NewAlertManager creates a new alert manager
func NewAlertManager(logger *slog.Logger) *AlertManager {
	return &AlertManager{
		logger:   logger,
		channels: make([]AlertChannel, 0),
	}
}

// Start starts the transaction monitor
func (tm *TransactionMonitor) Start(ctx context.Context) error {
	if !tm.config.Enabled {
		tm.logger.Info("Transaction monitor is disabled")
		return nil
	}

	tm.mu.Lock()
	if tm.isRunning {
		tm.mu.Unlock()
		return fmt.Errorf("transaction monitor is already running")
	}
	tm.isRunning = true
	tm.mu.Unlock()

	tm.logger.Info("Starting transaction monitor",
		"check_interval", tm.config.CheckInterval,
		"slow_query_threshold", tm.config.SlowQueryThreshold,
	)

	// Start monitoring loop
	go tm.monitorLoop(ctx)

	// Start metrics collection
	go tm.collectMetrics(ctx)

	return nil
}

// Stop stops the transaction monitor
func (tm *TransactionMonitor) Stop() {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	tm.isRunning = false
	tm.logger.Info("Transaction monitor stopped")
}

// monitorLoop runs the main monitoring loop
func (tm *TransactionMonitor) monitorLoop(ctx context.Context) {
	ticker := time.NewTicker(tm.config.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			tm.performHealthCheck(ctx)
		}
	}
}

// performHealthCheck performs comprehensive health checks
func (tm *TransactionMonitor) performHealthCheck(ctx context.Context) {
	// Check database connectivity
	if err := tm.checkDatabaseHealth(ctx); err != nil {
		tm.sendAlert(ctx, Alert{
			Type:        AlertTypeDatabaseUnreachable,
			Severity:    SeverityCritical,
			Title:       "Database Unreachable",
			Description: fmt.Sprintf("Database health check failed: %v", err),
			Metadata: map[string]interface{}{
				"error": err.Error(),
			},
		})
	}

	// Check transaction metrics
	tm.checkTransactionMetrics(ctx)

	// Check connection pool status
	tm.checkConnectionPool(ctx)

	// Check dead letter queue
	tm.checkDeadLetterQueue(ctx)

	// Clean up old metrics
	tm.cleanupOldMetrics()
}

// checkDatabaseHealth checks if the database is reachable and responsive
func (tm *TransactionMonitor) checkDatabaseHealth(ctx context.Context) error {
	start := time.Now()

	// Simple ping test
	var result int
	err := tm.db.QueryRow(ctx, "SELECT 1").Scan(&result)
	duration := time.Since(start)

	if err != nil {
		return fmt.Errorf("database ping failed: %w", err)
	}

	// Check if ping is slow
	if duration > 5*time.Second {
		tm.sendAlert(ctx, Alert{
			Type:        AlertTypeSlowTransactions,
			Severity:    SeverityWarning,
			Title:       "Slow Database Response",
			Description: fmt.Sprintf("Database ping took %v", duration),
			Metadata: map[string]interface{}{
				"duration": duration.String(),
			},
		})
	}

	return nil
}

// checkTransactionMetrics analyzes transaction metrics and triggers alerts
func (tm *TransactionMonitor) checkTransactionMetrics(ctx context.Context) {
	tm.metrics.mu.RLock()
	defer tm.metrics.mu.RUnlock()

	total := tm.metrics.TotalTransactions
	if total == 0 {
		return
	}

	// Calculate failure rate
	failureRate := float64(tm.metrics.FailedTransactions) / float64(total) * 100
	if failureRate > tm.config.AlertThresholds.FailedTransactionRate {
		tm.sendAlert(ctx, Alert{
			Type:        AlertTypeHighFailureRate,
			Severity:    SeverityError,
			Title:       "High Transaction Failure Rate",
			Description: fmt.Sprintf("Transaction failure rate is %.2f%% (%d/%d)",
				failureRate, tm.metrics.FailedTransactions, total),
			Metadata: map[string]interface{}{
				"failure_rate":       failureRate,
				"failed_transactions": tm.metrics.FailedTransactions,
				"total_transactions":  total,
			},
		})
	}

	// Check slow transaction rate
	slowRate := float64(tm.metrics.SlowTransactions) / float64(total) * 100
	if slowRate > tm.config.AlertThresholds.SlowTransactionRate {
		tm.sendAlert(ctx, Alert{
			Type:        AlertTypeSlowTransactions,
			Severity:    SeverityWarning,
			Title:       "High Slow Transaction Rate",
			Description: fmt.Sprintf("Slow transaction rate is %.2f%% (%d/%d)",
				slowRate, tm.metrics.SlowTransactions, total),
			Metadata: map[string]interface{}{
				"slow_rate":          slowRate,
				"slow_transactions":  tm.metrics.SlowTransactions,
				"total_transactions":  total,
			},
		})
	}

	// Check average transaction time
	if tm.metrics.AverageTransactionTime > tm.config.AlertThresholds.AverageTransactionTime {
		tm.sendAlert(ctx, Alert{
			Type:        AlertTypeSlowTransactions,
			Severity:    SeverityWarning,
			Title:       "High Average Transaction Time",
			Description: fmt.Sprintf("Average transaction time is %v",
				tm.metrics.AverageTransactionTime),
			Metadata: map[string]interface{}{
				"average_time": tm.metrics.AverageTransactionTime.String(),
				"threshold":    tm.config.AlertThresholds.AverageTransactionTime.String(),
			},
		})
	}

	// Check retry rate
	if total > 0 {
		retryRate := float64(tm.metrics.RetriedTransactions) / float64(total) * 100
		if retryRate > tm.config.AlertThresholds.MaxRetryRate {
			tm.sendAlert(ctx, Alert{
				Type:        AlertTypeRetryThreshold,
				Severity:    SeverityWarning,
				Title:       "High Transaction Retry Rate",
				Description: fmt.Sprintf("Transaction retry rate is %.2f%%", retryRate),
				Metadata: map[string]interface{}{
					"retry_rate": retryRate,
				},
			})
		}
	}

	// Check dead letter rate
	if total > 0 {
		deadLetterRate := float64(tm.metrics.DeadLetterTransactions) / float64(total) * 100
		if deadLetterRate > tm.config.AlertThresholds.DeadLetterRate {
			tm.sendAlert(ctx, Alert{
				Type:        AlertTypeDeadLetterBuildup,
				Severity:    SeverityError,
				Title:       "High Dead Letter Rate",
				Description: fmt.Sprintf("Dead letter rate is %.2f%%", deadLetterRate),
				Metadata: map[string]interface{}{
					"dead_letter_rate": deadLetterRate,
				},
			})
		}
	}
}

// checkConnectionPool checks connection pool utilization
func (tm *TransactionMonitor) checkConnectionPool(ctx context.Context) {
	// This would need access to the connection pool manager
	// For now, we'll implement a basic check using runtime stats
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	// Check if we're under memory pressure (could affect connection pool)
	if m.Alloc > 100*1024*1024 { // 100MB
		tm.sendAlert(ctx, Alert{
			Type:        AlertTypeConnectionPool,
			Severity:    SeverityWarning,
			Title:       "High Memory Usage",
			Description: fmt.Sprintf("Memory usage is %.2f MB", float64(m.Alloc)/1024/1024),
			Metadata: map[string]interface{}{
				"memory_alloc": m.Alloc,
				"memory_total_alloc": m.TotalAlloc,
			},
		})
	}
}

// checkDeadLetterQueue checks for dead letter queue buildup
func (tm *TransactionMonitor) checkDeadLetterQueue(ctx context.Context) {
	query := `
		SELECT COUNT(*) FROM outbox_event
		WHERE status = $1 AND created_at > $2`

	var count int
	err := tm.db.QueryRow(ctx, query, OutboxStatusDeadLetter,
		time.Now().Add(-24*time.Hour)).Scan(&count)

	if err != nil {
		tm.logger.Error("Failed to check dead letter queue", "error", err)
		return
	}

	if count > 100 { // Threshold for dead letter buildup
		tm.sendAlert(ctx, Alert{
			Type:        AlertTypeDeadLetterBuildup,
			Severity:    SeverityError,
			Title:       "Dead Letter Queue Buildup",
			Description: fmt.Sprintf("%d dead letter events in last 24 hours", count),
			Metadata: map[string]interface{}{
				"dead_letter_count": count,
			},
		})
	}
}

// cleanupOldMetrics removes old transaction records
func (tm *TransactionMonitor) cleanupOldMetrics() {
	tm.metrics.mu.Lock()
	defer tm.metrics.mu.Unlock()

	cutoff := time.Now().Add(-tm.config.MetricsRetentionPeriod)
	remaining := make([]TransactionRecord, 0)

	for _, record := range tm.metrics.TransactionHistory {
		if record.StartTime.After(cutoff) {
			remaining = append(remaining, record)
		}
	}

	tm.metrics.TransactionHistory = remaining
}

// collectMetrics periodically collects system metrics
func (tm *TransactionMonitor) collectMetrics(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			tm.collectSystemMetrics()
		}
	}
}

// collectSystemMetrics collects system-level metrics
func (tm *TransactionMonitor) collectSystemMetrics() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	tm.logger.Debug("System metrics",
		"goroutines", runtime.NumGoroutine(),
		"memory_alloc_mb", float64(m.Alloc)/1024/1024,
		"memory_sys_mb", float64(m.Sys)/1024/1024,
		"gc_cycles", m.NumGC,
	)
}

// RecordTransaction records a transaction execution
func (tm *TransactionMonitor) RecordTransaction(record TransactionRecord) {
	if !tm.config.Enabled {
		return
	}

	tm.metrics.mu.Lock()
	defer tm.metrics.mu.Unlock()

	// Update counters
	tm.metrics.TotalTransactions++
	tm.metrics.LastTransactionTime = record.EndTime

	// Update status-specific counters
	switch record.Status {
	case "success":
		tm.metrics.SuccessfulTransactions++
	case "failed":
		tm.metrics.FailedTransactions++
		if record.Error != "" {
			tm.metrics.ErrorsByType[record.Error]++
		}
	case "retried":
		tm.metrics.RetriedTransactions++
	case "dead_letter":
		tm.metrics.DeadLetterTransactions++
	}

	// Update performance metrics
	if record.Duration > tm.config.SlowQueryThreshold {
		tm.metrics.SlowTransactions++
		record.IsSlow = true
	}

	// Update timing metrics
	if record.Duration > tm.metrics.PeakTransactionTime {
		tm.metrics.PeakTransactionTime = record.Duration
	}

	// Calculate rolling average
	if tm.metrics.TotalTransactions > 0 {
		totalDuration := tm.metrics.AverageTransactionTime * time.Duration(tm.metrics.TotalTransactions-1)
		totalDuration += record.Duration
		tm.metrics.AverageTransactionTime = totalDuration / time.Duration(tm.metrics.TotalTransactions)
	}

	// Update type-specific metrics
	typeMetrics, exists := tm.metrics.PerformanceByType[record.Type]
	if !exists {
		typeMetrics = TransactionTypeMetrics{
			MinTime: record.Duration,
			MaxTime: record.Duration,
		}
	}

	typeMetrics.Count++
	typeMetrics.LastUpdated = time.Now()

	if record.Status == "success" {
		typeMetrics.SuccessCount++
	} else {
		typeMetrics.FailureCount++
	}

	// Update timing metrics for this type
	if record.Duration < typeMetrics.MinTime {
		typeMetrics.MinTime = record.Duration
	}
	if record.Duration > typeMetrics.MaxTime {
		typeMetrics.MaxTime = record.Duration
	}

	// Calculate average for this type
	if typeMetrics.Count > 0 {
		typeMetrics.AverageTime = (typeMetrics.AverageTime*time.Duration(typeMetrics.Count-1) + record.Duration) / time.Duration(typeMetrics.Count)
	}

	tm.metrics.PerformanceByType[record.Type] = typeMetrics

	// Add to history (keep last 1000 records)
	tm.metrics.TransactionHistory = append(tm.metrics.TransactionHistory, record)
	if len(tm.metrics.TransactionHistory) > 1000 {
		tm.metrics.TransactionHistory = tm.metrics.TransactionHistory[1:]
	}
}

// sendAlert sends an alert through all configured channels
func (tm *TransactionMonitor) sendAlert(ctx context.Context, alert Alert) {
	alert.ID = uuid.New()
	alert.Timestamp = time.Now()
	alert.Source = "transaction_monitor"

	tm.logger.Warn("Transaction alert triggered",
		"type", alert.Type,
		"severity", alert.Severity,
		"title", alert.Title,
		"description", alert.Description,
	)

	// Send through all alert channels
	tm.alertManager.mu.RLock()
	channels := make([]AlertChannel, len(tm.alertManager.channels))
	copy(channels, tm.alertManager.channels)
	tm.alertManager.mu.RUnlock()

	for _, channel := range channels {
		if err := channel.Send(ctx, alert); err != nil {
			tm.logger.Error("Failed to send alert",
				"channel", channel.GetChannelName(),
				"alert_id", alert.ID,
				"error", err,
			)
		}
	}
}

// GetMetrics returns current transaction metrics
func (tm *TransactionMonitor) GetMetrics() TransactionMetrics {
	tm.metrics.mu.RLock()
	defer tm.metrics.mu.RUnlock()

	// Return a copy to avoid race conditions
	metrics := *tm.metrics

	// Copy maps
	metrics.ErrorsByType = make(map[string]int64)
	for k, v := range tm.metrics.ErrorsByType {
		metrics.ErrorsByType[k] = v
	}

	metrics.PerformanceByType = make(map[string]TransactionTypeMetrics)
	for k, v := range tm.metrics.PerformanceByType {
		metrics.PerformanceByType[k] = v
	}

	return metrics
}

// AddAlertChannel adds an alert channel
func (tm *TransactionMonitor) AddAlertChannel(channel AlertChannel) {
	tm.alertManager.mu.Lock()
	defer tm.alertManager.mu.Unlock()
	tm.alertManager.channels = append(tm.alertManager.channels, channel)
}

// LogAlertChannel implements a simple logging alert channel
type LogAlertChannel struct {
	logger *slog.Logger
}

func NewLogAlertChannel(logger *slog.Logger) *LogAlertChannel {
	return &LogAlertChannel{logger: logger}
}

func (lac *LogAlertChannel) GetChannelName() string {
	return "log"
}

func (lac *LogAlertChannel) Send(ctx context.Context, alert Alert) error {
	lac.logger.Error("ALERT",
		"id", alert.ID,
		"type", alert.Type,
		"severity", alert.Severity,
		"title", alert.Title,
		"description", alert.Description,
		"timestamp", alert.Timestamp,
		"source", alert.Source,
		"metadata", alert.Metadata,
	)
	return nil
}