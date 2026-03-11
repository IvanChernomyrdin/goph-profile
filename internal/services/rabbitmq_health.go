package services

import (
	"context"

	amqp "github.com/rabbitmq/amqp091-go"
)

type RabbitMQHealthService struct {
	conn *amqp.Connection
}

func NewRabbitMQHealthService(conn *amqp.Connection) *RabbitMQHealthService {
	return &RabbitMQHealthService{conn: conn}
}

func (s *RabbitMQHealthService) Check(ctx context.Context) error {
	if s.conn == nil {
		return amqp.ErrClosed
	}

	if s.conn.IsClosed() {
		return amqp.ErrClosed
	}

	ch, err := s.conn.Channel()
	if err != nil {
		return err
	}
	defer ch.Close()

	return nil
}
