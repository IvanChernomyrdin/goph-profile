package services

import (
	"context"
	"fmt"
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

type DownloadResult struct {
	Reader      io.ReadCloser
	Size        int64
	ContentType string
}

// Download возвращает объект из MinIO.
// Закрывать reader должен вызывающий код.
func (s *MinIOStorage) Download(ctx context.Context, key string) (*DownloadResult, error) {
	obj, err := s.client.GetObject(ctx, s.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, err
	}

	info, err := obj.Stat()
	if err != nil {
		_ = obj.Close()
		return nil, err
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
}

func (s *MinIOStorage) Delete(ctx context.Context, objectKey string) error {
	err := s.client.RemoveObject(ctx, s.bucket, objectKey, minio.RemoveObjectOptions{})
	if err != nil {
		return fmt.Errorf("remove object %s: %w", objectKey, err)
	}
	return nil
}
