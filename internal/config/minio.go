package config

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

var minioClient *minio.Client

func MinIOAWSInit(cfg S3Config) error {
	logger := slog.Default().With(
		"component", "minio-init",
		"endpoint", cfg.Endpoint,
		"bucket", cfg.Bucket,
	)

	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.UseSSL,
		Region: cfg.Region,
	})
	if err != nil {
		logger.Error("failed to init minio", "error", err)
		return fmt.Errorf("init minio client: %w", err)
	}

	// Делаем отдельный контекст с таймаутом на проверку и создание bucket.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Проверяем, существует ли bucket.
	exists, err := client.BucketExists(ctx, cfg.Bucket)
	if err != nil {
		logger.Error("failed to check", "bucket", cfg.Bucket, "error", err)
		return fmt.Errorf("check bucket exists: %w", err)
	}

	// Если bucket нет — создаём.
	if !exists {
		err = client.MakeBucket(ctx, cfg.Bucket, minio.MakeBucketOptions{
			Region: cfg.Region,
		})
		if err != nil {
			logger.Error("failed to create bucket", "bucket", cfg.Bucket, "error", err)
			return fmt.Errorf("create bucket: %w", err)
		}

		logger.Info("minio bucket created successfully", "bucked", cfg.Bucket)
	} else {
		logger.Info("minio bucket already exists", "bucked", cfg.Bucket)
	}

	minioClient = client
	logger.Info("minio connected successfully")

	return nil
}

func GetMinIOClient() *minio.Client {
	return minioClient
}
