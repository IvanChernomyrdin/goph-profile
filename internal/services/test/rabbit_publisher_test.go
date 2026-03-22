package test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"goph-profile-avatars/internal/services"
)

// mockChannel мок для amqp.Channel
type mockChannel struct {
	mock.Mock
}

func (m *mockChannel) PublishWithContext(ctx context.Context, exchange, key string, mandatory, immediate bool, msg amqp.Publishing) error {
	args := m.Called(ctx, exchange, key, mandatory, immediate, msg)
	return args.Error(0)
}

func (m *mockChannel) Close() error {
	args := m.Called()
	return args.Error(0)
}

func TestNewRabbitPublisher(t *testing.T) {
	mockCh := new(mockChannel)
	publisher := services.NewRabbitPublisher(mockCh, "test-exchange", "upload.key", "delete.key")

	assert.NotNil(t, publisher)
}

func TestRabbitPublisher_PublishUploadEvent_Success(t *testing.T) {
	mockCh := new(mockChannel)
	publisher := services.NewRabbitPublisher(mockCh, "test-exchange", "upload.key", "delete.key")

	ctx := context.Background()
	event := services.AvatarUploadEvent{
		AvatarID: "avatar-123",
		UserID:   "user-456",
		S3Key:    "avatars/user-456/avatar-123.jpg",
	}

	expectedBody, err := json.Marshal(event)
	assert.NoError(t, err)

	mockCh.On("PublishWithContext", ctx, "test-exchange", "upload.key", false, false, amqp.Publishing{
		ContentType: "application/json",
		Body:        expectedBody,
	}).Return(nil)

	err = publisher.PublishUploadEvent(ctx, event)

	assert.NoError(t, err)
	mockCh.AssertExpectations(t)
}

func TestRabbitPublisher_PublishUploadEvent_PublishError(t *testing.T) {
	mockCh := new(mockChannel)
	publisher := services.NewRabbitPublisher(mockCh, "test-exchange", "upload.key", "delete.key")

	ctx := context.Background()
	event := services.AvatarUploadEvent{
		AvatarID: "avatar-123",
		UserID:   "user-456",
		S3Key:    "avatars/user-456/avatar-123.jpg",
	}

	expectedBody, err := json.Marshal(event)
	assert.NoError(t, err)

	expectedErr := errors.New("publish failed")
	mockCh.On("PublishWithContext", ctx, "test-exchange", "upload.key", false, false, amqp.Publishing{
		ContentType: "application/json",
		Body:        expectedBody,
	}).Return(expectedErr)

	err = publisher.PublishUploadEvent(ctx, event)

	assert.Error(t, err)
	assert.Equal(t, expectedErr, err)
	mockCh.AssertExpectations(t)
}

func TestRabbitPublisher_PublishDeleteEvent_Success(t *testing.T) {
	mockCh := new(mockChannel)
	publisher := services.NewRabbitPublisher(mockCh, "test-exchange", "upload.key", "delete.key")

	ctx := context.Background()
	event := services.AvatarDeleteEvent{
		AvatarID: "avatar-123",
		UserID:   "user-456",
		S3Key:    "avatars/user-456/avatar-123.jpg",
	}

	expectedBody, err := json.Marshal(event)
	assert.NoError(t, err)

	mockCh.On("PublishWithContext", ctx, "test-exchange", "delete.key", false, false, amqp.Publishing{
		ContentType: "application/json",
		Body:        expectedBody,
	}).Return(nil)

	err = publisher.PublishDeleteEvent(ctx, event)

	assert.NoError(t, err)
	mockCh.AssertExpectations(t)
}

func TestRabbitPublisher_PublishDeleteEvent_PublishError(t *testing.T) {
	mockCh := new(mockChannel)
	publisher := services.NewRabbitPublisher(mockCh, "test-exchange", "upload.key", "delete.key")

	ctx := context.Background()
	event := services.AvatarDeleteEvent{
		AvatarID: "avatar-123",
		UserID:   "user-456",
		S3Key:    "avatars/user-456/avatar-123.jpg",
	}

	expectedBody, err := json.Marshal(event)
	assert.NoError(t, err)

	expectedErr := errors.New("publish failed")
	mockCh.On("PublishWithContext", ctx, "test-exchange", "delete.key", false, false, amqp.Publishing{
		ContentType: "application/json",
		Body:        expectedBody,
	}).Return(expectedErr)

	err = publisher.PublishDeleteEvent(ctx, event)

	assert.Error(t, err)
	assert.Equal(t, expectedErr, err)
	mockCh.AssertExpectations(t)
}

func TestRabbitPublisher_PublishUploadEvent_WithCanceledContext(t *testing.T) {
	mockCh := new(mockChannel)
	publisher := services.NewRabbitPublisher(mockCh, "test-exchange", "upload.key", "delete.key")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	event := services.AvatarUploadEvent{
		AvatarID: "avatar-123",
		UserID:   "user-456",
		S3Key:    "avatars/user-456/avatar-123.jpg",
	}

	expectedErr := context.Canceled
	mockCh.On("PublishWithContext", ctx, "test-exchange", "upload.key", false, false, mock.Anything).Return(expectedErr)

	err := publisher.PublishUploadEvent(ctx, event)

	assert.Error(t, err)
	assert.Equal(t, expectedErr, err)
	mockCh.AssertExpectations(t)
}
