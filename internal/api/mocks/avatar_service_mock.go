package mocks

import (
	"goph-profile-avatars/internal/api"

	"github.com/stretchr/testify/mock"
)

// AvatarService мок для тестирования
type AvatarService struct {
	mock.Mock
}

func (m *AvatarService) UploadAvatar(input api.UploadAvatarInput) (*api.UploadAvatarResult, error) {
	args := m.Called(input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*api.UploadAvatarResult), args.Error(1)
}

func (m *AvatarService) GetAvatarByID(avatarID, size string) (*api.GetAvatarResult, error) {
	args := m.Called(avatarID, size)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*api.GetAvatarResult), args.Error(1)
}

func (m *AvatarService) GetUserAvatar(userID string) (*api.GetAvatarResult, error) {
	args := m.Called(userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*api.GetAvatarResult), args.Error(1)
}

func (m *AvatarService) GetListUserAvatar(userID string) ([]api.AvatarItem, error) {
	args := m.Called(userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]api.AvatarItem), args.Error(1)
}

func (m *AvatarService) UpdateCurrentAvatar(userID, avatarID string) error {
	args := m.Called(userID, avatarID)
	return args.Error(0)
}

func (m *AvatarService) DeleteCurrentUserAvatar(userID string) error {
	args := m.Called(userID)
	return args.Error(0)
}

func (m *AvatarService) DeleteAvatarByID(avatarID, userID string) error {
	args := m.Called(avatarID, userID)
	return args.Error(0)
}
