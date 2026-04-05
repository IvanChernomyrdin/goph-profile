package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"goph-profile-avatars/internal/config"
	"goph-profile-avatars/internal/logging"
	"goph-profile-avatars/internal/repository"
	"goph-profile-avatars/internal/services"
	"goph-profile-avatars/internal/worker"
)

func main() {
	ctx := context.Background()

	// подключаем переменные окружения для воркера
	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		configPath = "./configs/server.yaml"
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Errorf("failed to load config: %w, config_path: %w", err, configPath)
		return
	}

	// Инициализация slog + OTEL
	logger, shutdownObservability := logging.InitObservability(
		ctx,
		"gophprofile-worker",
		"1.0.0",
		fmt.Sprintf("%s:%d", cfg.Jaeger.Name, cfg.Jaeger.Port),
	)
	defer shutdownObservability()

	logger.Info("starting gophprofile worker")

	logger.Info("worker config loaded", "config_path", configPath)

	// подключаем PostgreSQL
	if err := config.PostgresInit(cfg.Postgres.DSN); err != nil {
		logger.Error("failed to init Postgres", "error", err)
		return
	}

	// подключаем MinIO / S3
	if err := config.MinIOAWSInit(cfg.S3); err != nil {
		logger.Error("failed to init MinIO/S3", "error", err)
		return
	}

	// подключаем RabbitMQ
	if err := config.RabbitMQInit(cfg.RabbitMQ); err != nil {
		logger.Error("failed to init RabbitMQ", "error", err)
		return
	}
	defer func() {
		if err := config.CloseRabbitMQ(); err != nil {
			logger.Error("rabbitmq close error", "error", err)
		}
	}()

	// зависимости воркера
	avatarRepo := repository.NewAvatarRepository(config.GetDB())
	storage := services.NewMinIOStorage(config.GetMinIOClient(), cfg.S3.Bucket)

	// сервис обработки изображений
	avatarWorkerService := worker.NewAvatarWorkerService(
		avatarRepo,
		storage,
		logger,
	)

	// consumer RabbitMQ
	consumer := worker.NewRabbitConsumer(
		config.GetRabbitChannel(),
		cfg.RabbitMQ,
		avatarWorkerService,
		avatarWorkerService,
		logger,
	)

	// graceful shutdown
	ctxShutdown, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	logger.Info(
		"worker started",
		"upload_queue", cfg.RabbitMQ.QueueUpload,
		"delete_queue", cfg.RabbitMQ.QueueDelete,
	)

	if err := consumer.Run(ctxShutdown); err != nil {
		logger.Error("consumer run error", "error", err)
	}

	logger.Info("worker stopped")
}
