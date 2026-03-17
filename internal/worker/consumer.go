package worker

import (
	"context"
	"encoding/json"
	"fmt"

	"goph-profile-avatars/internal/config"

	amqp "github.com/rabbitmq/amqp091-go"
)

// Logger — минимальный интерфейс под логгер.
type Logger interface {
	Info(args ...any)
	Infof(template string, args ...any)
	Error(args ...any)
	Errorf(template string, args ...any)
}

// UploadHandler — интерфейс сервиса обработки события загрузки.
type UploadHandler interface {
	HandleUpload(ctx context.Context, event AvatarUploadEvent) error
}

type DeleteHandler interface {
	HandleDelete(ctx context.Context, event AvatarDeleteEvent) error
}

// RabbitConsumer слушает очередь RabbitMQ и передаёт сообщения в handler.
type RabbitConsumer struct {
	ch            *amqp.Channel
	cfg           config.RabbitMQConfig
	uploadHandler UploadHandler
	deleteHandler DeleteHandler
	log           Logger
}

func NewRabbitConsumer(
	ch *amqp.Channel,
	cfg config.RabbitMQConfig,
	uploadHandler UploadHandler,
	deleteHandler DeleteHandler,
	log Logger,
) *RabbitConsumer {
	return &RabbitConsumer{
		ch:            ch,
		cfg:           cfg,
		uploadHandler: uploadHandler,
		deleteHandler: deleteHandler,
		log:           log,
	}
}

func (c *RabbitConsumer) Run(ctx context.Context) error {
	if err := c.declareInfrastructure(); err != nil {
		return fmt.Errorf("declare rabbitmq infrastructure: %w", err)
	}

	uploadMsgs, err := c.ch.Consume(
		c.cfg.QueueUpload,
		"",
		false,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return fmt.Errorf("consume message: %w", err)
	}

	deleteMsgs, err := c.ch.Consume(
		c.cfg.QueueDelete,
		"",
		false,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return fmt.Errorf("consume delete messages: %w", err)
	}

	c.log.Infof(
		"rabbit consumer started, upload_queue=%s delete_queue=%s",
		c.cfg.QueueUpload,
		c.cfg.QueueDelete,
	)

	for {
		select {
		case <-ctx.Done():
			c.log.Info("rabbit consumer stopped by context")
			return nil

		case msg, ok := <-uploadMsgs:
			if !ok {
				return fmt.Errorf("upload consumer channel closed")
			}

			if err := c.handleUploadMessage(ctx, msg); err != nil {
				c.log.Errorf("handle upload message error: %v", err)

				if nackErr := msg.Nack(false, false); nackErr != nil {
					c.log.Errorf("message upload nack error: %v", nackErr)
				}
				continue
			}

			if err := msg.Ack(false); err != nil {
				c.log.Errorf("message upload ack error: %v", err)
			}
		case msg, ok := <-deleteMsgs:
			if !ok {
				return fmt.Errorf("delete consumer channel closed")
			}

			if err := c.handleDeleteMessage(ctx, msg); err != nil {
				c.log.Errorf("handle delete message error: %v", err)

				if nackErr := msg.Nack(false, false); nackErr != nil {
					c.log.Errorf("message delete nack error: %v", nackErr)
				}
				continue
			}

			if err := msg.Ack(false); err != nil {
				c.log.Errorf("message delete ack error: %v", err)
			}
		}
	}
}

func (c *RabbitConsumer) declareInfrastructure() error {
	if err := c.ch.ExchangeDeclare(
		c.cfg.Exchange,
		c.cfg.ExchangeType,
		true,
		false,
		false,
		false,
		nil,
	); err != nil {
		return fmt.Errorf("declare exchange: %w", err)
	}

	_, err := c.ch.QueueDeclare(
		c.cfg.QueueUpload,
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return fmt.Errorf("declare upload queue: %w", err)
	}

	_, err = c.ch.QueueDeclare(
		c.cfg.QueueDelete,
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return fmt.Errorf("declare delete queue: %w", err)
	}

	if err := c.ch.QueueBind(
		c.cfg.QueueUpload,
		c.cfg.UploadRoutingKey,
		c.cfg.Exchange,
		false,
		nil,
	); err != nil {
		return fmt.Errorf("bind upload queue: %w", err)
	}

	if err := c.ch.QueueBind(
		c.cfg.QueueDelete,
		c.cfg.DeleteRoutingKey,
		c.cfg.Exchange,
		false,
		nil,
	); err != nil {
		return fmt.Errorf("bind delete queue: %w", err)
	}

	if err := c.ch.Qos(1, 0, false); err != nil {
		return fmt.Errorf("set qos: %w", err)
	}

	return nil
}

func (c *RabbitConsumer) handleUploadMessage(ctx context.Context, msg amqp.Delivery) error {
	var event AvatarUploadEvent

	if err := json.Unmarshal(msg.Body, &event); err != nil {
		return fmt.Errorf("unmarshal upload event: %w", err)
	}

	if event.AvatarID == "" {
		return fmt.Errorf("empty avatar_id")
	}
	if event.S3Key == "" {
		return fmt.Errorf("empty s3_key")
	}
	if event.UserID == "" {
		return fmt.Errorf("empty user_id")
	}

	c.log.Infof(
		"received upload event: avatar_id=%s user_id=%s s3_key=%s",
		event.AvatarID,
		event.UserID,
		event.S3Key,
	)

	if err := c.uploadHandler.HandleUpload(ctx, event); err != nil {
		return fmt.Errorf("handle upload event: %w", err)
	}

	c.log.Infof("upload event processed successfully: avatar_id=%s", event.AvatarID)
	return nil
}
func (c *RabbitConsumer) handleDeleteMessage(ctx context.Context, msg amqp.Delivery) error {
	var event AvatarDeleteEvent

	if err := json.Unmarshal(msg.Body, &event); err != nil {
		return fmt.Errorf("unmarshal delete event: %w", err)
	}

	if event.AvatarID == "" {
		return fmt.Errorf("empty avatar_id")
	}
	if event.S3Key == "" {
		return fmt.Errorf("empty s3_key")
	}
	if event.UserID == "" {
		return fmt.Errorf("empty user_id")
	}

	c.log.Infof(
		"received delete event: avatar_id=%s user_id=%s s3_key=%s",
		event.AvatarID,
		event.UserID,
		event.S3Key,
	)

	if err := c.deleteHandler.HandleDelete(ctx, event); err != nil {
		return fmt.Errorf("handle delete event: %w", err)
	}

	c.log.Infof("delete event processed successfully: avatar_id=%s", event.AvatarID)
	return nil
}
