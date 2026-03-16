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
	IsCurrent        bool
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

// получение главной аватарки пользователя
func (r *AvatarRepository) GetUserAvatar(ctx context.Context, userID string) (*Avatar, error) {
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
			deleted_at,
			is_current
		FROM avatars
		WHERE user_id = $1
		  AND is_current
		  AND deleted_at IS NULL
		LIMIT 1
	`

	var avatar Avatar
	err := r.db.QueryRowContext(ctx, query, userID).Scan(
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
		&avatar.IsCurrent,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrAvatarNotFound
		}
		return nil, err
	}

	return &avatar, nil
}

// получение списка аватарок пользователя
func (r *AvatarRepository) GetListUserAvatar(ctx context.Context, userID string) ([]Avatar, error) {
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
			deleted_at,
			is_current
		FROM avatars
		WHERE user_id = $1
		  AND deleted_at IS NULL
		ORDER BY created_at DESC
	`
	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	avatars := make([]Avatar, 0)
	for rows.Next() {
		var avatar Avatar

		if err := rows.Scan(
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
			&avatar.IsCurrent,
		); err != nil {
			return nil, err
		}

		avatars = append(avatars, avatar)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return avatars, nil
}

// проверка, сброс всех и обновление у выставление нужной аватарки у пользователя в качестве главной
func (r *AvatarRepository) SetCurrentAvatar(ctx context.Context, userID, avatarID string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Проверяем, что такая аватарка вообще есть у этого пользователя и не удалена
	const checkQuery = `
		SELECT 1
		FROM avatars
		WHERE id = $1
		  AND user_id = $2
		  AND deleted_at IS NULL
	`

	var exists int
	if err := tx.QueryRowContext(ctx, checkQuery, avatarID, userID).Scan(&exists); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrAvatarNotFound
		}
		return err
	}

	// Снимаем current у всех аватарок пользователя
	const resetQuery = `
		UPDATE avatars
		SET
			is_current = false,
			updated_at = NOW()
		WHERE user_id = $1
		  AND deleted_at IS NULL
		  AND is_current = true
	`

	if _, err := tx.ExecContext(ctx, resetQuery, userID); err != nil {
		return err
	}

	// Ставим current у выбранной аватарки
	const setQuery = `
		UPDATE avatars
		SET
			is_current = true,
			updated_at = NOW()
		WHERE id = $1
		  AND user_id = $2
		  AND deleted_at IS NULL
	`

	result, err := tx.ExecContext(ctx, setQuery, avatarID, userID)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return ErrAvatarNotFound
	}

	return tx.Commit()
}
