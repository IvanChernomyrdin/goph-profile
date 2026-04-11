package test

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"goph-profile-avatars/internal/services"
)

// mockConnection мок для amqp.Connection
type mockConnection struct {
	mock.Mock
}

func (m *mockConnection) Channel() (*amqp.Channel, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*amqp.Channel), args.Error(1)
}

func (m *mockConnection) IsClosed() bool {
	args := m.Called()
	return args.Bool(0)
}

func (m *mockConnection) Close() error {
	args := m.Called()
	return args.Error(0)
}

func TestNewRabbitMQHealthService(t *testing.T) {
	mockConn := new(mockConnection)
	service := services.NewRabbitMQHealthService(mockConn, &slog.Logger{})

	assert.NotNil(t, service)
}

func TestRabbitMQHealthService_Check_NilConnection(t *testing.T) {
	service := services.NewRabbitMQHealthService(nil, &slog.Logger{})

	err := service.Check(context.Background())

	assert.Error(t, err)
	assert.Equal(t, amqp.ErrClosed, err)
}

func TestRabbitMQHealthService_Check_ClosedConnection(t *testing.T) {
	mockConn := new(mockConnection)
	service := services.NewRabbitMQHealthService(mockConn, &slog.Logger{})

	mockConn.On("IsClosed").Return(true)

	err := service.Check(context.Background())

	assert.Error(t, err)
	assert.Equal(t, amqp.ErrClosed, err)
	mockConn.AssertExpectations(t)
}

func TestRabbitMQHealthService_Check_ChannelCreationError(t *testing.T) {
	mockConn := new(mockConnection)
	service := services.NewRabbitMQHealthService(mockConn, &slog.Logger{})

	mockConn.On("IsClosed").Return(false)
	mockConn.On("Channel").Return(nil, errors.New("channel creation failed"))

	err := service.Check(context.Background())

	assert.Error(t, err)
	mockConn.AssertExpectations(t)
}
