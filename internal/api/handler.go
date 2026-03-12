package api

type AvatarService interface {
	UploadAvatar(input UploadAvatarInput) (*UploadAvatarResult, error)
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
