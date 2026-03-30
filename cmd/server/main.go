package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	apis "goph-profile-avatars/internal/api"
	"goph-profile-avatars/internal/config"
	"goph-profile-avatars/internal/logging"
	routerhttp "goph-profile-avatars/internal/net/http"
	"goph-profile-avatars/internal/repository"
	"goph-profile-avatars/internal/services"
)

func main() {
	ctx := context.Background()

	logger, shutdownObservability := logging.InitObservability(
		ctx,
		"gophprofile-server",
		"1.0.0",
	)
	defer shutdownObservability()

	logger.Info("starting gophprofile server")

	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		configPath = "./configs/server.yaml"
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		logger.Error("failed to load config", "error", err, "config_path", configPath)
		return
	}

	logger.Info("server config loaded", "config_path", configPath)

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

	healthService := apis.NewHealthService(
		config.GetDB(),
		services.NewMinIOHealthService(config.GetMinIOClient()),
		services.NewRabbitMQHealthService(config.GetRabbitConn()),
	)

	avatarRepo := repository.NewAvatarRepository(config.GetDB())
	storage := services.NewMinIOStorage(config.GetMinIOClient(), cfg.S3.Bucket)
	publisher := services.NewRabbitPublisher(
		config.GetRabbitChannel(),
		cfg.RabbitMQ.Exchange,
		cfg.RabbitMQ.UploadRoutingKey,
		cfg.RabbitMQ.DeleteRoutingKey,
	)

	avatarService := services.NewAvatarService(
		avatarRepo,
		storage,
		publisher,
	)

	handler := apis.NewHandler(
		healthService,
		avatarService,
	)

	router := routerhttp.NewRouter(handler, cfg.RateLimit.RequestPerMinute)

	addr := cfg.App.Host + ":" + cfg.App.Port
	logger.Info("server configured", "addr", addr)

	srv := &http.Server{
		Addr:              addr,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		logger.Info("server started", "addr", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("listen error", "error", err)
		}
	}()

	<-stop
	logger.Info("shutting down server")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("server forced to shutdown", "error", err)
	}

	logger.Info("server exiting")
}
