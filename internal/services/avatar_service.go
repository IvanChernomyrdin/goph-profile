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
}

type objectStorage interface {
	Upload(ctx context.Context, key string, body io.Reader, size int64, contentType string) error
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

	thumbnailKeys, err := json.Marshal([]string{})
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

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
