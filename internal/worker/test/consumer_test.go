package test

import (
	"context"
	"io"
	"log/slog"
	"testing"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"goph-profile-avatars/internal/config"
	"goph-profile-avatars/internal/worker"
)

// mockChannel мок для amqp.Channel
type mockChannel struct {
	mock.Mock
}

func (m *mockChannel) ExchangeDeclare(name, kind string, durable, autoDelete, internal, noWait bool, args amqp.Table) error {
	argsCall := m.Called(name, kind, durable, autoDelete, internal, noWait, args)
	return argsCall.Error(0)
}

func (m *mockChannel) QueueDeclare(name string, durable, autoDelete, exclusive, noWait bool, args amqp.Table) (amqp.Queue, error) {
	argsCall := m.Called(name, durable, autoDelete, exclusive, noWait, args)
	return argsCall.Get(0).(amqp.Queue), argsCall.Error(1)
}

func (m *mockChannel) QueueBind(name, key, exchange string, noWait bool, args amqp.Table) error {
	argsCall := m.Called(name, key, exchange, noWait, args)
	return argsCall.Error(0)
}

func (m *mockChannel) Qos(prefetchCount, prefetchSize int, global bool) error {
	argsCall := m.Called(prefetchCount, prefetchSize, global)
	return argsCall.Error(0)
}

func (m *mockChannel) Consume(queue, consumer string, autoAck, exclusive, noLocal, noWait bool, args amqp.Table) (<-chan amqp.Delivery, error) {
	argsCall := m.Called(queue, consumer, autoAck, exclusive, noLocal, noWait, args)
	if argsCall.Get(0) == nil {
		return nil, argsCall.Error(1)
	}
	return argsCall.Get(0).(<-chan amqp.Delivery), argsCall.Error(1)
}

// mockUploadHandler мок для UploadHandler
type mockUploadHandler struct {
	mock.Mock
}

func (m *mockUploadHandler) HandleUpload(ctx context.Context, event worker.AvatarUploadEvent) error {
	args := m.Called(ctx, event)
	return args.Error(0)
}

// mockDeleteHandler мок для DeleteHandler
type mockDeleteHandler struct {
	mock.Mock
}

func (m *mockDeleteHandler) HandleDelete(ctx context.Context, event worker.AvatarDeleteEvent) error {
	args := m.Called(ctx, event)
	return args.Error(0)
}

func TestNewRabbitConsumer(t *testing.T) {
	mockCh := new(mockChannel)
	cfg := config.RabbitMQConfig{}
	uploadHandler := new(mockUploadHandler)
	deleteHandler := new(mockDeleteHandler)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	consumer := worker.NewRabbitConsumer(mockCh, cfg, uploadHandler, deleteHandler, log)

	assert.NotNil(t, consumer)
}
