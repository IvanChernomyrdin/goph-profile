package services

import (
	"context"

	amqp "github.com/rabbitmq/amqp091-go"
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
	defer func() {
		_ = ch.Close()
	}()

	return nil
}
