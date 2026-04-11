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

// Liveness godoc
// @Summary Liveness probe
// @Description Проверка, что HTTP-сервис запущен.
// @Tags health
// @Produce plain
// @Success 200 {string} string "ok"
// @Router /health/live [get]
func (h *Handler) Liveness(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

// Readiness godoc
// @Summary Readiness probe
// @Description Проверка готовности сервиса к обработке трафика и доступности зависимостей.
// @Tags health
// @Produce json
// @Success 200 {object} HealthResponse
// @Failure 503 {object} HealthResponse
// @Router /health/ready [get]
func (h *Handler) Readiness(w http.ResponseWriter, r *http.Request) {
	if h.isReady != nil && !h.isReady.Load() {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(HealthResponse{
			Status: "shutting_down",
			Services: map[string]string{
				"server": "not_ready",
			},
			Timestamp: time.Now().Format(time.RFC3339),
		})
		return
	}

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
		resp.Services["rabbitmq"] = "error"
	}

	w.Header().Set("Content-Type", "application/json")

	if resp.Status != "ok" {
		w.WriteHeader(http.StatusServiceUnavailable)
	} else {
		w.WriteHeader(http.StatusOK)
	}

	if err := json.NewEncoder(w).Encode(resp); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}
