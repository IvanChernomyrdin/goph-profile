package mocks

import (
	"context"

	"github.com/stretchr/testify/mock"

	"goph-profile-avatars/internal/repository"
)

type AvatarRepo struct {
	mock.Mock
}

func (m *AvatarRepo) CreateAvatar(ctx context.Context, avatar repository.CreateAvatarParams) error {
	args := m.Called(ctx, avatar)
	return args.Error(0)
}

func (m *AvatarRepo) GetAvatarByID(ctx context.Context, avatarID string) (*repository.Avatar, error) {
	args := m.Called(ctx, avatarID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.Avatar), args.Error(1)
}

func (m *AvatarRepo) GetUserAvatar(ctx context.Context, userID string) (*repository.Avatar, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.Avatar), args.Error(1)
}

func (m *AvatarRepo) GetListUserAvatar(ctx context.Context, userID string) ([]repository.Avatar, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]repository.Avatar), args.Error(1)
}

func (m *AvatarRepo) SetCurrentAvatar(ctx context.Context, userID, avatarID string) error {
	args := m.Called(ctx, userID, avatarID)
	return args.Error(0)
}

func (m *AvatarRepo) DeleteCurrentUserAvatar(ctx context.Context, userID string) error {
	args := m.Called(ctx, userID)
	return args.Error(0)
}
