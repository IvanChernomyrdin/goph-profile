package test

import (
	"bytes"
	"context"
	"io"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"goph-profile-avatars/internal/api"
	"goph-profile-avatars/internal/repository"
	"goph-profile-avatars/internal/services"
	"goph-profile-avatars/internal/services/mocks"

	status "goph-profile-avatars/internal/config/status"
)

const maxAvatarSize = 10 << 20

func validJPEG() []byte {
	return []byte{
		0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46,
		0x49, 0x46, 0x00, 0x01, 0x01, 0x00, 0x00, 0x01,
		0x00, 0x01, 0x00, 0x00, 0xFF, 0xDB,
	}
}

func TestUploadAvatar_Success(t *testing.T) {
	repo := new(mocks.AvatarRepo)
	storage := new(mocks.ObjectStorage)
	publisher := new(mocks.EventPublisher)

	service := services.NewAvatarService(repo, storage, publisher)

	userID := "user123"
	fileName := "avatar.jpg"
	fileContent := validJPEG()

	input := api.UploadAvatarInput{
		UserID:      userID,
		FileName:    fileName,
		ContentType: "image/jpeg",
		Size:        int64(len(fileContent)),
		File:        bytes.NewReader(fileContent),
	}

	storage.On("Upload", mock.Anything, mock.Anything, mock.Anything, int64(len(fileContent)), "image/jpeg").Return(nil)
	repo.On("CreateAvatar", mock.Anything, mock.MatchedBy(func(params repository.CreateAvatarParams) bool {
		return params.UserID == userID && params.FileName == fileName
	})).Return(nil)
	publisher.On("PublishUploadEvent", mock.Anything, mock.MatchedBy(func(event services.AvatarUploadEvent) bool {
		return event.UserID == userID
	})).Return(nil)

	result, err := service.UploadAvatar(context.Background(), input)

	require.NoError(t, err)
	assert.NotEmpty(t, result.ID)
	assert.Equal(t, userID, result.UserID)
	assert.Equal(t, "processing", result.Status)
	assert.NotEmpty(t, result.CreatedAt)

	storage.AssertExpectations(t)
	repo.AssertExpectations(t)
	publisher.AssertExpectations(t)
}

func TestUploadAvatar_FileTooLarge(t *testing.T) {
	repo := new(mocks.AvatarRepo)
	storage := new(mocks.ObjectStorage)
	publisher := new(mocks.EventPublisher)

	service := services.NewAvatarService(repo, storage, publisher)

	largeData := make([]byte, maxAvatarSize+1)
	input := api.UploadAvatarInput{
		UserID:   "user123",
		FileName: "large.jpg",
		Size:     int64(len(largeData)),
		File:     bytes.NewReader(largeData),
	}

	result, err := service.UploadAvatar(context.Background(), input)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "file too large")
	assert.Nil(t, result)
}

func TestUploadAvatar_InvalidFileFormat(t *testing.T) {
	repo := new(mocks.AvatarRepo)
	storage := new(mocks.ObjectStorage)
	publisher := new(mocks.EventPublisher)

	service := services.NewAvatarService(repo, storage, publisher)

	input := api.UploadAvatarInput{
		UserID:   "user123",
		FileName: "avatar.txt",
		Size:     100,
		File:     bytes.NewReader([]byte("text file content")),
	}

	result, err := service.UploadAvatar(context.Background(), input)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid file format")
	assert.Nil(t, result)
}

func TestUploadAvatar_EmptyFile(t *testing.T) {
	repo := new(mocks.AvatarRepo)
	storage := new(mocks.ObjectStorage)
	publisher := new(mocks.EventPublisher)

	service := services.NewAvatarService(repo, storage, publisher)

	input := api.UploadAvatarInput{
		UserID:   "user123",
		FileName: "empty.jpg",
		Size:     0,
		File:     bytes.NewReader([]byte{}),
	}

	result, err := service.UploadAvatar(context.Background(), input)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "empty file")
	assert.Nil(t, result)
}

func TestGetAvatarByID_Success(t *testing.T) {
	repo := new(mocks.AvatarRepo)
	storage := new(mocks.ObjectStorage)
	publisher := new(mocks.EventPublisher)

	service := services.NewAvatarService(repo, storage, publisher)

	avatarID := uuid.New().String()
	now := time.Now()

	avatar := &repository.Avatar{
		ID:        avatarID,
		UserID:    "user123",
		FileName:  "avatar.jpg",
		MimeType:  "image/jpeg",
		SizeBytes: 1024,
		S3Key:     "avatars/user123/avatar.jpg",
		CreatedAt: now,
		UpdatedAt: now,
	}

	downloadResult := &services.DownloadResult{
		Reader:      io.NopCloser(bytes.NewReader([]byte("image data"))),
		ContentType: "image/jpeg",
		Size:        1024,
	}

	repo.On("GetAvatarByID", mock.Anything, avatarID).Return(avatar, nil)
	storage.On("Download", mock.Anything, avatar.S3Key).Return(downloadResult, nil)

	result, err := service.GetAvatarByID(context.Background(), avatarID, "original")

	require.NoError(t, err)
	assert.Equal(t, avatarID, result.ID)
	assert.Equal(t, "user123", result.UserID)
	assert.Equal(t, "avatar.jpg", result.FileName)
	assert.Equal(t, int64(1024), result.SizeBytes)
	assert.NotNil(t, result.Reader)

	repo.AssertExpectations(t)
	storage.AssertExpectations(t)
}

func TestGetAvatarByID_NotFound(t *testing.T) {
	repo := new(mocks.AvatarRepo)
	storage := new(mocks.ObjectStorage)
	publisher := new(mocks.EventPublisher)

	service := services.NewAvatarService(repo, storage, publisher)

	avatarID := uuid.New().String()

	repo.On("GetAvatarByID", mock.Anything, avatarID).Return(nil, repository.ErrAvatarNotFound)

	result, err := service.GetAvatarByID(context.Background(), avatarID, "original")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), repository.ErrAvatarNotFound.Error())
	assert.Nil(t, result)

	repo.AssertExpectations(t)
}

func TestGetUserAvatar_Success(t *testing.T) {
	repo := new(mocks.AvatarRepo)
	storage := new(mocks.ObjectStorage)
	publisher := new(mocks.EventPublisher)

	service := services.NewAvatarService(repo, storage, publisher)

	userID := "user123"
	avatarID := uuid.New().String()
	now := time.Now()

	avatar := &repository.Avatar{
		ID:        avatarID,
		UserID:    userID,
		FileName:  "avatar.jpg",
		MimeType:  "image/jpeg",
		SizeBytes: 1024,
		S3Key:     "avatars/user123/avatar.jpg",
		CreatedAt: now,
		UpdatedAt: now,
		IsCurrent: true,
	}

	downloadResult := &services.DownloadResult{
		Reader:      io.NopCloser(bytes.NewReader([]byte("image data"))),
		ContentType: "image/jpeg",
		Size:        1024,
	}

	repo.On("GetUserAvatar", mock.Anything, userID).Return(avatar, nil)
	storage.On("Download", mock.Anything, avatar.S3Key).Return(downloadResult, nil)

	result, err := service.GetUserAvatar(context.Background(), userID)

	require.NoError(t, err)
	assert.Equal(t, avatarID, result.ID)
	assert.Equal(t, userID, result.UserID)
	assert.Equal(t, "avatar.jpg", result.FileName)
	assert.NotNil(t, result.Reader)

	repo.AssertExpectations(t)
	storage.AssertExpectations(t)
}

func TestGetListUserAvatar_Success(t *testing.T) {
	repo := new(mocks.AvatarRepo)
	storage := new(mocks.ObjectStorage)
	publisher := new(mocks.EventPublisher)

	service := services.NewAvatarService(repo, storage, publisher)

	userID := "user123"
	now := time.Now()

	avatars := []repository.Avatar{
		{
			ID:               uuid.New().String(),
			UserID:           userID,
			FileName:         "avatar1.jpg",
			MimeType:         "image/jpeg",
			SizeBytes:        1024,
			UploadStatus:     status.Uploaded,
			ProcessingStatus: "completed",
			CreatedAt:        now,
			UpdatedAt:        now,
			IsCurrent:        true,
			ThumbnailS3Keys:  []byte(`{"100x100":"key1","300x300":"key2"}`),
		},
		{
			ID:               uuid.New().String(),
			UserID:           userID,
			FileName:         "avatar2.jpg",
			MimeType:         "image/jpeg",
			SizeBytes:        2048,
			UploadStatus:     status.Uploaded,
			ProcessingStatus: "completed",
			CreatedAt:        now,
			UpdatedAt:        now,
			IsCurrent:        false,
			ThumbnailS3Keys:  []byte(`{"100x100":"key3"}`),
		},
	}

	repo.On("GetListUserAvatar", mock.Anything, userID).Return(avatars, nil)

	result, err := service.GetListUserAvatar(context.Background(), userID)

	require.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Equal(t, "avatar1.jpg", result[0].FileName)
	assert.True(t, result[0].IsCurrent)
	assert.Equal(t, "avatar2.jpg", result[1].FileName)
	assert.False(t, result[1].IsCurrent)
	assert.NotEmpty(t, result[0].ThumbnailURLs)

	repo.AssertExpectations(t)
}

func TestUpdateCurrentAvatar_Success(t *testing.T) {
	repo := new(mocks.AvatarRepo)
	storage := new(mocks.ObjectStorage)
	publisher := new(mocks.EventPublisher)

	service := services.NewAvatarService(repo, storage, publisher)

	userID := "user123"
	avatarID := uuid.New().String()

	repo.On("SetCurrentAvatar", mock.Anything, userID, avatarID).Return(nil)

	err := service.UpdateCurrentAvatar(context.Background(), userID, avatarID)

	assert.NoError(t, err)
	repo.AssertExpectations(t)
}

func TestDeleteCurrentUserAvatar_Success(t *testing.T) {
	repo := new(mocks.AvatarRepo)
	storage := new(mocks.ObjectStorage)
	publisher := new(mocks.EventPublisher)

	service := services.NewAvatarService(repo, storage, publisher)

	userID := "user123"

	repo.On("DeleteCurrentUserAvatar", mock.Anything, userID).Return(nil)

	err := service.DeleteCurrentUserAvatar(context.Background(), userID)

	assert.NoError(t, err)
	repo.AssertExpectations(t)
}

func TestDeleteAvatarByID_Success(t *testing.T) {
	repo := new(mocks.AvatarRepo)
	storage := new(mocks.ObjectStorage)
	publisher := new(mocks.EventPublisher)

	service := services.NewAvatarService(repo, storage, publisher)

	avatarID := uuid.New().String()
	userID := "user123"
	now := time.Now()

	avatar := &repository.Avatar{
		ID:        avatarID,
		UserID:    userID,
		FileName:  "avatar.jpg",
		S3Key:     "avatars/user123/avatar.jpg",
		CreatedAt: now,
		UpdatedAt: now,
	}

	repo.On("GetAvatarByID", mock.Anything, avatarID).Return(avatar, nil)
	publisher.On("PublishDeleteEvent", mock.Anything, mock.MatchedBy(func(event services.AvatarDeleteEvent) bool {
		return event.AvatarID == avatarID && event.UserID == userID
	})).Return(nil)

	err := service.DeleteAvatarByID(context.Background(), avatarID, userID)

	assert.NoError(t, err)
	repo.AssertExpectations(t)
	publisher.AssertExpectations(t)
}

func TestDeleteAvatarByID_NotFound(t *testing.T) {
	repo := new(mocks.AvatarRepo)
	storage := new(mocks.ObjectStorage)
	publisher := new(mocks.EventPublisher)

	service := services.NewAvatarService(repo, storage, publisher)

	avatarID := uuid.New().String()
	userID := "user123"

	repo.On("GetAvatarByID", mock.Anything, avatarID).Return(nil, repository.ErrAvatarNotFound)

	err := service.DeleteAvatarByID(context.Background(), avatarID, userID)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), repository.ErrAvatarNotFound.Error())
	repo.AssertExpectations(t)
}
