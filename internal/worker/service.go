package worker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"path"

	"github.com/disintegration/imaging"

	"goph-profile-avatars/internal/repository"
	"goph-profile-avatars/internal/services"
)

type AvatarRepositoryInterface interface {
	GetAvatarByID(ctx context.Context, avatarID string) (*repository.Avatar, error)
	UpdateProcessingStatus(ctx context.Context, avatarID, status string) error
	FailProcessing(ctx context.Context, avatarID string) error
	CompleteProcessing(ctx context.Context, avatarID string, thumbnailKeys []byte) error
	SoftDeleteAvatar(ctx context.Context, avatarID string) error
}

type StorageInterface interface {
	Download(ctx context.Context, key string) (*services.DownloadResult, error)
	Upload(ctx context.Context, key string, body io.Reader, size int64, contentType string) error
	Delete(ctx context.Context, key string) error
}

type Service struct {
	avatarRepo AvatarRepositoryInterface
	storage    StorageInterface
	log        *slog.Logger
}

func NewAvatarWorkerService(
	avatarRepo AvatarRepositoryInterface,
	storage StorageInterface,
	log *slog.Logger,
) *Service {
	return &Service{
		avatarRepo: avatarRepo,
		storage:    storage,
		log:        log,
	}
}

func (s *Service) HandleUpload(ctx context.Context, event AvatarUploadEvent) error {
	s.log.Info("handle upload started", "avatar_id", event.AvatarID)

	avatar, err := s.avatarRepo.GetAvatarByID(ctx, event.AvatarID)
	if err != nil {
		return fmt.Errorf("get avatar by id: %w", err)
	}

	switch avatar.ProcessingStatus {
	case "completed":
		s.log.Info("avatar already completed", "avatar_id", avatar.ID)
		return nil
	case "processing":
		s.log.Info("avatar already processing", "avatar_id", avatar.ID)
		return nil
	}

	if err := s.avatarRepo.UpdateProcessingStatus(ctx, avatar.ID, "processing"); err != nil {
		return fmt.Errorf("update processing status to processing: %w", err)
	}

	if err := s.processAvatar(ctx, avatar); err != nil {
		if failErr := s.avatarRepo.FailProcessing(ctx, avatar.ID); failErr != nil {
			s.log.Error(
				"set failed processing status failed",
				"avatar_id", avatar.ID,
				"error", failErr,
			)
		}
		return err
	}

	s.log.Info("handle upload finished successfully", "avatar_id", avatar.ID)
	return nil
}

func (s *Service) HandleDelete(ctx context.Context, event AvatarDeleteEvent) error {
	s.log.Info("handle delete started", "avatar_id", event.AvatarID)

	avatar, err := s.avatarRepo.GetAvatarByID(ctx, event.AvatarID)
	if err != nil {
		if err == repository.ErrAvatarNotFound {
			s.log.Info("avatar already deleted or not found", "avatar_id", event.AvatarID)
			return nil
		}
		return fmt.Errorf("get avatar by id: %w", err)
	}

	if err := s.deleteAvatarFiles(ctx, avatar); err != nil {
		return fmt.Errorf("delete avatar files: %w", err)
	}

	if err := s.avatarRepo.SoftDeleteAvatar(ctx, avatar.ID); err != nil {
		return fmt.Errorf("soft delete avatar in db: %w", err)
	}

	s.log.Info("handle delete finished successfully", "avatar_id", avatar.ID)
	return nil
}

func (s *Service) processAvatar(ctx context.Context, avatar *repository.Avatar) error {
	downloaded, err := s.storage.Download(ctx, avatar.S3Key)
	if err != nil {
		return fmt.Errorf("download original from storage: %w", err)
	}
	defer func() {
		_ = downloaded.Reader.Close()
	}()

	originalBytes, err := io.ReadAll(downloaded.Reader)
	if err != nil {
		return fmt.Errorf("read original image: %w", err)
	}

	srcImage, err := imaging.Decode(bytes.NewReader(originalBytes))
	if err != nil {
		return fmt.Errorf("decode image: %w", err)
	}

	thumb100 := imaging.Fill(srcImage, 100, 100, imaging.Center, imaging.Lanczos)
	thumb300 := imaging.Fill(srcImage, 300, 300, imaging.Center, imaging.Lanczos)

	thumb100Buf := new(bytes.Buffer)
	if err := imaging.Encode(thumb100Buf, thumb100, imaging.JPEG); err != nil {
		return fmt.Errorf("encode 100x100 thumbnail: %w", err)
	}

	thumb300Buf := new(bytes.Buffer)
	if err := imaging.Encode(thumb300Buf, thumb300, imaging.JPEG); err != nil {
		return fmt.Errorf("encode 300x300 thumbnail: %w", err)
	}

	baseDir := path.Dir(avatar.S3Key)
	thumb100Key := fmt.Sprintf("%s/100x100.jpg", baseDir)
	thumb300Key := fmt.Sprintf("%s/300x300.jpg", baseDir)

	if err := s.storage.Upload(
		ctx,
		thumb100Key,
		bytes.NewReader(thumb100Buf.Bytes()),
		int64(thumb100Buf.Len()),
		"image/jpeg",
	); err != nil {
		return fmt.Errorf("upload 100x100 thumbnail: %w", err)
	}

	if err := s.storage.Upload(
		ctx,
		thumb300Key,
		bytes.NewReader(thumb300Buf.Bytes()),
		int64(thumb300Buf.Len()),
		"image/jpeg",
	); err != nil {
		return fmt.Errorf("upload 300x300 thumbnail: %w", err)
	}

	thumbnailKeys := map[string]string{
		"100x100": thumb100Key,
		"300x300": thumb300Key,
	}

	thumbnailKeysJSON, err := json.Marshal(thumbnailKeys)
	if err != nil {
		return fmt.Errorf("marshal thumbnail keys: %w", err)
	}

	if err := s.avatarRepo.CompleteProcessing(ctx, avatar.ID, thumbnailKeysJSON); err != nil {
		return fmt.Errorf("complete processing: %w", err)
	}

	return nil
}

func (s *Service) deleteAvatarFiles(ctx context.Context, avatar *repository.Avatar) error {
	if avatar.S3Key != "" {
		if err := s.storage.Delete(ctx, avatar.S3Key); err != nil {
			return fmt.Errorf("delete original file: %w", err)
		}
	}

	if len(avatar.ThumbnailS3Keys) == 0 {
		return nil
	}

	var thumbnailKeys map[string]string
	if err := json.Unmarshal(avatar.ThumbnailS3Keys, &thumbnailKeys); err != nil {
		return fmt.Errorf("unmarshal thumbnail keys: %w", err)
	}

	for size, key := range thumbnailKeys {
		if key == "" {
			continue
		}

		if err := s.storage.Delete(ctx, key); err != nil {
			return fmt.Errorf("delete thumbnail %s: %w", size, err)
		}
	}

	return nil
}
