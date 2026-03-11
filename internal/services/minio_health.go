package services

import (
	"context"

	"github.com/minio/minio-go/v7"
)

type MinIOHealthService struct {
	client *minio.Client
}

func NewMinIOHealthService(client *minio.Client) *MinIOHealthService {
	return &MinIOHealthService{
		client: client,
	}
}

func (s *MinIOHealthService) Check(ctx context.Context) error {
	_, err := s.client.ListBuckets(ctx)
	return err
}
