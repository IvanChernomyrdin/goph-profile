package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"goph-profile-avatars/internal/config"
	"goph-profile-avatars/internal/logging"
	"goph-profile-avatars/internal/repository"
	"goph-profile-avatars/internal/services"
	"goph-profile-avatars/internal/worker"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	ctx := context.Background()

	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		configPath = "./configs/server.yaml"
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Errorf("failed to load config: %w, config_path: %w", err, configPath)
		return
	}

	logger, shutdownObservability := logging.InitObservability(
		ctx,
		"gophprofile-worker",
		"1.0.0",
		fmt.Sprintf("%s:%d", cfg.Jaeger.Name, cfg.Jaeger.Port),
	)
	defer shutdownObservability()

	logger.Info("starting gophprofile worker")

	if err := config.PostgresInit(cfg.Postgres.DSN); err != nil {
		logger.Error("failed to init Postgres", "error", err)
		return
	}

	if err := config.MinIOAWSInit(cfg.S3); err != nil {
		logger.Error("failed to init MinIO/S3", "error", err)
		return
	}

	if err := config.RabbitMQInit(cfg.RabbitMQ); err != nil {
		logger.Error("failed to init RabbitMQ", "error", err)
		return
	}
	defer func() {
		if err := config.CloseRabbitMQ(); err != nil {
			logger.Error("rabbitmq close error", "error", err)
		}
	}()

	avatarRepo := repository.NewAvatarRepository(config.GetDB())
	storage := services.NewMinIOStorage(config.GetMinIOClient(), cfg.S3.Bucket)

	avatarWorkerService := worker.NewAvatarWorkerService(
		avatarRepo,
		storage,
		logger,
	)

	consumer := worker.NewRabbitConsumer(
		config.GetRabbitChannel(),
		cfg.RabbitMQ,
		avatarWorkerService,
		avatarWorkerService,
		logger,
	)

	metricsMux := http.NewServeMux()
	metricsMux.Handle("/metrics", promhttp.Handler())

	metricsSrv := &http.Server{
		Addr:              ":9091",
		Handler:           metricsMux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		logger.Info("worker metrics server started", "addr", ":9091")
		if err := metricsSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("worker metrics server error", "error", err)
		}
	}()

	ctxShutdown, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		<-ctxShutdown.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := metricsSrv.Shutdown(shutdownCtx); err != nil {
			logger.Error("worker metrics shutdown error", "error", err)
		}
	}()

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
