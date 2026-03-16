package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"goph-profile-avatars/internal/api"
	"goph-profile-avatars/internal/repository"

	"github.com/google/uuid"
)

const maxAvatarSize = 10 << 20 // 10 MB

type avatarRepo interface {
	CreateAvatar(ctx context.Context, avatar repository.CreateAvatarParams) error
	// получение аватарки по ID
	GetAvatarByID(ctx context.Context, avatarID string) (*repository.Avatar, error)
	// получение главной аватарки пользователя
	GetUserAvatar(ctx context.Context, userID string) (*repository.Avatar, error)
	// получение всех аватарок пользователя
	GetListUserAvatar(ctx context.Context, userID string) ([]repository.Avatar, error)
	// выставление главной аватарки пользователя
	SetCurrentAvatar(ctx context.Context, userID, avatarID string) error
}

type objectStorage interface {
	Upload(ctx context.Context, key string, body io.Reader, size int64, contentType string) error
	Download(ctx context.Context, key string) (*DownloadResult, error)
}

type eventPublisher interface {
	PublishUploadEvent(ctx context.Context, event AvatarUploadEvent) error
}

type AvatarService struct {
	repo      avatarRepo
	storage   objectStorage
	publisher eventPublisher
}

func NewAvatarService(
	repo avatarRepo,
	storage objectStorage,
	publisher eventPublisher,
) *AvatarService {
	return &AvatarService{
		repo:      repo,
		storage:   storage,
		publisher: publisher,
	}
}

// загрузка аватарки, сохранение в бд методанных, сохранение в minio оригинала, отправка в rabbitmq уведомление для воркера
func (s *AvatarService) UploadAvatar(input api.UploadAvatarInput) (*api.UploadAvatarResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	data, err := io.ReadAll(input.File)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}

	if len(data) == 0 {
		return nil, fmt.Errorf("empty file")
	}

	if int64(len(data)) > maxAvatarSize {
		return nil, fmt.Errorf("file too large")
	}

	mimeType := http.DetectContentType(data[:min(len(data), 512)])
	if !isAllowedImageMime(mimeType) {
		return nil, fmt.Errorf("invalid file format")
	}

	avatarID := uuid.NewString()
	ext := normalizeExtension(input.FileName, mimeType)
	s3Key := fmt.Sprintf("avatars/%s/original%s", avatarID, ext)

	if err := s.storage.Upload(
		ctx,
		s3Key,
		bytes.NewReader(data),
		int64(len(data)),
		mimeType,
	); err != nil {
		return nil, fmt.Errorf("upload to minio: %w", err)
	}

	thumbnailKeys, err := json.Marshal(map[string]string{})
	if err != nil {
		return nil, fmt.Errorf("marshal thumbnails: %w", err)
	}

	if err := s.repo.CreateAvatar(ctx, repository.CreateAvatarParams{
		ID:               avatarID,
		UserID:           input.UserID,
		FileName:         safeFileName(input.FileName),
		MimeType:         mimeType,
		SizeBytes:        int64(len(data)),
		S3Key:            s3Key,
		ThumbnailS3Keys:  thumbnailKeys,
		UploadStatus:     "uploaded",
		ProcessingStatus: "pending",
	}); err != nil {
		return nil, fmt.Errorf("save avatar metadata: %w", err)
	}

	if err := s.publisher.PublishUploadEvent(ctx, AvatarUploadEvent{
		AvatarID: avatarID,
		UserID:   input.UserID,
		S3Key:    s3Key,
	}); err != nil {
		return nil, fmt.Errorf("publish upload event: %w", err)
	}

	now := time.Now().UTC()

	return &api.UploadAvatarResult{
		ID:        avatarID,
		UserID:    input.UserID,
		URL:       "/api/v1/avatars/" + avatarID,
		Status:    "processing",
		CreatedAt: now.Format(time.RFC3339),
	}, nil
}

func isAllowedImageMime(mime string) bool {
	switch mime {
	case "image/jpeg", "image/png", "image/webp":
		return true
	default:
		return false
	}
}

func normalizeExtension(fileName, mime string) string {
	ext := strings.ToLower(filepath.Ext(fileName))

	switch mime {
	case "image/jpeg":
		if ext == ".jpg" || ext == ".jpeg" {
			return ext
		}
		return ".jpg"
	case "image/png":
		if ext == ".png" {
			return ext
		}
		return ".png"
	case "image/webp":
		if ext == ".webp" {
			return ext
		}
		return ".webp"
	default:
		if ext == "" {
			return ".bin"
		}
		return ext
	}
}

func safeFileName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "avatar"
	}
	return filepath.Base(name)
}

// получаем методанные из бд, дальше загружаем данные из minio и возвращаем аватарку
func (s *AvatarService) GetAvatarByID(avatarID, size string) (*api.GetAvatarResult, error) {
	ctx := context.Background()

	avatar, err := s.repo.GetAvatarByID(ctx, avatarID)
	if err != nil {
		return nil, fmt.Errorf("get avatar by id: %w", err)
	}

	keyToDownload := avatar.S3Key

	if size != "" && size != "original" {
		thumbs := parseThumbnailKeys(avatar.ThumbnailS3Keys)
		if thumbKey, ok := thumbs[size]; ok && thumbKey != "" {
			keyToDownload = thumbKey
		}
	}

	downloaded, err := s.storage.Download(ctx, keyToDownload)
	if err != nil {
		return nil, fmt.Errorf("download avatar from storage: %w", err)
	}

	return &api.GetAvatarResult{
		ID:        avatar.ID,
		UserID:    avatar.UserID,
		FileName:  avatar.FileName,
		MimeType:  downloaded.ContentType,
		SizeBytes: downloaded.Size,
		Reader:    downloaded.Reader,
	}, nil
}

// вспомогательная функция мы тянем из бд jsonb и парсим данные там лежат миниатюры 100х100 и 300х300
func parseThumbnailKeys(raw []byte) map[string]string {
	if len(raw) == 0 {
		return map[string]string{}
	}

	var result map[string]string
	if err := json.Unmarshal(raw, &result); err == nil && result != nil {
		return result
	}

	return map[string]string{}
}

// получаем одну главную аватарку пользователя
func (s *AvatarService) GetUserAvatar(userID string) (*api.GetAvatarResult, error) {
	ctx := context.Background()

	avatar, err := s.repo.GetUserAvatar(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get user avatar: %w", err)
	}

	downloaded, err := s.storage.Download(ctx, avatar.S3Key)
	if err != nil {
		return nil, fmt.Errorf("download user avatar from storage: %w", err)
	}

	return &api.GetAvatarResult{
		ID:        avatar.ID,
		UserID:    avatar.UserID,
		FileName:  avatar.FileName,
		MimeType:  downloaded.ContentType,
		SizeBytes: downloaded.Size,
		Reader:    downloaded.Reader,
	}, nil
}

// получаем список всех аватарок пользователя
func (s *AvatarService) GetListUserAvatar(userID string) ([]api.AvatarItem, error) {
	ctx := context.Background()

	avatars, err := s.repo.GetListUserAvatar(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get list user avatar: %w", err)
	}

	result := make([]api.AvatarItem, 0, len(avatars))
	for _, avatar := range avatars {
		thumbs := parseThumbnailKeys(avatar.ThumbnailS3Keys)

		thumbnailURLs := map[string]string{}
		if _, ok := thumbs["100x100"]; ok {
			thumbnailURLs["100x100"] = "/api/v1/avatars/" + avatar.ID + "?size=100x100"
		}
		if _, ok := thumbs["300x300"]; ok {
			thumbnailURLs["300x300"] = "/api/v1/avatars/" + avatar.ID + "?size=300x300"
		}

		result = append(result, api.AvatarItem{
			ID:               avatar.ID,
			UserID:           avatar.UserID,
			FileName:         avatar.FileName,
			MimeType:         avatar.MimeType,
			SizeBytes:        avatar.SizeBytes,
			UploadStatus:     avatar.UploadStatus,
			ProcessingStatus: avatar.ProcessingStatus,
			CreatedAt:        avatar.CreatedAt.UTC().Format(time.RFC3339),
			UpdatedAt:        avatar.UpdatedAt.UTC().Format(time.RFC3339),
			URL:              "/api/v1/avatars/" + avatar.ID,
			IsCurrent:        avatar.IsCurrent,
			ThumbnailURLs:    thumbnailURLs,
		})
	}
	return result, nil
}

func (s *AvatarService) UpdateCurrentAvatar(userID, avatarID string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := s.repo.SetCurrentAvatar(ctx, userID, avatarID); err != nil {
		return fmt.Errorf("set current avatar: %w", err)
	}

	return nil
}
