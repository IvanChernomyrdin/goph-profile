package test

import (
	"context"
	"testing"
	"time"

	"goph-profile-avatars/internal/config"

	"github.com/stretchr/testify/require"
)

// Мок-конфиг для теста
var testCfg = config.S3Config{
	Endpoint:  "play.min.io", // публичный MinIO тестовый сервер
	AccessKey: "Q3AM3UQ867SPQQA43P2F",
	SecretKey: "zuf+tfteSlswRu7BJ86wekitnifILbZam1KYY3TG",
	Bucket:    "test-bucket",
	Region:    "us-east-1",
	UseSSL:    true,
}

func TestMinIOAWSInit(t *testing.T) {
	err := config.MinIOAWSInit(testCfg)
	require.NoError(t, err, "MinIOAWSInit should not return an error")

	client := config.GetMinIOClient()
	require.NotNil(t, client, "MinIO client should be initialized")

	// Проверяем существование bucket
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	exists, err := client.BucketExists(ctx, testCfg.Bucket)
	require.NoError(t, err)
	require.True(t, exists, "Bucket should exist")
}
