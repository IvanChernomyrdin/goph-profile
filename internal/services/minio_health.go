package services

import (
	"context"

	"github.com/minio/minio-go/v7"
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
	_, err := s.client.ListBuckets(ctx)
	return err
}
