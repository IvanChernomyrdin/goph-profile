package services

import (
	"context"
	"encoding/json"

	amqp "github.com/rabbitmq/amqp091-go"
)

type AvatarUploadEvent struct {
	AvatarID string `json:"avatar_id"`
	UserID   string `json:"user_id"`
	S3Key    string `json:"s3_key"`
}

type RabbitPublisher struct {
	ch         *amqp.Channel
	exchange   string
	routingKey string
}

func NewRabbitPublisher(ch *amqp.Channel, exchange, routingKey string) *RabbitPublisher {
	return &RabbitPublisher{
		ch:         ch,
		exchange:   exchange,
		routingKey: routingKey,
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
		p.routingKey,
		false,
		false,
		amqp.Publishing{
			ContentType: "application/json",
			Body:        body,
		},
	)
}
