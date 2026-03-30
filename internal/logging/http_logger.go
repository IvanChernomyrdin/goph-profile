package logging

import (
	"context"
	"log/slog"

	"go.opentelemetry.io/otel/trace"
)

func HTTPLoggerFromContext(ctx context.Context) *slog.Logger {
	spanCtx := trace.SpanFromContext(ctx).SpanContext()

	logger := slog.Default().With("component", "http")

	if spanCtx.IsValid() {
		logger = logger.With(
			"trace_id", spanCtx.TraceID().String(),
			"span_id", spanCtx.SpanID().String(),
		)
	}

	return logger
}
