package services

import (
	"context"
	"encoding/json"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
)

// ChannelInterface интерфейс для amqp.Channel
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
}

func NewRabbitPublisher(ch ChannelInterface, exchange, updateRoutingKey, deleteRoutingKey string) *RabbitPublisher {
	return &RabbitPublisher{
		ch:               ch,
		exchange:         exchange,
		updateRoutingKey: updateRoutingKey,
		deleteRoutingKey: deleteRoutingKey,
	}
}

func (p *RabbitPublisher) PublishUploadEvent(ctx context.Context, event AvatarUploadEvent) error {
	ctx, span := otel.Tracer("avatars-service/rabbit-publisher").Start(ctx, "publish-upload")
	defer span.End()

	body, err := json.Marshal(event)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "marshal failed")
		return err
	}

	err = p.ch.PublishWithContext(
		ctx,
		p.exchange,
		p.updateRoutingKey,
		false,
		false,
		amqp.Publishing{
			ContentType: "application/json",
			Body:        body,
		},
	)

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "publish failed")
	}

	return err
}

func (p *RabbitPublisher) PublishDeleteEvent(ctx context.Context, event AvatarDeleteEvent) error {
	ctx, span := otel.Tracer("avatars-service/rabbit-publisher").Start(ctx, "publish-delete")
	defer span.End()

	body, err := json.Marshal(event)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "marshal failed")
		return err
	}

	err = p.ch.PublishWithContext(
		ctx,
		p.exchange,
		p.deleteRoutingKey,
		false,
		false,
		amqp.Publishing{
			ContentType: "application/json",
			Body:        body,
		},
	)

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "publish error")
	}

	return err
}
