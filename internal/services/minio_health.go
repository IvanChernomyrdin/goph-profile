package services

import (
	"context"
	"log/slog"

	"github.com/minio/minio-go/v7"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

// MinioClient интерфейс для minio клиента
type MinioClient interface {
	ListBuckets(ctx context.Context) ([]minio.BucketInfo, error)
}

type MinIOHealthService struct {
	client MinioClient
}

func NewMinIOHealthService(client MinioClient) *MinIOHealthService {
	return &MinIOHealthService{
		client: client,
	}
}

func (s *MinIOHealthService) Check(ctx context.Context) error {
	ctx, span := otel.Tracer("avatar-service/minio-health").Start(ctx, "check")
	defer span.End()

	span.SetAttributes(
		attribute.String("storage.system", "minio"),
	)

	logger := slog.With("service", "minio-health", "trace_id", span.SpanContext().TraceID())

	_, err := s.client.ListBuckets(ctx)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "check minio")
		logger.Error("failed to check minio", "error", err)
	}

	return err
}
