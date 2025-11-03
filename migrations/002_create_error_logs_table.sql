CREATE TABLE IF NOT EXISTS error_logs (
    error_id UUID PRIMARY KEY,
    dlq_event_id VARCHAR(255) NOT NULL,
    payment_id VARCHAR(255),
    saga_id VARCHAR(255),
    error_type VARCHAR(100) NOT NULL,
    error_reason TEXT NOT NULL,
    original_event JSONB NOT NULL,
    failure_details JSONB,
    retry_history JSONB,
    first_occurred_at TIMESTAMP NOT NULL,
    last_occurred_at TIMESTAMP NOT NULL,
    resolved BOOLEAN DEFAULT FALSE,
    resolved_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Create indexes for efficient queries
CREATE INDEX IF NOT EXISTS idx_error_logs_payment_id ON error_logs (payment_id);
CREATE INDEX IF NOT EXISTS idx_error_logs_saga_id ON error_logs (saga_id);
CREATE INDEX IF NOT EXISTS idx_error_logs_unresolved ON error_logs (resolved, created_at);
CREATE INDEX IF NOT EXISTS idx_error_logs_error_type ON error_logs (error_type, created_at);
CREATE INDEX IF NOT EXISTS idx_error_logs_first_occurred_at ON error_logs (first_occurred_at);

