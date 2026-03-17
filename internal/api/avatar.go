package api

import (
	"encoding/json"
	"errors"
	"io"
	"strconv"
	"strings"

	"net/http"

	constErr "goph-profile-avatars/internal/errors"
	"goph-profile-avatars/internal/repository"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
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

func (h *Handler) GetAvatar(w http.ResponseWriter, r *http.Request) {
	avatarID := strings.TrimSpace(chi.URLParam(r, "avatar_id"))
	if avatarID == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{
			Error:   "avatar_id is required",
			Details: "path param avatar_id is required",
		})
		return
	}

	if _, err := uuid.Parse(avatarID); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{
			Error:   "invalid avatar_id",
			Details: "avatar_id must be a valid UUID",
		})
		return
	}

	size := strings.TrimSpace(r.URL.Query().Get("size"))
	if size != "" && size != "original" && size != "100x100" && size != "300x300" {
		writeJSON(w, http.StatusBadRequest, errorResponse{
			Error:   "invalid size",
			Details: "allowed values: original, 100x100, 300x300",
		})
		return
	}

	result, err := h.avatarService.GetAvatarByID(avatarID, size)
	if err != nil {
		if errors.Is(err, repository.ErrAvatarNotFound) {
			writeJSON(w, http.StatusNotFound, errorResponse{
				Error: "avatar not found",
			})
			return
		}

		writeJSON(w, http.StatusInternalServerError, errorResponse{
			Error:   "failed to get avatar",
			Details: err.Error(),
		})
		return
	}
	defer result.Reader.Close()

	w.Header().Set("Content-Type", result.MimeType)
	w.Header().Set("Content-Length", int64ToString(result.SizeBytes))
	w.Header().Set("Cache-Control", "max-age=86400")
	w.WriteHeader(http.StatusOK)

	io.Copy(w, result.Reader)
}

func (h *Handler) GetUserAvatar(w http.ResponseWriter, r *http.Request) {
	userID := strings.TrimSpace(chi.URLParam(r, "user_id"))
	if userID == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{
			Error:   "user_id is required",
			Details: "path param user_id is required",
		})
		return
	}

	result, err := h.avatarService.GetUserAvatar(userID)
	if err != nil {
		if errors.Is(err, repository.ErrAvatarNotFound) {
			writeJSON(w, http.StatusNotFound, errorResponse{
				Error: "avatar not found",
			})
			return
		}

		writeJSON(w, http.StatusInternalServerError, errorResponse{
			Error:   "failed to get user avatar",
			Details: err.Error(),
		})
		return
	}
	defer result.Reader.Close()

	w.Header().Set("Content-Type", result.MimeType)
	w.Header().Set("Content-Length", int64ToString(result.SizeBytes))
	w.Header().Set("Cache-Control", "max-age=86400")
	w.WriteHeader(http.StatusOK)

	io.Copy(w, result.Reader)
}

func (h *Handler) GetUserAvatars(w http.ResponseWriter, r *http.Request) {
	userID := strings.TrimSpace(chi.URLParam(r, "user_id"))
	if userID == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{
			Error:   "user_id is required",
			Details: "path param user_id is required",
		})
		return
	}

	result, err := h.avatarService.GetListUserAvatar(userID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{
			Error:   "failed to get user avatars",
			Details: err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) UpdateCurrentAvatar(w http.ResponseWriter, r *http.Request) {
	avatarID := strings.TrimSpace(chi.URLParam(r, "avatar_id"))
	if avatarID == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{
			Error:   "avatar_id is required",
			Details: "path param avatar_id is required",
		})
		return
	}

	if _, err := uuid.Parse(avatarID); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{
			Error:   "invalid avatar_id",
			Details: "avatar_id must be a valid UUID",
		})
		return
	}

	userID := strings.TrimSpace(r.Header.Get("X-User-ID"))
	if userID == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{
			Error:   constErr.ErrXUserID.Error(),
			Details: "X-User-ID header is required",
		})
		return
	}

	if err := h.avatarService.UpdateCurrentAvatar(userID, avatarID); err != nil {
		if errors.Is(err, repository.ErrAvatarNotFound) {
			writeJSON(w, http.StatusNotFound, errorResponse{
				Error: "avatar not found",
			})
			return
		}

		writeJSON(w, http.StatusInternalServerError, errorResponse{
			Error:   "failed to update current avatar",
			Details: err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"message":   "current avatar updated successfully",
		"avatar_id": avatarID,
		"user_id":   userID,
	})
}

func (h *Handler) DeleteUserCurrentAvatar(w http.ResponseWriter, r *http.Request) {
	userID := strings.TrimSpace(chi.URLParam(r, "user_id"))
	if userID == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{
			Error:   "user_id is required",
			Details: "path param user_id is required",
		})
		return
	}

	if err := h.avatarService.DeleteCurrentUserAvatar(userID); err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{
			Error:   "error deleting current user avatar",
			Details: err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "success",
		"message": "Current avatar status removed successfully.",
	})
}

func (h *Handler) DeleteAvatarByID(w http.ResponseWriter, r *http.Request) {
	avatarID := strings.TrimSpace(chi.URLParam(r, "avatar_id"))
	if avatarID == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{
			Error:   "avatar_id is required",
			Details: "path param avatar_id is required",
		})
		return
	}
	userID := strings.TrimSpace(r.Header.Get("X-User-ID"))
	if userID == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{
			Error:   constErr.ErrXUserID.Error(),
			Details: "X-User-ID header is required",
		})
		return
	}
	if _, err := uuid.Parse(avatarID); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{
			Error:   "invalid avatar_id",
			Details: "avatar_id must be a valid UUID",
		})
		return
	}
	// вызываем функцию сервисного слоя которая отправит в rabbitmq сообщение о запросе на удаление
	err := h.avatarService.DeleteAvatarByID(avatarID, userID)
	if err != nil {
		if errors.Is(err, constErr.ErrAvatarDeletionAlreadyQueued) {
			writeJSON(w, http.StatusAccepted, map[string]string{
				"status":  "already_queued",
				"message": "Avatar is already in deletion queue",
			})
			return
		}
		writeJSON(w, http.StatusInternalServerError, errorResponse{
			Error:   err.Error(),
			Details: "error sending to rabbitMQ",
		})
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{
		"status":  "queued",
		"message": "Avatar has been added to deletion queue",
	})
}

func int64ToString(v int64) string {
	return strconv.FormatInt(v, 10)
}

func writeJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(payload)
}
