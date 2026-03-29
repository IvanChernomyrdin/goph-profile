package services

import (
	"context"
	"encoding/json"

	amqp "github.com/rabbitmq/amqp091-go"
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
	body, err := json.Marshal(event)
	if err != nil {
		return err
	}

	return p.ch.PublishWithContext(
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
}

func (p *RabbitPublisher) PublishDeleteEvent(ctx context.Context, event AvatarDeleteEvent) error {
	body, err := json.Marshal(event)
	if err != nil {
		return err
	}

	return p.ch.PublishWithContext(
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
}
