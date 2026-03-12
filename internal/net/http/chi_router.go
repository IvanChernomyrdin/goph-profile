package http

import (
	"net/http"

	"goph-profile-avatars/internal/api"
	"goph-profile-avatars/internal/middleware"

	"github.com/go-chi/chi/v5"
)

func NewRouter(h *api.Handler) http.Handler {
	r := chi.NewRouter()
	// логирование всех запросов
	r.Use(middleware.LoggerMiddleware())

	// Проверка работоспособности
	r.Get("/health", h.Health)

	// API v1
	r.Route("/api/v1", func(r chi.Router) {
		// Загрузка аватарки
		r.Post("/avatars", h.UploadAvatar)
		// Получение аватарки
		r.Get("/avatars/{avatar_id}", h.GetAvatar)
		r.Get("/users/{user_id}/avatar", h.GetUserAvatar)
		// Удаление аватарки
		r.Delete("/avatars/{avatar_id}", h.DeleteAvatar)
		r.Delete("/users/{user_id}/avatar", h.DeleteUserAvatar)
		// Получение метаданных аватарки
		r.Get("/avatars/{avatar_id}/metadata", h.GetAvatarMetadata)
		// Список аватарок пользователя
		r.Get("/users/{user_id}/avatars", h.GetUserAvatars)
	})

	// // Веб-интерфейс
	// r.Route("/web", func(r chi.Router) {
	// 	// форма загрузки
	// 	r.Get("/", h.WebUploadPage)
	// 	// обработка загрузки
	// 	r.Post("/upload", h.WebUploadAvatar)
	// 	// галерея аватарок
	// 	r.Get("/gallery/{user_id}", h.WebGallery)
	// })

	// web
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "./web/static/index.html")
	})

	// статика для фронта
	fileServer := http.FileServer(http.Dir("./web/static"))
	r.Handle("/static/*", http.StripPrefix("/static/", fileServer))

	return r
}
