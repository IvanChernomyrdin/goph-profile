package test

import (
	"context"
	"errors"
	"goph-profile-avatars/internal/services"
	"testing"

	"github.com/minio/minio-go/v7"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func (m *mockMinioClient) ListBuckets(ctx context.Context) ([]minio.BucketInfo, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]minio.BucketInfo), args.Error(1)
}

func TestMinIOHealthService_Check_Success(t *testing.T) {
	mockClient := new(mockMinioClient)
	service := services.NewMinIOHealthService(mockClient)

	mockClient.On("ListBuckets", mock.Anything).Return([]minio.BucketInfo{}, nil)

	err := service.Check(context.Background())

	assert.NoError(t, err)
	mockClient.AssertExpectations(t)
}

func TestMinIOHealthService_Check_Failure(t *testing.T) {
	mockClient := new(mockMinioClient)
	service := services.NewMinIOHealthService(mockClient)

	expectedErr := errors.New("connection failed")
	mockClient.On("ListBuckets", mock.Anything).Return(nil, expectedErr)

	err := service.Check(context.Background())

	assert.Error(t, err)
	assert.Equal(t, expectedErr, err)
	mockClient.AssertExpectations(t)
}

func TestMinIOHealthService_Check_EmptyBuckets(t *testing.T) {
	mockClient := new(mockMinioClient)
	service := services.NewMinIOHealthService(mockClient)

	mockClient.On("ListBuckets", mock.Anything).Return([]minio.BucketInfo{}, nil)

	err := service.Check(context.Background())

	assert.NoError(t, err)
	mockClient.AssertExpectations(t)
}

func TestMinIOHealthService_Check_WithContextCancel(t *testing.T) {
	mockClient := new(mockMinioClient)
	service := services.NewMinIOHealthService(mockClient)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	expectedErr := context.Canceled
	mockClient.On("ListBuckets", ctx).Return(nil, expectedErr)

	err := service.Check(ctx)

	assert.Error(t, err)
	assert.Equal(t, expectedErr, err)
	mockClient.AssertExpectations(t)
}
