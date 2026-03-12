package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

type AvatarRepository struct {
	db *sql.DB
}

var ErrAvatarNotFound = errors.New("avatar not found")

type Avatar struct {
	ID               string
	UserID           string
	FileName         string
	MimeType         string
	SizeBytes        int64
	S3Key            string
	ThumbnailS3Keys  []byte
	UploadStatus     string
	ProcessingStatus string
	CreatedAt        time.Time
	UpdatedAt        time.Time
	DeletedAt        sql.NullTime
}

type CreateAvatarParams struct {
	ID               string
	UserID           string
	FileName         string
	MimeType         string
	SizeBytes        int64
	S3Key            string
	ThumbnailS3Keys  []byte
	UploadStatus     string
	ProcessingStatus string
}

func NewAvatarRepository(db *sql.DB) *AvatarRepository {
	return &AvatarRepository{db: db}
}

func (r *AvatarRepository) CreateAvatar(ctx context.Context, avatar CreateAvatarParams) error {
	const query = `
		INSERT INTO avatars (
			id,
			user_id,
			file_name,
			mime_type,
			size_bytes,
			s3_key,
			thumbnail_s3_keys,
			upload_status,
			processing_status
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
	`

	_, err := r.db.ExecContext(
		ctx,
		query,
		avatar.ID,
		avatar.UserID,
		avatar.FileName,
		avatar.MimeType,
		avatar.SizeBytes,
		avatar.S3Key,
		avatar.ThumbnailS3Keys,
		avatar.UploadStatus,
		avatar.ProcessingStatus,
	)

	return err
}

// GetAvatarByID получает запись аватара по ID.
// Удалённые записи сразу исключаем.
func (r *AvatarRepository) GetAvatarByID(ctx context.Context, avatarID string) (*Avatar, error) {
	const query = `
		SELECT
			id,
			user_id,
			file_name,
			mime_type,
			size_bytes,
			s3_key,
			thumbnail_s3_keys,
			upload_status,
			processing_status,
			created_at,
			updated_at,
			deleted_at
		FROM avatars
		WHERE id = $1
		  AND deleted_at IS NULL
	`

	var avatar Avatar
	err := r.db.QueryRowContext(ctx, query, avatarID).Scan(
		&avatar.ID,
		&avatar.UserID,
		&avatar.FileName,
		&avatar.MimeType,
		&avatar.SizeBytes,
		&avatar.S3Key,
		&avatar.ThumbnailS3Keys,
		&avatar.UploadStatus,
		&avatar.ProcessingStatus,
		&avatar.CreatedAt,
		&avatar.UpdatedAt,
		&avatar.DeletedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrAvatarNotFound
		}
		return nil, err
	}

	return &avatar, nil
}

// UpdateProcessingStatus обновляет только processing_status.
func (r *AvatarRepository) UpdateProcessingStatus(ctx context.Context, avatarID, status string) error {
	const query = `
		UPDATE avatars
		SET
			processing_status = $2,
			updated_at = NOW()
		WHERE id = $1
		  AND deleted_at IS NULL
	`

	_, err := r.db.ExecContext(ctx, query, avatarID, status)
	return err
}

// CompleteProcessing записывает ключи миниатюр и ставит completed.
func (r *AvatarRepository) CompleteProcessing(ctx context.Context, avatarID string, thumbnailKeys []byte) error {
	const query = `
		UPDATE avatars
		SET
			thumbnail_s3_keys = $2,
			processing_status = 'completed',
			updated_at = NOW()
		WHERE id = $1
		  AND deleted_at IS NULL
	`

	_, err := r.db.ExecContext(ctx, query, avatarID, thumbnailKeys)
	return err
}

// FailProcessing ставит failed.
func (r *AvatarRepository) FailProcessing(ctx context.Context, avatarID string) error {
	const query = `
		UPDATE avatars
		SET
			processing_status = 'failed',
			updated_at = NOW()
		WHERE id = $1
		  AND deleted_at IS NULL
	`

	_, err := r.db.ExecContext(ctx, query, avatarID)
	return err
}
