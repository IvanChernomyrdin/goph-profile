package mocks

import (
	"context"

	"github.com/stretchr/testify/mock"

	"goph-profile-avatars/internal/services"
)

type EventPublisher struct {
	mock.Mock
}

func (m *EventPublisher) PublishUploadEvent(ctx context.Context, event services.AvatarUploadEvent) error {
	args := m.Called(ctx, event)
	return args.Error(0)
}

func (m *EventPublisher) PublishDeleteEvent(ctx context.Context, event services.AvatarDeleteEvent) error {
	args := m.Called(ctx, event)
	return args.Error(0)
}
