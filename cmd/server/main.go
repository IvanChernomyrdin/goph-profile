package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	apis "goph-profile-avatars/internal/api"
	"goph-profile-avatars/internal/config"
	"goph-profile-avatars/internal/logging"
	routerhttp "goph-profile-avatars/internal/net/http"
	"goph-profile-avatars/internal/repository"
	"goph-profile-avatars/internal/services"
)

// @title GophProfile Avatar Service API
// @version 1.0
// @description API микросервиса управления аватарками пользователей.
// @description
// @description Возможности сервиса:
// @description - загрузка аватарки
// @description - получение текущей аватарки пользователя
// @description - получение списка аватарок пользователя
// @description - получение аватарки по ID
// @description - установка аватарки как основной
// @description - удаление аватарки
// @description - health checks и metrics
// @schemes http
func main() {
	appCtx := context.Background()

	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		configPath = "./configs/server.yaml"
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Printf("failed to load config: %v, config_path: %s\n", err, configPath)
		return
	}

	logger, shutdownObservability := logging.InitObservability(
		appCtx,
		"gophprofile-server",
		"1.0.0",
		fmt.Sprintf("%s:%d", cfg.Jaeger.Name, cfg.Jaeger.Port),
	)
	defer shutdownObservability()

	logger.Info("starting gophprofile server")

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

	var isReady atomic.Bool
	isReady.Store(true)

	healthService := apis.NewHealthService(
		config.GetDB(),
		services.NewMinIOHealthService(config.GetMinIOClient(), logger),
		services.NewRabbitMQHealthService(config.GetRabbitConn(), logger),
	)

	avatarRepo := repository.NewAvatarRepository(config.GetDB())
	storage := services.NewMinIOStorage(config.GetMinIOClient(), cfg.S3.Bucket, logger)
	publisher := services.NewRabbitPublisher(
		config.GetRabbitChannel(),
		cfg.RabbitMQ.Exchange,
		cfg.RabbitMQ.UploadRoutingKey,
		cfg.RabbitMQ.DeleteRoutingKey,
		logger,
	)

	avatarService := services.NewAvatarService(
		avatarRepo,
		storage,
		publisher,
	)

	handler := apis.NewHandler(
		healthService,
		avatarService,
		&isReady,
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

	serverErr := make(chan error, 1)

	go func() {
		logger.Info("server started", "addr", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErr <- err
		}
	}()

	select {
	case sig := <-stop:
		logger.Info("received shutdown signal", "signal", sig.String())
	case err := <-serverErr:
		logger.Error("server failed", "error", err)
		return
	}

	logger.Info("marking server as not ready")
	isReady.Store(false)

	time.Sleep(5 * time.Second)

	logger.Info("shutting down server")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("server forced to shutdown", "error", err)
	}

	logger.Info("server exiting")
}
