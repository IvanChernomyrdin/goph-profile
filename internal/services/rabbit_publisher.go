package services

import (
	"context"
	"encoding/json"
	"log/slog"

	"goph-profile-avatars/internal/metrics"
	"goph-profile-avatars/internal/resilience"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/sony/gobreaker"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

type ChannelInterface interface {
	PublishWithContext(ctx context.Context, exchange, key string, mandatory, immediate bool, msg amqp.Publishing) error
	Close() error
}

type AvatarUploadEvent struct {
	AvatarID string `json:"avatar_id"`
	UserID   string `json:"user_id"`
	S3Key    string `json:"s3_key"`
}

type AvatarDeleteEvent struct {
	AvatarID string `json:"avatar_id"`
	UserID   string `json:"user_id"`
	S3Key    string `json:"s3_key"`
}

type RabbitPublisher struct {
	ch               ChannelInterface
	exchange         string
	updateRoutingKey string
	deleteRoutingKey string
	uploadCB         *gobreaker.CircuitBreaker
	deleteCB         *gobreaker.CircuitBreaker
}

func NewRabbitPublisher(ch ChannelInterface,
	exchange, updateRoutingKey, deleteRoutingKey string,
	logger *slog.Logger,
) *RabbitPublisher {
	return &RabbitPublisher{
		ch:               ch,
		exchange:         exchange,
		updateRoutingKey: updateRoutingKey,
		deleteRoutingKey: deleteRoutingKey,
		uploadCB:         resilience.New("rabbitmq-publish-upload", logger),
		deleteCB:         resilience.New("rabbitmq-publish-delete", logger),
	}
}

type amqpHeaderCarrier amqp.Table

func (c amqpHeaderCarrier) Get(key string) string {
	v, ok := c[key]
	if !ok {
		return ""
	}
	s, _ := v.(string)
	return s
}

func (c amqpHeaderCarrier) Set(key, value string) {
	c[key] = value
}

func (c amqpHeaderCarrier) Keys() []string {
	keys := make([]string, 0, len(c))
	for k := range c {
		keys = append(keys, k)
	}
	return keys
}

func (p *RabbitPublisher) PublishUploadEvent(ctx context.Context, event AvatarUploadEvent) error {
	ctx, span := otel.Tracer("avatars-service/rabbit-publisher").Start(ctx, "publish-upload")
	defer span.End()

	span.SetAttributes(
		attribute.String("messaging.system", "rabbitmq"),
		attribute.String("messaging.destination.name", p.exchange),
		attribute.String("messaging.rabbitmq.routing_key", p.updateRoutingKey),
		attribute.String("event.type", "avatar.uploaded"),
		attribute.String("avatar.id", event.AvatarID),
		attribute.String("user.id", event.UserID),
	)

	logger := rabbitLogger(ctx).With(
		"exchange", p.exchange,
		"routing_key", p.updateRoutingKey,
		"event_type", "avatar.uploaded",
		"avatar_id", event.AvatarID,
		"user_id", event.UserID,
	)

	body, err := json.Marshal(event)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "marshal failed")
		metrics.RabbitPublishesTotal.WithLabelValues("avatar.uploaded", "error").Inc()
		logger.Error("failed to marshal upload event", "error", err)
		return err
	}

	headers := amqp.Table{}
	otel.GetTextMapPropagator().Inject(ctx, amqpHeaderCarrier(headers))

	_, err = p.uploadCB.Execute(func() (interface{}, error) {
		return nil, p.ch.PublishWithContext(
			ctx,
			p.exchange,
			p.updateRoutingKey,
			false,
			false,
			amqp.Publishing{
				ContentType: "application/json",
				Body:        body,
				Headers:     headers,
			},
		)
	})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "publish failed")
		metrics.RabbitPublishesTotal.WithLabelValues("avatar.uploaded", "error").Inc()
		logger.Error("failed to publish upload event", "error", err)
		return err
	}

	metrics.RabbitPublishesTotal.WithLabelValues("avatar.uploaded", "success").Inc()
	logger.Info("upload event published")
	return nil
}

func (p *RabbitPublisher) PublishDeleteEvent(ctx context.Context, event AvatarDeleteEvent) error {
	ctx, span := otel.Tracer("avatars-service/rabbit-publisher").Start(ctx, "publish-delete")
	defer span.End()

	span.SetAttributes(
		attribute.String("messaging.system", "rabbitmq"),
		attribute.String("messaging.destination.name", p.exchange),
		attribute.String("messaging.rabbitmq.routing_key", p.deleteRoutingKey),
		attribute.String("event.type", "avatar.deleted"),
		attribute.String("avatar.id", event.AvatarID),
		attribute.String("user.id", event.UserID),
	)

	logger := rabbitLogger(ctx).With(
		"exchange", p.exchange,
		"routing_key", p.deleteRoutingKey,
		"event_type", "avatar.deleted",
		"avatar_id", event.AvatarID,
		"user_id", event.UserID,
	)

	body, err := json.Marshal(event)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "marshal failed")
		metrics.RabbitPublishesTotal.WithLabelValues("avatar.deleted", "error").Inc()
		logger.Error("failed to marshal delete event", "error", err)
		return err
	}

	headers := amqp.Table{}
	otel.GetTextMapPropagator().Inject(ctx, amqpHeaderCarrier(headers))

	_, err = p.deleteCB.Execute(func() (interface{}, error) {
		return nil, p.ch.PublishWithContext(
			ctx,
			p.exchange,
			p.deleteRoutingKey,
			false,
			false,
			amqp.Publishing{
				ContentType: "application/json",
				Body:        body,
				Headers:     headers,
			},
		)
	})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "publish failed")
		metrics.RabbitPublishesTotal.WithLabelValues("avatar.deleted", "error").Inc()
		logger.Error("failed to publish delete event", "error", err)
		return err
	}

	metrics.RabbitPublishesTotal.WithLabelValues("avatar.deleted", "success").Inc()
	logger.Info("delete event published")
	return nil
}

func rabbitLogger(ctx context.Context) *slog.Logger {
	spanCtx := trace.SpanFromContext(ctx).SpanContext()

	logger := slog.With("service", "rabbit-publisher")
	if spanCtx.IsValid() {
		logger = logger.With(
			"trace_id", spanCtx.TraceID().String(),
			"span_id", spanCtx.SpanID().String(),
		)
	}
	return logger
}
