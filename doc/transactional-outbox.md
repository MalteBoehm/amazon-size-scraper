# Transactional Outbox Pattern Implementation

## Overview

The Amazon Scraper service implements the Transactional Outbox Pattern to ensure reliable event delivery to the Redis Streams-based event system. This guarantees that events are never lost, even if Redis is temporarily unavailable.

## Architecture

### Components

1. **Outbox Event Table**
   - Stores events pending delivery to Redis Streams
   - Ensures transactional consistency with business operations
   - Provides retry mechanism with exponential backoff

2. **Event Publisher**
   - Writes events to the outbox table within database transactions
   - No longer publishes directly to Redis

3. **Relay Service**
   - Background process that polls the outbox table
   - Publishes events to the appropriate Redis streams
   - Handles retries and dead letter queue

### Event Flow

```
1. Scraper finds product
   ↓
2. Publisher writes to outbox_event table (transactional)
   ↓
3. Relay polls outbox table (every 5 seconds)
   ↓
4. Relay publishes to stream:product_lifecycle
   ↓
5. Lifecycle Consumer processes event
```

## Database Schema

```sql
CREATE TABLE outbox_event (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    aggregate_type VARCHAR(50) NOT NULL,      -- e.g., 'product'
    aggregate_id VARCHAR(50) NOT NULL,        -- e.g., ASIN
    event_type VARCHAR(50) NOT NULL,          -- e.g., 'NEW_PRODUCT_DETECTED'
    payload JSONB NOT NULL,                   -- Event payload
    target_stream VARCHAR(50) NOT NULL,       -- Target Redis stream
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    retry_count INT NOT NULL DEFAULT 0,
    error_message TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    processed_at TIMESTAMPTZ,
    next_retry_at TIMESTAMPTZ DEFAULT NOW()
);
```

### Status Values
- `pending`: Event waiting to be processed
- `processed`: Successfully delivered to Redis
- `failed`: Delivery failed, will be retried
- `dead_letter`: Max retries exceeded

## Configuration

### Environment Variables

```bash
# Outbox Relay Configuration
OUTBOX_POLL_INTERVAL=5s      # How often to check for new events
OUTBOX_BATCH_SIZE=100         # Number of events to process per batch
OUTBOX_MAX_RETRIES=5          # Max retry attempts before dead letter
```

### Retry Strategy

The relay implements exponential backoff for failed events:
- 1st retry: 1 second
- 2nd retry: 2 seconds
- 3rd retry: 4 seconds
- 4th retry: 8 seconds
- 5th retry: 16 seconds
- After 5 retries: Event moves to dead_letter status

## Code Examples

### Publishing an Event

```go
// The publisher now writes to the outbox instead of Redis
func (p *Publisher) PublishNewProductDetected(ctx context.Context, payload *NewProductDetectedPayload) error {
    // Create outbox event
    outboxEvent := &database.OutboxEvent{
        AggregateType: "product",
        AggregateID:   payload.ASIN,
        EventType:     "NEW_PRODUCT_DETECTED",
        Payload:       marshalledPayload,
        TargetStream:  "stream:product_lifecycle",
    }

    // Use transaction to ensure atomicity
    return p.db.Transaction(ctx, func(tx pgx.Tx) error {
        return p.outbox.InsertWithTx(ctx, tx, outboxEvent)
    })
}
```

### Starting the Relay

```go
// In main.go
relay := database.NewRelay(db, redisClient, logger, database.RelayConfig{
    PollInterval: 5 * time.Second,
    BatchSize:    100,
})

go func() {
    if err := relay.Start(ctx); err != nil {
        logger.Error("relay stopped with error", "error", err)
    }
}()
```

## Monitoring

### Health Endpoint

The scraper service exposes a health endpoint that includes outbox metrics:

```bash
GET /health
```

Response:
```json
{
  "status": "ok",
  "outbox": {
    "pending": 15,
    "dead_letter": 0
  }
}
```

Status codes:
- `200 OK`: Everything is healthy
- `200 OK` with `status: "warning"`: High number of pending events (>1000)
- `503 Service Unavailable`: High number of dead letter events (>100)

### Monitoring Queries

Check pending events:
```sql
SELECT COUNT(*) FROM outbox_event WHERE status IN ('pending', 'failed');
```

Check dead letter events:
```sql
SELECT * FROM outbox_event WHERE status = 'dead_letter' ORDER BY created_at DESC;
```

Events by status:
```sql
SELECT status, COUNT(*) 
FROM outbox_event 
GROUP BY status;
```

Average processing time:
```sql
SELECT AVG(processed_at - created_at) as avg_processing_time
FROM outbox_event 
WHERE status = 'processed' 
  AND processed_at > NOW() - INTERVAL '1 hour';
```

## Troubleshooting

### High Number of Pending Events

If the pending count is growing:
1. Check if the Relay is running: `ps aux | grep amazon-scraper`
2. Check Redis connectivity
3. Look for errors in the logs: `grep "relay" scraper.log`

### Events in Dead Letter Queue

To reprocess dead letter events:
```sql
-- Reset specific events to pending
UPDATE outbox_event 
SET status = 'pending', 
    retry_count = 0, 
    next_retry_at = NOW()
WHERE status = 'dead_letter' 
  AND aggregate_id = 'SPECIFIC_ASIN';
```

### Manual Event Inspection

View a specific event:
```sql
SELECT id, aggregate_id, event_type, status, retry_count, error_message
FROM outbox_event 
WHERE aggregate_id = 'B001TEST';
```

## Testing

### Unit Tests

Run the outbox tests:
```bash
go test ./internal/database -run "Test.*Outbox"
go test ./internal/database -run "Test.*Relay"
```

### Integration Test

1. Start all services:
```bash
docker-compose up -d
./bin/amazon-scraper
./bin/lifecycle-consumer
```

2. Create a test job:
```bash
curl -X POST http://localhost:8084/api/v1/scraper/jobs \
  -H "Content-Type: application/json" \
  -d '{"search_query": "test product", "max_pages": 1}'
```

3. Verify outbox processing:
```bash
# Check outbox
PGPASSWORD=postgres psql -h localhost -p 5433 -U postgres -d tall_affiliate \
  -c "SELECT status, COUNT(*) FROM outbox_event GROUP BY status;"

# Check Redis stream
docker exec tall-affiliate-redis redis-cli XLEN stream:product_lifecycle
```

## Benefits

1. **Reliability**: Events are never lost, even during Redis outages
2. **Consistency**: Events are only published if the business transaction succeeds
3. **Observability**: Clear visibility into event processing status
4. **Resilience**: Automatic retries with exponential backoff
5. **Compatibility**: Works seamlessly with existing consumer infrastructure

## Migration from Direct Publishing

The migration from direct Redis publishing to the outbox pattern is transparent to consumers:
- Consumers continue reading from `stream:product_lifecycle`
- Event format remains unchanged
- No consumer code changes required

The only change is that events now flow through the outbox table, providing additional reliability guarantees.