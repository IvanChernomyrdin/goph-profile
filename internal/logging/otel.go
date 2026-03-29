package logging

import (
	"context"
	"log"
	"log/slog"
	"time"

	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

func InitLoggerProvider(ctx context.Context) (*slog.Logger, func()) {
	// gRPC Exporter для логов
	exporter, err := otlploggrpc.New(ctx)
	if err != nil {
		log.Fatalf("failed to create OTLP log exporter: %v", err)
	}

	// Метаинформация о сервисе
	res, err := resource.New(ctx,
		resource.WithFromEnv(),
		resource.WithTelemetrySDK(),
		resource.WithAttributes(
			semconv.ServiceNameKey.String("gophprofile-server"),
			semconv.ServiceVersionKey.String("1.0.0"),
		),
	)
	if err != nil {
		log.Fatalf("failed to create resource: %v", err)
	}

	// LoggerProvider с batch processor
	loggerProvider := sdklog.NewLoggerProvider(
		sdklog.WithResource(res),
		sdklog.WithProcessor(sdklog.NewBatchProcessor(exporter)),
	)

	// Создаем slog.Handler через otelslog bridge
	handler := otelslog.NewHandler(
		"gophprofile-server",
		otelslog.WithLoggerProvider(loggerProvider),
	)

	// Создаем slog.Logger
	logger := slog.New(handler)
	slog.SetDefault(logger)

	shutdown := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := loggerProvider.Shutdown(ctx); err != nil {
			otel.Handle(err)
		}
	}

	return logger, shutdown
}
