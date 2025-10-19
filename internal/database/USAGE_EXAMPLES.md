# Transaction Safety & Error Handling Usage Examples

This document provides comprehensive examples of how to use the enhanced transaction safety and error handling system.

## Basic Setup

```go
package main

import (
    "context"
    "log/slog"
    "time"

    "github.com/maltedev/amazon-size-scraper/internal/database"
)

func setupTransactionSafety() (*database.SafeProductRepository, error) {
    logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
        Level: slog.LevelInfo,
    }))

    // Database configuration
    config := database.Config{
        Host:     "localhost",
        Port:     5432,
        User:     "postgres",
        Password: "postgres",
        Database: "amazon_scraper",
        SSLMode:  "disable",
        MaxConns: 25,
        MinConns: 5,
    }

    ctx := context.Background()

    // Create database connection
    db, err := database.New(ctx, config)
    if err != nil {
        return nil, fmt.Errorf("failed to create database connection: %w", err)
    }

    // Create connection pool manager
    poolConfig := database.DefaultPoolConfig()
    poolConfig.Host = config.Host
    poolConfig.Port = config.Port
    poolConfig.User = config.User
    poolConfig.Password = config.Password
    poolConfig.Database = config.Database

    poolManager, err := database.NewConnectionPoolManager(poolConfig, logger)
    if err != nil {
        return nil, fmt.Errorf("failed to create connection pool manager: %w", err)
    }

    // Create transaction manager
    txConfig := database.DefaultTransactionConfig()
    txManager := database.NewTransactionManager(poolManager.GetDB(), logger, txConfig)

    // Create transaction monitor
    monitorConfig := database.DefaultMonitorConfig()
    monitorConfig.Enabled = true
    monitor := database.NewTransactionMonitor(poolManager.GetDB(), logger, monitorConfig)

    // Start monitoring
    go func() {
        if err := monitor.Start(ctx); err != nil {
            logger.Error("Failed to start transaction monitor", "error", err)
        }
    }()

    // Create safe product repository
    repoConfig := database.DefaultSafeRepositoryConfig()
    repo := database.NewSafeProductRepository(
        poolManager.GetDB(),
        logger,
        txManager,
        monitor,
        repoConfig,
    )

    return repo, nil
}
```

## Example 1: Safe Product Creation

```go
func createProductWithSafety(repo *database.SafeProductRepository) error {
    ctx := context.Background()

    // Create product
    product := &database.ProductLifecycle{
        ASIN:    "B08N5WRWNW",
        Title:   "Example Product",
        Brand:   "Example Brand",
        Status:  "pending",
        CreatedAt: time.Now(),
        UpdatedAt: time.Now(),
    }

    // Create with full transaction safety
    err := repo.CreateProductWithSafety(ctx, product)
    if err != nil {
        return fmt.Errorf("failed to create product: %w", err)
    }

    fmt.Printf("Product %s created successfully\n", product.ASIN)
    return nil
}
```

## Example 2: Product Creation with Size Table

```go
func createProductWithSizeTable(repo *database.SafeProductRepository) error {
    ctx := context.Background()

    // Create product
    product := &database.ProductLifecycle{
        ASIN:    "B08N5WRWNW",
        Title:   "T-Shirt with Size Table",
        Brand:   "Clothing Brand",
        Status:  "pending",
        CreatedAt: time.Now(),
        UpdatedAt: time.Now(),
    }

    // Create size table
    sizeTable := &database.SizeTable{
        Sizes: []string{"S", "M", "L", "XL"},
        Measurements: map[string]map[string]float64{
            "S": {"chest": 96, "length": 71},
            "M": {"chest": 104, "length": 74},
            "L": {"chest": 112, "length": 77},
            "XL": {"chest": 120, "length": 80},
        },
        Unit: "cm",
    }

    // Create product with size table atomically
    err := repo.CreateProductWithSizeTable(ctx, product, sizeTable)
    if err != nil {
        return fmt.Errorf("failed to create product with size table: %w", err)
    }

    fmt.Printf("Product %s with size table created successfully\n", product.ASIN)
    return nil
}
```

## Example 3: Ignoring Products with Proper Reason

```go
func ignoreProductWithReason(repo *database.SafeProductRepository) error {
    ctx := context.Background()

    asin := "B08N5WRWNW"
    reason := "no_valid_size_table"

    productData := map[string]interface{}{
        "title": "Product without size table",
        "brand": "Some Brand",
        "category": "Clothing",
    }

    // Create ignore event with proper error handling
    err := repo.IgnoreProductWithReason(ctx, asin, reason, productData)
    if err != nil {
        return fmt.Errorf("failed to ignore product: %w", err)
    }

    fmt.Printf("Product %s ignored with reason: %s\n", asin, reason)
    return nil
}
```

## Example 4: Batch Product Creation

```go
func batchCreateProducts(repo *database.SafeProductRepository) error {
    ctx := context.Background()

    products := []*database.ProductLifecycle{
        {
            ASIN:    "B08N5WRWNW",
            Title:   "Product 1",
            Brand:   "Brand 1",
            Status:  "pending",
            CreatedAt: time.Now(),
            UpdatedAt: time.Now(),
        },
        {
            ASIN:    "B08N5WRWNX",
            Title:   "Product 2",
            Brand:   "Brand 2",
            Status:  "pending",
            CreatedAt: time.Now(),
            UpdatedAt: time.Now(),
        },
        {
            ASIN:    "B08N5WRWNY",
            Title:   "Product 3",
            Brand:   "Brand 3",
            Status:  "pending",
            CreatedAt: time.Now(),
            UpdatedAt: time.Now(),
        },
    }

    // Batch create with transaction safety
    err := repo.BatchCreateProducts(ctx, products)
    if err != nil {
        return fmt.Errorf("failed to batch create products: %w", err)
    }

    fmt.Printf("Successfully created %d products\n", len(products))
    return nil
}
```

## Example 5: Using Dead Letter Processor

```go
func setupDeadLetterProcessing(db *database.DB, logger *slog.Logger) error {
    ctx := context.Background()

    // Create dead letter processor
    config := database.DefaultDeadLetterConfig()
    config.EnableAutoRecovery = true
    config.ProcessingInterval = 5 * time.Minute

    processor := database.NewDeadLetterProcessor(db, logger, config)

    // Start processing in background
    go func() {
        if err := processor.Start(ctx); err != nil {
            logger.Error("Dead letter processor failed", "error", err)
        }
    }()

    return nil
}
```

## Example 6: Custom Transaction Management

```go
func customTransactionOperation(db *database.DB) error {
    ctx := context.Background()
    logger := slog.Default()

    // Create transaction manager
    config := database.DefaultTransactionConfig()
    config.MaxRetries = 5
    config.RetryDelay = 200 * time.Millisecond

    txManager := database.NewTransactionManager(db, logger, config)

    // Execute custom transaction with options
    options := database.TransactionOptions{
        Timeout:        60 * time.Second,
        IsolationLevel: sql.LevelSerializable,
        Retryable:      true,
        DeadLetterOnFail: true,
    }

    err := txManager.ExecuteInTransaction(ctx, func(tx database.Tx) error {
        // Multiple operations within the same transaction

        // 1. Insert product
        _, err := tx.Exec(ctx, `
            INSERT INTO product (asin, title, status)
            VALUES ($1, $2, $3)
        `, "CUSTOM001", "Custom Product", "active")
        if err != nil {
            return fmt.Errorf("failed to insert product: %w", err)
        }

        // 2. Create outbox event
        _, err = tx.Exec(ctx, `
            INSERT INTO outbox_event (aggregate_type, aggregate_id, event_type, payload)
            VALUES ($1, $2, $3, $4)
        `, "product", "CUSTOM001", "PRODUCT_CREATED", `{"test": true}`)
        if err != nil {
            return fmt.Errorf("failed to insert outbox event: %w", err)
        }

        // 3. Update some other table
        _, err = tx.Exec(ctx, `
            UPDATE some_counter SET count = count + 1 WHERE id = 1
        `)
        if err != nil {
            return fmt.Errorf("failed to update counter: %w", err)
        }

        return nil
    }, options)

    if err != nil {
        // Handle transaction error with retry information
        if txErr, ok := err.(*database.TransactionError); ok {
            logger.Error("Transaction failed",
                "error", txErr.Err,
                "retry_count", txErr.RetryCount,
                "is_dead_letter", txErr.IsDeadLetter,
            )
        }
        return err
    }

    return nil
}
```

## Example 7: Connection Pool Monitoring

```go
func monitorConnectionPool(config database.Config) error {
    logger := slog.Default()

    // Create connection pool manager
    poolConfig := database.DefaultPoolConfig()
    poolConfig.Host = config.Host
    poolConfig.Port = config.Port
    poolConfig.User = config.User
    poolConfig.Password = config.Password
    poolConfig.Database = config.Database
    poolConfig.HealthCheckInterval = 30 * time.Second

    poolManager, err := database.NewConnectionPoolManager(poolConfig, logger)
    if err != nil {
        return fmt.Errorf("failed to create pool manager: %w", err)
    }
    defer poolManager.Close()

    // Monitor pool metrics
    ticker := time.NewTicker(10 * time.Second)
    defer ticker.Stop()

    for range ticker.C {
        metrics := poolManager.GetMetrics()

        logger.Info("Connection pool metrics",
            "open_connections", metrics.OpenConnections,
            "in_use_connections", metrics.InUseConnections,
            "idle_connections", metrics.IdleConnections,
            "total_queries", metrics.TotalQueries,
            "slow_queries", metrics.SlowQueries,
            "failed_queries", metrics.FailedQueries,
            "health_check_failures", metrics.HealthCheckFailures,
            "is_healthy", poolManager.IsHealthy(),
        )

        // Alert if pool utilization is high
        if metrics.OpenConnections > 0 {
            utilization := float64(metrics.InUseConnections) / float64(metrics.OpenConnections) * 100
            if utilization > 80 {
                logger.Warn("High connection pool utilization",
                    "utilization_percent", utilization,
                    "in_use", metrics.InUseConnections,
                    "total", metrics.OpenConnections,
                )
            }
        }
    }

    return nil
}
```

## Example 8: Transaction Monitoring with Alerts

```go
func setupTransactionMonitoringWithAlerts(db *database.DB, logger *slog.Logger) error {
    ctx := context.Background()

    // Create monitor with custom thresholds
    config := database.DefaultMonitorConfig()
    config.Enabled = true
    config.CheckInterval = 30 * time.Second
    config.AlertThresholds = database.AlertThresholds{
        FailedTransactionRate:     3.0,  // 3%
        SlowTransactionRate:       5.0,  // 5%
        AverageTransactionTime:    2 * time.Second,
        MaxRetryRate:              10.0, // 10%
        DeadLetterRate:            1.0,  // 1%
        ConnectionPoolUtilization: 90.0, // 90%
    }

    monitor := database.NewTransactionMonitor(db, logger, config)

    // Add alert channels
    monitor.AddAlertChannel(database.NewLogAlertChannel(logger))

    // Start monitoring
    if err := monitor.Start(ctx); err != nil {
        return fmt.Errorf("failed to start monitoring: %w", err)
    }

    // Get metrics periodically
    ticker := time.NewTicker(1 * time.Minute)
    defer ticker.Stop()

    go func() {
        for range ticker.C {
            metrics := monitor.GetMetrics()

            logger.Info("Transaction metrics",
                "total_transactions", metrics.TotalTransactions,
                "success_rate", float64(metrics.SuccessfulTransactions)/float64(metrics.TotalTransactions)*100,
                "average_time", metrics.AverageTransactionTime,
                "slow_transactions", metrics.SlowTransactions,
                "active_transactions", metrics.ActiveTransactions,
            )
        }
    }()

    return nil
}
```

## Error Handling Patterns

### 1. Retryable Errors

```go
func handleRetryableErrors(repo *database.SafeProductRepository, asin string) error {
    ctx := context.Background()

    // Get product with automatic retry
    product, err := repo.GetProductWithRetry(ctx, asin)
    if err != nil {
        return fmt.Errorf("failed to get product after retries: %w", err)
    }

    // Use product...
    _ = product
    return nil
}
```

### 2. Transaction Error Handling

```go
func handleTransactionErrors(txManager *database.TransactionManager) error {
    ctx := context.Background()

    err := txManager.ExecuteInTransaction(ctx, func(tx database.Tx) error {
        // Transaction logic here
        return nil
    })

    if err != nil {
        // Check if it's a transaction error with retry information
        if txErr, ok := err.(*database.TransactionError); ok {
            if txErr.IsDeadLetter {
                // Handle dead letter scenario
                return handleDeadLetter(txErr)
            } else if txErr.IsRetryable {
                // Retry logic might be handled automatically
                return fmt.Errorf("transaction failed but is retryable: %w", txErr)
            }
        }

        // Non-retryable error
        return fmt.Errorf("transaction failed: %w", err)
    }

    return nil
}
```

### 3. Graceful Degradation

```go
func gracefulDegradation(repo *database.SafeProductRepository, product *database.ProductLifecycle) error {
    ctx := context.Background()

    // Try with full transaction safety first
    err := repo.CreateProductWithSafety(ctx, product)
    if err == nil {
        return nil
    }

    // Log the error
    logger := slog.Default()
    logger.Warn("Transaction safety failed, attempting simple insert", "error", err)

    // Fallback to simpler operation
    // This would require implementing a fallback method
    return fallbackSimpleInsert(ctx, product)
}
```

## Best Practices

1. **Always use transaction safety** for critical operations
2. **Configure appropriate timeouts** based on operation complexity
3. **Monitor connection pool health** and adjust pool sizes
4. **Set up alerting** for failure thresholds
5. **Handle dead letter events** properly with analysis
6. **Use batch operations** when possible for efficiency
7. **Implement proper logging** for debugging transaction issues
8. **Test failure scenarios** to ensure robustness
9. **Configure retry logic** based on error types
10. **Monitor performance metrics** to identify bottlenecks

## Configuration Guidelines

### Production Settings
- **Connection Pool**: 25-50 max connections
- **Transaction Timeout**: 30-60 seconds
- **Retry Attempts**: 3-5 for critical operations
- **Health Check Interval**: 30 seconds
- **Monitoring**: Enabled with alerting
- **Dead Letter Processing**: Enabled with auto-recovery

### Development Settings
- **Connection Pool**: 5-10 max connections
- **Transaction Timeout**: 10-30 seconds
- **Retry Attempts**: 1-2 for faster feedback
- **Health Check Interval**: 10 seconds
- **Monitoring**: Enabled for debugging
- **Dead Letter Processing**: Optional

This comprehensive transaction safety system ensures ACID compliance, provides robust error handling, and includes monitoring and alerting for production reliability.