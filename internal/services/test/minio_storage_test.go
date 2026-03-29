package test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"

	"github.com/minio/minio-go/v7"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"goph-profile-avatars/internal/services"
)

// mockMinioClient мок для minio клиента
type mockMinioClient struct {
	mock.Mock
}

func (m *mockMinioClient) PutObject(ctx context.Context, bucketName, objectName string, reader io.Reader, objectSize int64, opts minio.PutObjectOptions) (minio.UploadInfo, error) {
	args := m.Called(ctx, bucketName, objectName, reader, objectSize, opts)
	return args.Get(0).(minio.UploadInfo), args.Error(1)
}

func (m *mockMinioClient) GetObject(ctx context.Context, bucketName, objectName string, opts minio.GetObjectOptions) (*minio.Object, error) {
	args := m.Called(ctx, bucketName, objectName, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*minio.Object), args.Error(1)
}

func (m *mockMinioClient) RemoveObject(ctx context.Context, bucketName, objectName string, opts minio.RemoveObjectOptions) error {
	args := m.Called(ctx, bucketName, objectName, opts)
	return args.Error(0)
}

func TestNewMinIOStorage(t *testing.T) {
	mockClient := new(mockMinioClient)
	storage := services.NewMinIOStorage(mockClient, "test-bucket")

	assert.NotNil(t, storage)
}

func TestMinIOStorage_Upload_Success(t *testing.T) {
	mockClient := new(mockMinioClient)
	storage := services.NewMinIOStorage(mockClient, "test-bucket")

	ctx := context.Background()
	key := "avatars/user123/avatar.jpg"
	body := bytes.NewReader([]byte("image data"))
	size := int64(10)
	contentType := "image/jpeg"

	mockClient.On("PutObject", ctx, "test-bucket", key, body, size, minio.PutObjectOptions{
		ContentType: contentType,
	}).Return(minio.UploadInfo{}, nil)

	err := storage.Upload(ctx, key, body, size, contentType)

	assert.NoError(t, err)
	mockClient.AssertExpectations(t)
}

func TestMinIOStorage_Upload_Error(t *testing.T) {
	mockClient := new(mockMinioClient)
	storage := services.NewMinIOStorage(mockClient, "test-bucket")

	ctx := context.Background()
	key := "avatars/user123/avatar.jpg"
	body := bytes.NewReader([]byte("image data"))
	size := int64(10)
	contentType := "image/jpeg"

	expectedErr := errors.New("upload failed")
	mockClient.On("PutObject", ctx, "test-bucket", key, body, size, minio.PutObjectOptions{
		ContentType: contentType,
	}).Return(minio.UploadInfo{}, expectedErr)

	err := storage.Upload(ctx, key, body, size, contentType)

	assert.Error(t, err)
	assert.Equal(t, expectedErr, err)
	mockClient.AssertExpectations(t)
}

func TestMinIOStorage_Delete_Success(t *testing.T) {
	mockClient := new(mockMinioClient)
	storage := services.NewMinIOStorage(mockClient, "test-bucket")

	ctx := context.Background()
	key := "avatars/user123/avatar.jpg"

	mockClient.On("RemoveObject", ctx, "test-bucket", key, minio.RemoveObjectOptions{}).Return(nil)

	err := storage.Delete(ctx, key)

	assert.NoError(t, err)
	mockClient.AssertExpectations(t)
}

func TestMinIOStorage_Delete_Error(t *testing.T) {
	mockClient := new(mockMinioClient)
	storage := services.NewMinIOStorage(mockClient, "test-bucket")

	ctx := context.Background()
	key := "avatars/user123/avatar.jpg"

	expectedErr := errors.New("delete failed")
	mockClient.On("RemoveObject", ctx, "test-bucket", key, minio.RemoveObjectOptions{}).Return(expectedErr)

	err := storage.Delete(ctx, key)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "remove object")
	mockClient.AssertExpectations(t)
}
