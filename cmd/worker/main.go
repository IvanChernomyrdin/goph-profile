package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"goph-profile-avatars/internal/config"
	logger "goph-profile-avatars/internal/logging"
	"goph-profile-avatars/internal/repository"
	"goph-profile-avatars/internal/services"
	"goph-profile-avatars/internal/worker"
)

func main() {
	// инициализировали логгер
	sugar := logger.NewHTTPLogger().Logger.Sugar()

	// подключаем переменные окружения для воркера
	cfg, err := config.Load("./configs/server.yaml")
	if err != nil {
		sugar.Fatal(err)
	}

	// подключаем PostgreSQL
	if err := config.PostgresInit(cfg.Postgres.DSN); err != nil {
		sugar.Fatal(err)
	}

	// подключаем MinIO / S3
	if err := config.MinIOAWSInit(cfg.S3); err != nil {
		sugar.Fatal(err)
	}

	// подключаем RabbitMQ
	if err := config.RabbitMQInit(cfg.RabbitMQ); err != nil {
		sugar.Fatal(err)
	}
	defer func() {
		if err := config.CloseRabbitMQ(); err != nil {
			sugar.Errorf("rabbitmq close error: %v", err)
		}
	}()

	// зависимости воркера
	avatarRepo := repository.NewAvatarRepository(config.GetDB())
	storage := services.NewMinIOStorage(config.GetMinIOClient(), cfg.S3.Bucket)

	// сервис обработки изображения/аватара
	avatarWorkerService := worker.NewAvatarWorkerService(
		avatarRepo,
		storage,
		sugar,
	)

	// consumer RabbitMQ
	consumer := worker.NewRabbitConsumer(
		config.GetRabbitChannel(),
		cfg.RabbitMQ,
		avatarWorkerService,
		sugar,
	)

	// graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	sugar.Infof("worker started, queue=%s", cfg.RabbitMQ.QueueUpload)

	if err := consumer.Run(ctx); err != nil {
		sugar.Fatal(err)
	}

	sugar.Info("worker stopped")
}
