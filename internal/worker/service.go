package worker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"path"
	"time"

	"github.com/disintegration/imaging"

	"goph-profile-avatars/internal/metrics"
	"goph-profile-avatars/internal/repository"
	"goph-profile-avatars/internal/services"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

type AvatarRepositoryInterface interface {
	GetAvatarByID(ctx context.Context, avatarID string) (*repository.Avatar, error)
	UpdateProcessingStatus(ctx context.Context, avatarID, status string) error
	FailProcessing(ctx context.Context, avatarID string) error
	CompleteProcessing(ctx context.Context, avatarID string, thumbnailKeys []byte) error
	SoftDeleteAvatar(ctx context.Context, avatarID string) error
}

type StorageInterface interface {
	Download(ctx context.Context, key string) (*services.DownloadResult, error)
	Upload(ctx context.Context, key string, body io.Reader, size int64, contentType string) error
	Delete(ctx context.Context, key string) error
}

type Service struct {
	avatarRepo AvatarRepositoryInterface
	storage    StorageInterface
	log        *slog.Logger
}

func NewAvatarWorkerService(
	avatarRepo AvatarRepositoryInterface,
	storage StorageInterface,
	log *slog.Logger,
) *Service {
	return &Service{
		avatarRepo: avatarRepo,
		storage:    storage,
		log:        log,
	}
}

func (s *Service) HandleUpload(ctx context.Context, event AvatarUploadEvent) error {
	ctx, span := otel.Tracer("avatar-worker").Start(ctx, "handle-upload")
	defer span.End()

	start := time.Now()

	logger := logWithTrace(ctx, s.log).With("avatar_id", event.AvatarID, "user_id", event.UserID)

	logger.Info("handle upload started")

	avatar, err := s.avatarRepo.GetAvatarByID(ctx, event.AvatarID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "get avatar failed")

		metrics.WorkerJobsTotal.WithLabelValues("upload", "error").Inc()
		metrics.WorkerJobDuration.WithLabelValues("upload", "error").Observe(time.Since(start).Seconds())

		return fmt.Errorf("get avatar by id: %w", err)
	}

	switch avatar.ProcessingStatus {
	case "completed":
		logger.Info("avatar already completed")
		return nil
	case "processing":
		logger.Info("avatar already processing")
		return nil
	}

	if err := s.avatarRepo.UpdateProcessingStatus(ctx, avatar.ID, "processing"); err != nil {
		span.RecordError(err)
		return fmt.Errorf("update processing status to processing: %w", err)
	}

	if err := s.processAvatar(ctx, avatar, logger); err != nil {
		span.RecordError(err)

		if failErr := s.avatarRepo.FailProcessing(ctx, avatar.ID); failErr != nil {
			logger.Error("failed to set failed status", "error", failErr)
		}

		metrics.WorkerJobsTotal.WithLabelValues("upload", "error").Inc()
		metrics.WorkerJobDuration.WithLabelValues("upload", "error").Observe(time.Since(start).Seconds())

		return err
	}

	duration := time.Since(start).Seconds()
	metrics.WorkerJobsTotal.WithLabelValues("upload", "success").Inc()
	metrics.WorkerJobDuration.WithLabelValues("upload", "success").Observe(duration)

	logger.Info("handle upload finished", "duration_sec", duration)

	return nil
}

func (s *Service) HandleDelete(ctx context.Context, event AvatarDeleteEvent) error {
	ctx, span := otel.Tracer("avatar-worker").Start(ctx, "handle-delete")
	defer span.End()

	start := time.Now()

	logger := logWithTrace(ctx, s.log).With("avatar_id", event.AvatarID)

	logger.Info("handle delete started")

	avatar, err := s.avatarRepo.GetAvatarByID(ctx, event.AvatarID)
	if err != nil {
		if err == repository.ErrAvatarNotFound {
			logger.Info("not found")
			return nil
		}
		span.RecordError(err)

		metrics.WorkerJobsTotal.WithLabelValues("delete", "error").Inc()
		metrics.WorkerJobDuration.WithLabelValues("delete", "error").Observe(time.Since(start).Seconds())

		return fmt.Errorf("get avatar by id: %w", err)
	}

	if err := s.deleteAvatarFiles(ctx, avatar); err != nil {
		span.RecordError(err)

		metrics.WorkerJobsTotal.WithLabelValues("delete", "error").Inc()
		metrics.WorkerJobDuration.WithLabelValues("delete", "error").Observe(time.Since(start).Seconds())

		return fmt.Errorf("delete avatar files: %w", err)
	}

	if err := s.avatarRepo.SoftDeleteAvatar(ctx, avatar.ID); err != nil {
		span.RecordError(err)

		return fmt.Errorf("soft delete avatar in db: %w", err)
	}

	duration := time.Since(start).Seconds()

	metrics.WorkerJobsTotal.WithLabelValues("delete", "success").Inc()
	metrics.WorkerJobDuration.WithLabelValues("delete", "success").Observe(duration)

	logger.Info("handle delete finished", "duration_sec", duration)

	return nil
}

func (s *Service) processAvatar(ctx context.Context, avatar *repository.Avatar, logger *slog.Logger) error {
	ctx, span := otel.Tracer("avatar-worker").Start(ctx, "process-avatar")
	defer span.End()

	start := time.Now()

	span.SetAttributes(
		attribute.String("avatar_id", avatar.ID),
	)

	ctx, dlSpan := otel.Tracer("avatar-worker").Start(ctx, "s3_download")
	downloaded, err := s.storage.Download(ctx, avatar.S3Key)
	if err != nil {
		dlSpan.RecordError(err)
		dlSpan.SetStatus(codes.Error, "download failed")
		metrics.WorkerFailuresTotal.WithLabelValues("download").Inc()
		return fmt.Errorf("download original from storage: %w", err)
	}
	dlSpan.End()

	metrics.WorkerProcessingStageDuration.WithLabelValues("download").Observe(time.Since(start).Seconds())

	defer func() {
		_ = downloaded.Reader.Close()
	}()

	start = time.Now()

	originalBytes, err := io.ReadAll(downloaded.Reader)
	if err != nil {
		span.RecordError(err)
		metrics.WorkerFailuresTotal.WithLabelValues("read").Inc()

		return fmt.Errorf("read original image: %w", err)
	}

	metrics.WorkerProcessingStageDuration.WithLabelValues("read").Observe(time.Since(start).Seconds())

	start = time.Now()

	srcImage, err := imaging.Decode(bytes.NewReader(originalBytes))
	if err != nil {
		span.RecordError(err)
		metrics.WorkerFailuresTotal.WithLabelValues("decode").Inc()

		return fmt.Errorf("decode image: %w", err)
	}
	metrics.WorkerProcessingStageDuration.WithLabelValues("decode").Observe(time.Since(start).Seconds())

	start = time.Now()

	thumb100 := imaging.Fill(srcImage, 100, 100, imaging.Center, imaging.Lanczos)
	thumb300 := imaging.Fill(srcImage, 300, 300, imaging.Center, imaging.Lanczos)

	metrics.WorkerProcessingStageDuration.WithLabelValues("resize").Observe(time.Since(start).Seconds())

	start = time.Now()

	thumb100Buf := new(bytes.Buffer)
	if err := imaging.Encode(thumb100Buf, thumb100, imaging.JPEG); err != nil {
		return fmt.Errorf("encode 100x100 thumbnail: %w", err)
	}

	thumb300Buf := new(bytes.Buffer)
	if err := imaging.Encode(thumb300Buf, thumb300, imaging.JPEG); err != nil {
		return fmt.Errorf("encode 300x300 thumbnail: %w", err)
	}

	metrics.WorkerProcessingStageDuration.WithLabelValues("encode").Observe(time.Since(start).Seconds())

	baseDir := path.Dir(avatar.S3Key)
	thumb100Key := fmt.Sprintf("%s/100x100.jpg", baseDir)
	thumb300Key := fmt.Sprintf("%s/300x300.jpg", baseDir)

	start = time.Now()

	ctx, up100Span := otel.Tracer("avatar-worker").Start(ctx, "upload_100")
	if err := s.storage.Upload(
		ctx,
		thumb100Key,
		bytes.NewReader(thumb100Buf.Bytes()),
		int64(thumb100Buf.Len()),
		"image/jpeg",
	); err != nil {
		up100Span.RecordError(err)
		up100Span.SetStatus(codes.Error, "upload 100 failed")
		metrics.WorkerFailuresTotal.WithLabelValues("upload_100").Inc()
		up100Span.End()
		return fmt.Errorf("upload 100x100 thumbnail: %w", err)
	}
	up100Span.End()

	ctx, up300Span := otel.Tracer("avatar-worker").Start(ctx, "upload_300")
	if err := s.storage.Upload(
		ctx,
		thumb300Key,
		bytes.NewReader(thumb300Buf.Bytes()),
		int64(thumb300Buf.Len()),
		"image/jpeg",
	); err != nil {
		up300Span.RecordError(err)
		up300Span.SetStatus(codes.Error, "upload 300 failed")
		metrics.WorkerFailuresTotal.WithLabelValues("upload_300").Inc()
		up300Span.End()
		return fmt.Errorf("upload 300x300 thumbnail: %w", err)
	}
	up300Span.End()

	metrics.WorkerProcessingStageDuration.WithLabelValues("upload").Observe(time.Since(start).Seconds())

	thumbnailKeys := map[string]string{
		"100x100": thumb100Key,
		"300x300": thumb300Key,
	}

	start = time.Now()

	thumbnailKeysJSON, err := json.Marshal(thumbnailKeys)
	if err != nil {
		return fmt.Errorf("marshal thumbnail keys: %w", err)
	}

	ctx, dbSpan := otel.Tracer("avatar-worker").Start(ctx, "db_complete")
	if err := s.avatarRepo.CompleteProcessing(ctx, avatar.ID, thumbnailKeysJSON); err != nil {
		dbSpan.RecordError(err)
		dbSpan.SetStatus(codes.Error, "db failed")
		metrics.WorkerFailuresTotal.WithLabelValues("db").Inc()
		return fmt.Errorf("complete processing: %w", err)
	}
	dbSpan.End()

	metrics.WorkerProcessingStageDuration.WithLabelValues("db").Observe(time.Since(start).Seconds())

	logger.Info("avatar processed successfully")

	return nil
}

func (s *Service) deleteAvatarFiles(ctx context.Context, avatar *repository.Avatar) error {
	if avatar.S3Key != "" {
		if err := s.storage.Delete(ctx, avatar.S3Key); err != nil {
			return fmt.Errorf("delete original file: %w", err)
		}
	}

	if len(avatar.ThumbnailS3Keys) == 0 {
		return nil
	}

	var thumbnailKeys map[string]string
	if err := json.Unmarshal(avatar.ThumbnailS3Keys, &thumbnailKeys); err != nil {
		return fmt.Errorf("unmarshal thumbnail keys: %w", err)
	}

	for size, key := range thumbnailKeys {
		if key == "" {
			continue
		}

		if err := s.storage.Delete(ctx, key); err != nil {
			return fmt.Errorf("delete thumbnail %s: %w", size, err)
		}
	}

	return nil
}
