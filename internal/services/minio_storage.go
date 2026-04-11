package services

import (
	"context"
	"fmt"
	"goph-profile-avatars/internal/metrics"
	"goph-profile-avatars/internal/resilience"
	"io"
	"log/slog"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/sony/gobreaker"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

// MinioClientInterface интерфейс для minio клиента
type MinioClientInterface interface {
	PutObject(ctx context.Context, bucketName, objectName string, reader io.Reader, objectSize int64, opts minio.PutObjectOptions) (minio.UploadInfo, error)
	GetObject(ctx context.Context, bucketName, objectName string, opts minio.GetObjectOptions) (*minio.Object, error)
	RemoveObject(ctx context.Context, bucketName, objectName string, opts minio.RemoveObjectOptions) error
}

type MinIOStorage struct {
	client     MinioClientInterface
	bucket     string
	uploadCB   *gobreaker.CircuitBreaker
	downloadCB *gobreaker.CircuitBreaker
	deleteCB   *gobreaker.CircuitBreaker
	checkCB    *gobreaker.CircuitBreaker
}

func NewMinIOStorage(client MinioClientInterface, bucket string, logger *slog.Logger) *MinIOStorage {
	return &MinIOStorage{
		client:     client,
		bucket:     bucket,
		uploadCB:   resilience.New("minio-upload", logger),
		downloadCB: resilience.New("minio-download", logger),
		deleteCB:   resilience.New("minio-delete", logger),
		checkCB:    resilience.New("minio-check", logger),
	}
}

func (s *MinIOStorage) Upload(ctx context.Context, key string, body io.Reader, size int64, contentType string) error {
	ctx, span := otel.Tracer("avatars-service/minio-storage").Start(ctx, "upload")
	defer span.End()

	span.SetAttributes(
		attribute.String("storage.system", "minio"),
		attribute.String("bucket.name", s.bucket),
		attribute.String("object.key", key),
		attribute.Int64("object.size", size),
		attribute.String("content_type", contentType),
	)

	logger := slog.With("service", "minio-storage", "key", key, "trace_id", span.SpanContext().TraceID())

	_, err := s.uploadCB.Execute(func() (any, error) {
		_, err := s.client.PutObject(ctx, s.bucket, key, body, size, minio.PutObjectOptions{
			ContentType: contentType,
		})
		return nil, err
	})

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "upload failed")
		logger.Error("failed to upload object", "error", err)
		metrics.UploadsTotal.WithLabelValues("error").Inc()
		return fmt.Errorf("upload object %s: %w", key, err)
	}

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

	span.SetAttributes(
		attribute.String("storage.system", "minio"),
		attribute.String("bucket.name", s.bucket),
		attribute.String("object.key", key),
	)

	start := time.Now()
	logger := slog.With("service", "minio-storage", "key", key, "trace_id", span.SpanContext().TraceID())

	v, err := s.downloadCB.Execute(func() (any, error) {
		obj, err := s.client.GetObject(ctx, s.bucket, key, minio.GetObjectOptions{})
		if err != nil {
			return nil, fmt.Errorf("get object %s: %w", key, err)
		}

		info, err := obj.Stat()
		if err != nil {
			_ = obj.Close()
			return nil, fmt.Errorf("stat object %s: %w", key, err)
		}

		contentType := info.ContentType
		if contentType == "" {
			contentType = "application/octet-stream"
		}

		return &DownloadResult{
			Reader:      obj,
			Size:        info.Size,
			ContentType: contentType,
		}, nil
	})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "download failed")
		logger.Error("failed to download object", "error", err)
		metrics.DownloadsTotal.WithLabelValues("error").Inc()
		metrics.DownloadDuration.WithLabelValues("error").Observe(time.Since(start).Seconds())
		return nil, err
	}

	result := v.(*DownloadResult)

	duration := time.Since(start).Seconds()
	logger.Info("download success", "size", result.Size, "duration_sec", duration)
	metrics.DownloadsTotal.WithLabelValues("success").Inc()
	metrics.DownloadDuration.WithLabelValues("success").Observe(duration)

	return result, nil
}

func (s *MinIOStorage) Delete(ctx context.Context, objectKey string) error {
	ctx, span := otel.Tracer("avatars-service/minio-storage").Start(ctx, "delete")
	defer span.End()

	span.SetAttributes(
		attribute.String("storage.system", "minio"),
		attribute.String("bucket.name", s.bucket),
		attribute.String("object.key", objectKey),
	)

	logger := slog.With("service", "minio-storage", "key", objectKey, "trace_id", span.SpanContext().TraceID())

	_, err := s.deleteCB.Execute(func() (any, error) {
		err := s.client.RemoveObject(ctx, s.bucket, objectKey, minio.RemoveObjectOptions{})
		return nil, err
	})
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
