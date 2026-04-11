package test

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
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

func (m *mockChannel) PublishWithContext(ctx context.Context,
	exchange, key string,
	mandatory, immediate bool,
	msg amqp.Publishing,
) error {
	args := m.Called(ctx, exchange, key, mandatory, immediate, msg)
	return args.Error(0)
}

func (m *mockChannel) Close() error {
	args := m.Called()
	return args.Error(0)
}

func TestNewRabbitPublisher(t *testing.T) {
	mockCh := new(mockChannel)
	publisher := services.NewRabbitPublisher(mockCh, "test-exchange", "upload.key", "delete.key", &slog.Logger{})

	assert.NotNil(t, publisher)
}

func TestRabbitPublisher_PublishUploadEvent_Success(t *testing.T) {
	mockCh := new(mockChannel)
	publisher := services.NewRabbitPublisher(mockCh, "test-exchange", "upload.key", "delete.key", &slog.Logger{})

	ctx := context.Background()
	event := services.AvatarUploadEvent{
		AvatarID: "avatar-123",
		UserID:   "user-456",
		S3Key:    "avatars/user-456/avatar-123.jpg",
	}

	mockCh.On(
		"PublishWithContext",
		mock.Anything,
		"test-exchange",
		"upload.key",
		false,
		false,
		mock.MatchedBy(func(p amqp.Publishing) bool {
			return p.ContentType == "application/json" &&
				string(p.Body) == `{"avatar_id":"avatar-123","user_id":"user-456","s3_key":"avatars/user-456/avatar-123.jpg"}`
		}),
	).Return(nil)

	err := publisher.PublishUploadEvent(ctx, event)

	assert.NoError(t, err)
	mockCh.AssertExpectations(t)
}

func TestRabbitPublisher_PublishUploadEvent_PublishError(t *testing.T) {
	mockCh := new(mockChannel)
	publisher := services.NewRabbitPublisher(mockCh, "test-exchange", "upload.key", "delete.key", &slog.Logger{})

	ctx := context.Background()
	event := services.AvatarUploadEvent{
		AvatarID: "avatar-123",
		UserID:   "user-456",
		S3Key:    "avatars/user-456/avatar-123.jpg",
	}

	expectedBody, err := json.Marshal(event)
	assert.NoError(t, err)

	expectedErr := errors.New("publish failed")

	mockCh.On(
		"PublishWithContext",
		mock.Anything,
		"test-exchange",
		"upload.key",
		false,
		false,
		mock.MatchedBy(func(p amqp.Publishing) bool {
			return p.ContentType == "application/json" &&
				string(p.Body) == string(expectedBody)
		}),
	).Return(expectedErr)

	err = publisher.PublishUploadEvent(ctx, event)

	assert.Error(t, err)
	assert.ErrorIs(t, err, expectedErr)
	mockCh.AssertExpectations(t)
}

func TestRabbitPublisher_PublishDeleteEvent_Success(t *testing.T) {
	mockCh := new(mockChannel)
	publisher := services.NewRabbitPublisher(mockCh, "test-exchange", "upload.key", "delete.key", &slog.Logger{})

	ctx := context.Background()
	event := services.AvatarDeleteEvent{
		AvatarID: "avatar-123",
		UserID:   "user-456",
		S3Key:    "avatars/user-456/avatar-123.jpg",
	}

	expectedBody, err := json.Marshal(event)
	assert.NoError(t, err)

	mockCh.On(
		"PublishWithContext",
		mock.Anything,
		"test-exchange",
		"delete.key",
		false,
		false,
		mock.MatchedBy(func(p amqp.Publishing) bool {
			return p.ContentType == "application/json" &&
				string(p.Body) == string(expectedBody)
		}),
	).Return(nil)

	err = publisher.PublishDeleteEvent(ctx, event)

	assert.NoError(t, err)
	mockCh.AssertExpectations(t)
}

func TestRabbitPublisher_PublishDeleteEvent_PublishError(t *testing.T) {
	mockCh := new(mockChannel)
	publisher := services.NewRabbitPublisher(mockCh, "test-exchange", "upload.key", "delete.key", &slog.Logger{})

	ctx := context.Background()
	event := services.AvatarDeleteEvent{
		AvatarID: "avatar-123",
		UserID:   "user-456",
		S3Key:    "avatars/user-456/avatar-123.jpg",
	}

	expectedBody, err := json.Marshal(event)
	assert.NoError(t, err)

	expectedErr := errors.New("publish failed")

	mockCh.On(
		"PublishWithContext",
		mock.Anything,
		"test-exchange",
		"delete.key",
		false,
		false,
		mock.MatchedBy(func(p amqp.Publishing) bool {
			return p.ContentType == "application/json" &&
				string(p.Body) == string(expectedBody)
		}),
	).Return(expectedErr)

	err = publisher.PublishDeleteEvent(ctx, event)

	assert.Error(t, err)
	assert.ErrorIs(t, err, expectedErr)
	mockCh.AssertExpectations(t)
}

func TestRabbitPublisher_PublishUploadEvent_WithCanceledContext(t *testing.T) {
	mockCh := new(mockChannel)
	publisher := services.NewRabbitPublisher(mockCh, "test-exchange", "upload.key", "delete.key", &slog.Logger{})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	event := services.AvatarUploadEvent{
		AvatarID: "avatar-123",
		UserID:   "user-456",
		S3Key:    "avatars/user-456/avatar-123.jpg",
	}

	expectedBody, err := json.Marshal(event)
	assert.NoError(t, err)

	expectedErr := context.Canceled

	mockCh.On(
		"PublishWithContext",
		mock.Anything,
		"test-exchange",
		"upload.key",
		false,
		false,
		mock.MatchedBy(func(p amqp.Publishing) bool {
			return p.ContentType == "application/json" &&
				string(p.Body) == string(expectedBody)
		}),
	).Return(expectedErr)

	err = publisher.PublishUploadEvent(ctx, event)

	assert.Error(t, err)
	assert.ErrorIs(t, err, expectedErr)
	mockCh.AssertExpectations(t)
}
