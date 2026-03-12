package config

import (
	"context"
	"fmt"
	"time"

	logger "goph-profile-avatars/internal/logging"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

var minioClient *minio.Client

func MinIOAWSInit(cfg S3Config) error {
	customLog := logger.NewHTTPLogger().Logger.Sugar()

	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.UseSSL,
		Region: cfg.Region,
	})
	if err != nil {
		customLog.Errorf("failed to init minio client: %v", err)
		return fmt.Errorf("init minio client: %w", err)
	}

	// Делаем отдельный контекст с таймаутом на проверку и создание bucket.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Проверяем, существует ли bucket.
	exists, err := client.BucketExists(ctx, cfg.Bucket)
	if err != nil {
		customLog.Errorf("failed to check bucket %q: %v", cfg.Bucket, err)
		return fmt.Errorf("check bucket exists: %w", err)
	}

	// Если bucket нет — создаём.
	if !exists {
		err = client.MakeBucket(ctx, cfg.Bucket, minio.MakeBucketOptions{
			Region: cfg.Region,
		})
		if err != nil {
			customLog.Errorf("failed to create bucket %q: %v", cfg.Bucket, err)
			return fmt.Errorf("create bucket: %w", err)
		}

		customLog.Infof("minio bucket %q created successfully", cfg.Bucket)
	} else {
		customLog.Infof("minio bucket %q already exists", cfg.Bucket)
	}

	minioClient = client
	customLog.Info("minio connected successfully")

	return nil
}

func GetMinIOClient() *minio.Client {
	return minioClient
}
