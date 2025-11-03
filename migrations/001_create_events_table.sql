CREATE TABLE IF NOT EXISTS events (
    event_id UUID PRIMARY KEY,
    aggregate_id VARCHAR(255) NOT NULL,
    aggregate_type VARCHAR(100) NOT NULL,
    event_type VARCHAR(100) NOT NULL,
    event_version INT NOT NULL DEFAULT 1,
    event_data JSONB NOT NULL,
    event_metadata JSONB,
    timestamp TIMESTAMP NOT NULL DEFAULT NOW(),
    sequence_number BIGSERIAL NOT NULL
);

-- Create index for efficient event retrieval by aggregate
CREATE INDEX IF NOT EXISTS idx_events_aggregate ON events (aggregate_id, sequence_number);

