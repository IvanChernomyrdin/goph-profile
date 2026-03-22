package mocks

import (
	"context"
	"io"

	"github.com/stretchr/testify/mock"

	"goph-profile-avatars/internal/services"
)

type ObjectStorage struct {
	mock.Mock
}

func (m *ObjectStorage) Upload(ctx context.Context, key string, body io.Reader, size int64, contentType string) error {
	args := m.Called(ctx, key, body, size, contentType)
	return args.Error(0)
}

func (m *ObjectStorage) Download(ctx context.Context, key string) (*services.DownloadResult, error) {
	args := m.Called(ctx, key)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*services.DownloadResult), args.Error(1)
}
