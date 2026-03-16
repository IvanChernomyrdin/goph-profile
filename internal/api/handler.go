package api

type AvatarService interface {
	UploadAvatar(input UploadAvatarInput) (*UploadAvatarResult, error)
	GetAvatarByID(avatarID, size string) (*GetAvatarResult, error)
	GetUserAvatar(userID string) (*GetAvatarResult, error)
	GetListUserAvatar(userID string) ([]AvatarItem, error)
	UpdateCurrentAvatar(userID, avatarID string) error
}

type Handler struct {
	healthService *HealthService
	avatarService AvatarService
}

func NewHandler(
	healthService *HealthService,
	avatarService AvatarService,
) *Handler {
	return &Handler{
		healthService: healthService,
		avatarService: avatarService,
	}
}
