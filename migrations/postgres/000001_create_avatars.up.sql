CREATE EXTENSION IF NOT EXISTS pgcrypto;

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
    upload_status     VARCHAR(50) NOT NULL DEFAULT 'uploading'
        CONSTRAINT avatars_upload_status_chk
            CHECK (
                upload_status IN ('uploading', 'uploaded', 'failed', 'deleted')
            ),
    processing_status VARCHAR(50) NOT NULL DEFAULT 'pending'
        CONSTRAINT avatars_processing_status_chk
            CHECK (
                processing_status IN ('pending', 'processing', 'completed', 'failed')
            ),
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