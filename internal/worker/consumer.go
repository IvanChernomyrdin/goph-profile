package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"goph-profile-avatars/internal/config"
	"goph-profile-avatars/internal/metrics"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.opentelemetry.io/otel/trace"
)

// UploadHandler — интерфейс сервиса обработки события загрузки.
type UploadHandler interface {
	HandleUpload(ctx context.Context, event AvatarUploadEvent) error
}

type DeleteHandler interface {
	HandleDelete(ctx context.Context, event AvatarDeleteEvent) error
}

// ChannelInterface интерфейс для amqp.Channel
type ChannelInterface interface {
	ExchangeDeclare(name, kind string, durable, autoDelete, internal, noWait bool, args amqp.Table) error
	QueueDeclare(name string, durable, autoDelete, exclusive, noWait bool, args amqp.Table) (amqp.Queue, error)
	QueueBind(name, key, exchange string, noWait bool, args amqp.Table) error
	Qos(prefetchCount, prefetchSize int, global bool) error
	Consume(queue, consumer string, autoAck, exclusive, noLocal, noWait bool, args amqp.Table) (<-chan amqp.Delivery, error)
}

// RabbitConsumer слушает очередь RabbitMQ и передаёт сообщения в handler.
type RabbitConsumer struct {
	ch            ChannelInterface
	cfg           config.RabbitMQConfig
	uploadHandler UploadHandler
	deleteHandler DeleteHandler
	log           *slog.Logger
}

func NewRabbitConsumer(
	ch ChannelInterface,
	cfg config.RabbitMQConfig,
	uploadHandler UploadHandler,
	deleteHandler DeleteHandler,
	log *slog.Logger,
) *RabbitConsumer {
	return &RabbitConsumer{
		ch:            ch,
		cfg:           cfg,
		uploadHandler: uploadHandler,
		deleteHandler: deleteHandler,
		log:           log,
	}
}

func logWithTrace(ctx context.Context, baseLogger *slog.Logger) *slog.Logger {
	var traceID, spanID string
	span := trace.SpanFromContext(ctx)
	if span != nil && span.SpanContext().IsValid() {
		traceID = span.SpanContext().TraceID().String()
		spanID = span.SpanContext().SpanID().String()
	}

	return baseLogger.With(
		"trace_id", traceID,
		"span_id", spanID,
	)
}

func (c *RabbitConsumer) Run(ctx context.Context) error {
	if err := c.declareInfrastructure(); err != nil {
		logWithTrace(ctx, c.log).Error("declare rabbitmq infrastructure failed", "error", err)
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
		logWithTrace(ctx, c.log).Error("start upload consumer failed", "error", err)
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
		logWithTrace(ctx, c.log).Error("consume delete messages", "error", err)
		return fmt.Errorf("consume delete messages: %w", err)
	}

	logWithTrace(ctx, c.log).Info(
		"rabbit consumer started",
		"upload_queue", c.cfg.QueueUpload,
		"delete_queue", c.cfg.QueueDelete,
		"exchange", c.cfg.Exchange,
		"exchange_type", c.cfg.ExchangeType,
		"upload_routing_key", c.cfg.UploadRoutingKey,
		"delete_routing_key", c.cfg.DeleteRoutingKey,
	)

	for {
		select {
		case <-ctx.Done():
			logWithTrace(ctx, c.log).Info("rabbit consumer stopped by context")
			return nil

		case msg, ok := <-uploadMsgs:
			if !ok {
				logWithTrace(ctx, c.log).Warn("upload consumer channel closed")
				return fmt.Errorf("upload consumer channel closed")
			}

			if err := c.handleUploadMessage(ctx, msg); err != nil {
				logWithTrace(ctx, c.log).Error(
					"handle upload message failed",
					"error", err,
					"routing_key", msg.RoutingKey,
					"message_id", msg.MessageId,
					"correlation_id", msg.CorrelationId,
				)

				if nackErr := msg.Nack(false, false); nackErr != nil {
					metrics.RabbitMQMessageNackFailures.WithLabelValues("upload").Inc()
					logWithTrace(ctx, c.log).Error("message upload nack failed", "error", nackErr)
				} else {
					metrics.RabbitMQMessageNacks.WithLabelValues("upload").Inc()
				}
				continue
			}

			if err := msg.Ack(false); err != nil {
				metrics.RabbitMQMessageAckFailures.WithLabelValues("upload").Inc()
				logWithTrace(ctx, c.log).Error("message upload ack failed", "error", err)
			} else {
				metrics.RabbitMQMessageAcks.WithLabelValues("upload").Inc()
			}
		case msg, ok := <-deleteMsgs:
			if !ok {
				logWithTrace(ctx, c.log).Warn("delete consumer channel closed")
				return fmt.Errorf("delete consumer channel closed")
			}

			if err := c.handleDeleteMessage(ctx, msg); err != nil {
				logWithTrace(ctx, c.log).Error(
					"handle delete message failed",
					"error", err,
					"routing_key", msg.RoutingKey,
					"message_id", msg.MessageId,
					"correlation_id", msg.CorrelationId,
				)

				if nackErr := msg.Nack(false, false); nackErr != nil {
					metrics.RabbitMQMessageNackFailures.WithLabelValues("delete").Inc()
					logWithTrace(ctx, c.log).Error("message delete nack failed", "error", nackErr)
				} else {
					metrics.RabbitMQMessageNacks.WithLabelValues("delete").Inc()
				}
				continue
			}

			if err := msg.Ack(false); err != nil {
				metrics.RabbitMQMessageNackFailures.WithLabelValues("upload").Inc()
				logWithTrace(ctx, c.log).Error("message delete ack failed", "error", err)
			} else {
				metrics.RabbitMQMessageNacks.WithLabelValues("upload").Inc()
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

	logWithTrace(ctx, c.log).Info(
		"received upload event",
		"avatar_id", event.AvatarID,
		"user_id", event.UserID,
		"s3_key", event.S3Key,
		"routing_key", msg.RoutingKey,
		"message_id", msg.MessageId,
		"correlation_id", msg.CorrelationId,
	)

	msgCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	if err := c.uploadHandler.HandleUpload(msgCtx, event); err != nil {
		return fmt.Errorf("handle upload event: %w", err)
	}

	logWithTrace(ctx, c.log).Info("upload event processed successfully", "avatar_id", event.AvatarID)
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

	logWithTrace(ctx, c.log).Info(
		"received delete event",
		"avatar_id", event.AvatarID,
		"user_id", event.UserID,
		"s3_key", event.S3Key,
		"routing_key", msg.RoutingKey,
		"message_id", msg.MessageId,
		"correlation_id", msg.CorrelationId,
	)

	msgCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	if err := c.deleteHandler.HandleDelete(msgCtx, event); err != nil {
		return fmt.Errorf("handle delete event: %w", err)
	}

	logWithTrace(ctx, c.log).Info("delete event processed successfully", "avatar_id", event.AvatarID)
	return nil
}
