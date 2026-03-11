package api

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

type PostgresChecker interface {
	PingContext(ctx context.Context) error
}

type MinIOChecker interface {
	Check(ctx context.Context) error
}

type RabbitMQChecker interface {
	Check(ctx context.Context) error
}

type HealthService struct {
	postgres PostgresChecker
	minio    MinIOChecker
	rabbitmq RabbitMQChecker
}

type HealthResponse struct {
	Status    string            `json:"status"`
	Services  map[string]string `json:"services"`
	Timestamp string            `json:"timestamp"`
}

func NewHealthService(postgres PostgresChecker, minio MinIOChecker, rabbitmq RabbitMQChecker) *HealthService {
	return &HealthService{
		postgres: postgres,
		minio:    minio,
		rabbitmq: rabbitmq,
	}
}

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	resp := HealthResponse{
		Status: "ok",
		Services: map[string]string{
			"postgres": "ok",
			"minio":    "ok",
			"rabbitmq": "ok",
		},
		Timestamp: time.Now().Format(time.RFC3339),
	}

	if err := h.healthService.postgres.PingContext(ctx); err != nil {
		resp.Status = "degraded"
		resp.Services["postgres"] = "error"
	}

	if err := h.healthService.minio.Check(ctx); err != nil {
		resp.Status = "degraded"
		resp.Services["minio"] = "error"
	}

	if err := h.healthService.rabbitmq.Check(ctx); err != nil {
		resp.Status = "degraded"
		resp.Services["rabbitmq"] = "error"
	}

	w.Header().Set("Content-Type", "application/json")

	if resp.Status != "ok" {
		w.WriteHeader(http.StatusServiceUnavailable)
	} else {
		w.WriteHeader(http.StatusOK)
	}

	json.NewEncoder(w).Encode(resp)
}
