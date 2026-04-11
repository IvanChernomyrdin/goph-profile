package http

import (
	"net/http"

	"goph-profile-avatars/internal/api"
	"goph-profile-avatars/internal/middleware"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	_ "goph-profile-avatars/docs"

	"github.com/go-chi/chi/v5"
	httpSwagger "github.com/swaggo/http-swagger/v2"
)

func NewRouter(h *api.Handler, rateLimit int) http.Handler {
	r := chi.NewRouter()
	// observability
	r.Use(middleware.TracingMiddleware("gophprofile-http"))
	r.Use(middleware.LoggerMiddleware())
	r.Get("/metrics", promhttp.Handler().ServeHTTP)

	r.Get("/swagger/*", httpSwagger.Handler(
		httpSwagger.URL("/swagger/doc.json"),
	))

	r.Route("/health", func(r chi.Router) {
		r.Get("/live", h.Liveness)
		r.Get("/ready", h.Readiness)
	})

	r.Route("/api/v1", func(r chi.Router) {
		r.Use(middleware.RateLimitMiddleware(rateLimit))

		// Загрузка аватарки
		r.Post("/avatars", h.UploadAvatar)
		// получение главной аватарки пользователя
		r.Get("/users/{user_id}/avatar", h.GetUserAvatar)
		// получение списка аватарок пользователя
		r.Get("/users/{user_id}/avatars", h.GetUserAvatars)
		// получение аватарки по ID
		r.Get("/avatars/{avatar_id}", h.GetAvatar)
		// установка аватарки на главную
		r.Patch("/avatars/{avatar_id}/current", h.UpdateCurrentAvatar)
		// удаление аватарки по ID
		r.Delete("/avatars/{avatar_id}", h.DeleteAvatarByID)
		// удаление аватарки пользователя с позиции главной
		r.Delete("/users/{user_id}/avatar", h.DeleteUserCurrentAvatar)
	})

	// web отрисовка
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "./web/static/index.html")
	})
	// статика для фронта
	fileServer := http.FileServer(http.Dir("./web/static"))
	r.Handle("/static/*", http.StripPrefix("/static/", fileServer))

	return r
}
