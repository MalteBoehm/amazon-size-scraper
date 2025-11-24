-- Create outbox event table for transactional outbox pattern
CREATE TABLE IF NOT EXISTS outbox_event (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    aggregate_type VARCHAR(50) NOT NULL,
    aggregate_id VARCHAR(50) NOT NULL,
    event_type VARCHAR(50) NOT NULL,
    payload JSONB NOT NULL,
    target_stream VARCHAR(50) NOT NULL DEFAULT 'stream:product_lifecycle',
    status VARCHAR(20) NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'processed', 'failed', 'dead_letter')),
    retry_count INT NOT NULL DEFAULT 0,
    error_message TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    processed_at TIMESTAMPTZ,
    next_retry_at TIMESTAMPTZ DEFAULT NOW()
);

-- Indexes for efficient querying
CREATE INDEX idx_outbox_event_status_retry ON outbox_event(status, next_retry_at) 
    WHERE status IN ('pending', 'failed');
CREATE INDEX idx_outbox_event_created_at ON outbox_event(created_at);
CREATE INDEX idx_outbox_event_aggregate ON outbox_event(aggregate_type, aggregate_id);

-- Comments
COMMENT ON TABLE outbox_event IS 'Transactional outbox for reliable event publishing';
COMMENT ON COLUMN outbox_event.aggregate_type IS 'Type of aggregate (e.g., product, job)';
COMMENT ON COLUMN outbox_event.aggregate_id IS 'ID of the aggregate (e.g., ASIN, job_id)';
COMMENT ON COLUMN outbox_event.event_type IS 'Type of event (e.g., NEW_PRODUCT_DETECTED)';
COMMENT ON COLUMN outbox_event.payload IS 'Event payload as JSON';
COMMENT ON COLUMN outbox_event.target_stream IS 'Target Redis stream for this event';
COMMENT ON COLUMN outbox_event.status IS 'Processing status: pending, processed, failed, dead_letter';
COMMENT ON COLUMN outbox_event.retry_count IS 'Number of processing attempts';
COMMENT ON COLUMN outbox_event.next_retry_at IS 'When to retry next (for exponential backoff)';