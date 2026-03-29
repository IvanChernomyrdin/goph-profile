package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"goph-profile-avatars/internal/metrics"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"goph-profile-avatars/internal/api"
	"goph-profile-avatars/internal/repository"

	status "goph-profile-avatars/internal/config/status"

	"github.com/google/uuid"

	"log/slog"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

const maxAvatarSize = 10 << 20 // 10 MB

var (
	ErrUnauthorized   = fmt.Errorf("unauthorized access to avatar")
	ErrAlreadyDeleted = fmt.Errorf("avatar already deleted")
)

type avatarRepo interface {
	CreateAvatar(ctx context.Context, avatar repository.CreateAvatarParams) error
	// получение аватарки по ID
	GetAvatarByID(ctx context.Context, avatarID string) (*repository.Avatar, error)
	// получение главной аватарки пользователя
	GetUserAvatar(ctx context.Context, userID string) (*repository.Avatar, error)
	// получение всех аватарок пользователя
	GetListUserAvatar(ctx context.Context, userID string) ([]repository.Avatar, error)
	// выставление главной аватарки пользователя
	SetCurrentAvatar(ctx context.Context, userID, avatarID string) error
	// удаление статуса текущей аватарки у пользователя
	DeleteCurrentUserAvatar(ctx context.Context, userID string) error
}

type objectStorage interface {
	Upload(ctx context.Context, key string, body io.Reader, size int64, contentType string) error
	Download(ctx context.Context, key string) (*DownloadResult, error)
}

type eventPublisher interface {
	PublishUploadEvent(ctx context.Context, event AvatarUploadEvent) error
	PublishDeleteEvent(ctx context.Context, event AvatarDeleteEvent) error
}

type AvatarService struct {
	repo      avatarRepo
	storage   objectStorage
	publisher eventPublisher
}

func NewAvatarService(
	repo avatarRepo,
	storage objectStorage,
	publisher eventPublisher,
) *AvatarService {
	return &AvatarService{
		repo:      repo,
		storage:   storage,
		publisher: publisher,
	}
}

// загрузка аватарки, сохранение в бд методанных, сохранение в minio оригинала, отправка в rabbitmq уведомление для воркера
func (s *AvatarService) UploadAvatar(ctx context.Context, input api.UploadAvatarInput) (*api.UploadAvatarResult, error) {
	ctx, span := otel.Tracer("avatar-service").Start(ctx, "upload-avatar")
	defer span.End()

	start := time.Now()

	logger := newLogger(ctx, input.UserID)

	logger.Info("upload started",
		"user_id", input.UserID,
		"file_name", input.FileName,
	)

	data, err := io.ReadAll(input.File)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "read file failed")

		metrics.UploadsTotal.WithLabelValues("error", input.UserID).Inc()
		metrics.UploadDuration.WithLabelValues("error").Observe(time.Since(start).Seconds())

		logger.Error("failed to read file", "error", err)

		return nil, fmt.Errorf("read file: %w", err)
	}

	if len(data) == 0 {
		span.RecordError(err)
		span.SetStatus(codes.Error, "empty file")

		metrics.UploadsTotal.WithLabelValues("error", input.UserID).Inc()
		metrics.UploadDuration.WithLabelValues("error").Observe(time.Since(start).Seconds())

		logger.Error("empty file")

		return nil, fmt.Errorf("empty file")
	}

	if int64(len(data)) > maxAvatarSize {
		err := fmt.Errorf("file too large")

		span.RecordError(err)
		span.SetStatus(codes.Error, "file too large")

		metrics.UploadsTotal.WithLabelValues("error", input.UserID).Inc()
		metrics.UploadDuration.WithLabelValues("error").Observe(time.Since(start).Seconds())

		logger.Error("file too large", "size", len(data))

		return nil, err
	}

	span.SetAttributes(
		attribute.String("user_id", input.UserID),
		attribute.String("file_name", input.FileName),
		attribute.Int64("file_size", int64(len(data))),
	)

	mimeType := http.DetectContentType(data[:min(len(data), 512)])
	if !isAllowedImageMime(mimeType) {
		err := fmt.Errorf("invalid file format")

		span.RecordError(err)
		span.SetStatus(codes.Error, "invalid mime")

		metrics.UploadsTotal.WithLabelValues("error", input.UserID).Inc()
		metrics.UploadDuration.WithLabelValues("error").Observe(time.Since(start).Seconds())

		logger.Error("invalid mime type", "mime", mimeType)

		return nil, err
	}

	avatarID := uuid.NewString()
	ext := normalizeExtension(input.FileName, mimeType)
	s3Key := fmt.Sprintf("avatars/%s/original%s", avatarID, ext)

	ctx, s3Span := otel.Tracer("avatar-service").Start(ctx, "s3_upload")
	err = s.storage.Upload(
		ctx,
		s3Key,
		bytes.NewReader(data),
		int64(len(data)),
		mimeType,
	)
	s3Span.End()

	if err != nil {
		s3Span.RecordError(err)
		s3Span.SetStatus(codes.Error, "s3 upload failed")

		span.RecordError(err)

		metrics.UploadsTotal.WithLabelValues("error", input.UserID).Inc()
		metrics.UploadDuration.WithLabelValues("error").Observe(time.Since(start).Seconds())

		logger.Error("failed to upload to storage", "error", err)

		return nil, fmt.Errorf("upload to minio: %w", err)
	}

	thumbnailKeys, err := json.Marshal(map[string]string{})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "marshal failed")

		metrics.UploadsTotal.WithLabelValues("error", input.UserID).Inc()
		metrics.UploadDuration.WithLabelValues("error").Observe(time.Since(start).Seconds())

		logger.Error("failed to marshal thumbnails", "error", err)

		return nil, fmt.Errorf("marshal thumbnails: %w", err)
	}

	ctx, dbSpan := otel.Tracer("avatar-service").Start(ctx, "db_create_avatar")
	err = s.repo.CreateAvatar(ctx, repository.CreateAvatarParams{
		ID:               avatarID,
		UserID:           input.UserID,
		FileName:         safeFileName(input.FileName),
		MimeType:         mimeType,
		SizeBytes:        int64(len(data)),
		S3Key:            s3Key,
		ThumbnailS3Keys:  thumbnailKeys,
		UploadStatus:     status.Uploaded,
		ProcessingStatus: status.Pending,
	})
	dbSpan.End()

	if err != nil {
		dbSpan.RecordError(err)
		dbSpan.SetStatus(codes.Error, "db insert failed")

		span.RecordError(err)

		metrics.UploadsTotal.WithLabelValues("error", input.UserID).Inc()
		metrics.UploadDuration.WithLabelValues("error").Observe(time.Since(start).Seconds())

		logger.Error("failed to save metadata", "error", err)

		return nil, fmt.Errorf("save avatar metadata: %w", err)
	}

	ctx, mqSpan := otel.Tracer("avatar-service").Start(ctx, "publish_event")
	err = s.publisher.PublishUploadEvent(ctx, AvatarUploadEvent{
		AvatarID: avatarID,
		UserID:   input.UserID,
		S3Key:    s3Key,
	})
	mqSpan.End()

	if err != nil {
		mqSpan.RecordError(err)
		mqSpan.SetStatus(codes.Error, "publish failed")

		span.RecordError(err)

		metrics.UploadsTotal.WithLabelValues("error", input.UserID).Inc()
		metrics.UploadDuration.WithLabelValues("error").Observe(time.Since(start).Seconds())

		logger.Error("failed to publish event", "error", err)

		return nil, fmt.Errorf("publish upload event: %w", err)
	}

	duration := time.Since(start).Seconds()
	metrics.UploadsTotal.WithLabelValues("success", input.UserID).Inc()
	metrics.UploadDuration.WithLabelValues("success").Observe(duration)

	logger.Info("upload complete",
		"user_id", input.UserID,
		"avatar_id", avatarID,
		"duration_sec", duration,
	)

	now := time.Now().UTC()

	return &api.UploadAvatarResult{
		ID:        avatarID,
		UserID:    input.UserID,
		URL:       "/api/v1/avatars/" + avatarID,
		Status:    "processing",
		CreatedAt: now.Format(time.RFC3339),
	}, nil
}

func isAllowedImageMime(mime string) bool {
	switch mime {
	case "image/jpeg", "image/png", "image/webp":
		return true
	default:
		return false
	}
}

func normalizeExtension(fileName, mime string) string {
	ext := strings.ToLower(filepath.Ext(fileName))

	switch mime {
	case "image/jpeg":
		if ext == ".jpg" || ext == ".jpeg" {
			return ext
		}
		return ".jpg"
	case "image/png":
		if ext == ".png" {
			return ext
		}
		return ".png"
	case "image/webp":
		if ext == ".webp" {
			return ext
		}
		return ".webp"
	default:
		if ext == "" {
			return ".bin"
		}
		return ext
	}
}

func safeFileName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "avatar"
	}
	return filepath.Base(name)
}

// получаем методанные из бд, дальше загружаем данные из minio и возвращаем аватарку
func (s *AvatarService) GetAvatarByID(ctx context.Context, avatarID, size string) (*api.GetAvatarResult, error) {
	ctx, span := otel.Tracer("avatar-service").Start(ctx, "get-avatar")
	defer span.End()

	start := time.Now()

	logger := newLogger(ctx, "").With("avatar_id", avatarID)

	logger.Info("get avatar started",
		"size", size,
	)

	span.SetAttributes(
		attribute.String("avatar_id", avatarID),
		attribute.String("size", size),
	)

	avatar, err := s.repo.GetAvatarByID(ctx, avatarID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "db fetch failed")

		logger.Error("failed to get avatar from db", "error", err)
		return nil, fmt.Errorf("get avatar by id: %w", err)
	}

	if avatar == nil {
		err := fmt.Errorf("avatar not found")

		span.RecordError(err)
		span.SetStatus(codes.Error, "not found")

		logger.Warn("avatar not found")
		return nil, err
	}

	logger = logger.With("user_id", avatar.UserID)

	keyToDownload := avatar.S3Key

	if size != "" && size != "original" {
		thumbs := parseThumbnailKeys(avatar.ThumbnailS3Keys)

		if thumbKey, ok := thumbs[size]; ok && thumbKey != "" {
			keyToDownload = thumbKey

			logger.Info("using thumbnail",
				"size", size,
				"key", thumbKey,
			)
		} else {
			logger.Warn("thumbnail not found, fallback to original",
				"requested_size", size,
			)
		}
	}

	ctx, s3Span := otel.Tracer("avatar-service").Start(ctx, "s3_download")

	downloaded, err := s.storage.Download(ctx, keyToDownload)
	s3Span.End()

	if err != nil {
		s3Span.RecordError(err)
		s3Span.SetStatus(codes.Error, "download failed")

		span.RecordError(err)

		logger.Error("failed to download avatar", "error", err)
		return nil, fmt.Errorf("download avatar from storage: %w", err)
	}

	logger.Info("get avatar success",
		"duration_sec", time.Since(start).Seconds(),
	)

	return &api.GetAvatarResult{
		ID:        avatar.ID,
		UserID:    avatar.UserID,
		FileName:  avatar.FileName,
		MimeType:  downloaded.ContentType,
		SizeBytes: downloaded.Size,
		Reader:    downloaded.Reader,
	}, nil
}

// вспомогательная функция мы тянем из бд jsonb и парсим данные там лежат миниатюры 100х100 и 300х300
func parseThumbnailKeys(raw []byte) map[string]string {
	if len(raw) == 0 {
		return map[string]string{}
	}

	var result map[string]string
	if err := json.Unmarshal(raw, &result); err == nil && result != nil {
		return result
	}

	return map[string]string{}
}

// получаем одну главную аватарку пользователя
func (s *AvatarService) GetUserAvatar(ctx context.Context, userID string) (*api.GetAvatarResult, error) {
	ctx, span := otel.Tracer("avatar-service").Start(ctx, "get-user-avatar")
	defer span.End()

	start := time.Now()

	logger := newLogger(ctx, userID)

	logger.Info("get user avatar started",
		"user_id", userID,
	)

	span.SetAttributes(
		attribute.String("user_id", userID),
	)

	avatar, err := s.repo.GetUserAvatar(ctx, userID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "db fetch failed")

		logger.Error("failed to get user avatar from db", "error", err)
		return nil, fmt.Errorf("get user avatar: %w", err)
	}

	if avatar == nil {
		err := fmt.Errorf("user avatar not found")

		span.RecordError(err)
		span.SetStatus(codes.Error, "not found")

		logger.Warn("user avatar not found")
		return nil, err
	}

	ctx, s3Span := otel.Tracer("avatar-service").Start(ctx, "s3_download")

	downloaded, err := s.storage.Download(ctx, avatar.S3Key)
	s3Span.End()

	if err != nil {
		s3Span.RecordError(err)
		s3Span.SetStatus(codes.Error, "download failed")

		span.RecordError(err)

		logger.Error("failed to download avatar", "error", err)
		return nil, fmt.Errorf("download user avatar from storage: %w", err)
	}

	logger.Info("get user avatar success",
		"avatar_id", avatar.ID,
		"duration_sec", time.Since(start).Seconds(),
	)

	return &api.GetAvatarResult{
		ID:        avatar.ID,
		UserID:    avatar.UserID,
		FileName:  avatar.FileName,
		MimeType:  downloaded.ContentType,
		SizeBytes: downloaded.Size,
		Reader:    downloaded.Reader,
	}, nil
}

// получаем список всех аватарок пользователя
func (s *AvatarService) GetListUserAvatar(ctx context.Context, userID string) ([]api.AvatarItem, error) {
	ctx, span := otel.Tracer("avatar-service").Start(ctx, "get-list-user-avatar")
	defer span.End()

	span.SetAttributes(attribute.String("user_id", userID))

	start := time.Now()

	logger := newLogger(ctx, userID)

	logger.Info("get list avatars started")

	avatars, err := s.repo.GetListUserAvatar(ctx, userID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "db error")

		logger.Error("failed to get avatars", "error", err)
		return nil, fmt.Errorf("get list user avatar: %w", err)
	}

	result := make([]api.AvatarItem, 0, len(avatars))

	for _, avatar := range avatars {
		thumbs := parseThumbnailKeys(avatar.ThumbnailS3Keys)

		thumbnailURLs := map[string]string{}
		if _, ok := thumbs["100x100"]; ok {
			thumbnailURLs["100x100"] = "/api/v1/avatars/" + avatar.ID + "?size=100x100"
		}
		if _, ok := thumbs["300x300"]; ok {
			thumbnailURLs["300x300"] = "/api/v1/avatars/" + avatar.ID + "?size=300x300"
		}

		result = append(result, api.AvatarItem{
			ID:               avatar.ID,
			UserID:           avatar.UserID,
			FileName:         avatar.FileName,
			MimeType:         avatar.MimeType,
			SizeBytes:        avatar.SizeBytes,
			UploadStatus:     avatar.UploadStatus,
			ProcessingStatus: avatar.ProcessingStatus,
			CreatedAt:        avatar.CreatedAt.UTC().Format(time.RFC3339),
			UpdatedAt:        avatar.UpdatedAt.UTC().Format(time.RFC3339),
			URL:              "/api/v1/avatars/" + avatar.ID,
			IsCurrent:        avatar.IsCurrent,
			ThumbnailURLs:    thumbnailURLs,
		})
	}

	duration := time.Since(start).Seconds()

	logger.Info("get list avatars completed",
		"count", len(result),
		"duration_sec", duration,
	)

	return result, nil
}

func (s *AvatarService) UpdateCurrentAvatar(ctx context.Context, userID, avatarID string) error {
	ctx, span := otel.Tracer("avatar-service").Start(ctx, "update-current-avatar")
	defer span.End()

	start := time.Now()

	logger := newLogger(ctx, userID)

	logger.Info("update current avatar started")

	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	avatar, err := s.repo.GetAvatarByID(ctx, avatarID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "avatar not found")

		logger.Error("failed to get avatar", "error", err)
		return fmt.Errorf("get avatar by id: %w", err)
	}

	if avatar == nil {
		err := fmt.Errorf("avatar not found")

		span.RecordError(err)
		span.SetStatus(codes.Error, "not found")

		logger.Warn("avatar not found")
		return err
	}

	if avatar.UserID != userID {
		span.RecordError(ErrUnauthorized)
		span.SetStatus(codes.Error, "unauthorized")

		logger.Warn("avatar does not belong to user")

		return ErrUnauthorized
	}

	if err := s.repo.SetCurrentAvatar(ctx, userID, avatarID); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "update failed")

		logger.Error("failed to set current avatar", "error", err)
		return fmt.Errorf("set current avatar: %w", err)
	}

	logger.Info("update current avatar completed",
		"duration_sec", time.Since(start).Seconds(),
	)

	return nil
}

// отправка сообщения в брокер на удаление
func (s *AvatarService) DeleteAvatarByID(ctx context.Context, avatarID, userID string) error {
	ctx, span := otel.Tracer("avatar-service").Start(ctx, "delete-avatar-by-id")
	defer span.End()

	span.SetAttributes(
		attribute.String("user_id", userID),
		attribute.String("avatar_id", avatarID),
	)

	start := time.Now()

	logger := newLogger(ctx, userID)

	logger.Info("delete avatar started")

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	res, err := s.repo.GetAvatarByID(ctx, avatarID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "db error")

		logger.Error("failed to get avatar", "error", err)
		return fmt.Errorf("get avatar by id: %w", err)
	}

	if res == nil {
		err := fmt.Errorf("avatar not found")

		span.RecordError(err)
		span.SetStatus(codes.Error, "not found")

		logger.Warn("avatar not found")
		return err
	}

	if res.UserID != userID {
		span.RecordError(ErrUnauthorized)
		span.SetStatus(codes.Error, "unauthorized")

		logger.Warn("avatar does not belong to user")
		return ErrUnauthorized
	}

	if res.DeletedAt.Valid {
		span.RecordError(ErrAlreadyDeleted)
		span.SetStatus(codes.Error, "already deleted")

		logger.Warn("avatar already deleted")
		return ErrAlreadyDeleted
	}

	ctx, mqSpan := otel.Tracer("avatar-service").Start(ctx, "publish-delete-event")

	err = s.publisher.PublishDeleteEvent(ctx, AvatarDeleteEvent{
		AvatarID: res.ID,
		UserID:   res.UserID,
		S3Key:    res.S3Key,
	})

	mqSpan.End()

	if err != nil {
		mqSpan.RecordError(err)
		mqSpan.SetStatus(codes.Error, "publish failed")

		span.RecordError(err)

		logger.Error("failed to publish delete event", "error", err)
		return fmt.Errorf("publish delete event: %w", err)
	}

	logger.Info("delete avatar event published",
		"duration_sec", time.Since(start).Seconds(),
	)

	return nil
}

func (s *AvatarService) DeleteCurrentUserAvatar(ctx context.Context, userID string) error {
	ctx, span := otel.Tracer("avatar-service").Start(ctx, "delete-current-avatar")
	defer span.End()

	start := time.Now()

	logger := newLogger(ctx, userID)

	logger.Info("delete current avatar started")

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if err := s.repo.DeleteCurrentUserAvatar(ctx, userID); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "db error")

		logger.Error("failed to delete current avatar", "error", err)
		return err
	}

	logger.Info("delete current avatar completed",
		"duration_sec", time.Since(start).Seconds(),
	)

	return nil
}

func newLogger(ctx context.Context, userID string) *slog.Logger {
	return slog.With(
		"service", "avatar-service",
		"trace_id", trace.SpanFromContext(ctx).SpanContext().TraceID(),
		"user_id", userID,
	)
}
