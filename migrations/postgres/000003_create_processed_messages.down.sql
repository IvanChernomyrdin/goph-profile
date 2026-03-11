DROP INDEX IF EXISTS idx_processed_messages_processed_at;
DROP INDEX IF EXISTS idx_processed_messages_event_type;
DROP INDEX IF EXISTS idx_processed_messages_consumer_name;

DROP TABLE IF EXISTS processed_messages;