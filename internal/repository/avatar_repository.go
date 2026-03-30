package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

type AvatarRepository struct {
	db *sql.DB
}

var (
	ErrAvatarNotFound = errors.New("avatar not found")
	ErrDuplicateKey   = errors.New("duplicate key violation")
)

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

type ProcessMessage struct {
	MessageID    string
	ConsumerName string
	EventType    string
	EntityID     string
	ProcessedAt  time.Time
}

func NewAvatarRepository(db *sql.DB) *AvatarRepository {
	return &AvatarRepository{db: db}
}

func repoLogger(ctx context.Context, operation string) *slog.Logger {
	return slog.Default().With(
		"component", "avatar-repository",
		"operation", operation,
	)
}

func (r *AvatarRepository) CreateAvatar(ctx context.Context, avatar CreateAvatarParams) error {
	ctx, span := otel.Tracer("avatars-repository").Start(ctx, "avatars.insert")
	defer span.End()

	span.SetAttributes(
		attribute.String("db.system", "postgresql"),
		attribute.String("db.operation", "INSERT"),
		attribute.String("db.sql.table", "avatars"),
		attribute.String("avatar.id", avatar.ID),
		attribute.String("user.id", avatar.UserID),
	)

	logger := repoLogger(ctx, "CreateAvatar").With(
		"avatar_id", avatar.ID,
		"user_id", avatar.UserID,
	)

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

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "insert avatar failed")
		logger.Error("failed to insert avatar", "error", err)
		return err
	}

	logger.Info("avatar inserted successfully")

	return err
}

// GetAvatarByID получает запись аватара по ID.
// Удалённые записи сразу исключаем.
func (r *AvatarRepository) GetAvatarByID(ctx context.Context, avatarID string) (*Avatar, error) {
	ctx, span := otel.Tracer("avatars-repository").Start(ctx, "avatars.select_by_id")
	defer span.End()

	span.SetAttributes(
		attribute.String("db.system", "postgresql"),
		attribute.String("db.operation", "SELECT"),
		attribute.String("db.sql.table", "avatars"),
		attribute.String("avatar.id", avatarID),
	)

	logger := repoLogger(ctx, "GetAvatarByID").With(
		"avatar_id", avatarID,
	)
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
			logger.Warn("avatar not found")
			return nil, ErrAvatarNotFound
		}
		span.RecordError(err)
		span.SetStatus(codes.Error, "select avatar failed")
		logger.Error("failed to select avatar", "error", err)
		return nil, err
	}

	logger.Info("avatar selected successfully")
	return &avatar, nil
}

// UpdateProcessingStatus обновляет только processing_status.
func (r *AvatarRepository) UpdateProcessingStatus(ctx context.Context, avatarID, status string) error {
	ctx, span := otel.Tracer("avatars-repository").Start(ctx, "avatars.update_processing_status")
	defer span.End()

	span.SetAttributes(
		attribute.String("db.system", "postgresql"),
		attribute.String("db.operation", "UPDATE"),
		attribute.String("db.sql.table", "avatars"),
		attribute.String("avatar.id", avatarID),
		attribute.String("processing.status", status),
	)

	logger := repoLogger(ctx, "UpdateProcessingStatus").With(
		"avatar_id", avatarID,
		"processing_status", status,
	)
	const query = `
		UPDATE avatars
		SET
			processing_status = $2,
			updated_at = NOW()
		WHERE id = $1
		  AND deleted_at IS NULL
	`

	_, err := r.db.ExecContext(ctx, query, avatarID, status)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "update processing status failed")
		logger.Error("failed to update processing status", "error", err)
		return err
	}

	logger.Info("processing status updated successfully")
	return nil
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
	ctx, span := otel.Tracer("avatars-repository").Start(ctx, "avatars.fail_processing")
	defer span.End()

	span.SetAttributes(
		attribute.String("db.system", "postgresql"),
		attribute.String("db.operation", "UPDATE"),
		attribute.String("db.sql.table", "avatars"),
		attribute.String("avatar.id", avatarID),
	)

	logger := repoLogger(ctx, "FailProcessing").With(
		"avatar_id", avatarID,
	)

	const query = `
		UPDATE avatars
		SET
			processing_status = 'failed',
			updated_at = NOW()
		WHERE id = $1
		  AND deleted_at IS NULL
	`

	_, err := r.db.ExecContext(ctx, query, avatarID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "fail processing update failed")
		logger.Error("failed to set failed processing status", "error", err)
		return err
	}

	logger.Info("processing marked as failed")
	return nil
}

// получение главной аватарки пользователя
func (r *AvatarRepository) GetUserAvatar(ctx context.Context, userID string) (*Avatar, error) {
	ctx, span := otel.Tracer("avatars-repository").Start(ctx, "avatars.select_user_current")
	defer span.End()

	span.SetAttributes(
		attribute.String("db.system", "postgresql"),
		attribute.String("db.operation", "SELECT"),
		attribute.String("db.sql.table", "avatars"),
		attribute.String("user.id", userID),
	)

	logger := repoLogger(ctx, "GetUserAvatar").With(
		"user_id", userID,
	)

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
			logger.Warn("current user avatar not found")
			return nil, ErrAvatarNotFound
		}
		span.RecordError(err)
		span.SetStatus(codes.Error, "select current user avatar failed")
		logger.Error("failed to select current user avatar", "error", err)
		return nil, err
	}

	logger.Info("current user avatar selected successfully", "avatar_id", avatar.ID)
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
	defer func() {
		_ = rows.Close()
	}()

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
	ctx, span := otel.Tracer("avatars-repository").Start(ctx, "avatars.set_current")
	defer span.End()

	span.SetAttributes(
		attribute.String("db.system", "postgresql"),
		attribute.String("db.operation", "UPDATE"),
		attribute.String("db.sql.table", "avatars"),
		attribute.String("avatar.id", avatarID),
		attribute.String("user.id", userID),
	)

	logger := repoLogger(ctx, "SetCurrentAvatar").With(
		"avatar_id", avatarID,
		"user_id", userID,
	)

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "begin transaction failed")
		logger.Error("failed to begin transaction", "error", err)
		return err
	}
	defer func() {
		_ = tx.Rollback()
	}()

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
			logger.Warn("avatar not found for user")
			return ErrAvatarNotFound
		}
		span.RecordError(err)
		span.SetStatus(codes.Error, "check avatar ownership failed")
		logger.Error("failed to check avatar ownership", "error", err)
		return err
	}

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
		span.RecordError(err)
		span.SetStatus(codes.Error, "reset current avatar failed")
		logger.Error("failed to reset current avatar", "error", err)
		return err
	}

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
		span.RecordError(err)
		span.SetStatus(codes.Error, "set current avatar failed")
		logger.Error("failed to set current avatar", "error", err)
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "rows affected failed")
		logger.Error("failed to get rows affected", "error", err)
		return err
	}
	if rowsAffected == 0 {
		logger.Warn("avatar not found after update")
		return ErrAvatarNotFound
	}

	if err := tx.Commit(); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "commit transaction failed")
		logger.Error("failed to commit transaction", "error", err)
		return err
	}

	logger.Info("current avatar updated successfully")
	return nil
}

func (r *AvatarRepository) DeleteCurrentUserAvatar(ctx context.Context, userID string) error {
	ctx, span := otel.Tracer("avatars-repository").Start(ctx, "avatars.delete_current_flag")
	defer span.End()

	span.SetAttributes(
		attribute.String("db.system", "postgresql"),
		attribute.String("db.operation", "UPDATE"),
		attribute.String("db.sql.table", "avatars"),
		attribute.String("user.id", userID),
	)

	logger := repoLogger(ctx, "DeleteCurrentUserAvatar").With(
		"user_id", userID,
	)

	const query = `
		UPDATE avatars
		SET is_current = FALSE
		WHERE user_id = $1
		  AND is_current
		  AND deleted_at IS NULL
	`

	_, err := r.db.ExecContext(ctx, query, userID)
	if err != nil {
		wrappedErr := fmt.Errorf("update current avatar: %w", err)
		span.RecordError(wrappedErr)
		span.SetStatus(codes.Error, "delete current avatar failed")
		logger.Error("failed to delete current avatar flag", "error", wrappedErr)
		return wrappedErr
	}

	logger.Info("current avatar flag removed successfully")
	return nil
}

func (r *AvatarRepository) SoftDeleteAvatar(ctx context.Context, avatarID string) error {
	ctx, span := otel.Tracer("avatars-repository").Start(ctx, "avatars.soft_delete")
	defer span.End()

	span.SetAttributes(
		attribute.String("db.system", "postgresql"),
		attribute.String("db.operation", "UPDATE"),
		attribute.String("db.sql.table", "avatars"),
		attribute.String("avatar.id", avatarID),
	)

	logger := repoLogger(ctx, "SoftDeleteAvatar").With(
		"avatar_id", avatarID,
	)

	const query = `
		UPDATE avatars
		SET
			deleted_at = NOW(),
			is_current = FALSE,
			updated_at = NOW()
		WHERE id = $1
		  AND deleted_at IS NULL
	`

	_, err := r.db.ExecContext(ctx, query, avatarID)
	if err != nil {
		wrappedErr := fmt.Errorf("soft delete avatar: %w", err)
		span.RecordError(wrappedErr)
		span.SetStatus(codes.Error, "soft delete failed")
		logger.Error("failed to soft delete avatar", "error", wrappedErr)
		return wrappedErr
	}

	logger.Info("avatar soft deleted successfully")
	return nil
}
