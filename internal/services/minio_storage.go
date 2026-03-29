package services

import (
	"context"
	"fmt"
	"goph-profile-avatars/internal/metrics"
	"io"
	"log/slog"
	"time"

	"github.com/minio/minio-go/v7"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
)

// MinioClientInterface интерфейс для minio клиента
type MinioClientInterface interface {
	PutObject(ctx context.Context, bucketName, objectName string, reader io.Reader, objectSize int64, opts minio.PutObjectOptions) (minio.UploadInfo, error)
	GetObject(ctx context.Context, bucketName, objectName string, opts minio.GetObjectOptions) (*minio.Object, error)
	RemoveObject(ctx context.Context, bucketName, objectName string, opts minio.RemoveObjectOptions) error
}

type MinIOStorage struct {
	client MinioClientInterface
	bucket string
}

func NewMinIOStorage(client MinioClientInterface, bucket string) *MinIOStorage {
	return &MinIOStorage{
		client: client,
		bucket: bucket,
	}
}

func (s *MinIOStorage) Upload(ctx context.Context, key string, body io.Reader, size int64, contentType string) error {
	ctx, span := otel.Tracer("avatars-service/minio-storage").Start(ctx, "upload")
	defer span.End()

	start := time.Now()
	logger := slog.With("service", "minio-storage", "key", key, "trace_id", span.SpanContext().TraceID())

	_, err := s.client.PutObject(ctx, s.bucket, key, body, size, minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "upload failed")
		logger.Error("failed to upload object", "error", err)
		metrics.UploadsTotal.WithLabelValues("error").Inc()
		return fmt.Errorf("upload object %s: %w", key, err)
	}

	duration := time.Since(start).Seconds()
	logger.Info("upload success", "duration_sec", duration)
	metrics.UploadsTotal.WithLabelValues("success").Inc()
	metrics.UploadDuration.WithLabelValues("success").Observe(duration)

	return err
}

type DownloadResult struct {
	Reader      io.ReadCloser
	Size        int64
	ContentType string
}

// Download возвращает объект из MinIO.
// Закрывать reader должен вызывающий код.
func (s *MinIOStorage) Download(ctx context.Context, key string) (*DownloadResult, error) {
	ctx, span := otel.Tracer("avatars-service/minio-storage").Start(ctx, "download")
	defer span.End()

	start := time.Now()
	logger := slog.With("service", "minio-storage", "key", key, "trace_id", span.SpanContext().TraceID())

	obj, err := s.client.GetObject(ctx, s.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "get object failed")
		logger.Error("failed to get object", "error", err)
		return nil, fmt.Errorf("get object %s: %w", key, err)
	}

	info, err := obj.Stat()
	if err != nil {
		_ = obj.Close()
		span.RecordError(err)
		span.SetStatus(codes.Error, "stat object failed")
		logger.Error("failed to stat object", "error", err)
		return nil, err
	}

	contentType := info.ContentType
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	duration := time.Since(start).Seconds()
	logger.Info("download success", "size", info.Size, "duration_sec", duration)
	metrics.DownloadsTotal.WithLabelValues("success").Inc()
	metrics.DownloadDuration.WithLabelValues("success").Observe(duration)

	return &DownloadResult{
		Reader:      obj,
		Size:        info.Size,
		ContentType: contentType,
	}, nil
}

func (s *MinIOStorage) Delete(ctx context.Context, objectKey string) error {
	ctx, span := otel.Tracer("avatars-service/minio-storage").Start(ctx, "delete")
	defer span.End()

	logger := slog.With("service", "minio-storage", "key", objectKey, "trace_id", span.SpanContext().TraceID())

	err := s.client.RemoveObject(ctx, s.bucket, objectKey, minio.RemoveObjectOptions{})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "delete failed")
		logger.Error("failed to delete object", "error", err)
		metrics.DeletesTotal.WithLabelValues("error").Inc()
		return fmt.Errorf("remove object %s: %w", objectKey, err)
	}

	logger.Info("delete success")
	metrics.DeletesTotal.WithLabelValues("success").Inc()
	return nil
}
