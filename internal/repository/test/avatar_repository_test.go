package repository

import (
	"context"
	"database/sql"
	"goph-profile-avatars/internal/repository"
	"regexp"
	"testing"
	"time"

	status "goph-profile-avatars/internal/config/status"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewAvatarRepository(t *testing.T) {
	db, _, err := sqlmock.New()
	require.NoError(t, err)
	defer func() {
		_ = db.Close()
	}()

	repo := repository.NewAvatarRepository(db)
	assert.NotNil(t, repo)
}

func TestAvatarRepository_CreateAvatar(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() {
		_ = db.Close()
	}()

	repo := repository.NewAvatarRepository(db)
	ctx := context.Background()

	avatarID := uuid.New().String()
	params := repository.CreateAvatarParams{
		ID:               avatarID,
		UserID:           "user123",
		FileName:         "avatar.jpg",
		MimeType:         "image/jpeg",
		SizeBytes:        1024,
		S3Key:            "avatars/user123/avatar.jpg",
		ThumbnailS3Keys:  []byte(`{"small":"key1","medium":"key2"}`),
		UploadStatus:     status.Uploaded,
		ProcessingStatus: status.Pending,
	}

	mock.ExpectExec(regexp.QuoteMeta(`
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
	`)).
		WithArgs(
			params.ID,
			params.UserID,
			params.FileName,
			params.MimeType,
			params.SizeBytes,
			params.S3Key,
			params.ThumbnailS3Keys,
			params.UploadStatus,
			params.ProcessingStatus,
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err = repo.CreateAvatar(ctx, params)
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestAvatarRepository_GetAvatarByID(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() {
		_ = db.Close()
	}()

	repo := repository.NewAvatarRepository(db)
	ctx := context.Background()

	avatarID := uuid.New().String()
	now := time.Now()

	rows := sqlmock.NewRows([]string{
		"id", "user_id", "file_name", "mime_type", "size_bytes",
		"s3_key", "thumbnail_s3_keys", "upload_status", "processing_status",
		"created_at", "updated_at", "deleted_at",
	}).AddRow(
		avatarID, "user123", "avatar.jpg", "image/jpeg", 1024,
		"s3://bucket/avatar.jpg", []byte(`{"small":"key"}`), status.Uploaded, "completed",
		now, now, nil,
	)

	mock.ExpectQuery(regexp.QuoteMeta(`
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
	`)).
		WithArgs(avatarID).
		WillReturnRows(rows)

	avatar, err := repo.GetAvatarByID(ctx, avatarID)
	require.NoError(t, err)
	assert.Equal(t, avatarID, avatar.ID)
	assert.Equal(t, "user123", avatar.UserID)
	assert.Equal(t, "avatar.jpg", avatar.FileName)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestAvatarRepository_GetAvatarByID_NotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() {
		_ = db.Close()
	}()

	repo := repository.NewAvatarRepository(db)
	ctx := context.Background()

	avatarID := uuid.New().String()

	mock.ExpectQuery(regexp.QuoteMeta(`
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
	`)).
		WithArgs(avatarID).
		WillReturnError(sql.ErrNoRows)

	avatar, err := repo.GetAvatarByID(ctx, avatarID)
	assert.Nil(t, avatar)
	assert.ErrorIs(t, err, repository.ErrAvatarNotFound)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestAvatarRepository_UpdateProcessingStatus(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() {
		_ = db.Close()
	}()

	repo := repository.NewAvatarRepository(db)
	ctx := context.Background()

	avatarID := uuid.New().String()
	status := "processing"

	mock.ExpectExec(regexp.QuoteMeta(`
		UPDATE avatars
		SET
			processing_status = $2,
			updated_at = NOW()
		WHERE id = $1
		  AND deleted_at IS NULL
	`)).
		WithArgs(avatarID, status).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = repo.UpdateProcessingStatus(ctx, avatarID, status)
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestAvatarRepository_CompleteProcessing(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() {
		_ = db.Close()
	}()

	repo := repository.NewAvatarRepository(db)
	ctx := context.Background()

	avatarID := uuid.New().String()
	thumbnailKeys := []byte(`{"small":"key"}`)

	mock.ExpectExec(regexp.QuoteMeta(`
		UPDATE avatars
		SET
			thumbnail_s3_keys = $2,
			processing_status = 'completed',
			updated_at = NOW()
		WHERE id = $1
		  AND deleted_at IS NULL
	`)).
		WithArgs(avatarID, thumbnailKeys).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = repo.CompleteProcessing(ctx, avatarID, thumbnailKeys)
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestAvatarRepository_FailProcessing(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() {
		_ = db.Close()
	}()

	repo := repository.NewAvatarRepository(db)
	ctx := context.Background()

	avatarID := uuid.New().String()

	mock.ExpectExec(regexp.QuoteMeta(`
		UPDATE avatars
		SET
			processing_status = 'failed',
			updated_at = NOW()
		WHERE id = $1
		  AND deleted_at IS NULL
	`)).
		WithArgs(avatarID).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = repo.FailProcessing(ctx, avatarID)
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestAvatarRepository_GetUserAvatar(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() {
		_ = db.Close()
	}()

	repo := repository.NewAvatarRepository(db)
	ctx := context.Background()

	userID := "user123"
	avatarID := uuid.New().String()
	now := time.Now()

	rows := sqlmock.NewRows([]string{
		"id", "user_id", "file_name", "mime_type", "size_bytes",
		"s3_key", "thumbnail_s3_keys", "upload_status", "processing_status",
		"created_at", "updated_at", "deleted_at", "is_current",
	}).AddRow(
		avatarID, userID, "avatar.jpg", "image/jpeg", 1024,
		"s3://bucket/avatar.jpg", []byte(`{"small":"key"}`), status.Uploaded, "completed",
		now, now, nil, true,
	)

	mock.ExpectQuery(regexp.QuoteMeta(`
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
	`)).
		WithArgs(userID).
		WillReturnRows(rows)

	avatar, err := repo.GetUserAvatar(ctx, userID)
	require.NoError(t, err)
	assert.Equal(t, avatarID, avatar.ID)
	assert.Equal(t, userID, avatar.UserID)
	assert.True(t, avatar.IsCurrent)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestAvatarRepository_GetListUserAvatar(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() {
		_ = db.Close()
	}()

	repo := repository.NewAvatarRepository(db)
	ctx := context.Background()

	userID := "user123"
	now := time.Now()

	rows := sqlmock.NewRows([]string{
		"id", "user_id", "file_name", "mime_type", "size_bytes",
		"s3_key", "thumbnail_s3_keys", "upload_status", "processing_status",
		"created_at", "updated_at", "deleted_at", "is_current",
	}).AddRow(
		uuid.New().String(), userID, "avatar1.jpg", "image/jpeg", 1024,
		"s3://bucket/avatar1.jpg", []byte(`{"small":"key1"}`), status.Uploaded, "completed",
		now, now, nil, true,
	).AddRow(
		uuid.New().String(), userID, "avatar2.jpg", "image/png", 2048,
		"s3://bucket/avatar2.jpg", []byte(`{"small":"key2"}`), status.Uploaded, "completed",
		now, now, nil, false,
	)

	mock.ExpectQuery(regexp.QuoteMeta(`
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
	`)).
		WithArgs(userID).
		WillReturnRows(rows)

	avatars, err := repo.GetListUserAvatar(ctx, userID)
	require.NoError(t, err)
	assert.Len(t, avatars, 2)
	assert.Equal(t, "avatar1.jpg", avatars[0].FileName)
	assert.Equal(t, "avatar2.jpg", avatars[1].FileName)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestAvatarRepository_SetCurrentAvatar(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() {
		_ = db.Close()
	}()

	repo := repository.NewAvatarRepository(db)
	ctx := context.Background()

	userID := "user123"
	avatarID := uuid.New().String()

	// Начинаем транзакцию
	mock.ExpectBegin()

	// Проверяем существование аватарки
	mock.ExpectQuery(regexp.QuoteMeta(`
		SELECT 1
		FROM avatars
		WHERE id = $1
		  AND user_id = $2
		  AND deleted_at IS NULL
	`)).
		WithArgs(avatarID, userID).
		WillReturnRows(sqlmock.NewRows([]string{"1"}).AddRow(1))

	// Снимаем current у всех аватарок
	mock.ExpectExec(regexp.QuoteMeta(`
		UPDATE avatars
		SET
			is_current = false,
			updated_at = NOW()
		WHERE user_id = $1
		  AND deleted_at IS NULL
		  AND is_current = true
	`)).
		WithArgs(userID).
		WillReturnResult(sqlmock.NewResult(0, 1))

	// Ставим current у выбранной аватарки
	mock.ExpectExec(regexp.QuoteMeta(`
		UPDATE avatars
		SET
			is_current = true,
			updated_at = NOW()
		WHERE id = $1
		  AND user_id = $2
		  AND deleted_at IS NULL
	`)).
		WithArgs(avatarID, userID).
		WillReturnResult(sqlmock.NewResult(0, 1))

	mock.ExpectCommit()

	err = repo.SetCurrentAvatar(ctx, userID, avatarID)
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestAvatarRepository_DeleteCurrentUserAvatar(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() {
		_ = db.Close()
	}()

	repo := repository.NewAvatarRepository(db)
	ctx := context.Background()

	userID := "user123"

	mock.ExpectExec(regexp.QuoteMeta(`
		UPDATE avatars
		SET is_current = FALSE
		WHERE user_id = $1
		  AND is_current
		  AND deleted_at IS NULL
	`)).
		WithArgs(userID).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = repo.DeleteCurrentUserAvatar(ctx, userID)
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestAvatarRepository_SoftDeleteAvatar(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() {
		_ = db.Close()
	}()

	repo := repository.NewAvatarRepository(db)
	ctx := context.Background()

	avatarID := uuid.New().String()

	mock.ExpectExec(regexp.QuoteMeta(`
		UPDATE avatars
		SET
			deleted_at = NOW(),
			is_current = FALSE,
			updated_at = NOW()
		WHERE id = $1
		  AND deleted_at IS NULL
	`)).
		WithArgs(avatarID).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = repo.SoftDeleteAvatar(ctx, avatarID)
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}
