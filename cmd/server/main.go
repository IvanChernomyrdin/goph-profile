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
	logging "goph-profile-avatars/internal/logging"
	routerhttp "goph-profile-avatars/internal/net/http"
	"goph-profile-avatars/internal/repository"
	"goph-profile-avatars/internal/services"
)

func main() {
	ctx := context.Background()

	// Инициализация slog + OTEL
	logger, otelShutdown := logging.InitLoggerProvider(ctx)
	defer otelShutdown()
	// Вместо sugar := logger.Sugar()
	logger.Info("Starting gophprofile server...")

	// подключаем переменные окружения для сервака
	cfg, err := config.Load("./configs/server.yaml")
	if err != nil {
		logger.Error("failed to load config", "error", err)
		return
	}

	// подключаем PostgreSQL для метаданных
	if err := config.PostgresInit(cfg.Postgres.DSN); err != nil {
		logger.Error("failed to init Postgres", "error", err)
		return
	}

	// подключаем MinIO/AWS S3 для хранения файлов
	if err := config.MinIOAWSInit(cfg.S3); err != nil {
		logger.Error("failed to init MinIO/S3", "error", err)
		return
	}

	// подключаем rabbitMQ
	if err := config.RabbitMQInit(cfg.RabbitMQ); err != nil {
		logger.Error("failed to init RabbitMQ", "error", err)
		return
	}
	defer func() {
		if err := config.CloseRabbitMQ(); err != nil {
			logger.Error("rabbitmq close error", "error", err)
		}
	}()

	// запускаем сервис
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

	// запускаем chi роутер
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
	// формирует строку подключения >> хост:порт
	addr := cfg.App.Host + ":" + cfg.App.Port
	logger.Info("server started", "addr", addr)

	srv := &http.Server{
		Addr:              addr,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	// канал для сигналов
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	// запуск сервера в отдельной горутине
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("listen error", "error", err)
		}
	}()

	// ждем сигнал
	<-stop
	logger.Info("shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("server forced to shutdown", "error", err)
	}

	logger.Info("server exiting")
}
