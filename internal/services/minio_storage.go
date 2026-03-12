package services

import (
	"context"
	"io"

	"github.com/minio/minio-go/v7"
)

type MinIOStorage struct {
	client *minio.Client
	bucket string
}

func NewMinIOStorage(client *minio.Client, bucket string) *MinIOStorage {
	return &MinIOStorage{
		client: client,
		bucket: bucket,
	}
}

func (s *MinIOStorage) Upload(ctx context.Context, key string, body io.Reader, size int64, contentType string) error {
	_, err := s.client.PutObject(ctx, s.bucket, key, body, size, minio.PutObjectOptions{
		ContentType: contentType,
	})
	return err
}

// Download возвращает объект из MinIO.
// Закрывать reader должен вызывающий код.
func (s *MinIOStorage) Download(ctx context.Context, key string) (io.ReadCloser, error) {
	obj, err := s.client.GetObject(ctx, s.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, err
	}

	if _, err := obj.Stat(); err != nil {
		_ = obj.Close()
		return nil, err
	}

	return obj, nil
}
