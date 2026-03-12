package worker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path"
	"path/filepath"
	"strings"

	"github.com/disintegration/imaging"

	"goph-profile-avatars/internal/repository"
	"goph-profile-avatars/internal/services"
)

type Service struct {
	avatarRepo *repository.AvatarRepository
	storage    *services.MinIOStorage
	log        Logger
}

func NewAvatarWorkerService(
	avatarRepo *repository.AvatarRepository,
	storage *services.MinIOStorage,
	log Logger,
) *Service {
	return &Service{
		avatarRepo: avatarRepo,
		storage:    storage,
		log:        log,
	}
}

// HandleUpload:
// 1. достаёт запись из БД
// 2. проверяет статус
// 3. ставит processing
// 4. скачивает оригинал из MinIO
// 5. создаёт 100x100 и 300x300
// 6. загружает их в MinIO
// 7. обновляет БД в completed
func (s *Service) HandleUpload(ctx context.Context, event AvatarUploadEvent) error {
	s.log.Infof("handle upload started: avatar_id=%s", event.AvatarID)

	avatar, err := s.avatarRepo.GetAvatarByID(ctx, event.AvatarID)
	if err != nil {
		return fmt.Errorf("get avatar by id: %w", err)
	}

	// Идемпотентность: если уже обработано — просто выходим.
	if avatar.ProcessingStatus == "completed" {
		s.log.Infof("avatar already completed: avatar_id=%s", avatar.ID)
		return nil
	}

	// Если вдруг уже в processing/completed/failed — можешь менять логику по желанию.
	if err := s.avatarRepo.UpdateProcessingStatus(ctx, avatar.ID, "processing"); err != nil {
		return fmt.Errorf("update processing status to processing: %w", err)
	}

	if err := s.processAvatar(ctx, avatar); err != nil {
		failErr := s.avatarRepo.FailProcessing(ctx, avatar.ID)
		if failErr != nil {
			s.log.Errorf("failed to set failed status for avatar_id=%s: %v", avatar.ID, failErr)
		}
		return err
	}

	s.log.Infof("handle upload finished successfully: avatar_id=%s", avatar.ID)
	return nil
}

func (s *Service) processAvatar(ctx context.Context, avatar *repository.Avatar) error {
	reader, err := s.storage.Download(ctx, avatar.S3Key)
	if err != nil {
		return fmt.Errorf("download original from minio: %w", err)
	}
	defer reader.Close()

	originalBytes, err := io.ReadAll(reader)
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
	originalExt := strings.ToLower(filepath.Ext(avatar.S3Key))
	if originalExt == "" {
		originalExt = ".jpg"
	}

	_ = originalExt // пока не используем, оставил на будущее

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
