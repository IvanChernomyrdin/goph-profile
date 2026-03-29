package test

import (
	"os"
	"path/filepath"
	"testing"

	"goph-profile-avatars/internal/config"
)

func TestLoadConfig(t *testing.T) {
	// создаем временный YAML-файл
	yamlContent := `
app:
  name: test-app
  env: test
  host: 127.0.0.1
  port: "8080"

postgres:
  dsn: "postgres://user:pass@localhost:5432/dbname?sslmode=disable"

s3:
  endpoint: "localhost:9000"
  access_key: "minio"
  secret_key: "minio123"
  bucket: "avatars"
  use_ssl: false
  region: "us-east-1"

rabbitmq:
  url: "amqp://guest:guest@localhost:5672/"
  exchange: "avatars-exchange"
  exchange_type: "direct"
  upload_routing_key: "upload"
  delete_routing_key: "delete"
  queue_upload: "avatars-upload"
  queue_delete: "avatars-delete"
`

	tmpFile := filepath.Join(os.TempDir(), "test_config.yaml")
	if err := os.WriteFile(tmpFile, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("failed to write temp config: %v", err)
	}
	defer func() {
		_ = os.Remove(tmpFile)
	}()
	// вызываем Load
	cfg, err := config.Load(tmpFile)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	// проверяем поля
	if cfg.App.Name != "test-app" {
		t.Errorf("expected App.Name to be 'test-app', got %s", cfg.App.Name)
	}
	if cfg.Postgres.DSN != "postgres://user:pass@localhost:5432/dbname?sslmode=disable" {
		t.Errorf("unexpected Postgres.DSN: %s", cfg.Postgres.DSN)
	}
	if cfg.S3.Bucket != "avatars" {
		t.Errorf("unexpected S3.Bucket: %s", cfg.S3.Bucket)
	}
	if cfg.RabbitMQ.QueueUpload != "avatars-upload" {
		t.Errorf("unexpected RabbitMQ.QueueUpload: %s", cfg.RabbitMQ.QueueUpload)
	}
}
