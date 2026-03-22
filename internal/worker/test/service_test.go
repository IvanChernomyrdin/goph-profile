package test

import (
	"context"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"goph-profile-avatars/internal/repository"
	"goph-profile-avatars/internal/services"
	"goph-profile-avatars/internal/worker"
)

// mockAvatarRepo мок для репозитория
type mockAvatarRepo struct {
	mock.Mock
}

func (m *mockAvatarRepo) GetAvatarByID(ctx context.Context, avatarID string) (*repository.Avatar, error) {
	args := m.Called(ctx, avatarID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.Avatar), args.Error(1)
}

func (m *mockAvatarRepo) UpdateProcessingStatus(ctx context.Context, avatarID, status string) error {
	args := m.Called(ctx, avatarID, status)
	return args.Error(0)
}

func (m *mockAvatarRepo) FailProcessing(ctx context.Context, avatarID string) error {
	args := m.Called(ctx, avatarID)
	return args.Error(0)
}

func (m *mockAvatarRepo) CompleteProcessing(ctx context.Context, avatarID string, thumbnailKeys []byte) error {
	args := m.Called(ctx, avatarID, thumbnailKeys)
	return args.Error(0)
}

func (m *mockAvatarRepo) SoftDeleteAvatar(ctx context.Context, avatarID string) error {
	args := m.Called(ctx, avatarID)
	return args.Error(0)
}

// mockStorage мок для хранилища
type mockStorage struct {
	mock.Mock
}

func (m *mockStorage) Download(ctx context.Context, key string) (*services.DownloadResult, error) {
	args := m.Called(ctx, key)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*services.DownloadResult), args.Error(1)
}

func (m *mockStorage) Upload(ctx context.Context, key string, body io.Reader, size int64, contentType string) error {
	args := m.Called(ctx, key, body, size, contentType)
	return args.Error(0)
}

func (m *mockStorage) Delete(ctx context.Context, key string) error {
	args := m.Called(ctx, key)
	return args.Error(0)
}

func TestNewAvatarWorkerService(t *testing.T) {
	repo := new(mockAvatarRepo)
	storage := new(mockStorage)
	log := new(mockLogger)

	service := worker.NewAvatarWorkerService(repo, storage, log)

	assert.NotNil(t, service)
}
