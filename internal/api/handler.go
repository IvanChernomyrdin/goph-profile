package api

import (
	"context"
	"sync/atomic"
)

type AvatarService interface {
	UploadAvatar(ctx context.Context, input UploadAvatarInput) (*UploadAvatarResult, error)
	GetAvatarByID(ctx context.Context, avatarID, size string) (*GetAvatarResult, error)
	GetUserAvatar(ctx context.Context, userID string) (*GetAvatarResult, error)
	GetListUserAvatar(ctx context.Context, userID string) ([]AvatarItem, error)
	UpdateCurrentAvatar(ctx context.Context, userID, avatarID string) error
	DeleteAvatarByID(ctx context.Context, avatarID, userID string) error
	DeleteCurrentUserAvatar(ctx context.Context, userID string) error
}

type Handler struct {
	healthService *HealthService
	avatarService AvatarService
	isReady       *atomic.Bool
}

func NewHandler(
	healthService *HealthService,
	avatarService AvatarService,
	isReady *atomic.Bool,
) *Handler {
	return &Handler{
		healthService: healthService,
		avatarService: avatarService,
		isReady:       isReady,
	}
}
