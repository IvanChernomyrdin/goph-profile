package services

import (
	"context"
	"goph-profile-avatars/internal/resilience"
	"log/slog"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/sony/gobreaker"
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
	conn  ConnectionInterface
	check *gobreaker.CircuitBreaker
}

func NewRabbitMQHealthService(conn ConnectionInterface, logger *slog.Logger) *RabbitMQHealthService {
	return &RabbitMQHealthService{
		conn:  conn,
		check: resilience.New("rabbitmq-health-check", logger),
	}
}

func (s *RabbitMQHealthService) Check(ctx context.Context) error {
	_, span := otel.Tracer("avatars-service/rabbit-check").Start(ctx, "check")
	defer span.End()

	span.SetAttributes(
		attribute.String("storage.system", "rabbitMQ"),
	)

	logger := slog.With("service", "rabbitmq-health", "trace-id", span.SpanContext().TraceID())
	_, err := s.check.Execute(func() (any, error) {
		if s.conn == nil {
			return nil, amqp.ErrClosed
		}

		if s.conn.IsClosed() {
			return nil, amqp.ErrClosed
		}

		ch, err := s.conn.Channel()
		if err != nil {
			return nil, err
		}
		defer func() {
			_ = ch.Close()
		}()

		return nil, nil
	})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "rabbitmq check failed")
		logger.Error("failed to check rabbitmq", "error", err)
		return err
	}
	return nil
}
