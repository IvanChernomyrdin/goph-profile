package api

import (
	"encoding/json"
	"errors"
	"strings"

	"net/http"

	constErr "goph-profile-avatars/internal/errors"
)

const maxAvatarSize = 10 << 20 // 10 MB

type errorResponse struct {
	Error   string `json:"error"`
	Details string `json:"details,omitempty"`
	MaxSize int64  `json:"max_size,omitempty"`
}

func (h *Handler) UploadAvatar(w http.ResponseWriter, r *http.Request) {
	// Получаем userID из обязательного заголовка.
	userID := strings.TrimSpace(r.Header.Get("X-User-ID"))
	if userID == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{
			Error:   constErr.ErrXUserID.Error(),
			Details: "X-User-ID header is required",
		})
		return
	}

	// Ограничиваем размер тела запроса
	r.Body = http.MaxBytesReader(w, r.Body, maxAvatarSize)

	// Парсим multipart/form-data
	if err := r.ParseMultipartForm(maxAvatarSize); err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			writeJSON(w, http.StatusRequestEntityTooLarge, errorResponse{
				Error:   "File too large",
				MaxSize: maxAvatarSize,
			})
			return
		}

		writeJSON(w, http.StatusBadRequest, errorResponse{
			Error:   "Invalid multipart form",
			Details: err.Error(),
		})
		return
	}

	// Получаем файл
	file, header, err := r.FormFile("file")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{
			Error:   "File is required",
			Details: "multipart field 'file' is missing",
		})
		return
	}
	defer file.Close()

	// Простейшая валидация имени файла
	if strings.TrimSpace(header.Filename) == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{
			Error:   "Invalid file",
			Details: "file name is empty",
		})
		return
	}

	// Передаём всё в сервисный слой.
	result, err := h.avatarService.UploadAvatar(UploadAvatarInput{
		UserID:      userID,
		FileName:    header.Filename,
		ContentType: header.Header.Get("Content-Type"),
		Size:        header.Size,
		File:        file,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{
			Error:   "Failed to upload avatar",
			Details: err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusCreated, result)
}

func (h *Handler) GetAvatar(w http.ResponseWriter, r *http.Request)         {}
func (h *Handler) GetUserAvatar(w http.ResponseWriter, r *http.Request)     {}
func (h *Handler) DeleteAvatar(w http.ResponseWriter, r *http.Request)      {}
func (h *Handler) DeleteUserAvatar(w http.ResponseWriter, r *http.Request)  {}
func (h *Handler) GetAvatarMetadata(w http.ResponseWriter, r *http.Request) {}
func (h *Handler) GetUserAvatars(w http.ResponseWriter, r *http.Request)    {}

func writeJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(payload)
}
