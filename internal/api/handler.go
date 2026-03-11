package api

type Handler struct {
	healthService *HealthService
}

func NewHandler(healthService *HealthService) *Handler {
	return &Handler{
		healthService: healthService,
	}
}
