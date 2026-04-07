package mocks

import (
	"context"
	"goph-profile-avatars/internal/api"

	"github.com/stretchr/testify/mock"
)

// AvatarService мок для тестирования
type AvatarService struct {
	mock.Mock
}

func (m *AvatarService) UploadAvatar(ctx context.Context, input api.UploadAvatarInput) (*api.UploadAvatarResult, error) {
	args := m.Called(ctx, input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*api.UploadAvatarResult), args.Error(1)
}

func (m *AvatarService) GetAvatarByID(ctx context.Context, avatarID, size string) (*api.GetAvatarResult, error) {
	args := m.Called(ctx, avatarID, size)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*api.GetAvatarResult), args.Error(1)
}

// и так для всех остальных методов:
func (m *AvatarService) GetUserAvatar(ctx context.Context, userID string) (*api.GetAvatarResult, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*api.GetAvatarResult), args.Error(1)
}

func (m *AvatarService) GetListUserAvatar(ctx context.Context, userID string) ([]api.AvatarItem, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]api.AvatarItem), args.Error(1)
}

func (m *AvatarService) UpdateCurrentAvatar(ctx context.Context, userID, avatarID string) error {
	args := m.Called(ctx, userID, avatarID)
	return args.Error(0)
}

func (m *AvatarService) DeleteCurrentUserAvatar(ctx context.Context, userID string) error {
	args := m.Called(ctx, userID)
	return args.Error(0)
}

func (m *AvatarService) DeleteAvatarByID(ctx context.Context, avatarID, userID string) error {
	args := m.Called(ctx, avatarID, userID)
	return args.Error(0)
}
