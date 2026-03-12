package main

import (
	"net/http"

	apis "goph-profile-avatars/internal/api"
	"goph-profile-avatars/internal/config"
	logger "goph-profile-avatars/internal/logging"
	routerhttp "goph-profile-avatars/internal/net/http"
	"goph-profile-avatars/internal/repository"
	"goph-profile-avatars/internal/services"
)

func main() {
	// инициализировали логер
	sugar := logger.NewHTTPLogger().Logger.Sugar()
	// httpLogger := logger.NewHTTPLogger()

	// подключаем переменные окружения для сервака
	cfg, err := config.Load("./configs/server.yaml")
	if err != nil {
		sugar.Fatal(err)
	}

	// подключаем PostgreSQL для метаданных
	if err := config.PostgresInit(cfg.Postgres.DSN); err != nil {
		sugar.Fatal(err)
	}

	// подключаем MinIO/AWS S3 для хранения файлов
	if err := config.MinIOAWSInit(cfg.S3); err != nil {
		sugar.Fatal(err)
	}

	// подключаем rabbitMQ
	if err := config.RabbitMQInit(cfg.RabbitMQ); err != nil {
		sugar.Fatal(err)
	}
	defer func() {
		if err := config.CloseRabbitMQ(); err != nil {
			sugar.Errorf("rabbitmq close error: %v", err)
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

	router := routerhttp.NewRouter(handler)
	// формирует строку подключения >> хост:порт
	addr := cfg.App.Host + ":" + cfg.App.Port
	sugar.Infof("server started on %s", addr)

	if err := http.ListenAndServe(addr, router); err != nil {
		sugar.Fatal(err)
	}
}
