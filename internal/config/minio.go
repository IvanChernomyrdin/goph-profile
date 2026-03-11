package config

import (
	"context"

	logger "goph-profile-avatars/internal/logging"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

var MinIOClient *minio.Client

func MinIOAWSInit(cfg S3Config) error {
	customLog := logger.NewHTTPLogger().Logger.Sugar()

	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.UseSSL,
		Region: cfg.Region,
	})
	if err != nil {
		customLog.Errorf("error connect minio: %v", err)
		return err
	}

	ctx := context.Background()

	exists, err := client.BucketExists(ctx, cfg.Bucket)
	if err != nil {
		customLog.Errorf("error checking bucket: %v", err)
		return err
	}

	if !exists {
		err = client.MakeBucket(ctx, cfg.Bucket, minio.MakeBucketOptions{
			Region: cfg.Region,
		})
		if err != nil {
			customLog.Errorf("error creating bucket: %v", err)
			return err
		}
		customLog.Infof("bucket %s created", cfg.Bucket)
	}

	MinIOClient = client
	customLog.Info("minio connected successfully")
	return nil
}

func GetMinIOClient() *minio.Client {
	return MinIOClient
}
