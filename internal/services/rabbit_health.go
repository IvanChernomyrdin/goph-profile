package services

import (
	"context"
	"errors"
	"log/slog"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

// ConnectionInterface интерфейс для amqp.Connection
type ConnectionInterface interface {
	Channel() (*amqp.Channel, error)
	IsClosed() bool
	Close() error
}

type RabbitMQHealthService struct {
	conn ConnectionInterface
}

func NewRabbitMQHealthService(conn ConnectionInterface) *RabbitMQHealthService {
	return &RabbitMQHealthService{conn: conn}
}

func (s *RabbitMQHealthService) Check(ctx context.Context) error {
	_, span := otel.Tracer("avatars-service/rabbit-check").Start(ctx, "check")
	defer span.End()

	span.SetAttributes(
		attribute.String("storage.system", "rabbitMQ"),
	)

	logger := slog.With("service", "rabbitmq-health", "trace-id", span.SpanContext().TraceID())
	if s.conn == nil {
		err := errors.New("rabbitmq connection is empty")
		span.RecordError(err)
		span.SetStatus(codes.Error, "rabbitmq check is empty")
		logger.Error("failed to check rabbitmq", "error", err)
		return amqp.ErrClosed
	}

	if s.conn.IsClosed() {
		err := errors.New("rabbitmq connection is closed")
		span.RecordError(err)
		span.SetStatus(codes.Error, "rabbitmq check is closed")
		return amqp.ErrClosed
	}

	ch, err := s.conn.Channel()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "rabbitmq channel open failed")
		logger.Error("failed to open rabbitmq channel", "error", err)
		return err
	}
	defer func() {
		_ = ch.Close()
	}()

	return nil
}
