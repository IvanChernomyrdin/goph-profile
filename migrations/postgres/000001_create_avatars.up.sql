CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- Создаем ENUM для статусов загрузки
CREATE TYPE upload_status_enum AS ENUM (
    'uploading',
    'uploaded', 
    'failed',
    'deleted'
);

-- Создаем ENUM для статусов обработки
CREATE TYPE processing_status_enum AS ENUM (
    'pending',
    'processing',
    'completed',
    'failed'
);

-- Создаем таблицу с использованием enum
CREATE TABLE IF NOT EXISTS public.avatars
(
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id           VARCHAR(255) NOT NULL,
    file_name         VARCHAR(255) NOT NULL,
    mime_type         VARCHAR(100) NOT NULL,
    size_bytes        BIGINT NOT NULL
        CONSTRAINT avatars_size_bytes_check
            CHECK (size_bytes > 0),
    s3_key            VARCHAR(500) NOT NULL,
    thumbnail_s3_keys JSONB,
    upload_status     upload_status_enum NOT NULL DEFAULT 'uploading',
    processing_status processing_status_enum NOT NULL DEFAULT 'pending',
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at        TIMESTAMPTZ,
    is_current        BOOLEAN NOT NULL DEFAULT FALSE
);


CREATE INDEX IF NOT EXISTS idx_avatars_user_id
    ON public.avatars (user_id)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_avatars_status
    ON public.avatars (upload_status, processing_status);

CREATE INDEX IF NOT EXISTS idx_avatars_created_at
    ON public.avatars (created_at);