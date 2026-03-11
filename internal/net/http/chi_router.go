// Package http реализует маршрутизацию HTTP-слоя сервера GophKeeper.
//
// Пакет отвечает за:
//   - регистрацию HTTP-маршрутов и настройку роутера (chi);
//   - логирование выполнения HTTP-запросов;
//   - выполняет проверку JWT access-токенов;
package http

import (
	"net/http"

	"goph-profile-avatars/internal/api"
	"goph-profile-avatars/internal/middleware"

	"github.com/go-chi/chi/v5"
	httpSwagger "github.com/swaggo/http-swagger"
)

// NewRouter создаёт и настраивает HTTP-роутер сервера.
//
// Роутер использует chi.Router и регистрирует:
//   - публичные эндпоинты аутентификации под префиксом /auth;
//   - middleware логирования для всех запросов;
//   - группу защищённых JWT эндпоинтов (пока без маршрутов secrets).
func NewRouter(h *api.Handler) http.Handler {
	r := chi.NewRouter()
	// логирование всех запросов
	r.Use(middleware.LoggerMiddleware())
	// добавляем swagger
	r.Get("/swagger/*", httpSwagger.WrapHandler)
	// проверка ответа от postgres и minIO
	r.Get("/health", h.Health)

	return r
}
