CREATE TABLE IF NOT EXISTS processed_messages (
    message_id VARCHAR(255) PRIMARY KEY,
    consumer_name VARCHAR(100) NOT NULL,
    event_type VARCHAR(100) NOT NULL,
    entity_id VARCHAR(255) NOT NULL,
    processed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT unique_consumer_event_entity UNIQUE (consumer_name, event_type, entity_id)
);

CREATE INDEX IF NOT EXISTS idx_processed_messages_consumer_name
    ON processed_messages (consumer_name);

CREATE INDEX IF NOT EXISTS idx_processed_messages_event_type
    ON processed_messages (event_type);

CREATE INDEX IF NOT EXISTS idx_processed_messages_entity_id
    ON processed_messages (entity_id);

CREATE INDEX IF NOT EXISTS idx_processed_messages_processed_at
    ON processed_messages (processed_at);