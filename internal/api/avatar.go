package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"

	constErr "goph-profile-avatars/internal/errors"
	"goph-profile-avatars/internal/repository"
	"goph-profile-avatars/internal/resilience"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

const maxAvatarSize = 10 << 20 // 10 MB

type ErrorResponse struct {
	Error   string `json:"error"`
	Details string `json:"details,omitempty"`
	MaxSize int64  `json:"max_size,omitempty"`
}

var allowedAvatarMIMETypes = map[string]struct{}{
	"image/jpeg": {},
	"image/png":  {},
	"image/webp": {},
}

// isContextCancelled проверяет отмену контекста
func isContextCancelled(ctx context.Context, w http.ResponseWriter) bool {
	select {
	case <-ctx.Done():
		writeJSON(w, http.StatusBadRequest, ErrorResponse{
			Error:   "Request cancelled",
			Details: ctx.Err().Error(),
		})
		return true
	default:
		return false
	}
}

// extractUserID извлекает и валидирует user ID из заголовка
func extractUserID(r *http.Request, w http.ResponseWriter) (string, bool) {
	userID := strings.TrimSpace(r.Header.Get("X-User-ID"))
	if userID == "" {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{
			Error:   constErr.ErrXUserID.Error(),
			Details: "X-User-ID header is required",
		})
		return "", false
	}
	return userID, true
}

// parseMultipartFile парсит multipart форму и извлекает файл
func parseMultipartFile(r *http.Request, w http.ResponseWriter) (multipart.File, *multipart.FileHeader, bool) {
	if err := r.ParseMultipartForm(maxAvatarSize); err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			writeJSON(w, http.StatusRequestEntityTooLarge, ErrorResponse{
				Error:   "File too large",
				MaxSize: maxAvatarSize,
			})
			return nil, nil, false
		}
		writeJSON(w, http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid multipart form",
			Details: err.Error(),
		})
		return nil, nil, false
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{
			Error:   "File is required",
			Details: "multipart field 'file' is missing",
		})
		return nil, nil, false
	}

	return file, header, true
}

// validateFileName проверяет имя файла
func validateFileName(filename string, w http.ResponseWriter) bool {
	if strings.TrimSpace(filename) == "" {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid file",
			Details: "file name is empty",
		})
		return false
	}
	return true
}

// validateFileSize проверяет размер файла
func validateFileSize(size int64, w http.ResponseWriter) bool {
	if size <= 0 {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid file",
			Details: "file size is invalid",
		})
		return false
	}
	if size > maxAvatarSize {
		writeJSON(w, http.StatusRequestEntityTooLarge, ErrorResponse{
			Error:   "File too large",
			MaxSize: maxAvatarSize,
		})
		return false
	}
	return true
}

// handleServiceError обрабатывает ошибки
func handleServiceError(err error, w http.ResponseWriter) bool {
	if resilience.IsOpen(err) {
		writeJSON(w, http.StatusServiceUnavailable, ErrorResponse{
			Error:   "Dependency unavailable",
			Details: "External dependency circuit breaker is open",
		})
		return true
	}
	if errors.Is(err, context.Canceled) {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{
			Error:   "Request cancelled",
			Details: "The request was cancelled by the client",
		})
		return true
	}
	if errors.Is(err, context.DeadlineExceeded) {
		writeJSON(w, http.StatusGatewayTimeout, ErrorResponse{
			Error:   "Request timeout",
			Details: "The request took too long to complete",
		})
		return true
	}
	if errors.Is(err, repository.ErrAvatarNotFound) {
		writeJSON(w, http.StatusNotFound, ErrorResponse{
			Error: "avatar not found",
		})
		return true
	}
	return false
}

// validateAvatarID проверяет корректность UUID аватарки
func validateAvatarID(avatarID string, w http.ResponseWriter) bool {
	if avatarID == "" {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{
			Error:   "avatar_id is required",
			Details: "path param avatar_id is required",
		})
		return false
	}
	if _, err := uuid.Parse(avatarID); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{
			Error:   "invalid avatar_id",
			Details: "avatar_id must be a valid UUID",
		})
		return false
	}
	return true
}

// validateUserID проверяет user ID из path параметра
func validateUserID(userID string, w http.ResponseWriter) bool {
	if userID == "" {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{
			Error:   "user_id is required",
			Details: "path param user_id is required",
		})
		return false
	}
	return true
}

// validateSizeParam проверяет параметр size
func validateSizeParam(size string, w http.ResponseWriter) bool {
	if size != "" && size != "original" && size != "100x100" && size != "300x300" {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{
			Error:   "invalid size",
			Details: "allowed values: original, 100x100, 300x300",
		})
		return false
	}
	return true
}

// writeFileResponse записывает файл в ответ с заголовками
func writeFileResponse(w http.ResponseWriter, result *GetAvatarResult) {
	w.Header().Set("Content-Type", result.MimeType)
	w.Header().Set("Content-Length", int64ToString(result.SizeBytes))
	w.Header().Set("Cache-Control", "max-age=86400")
	w.WriteHeader(http.StatusOK)

	if _, err := io.Copy(w, result.Reader); err != nil {
		return
	}
}

// UploadAvatar godoc
// @Summary Загрузить аватар
// @Description Загружает новый файл аватарки пользователя.
// @Tags avatars
// @Accept mpfd
// @Produce json
// @Param X-User-ID header string true "ID пользователя"
// @Param file formData file true "Файл аватарки (jpeg/png/webp, до 10MB)"
// @Success 201 {object} UploadAvatarResult
// @Failure 400 {object} ErrorResponse
// @Failure 408 {object} ErrorResponse
// @Failure 413 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /avatars [post]
func (h *Handler) UploadAvatar(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if isContextCancelled(ctx, w) {
		return
	}

	userID, ok := extractUserID(r, w)
	if !ok {
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxAvatarSize)

	file, header, ok := parseMultipartFile(r, w)
	if !ok {
		return
	}
	defer func() {
		_ = file.Close()
	}()

	if !validateFileName(header.Filename, w) {
		return
	}

	if !validateFileSize(header.Size, w) {
		return
	}

	detectedContentType, err := detectAndValidateAvatarMime(file)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{
			Error:   "Unsupported file type",
			Details: err.Error(),
		})
		return
	}

	result, err := h.avatarService.UploadAvatar(ctx, UploadAvatarInput{
		UserID:      userID,
		FileName:    header.Filename,
		ContentType: detectedContentType,
		Size:        header.Size,
		File:        file,
	})

	if handleServiceError(err, w) {
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to upload avatar",
			Details: err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusCreated, result)
}

// GetAvatar godoc
// @Summary Получить аватар по ID
// @Description Возвращает файл аватарки по avatar_id. Поддерживается параметр size.
// @Tags avatars
// @Produce octet-stream
// @Param avatar_id path string true "UUID аватарки"
// @Param size query string false "Размер аватарки" Enums(original,100x100,300x300)
// @Success 200 {file} file
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /avatars/{avatar_id} [get]
func (h *Handler) GetAvatar(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	avatarID := strings.TrimSpace(chi.URLParam(r, "avatar_id"))
	if !validateAvatarID(avatarID, w) {
		return
	}

	size := strings.TrimSpace(r.URL.Query().Get("size"))
	if !validateSizeParam(size, w) {
		return
	}

	result, err := h.avatarService.GetAvatarByID(ctx, avatarID, size)

	if handleServiceError(err, w) {
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{
			Error:   "failed to get avatar",
			Details: err.Error(),
		})
		return
	}
	defer func() {
		_ = result.Reader.Close()
	}()

	writeFileResponse(w, result)
}

// GetUserAvatar godoc
// @Summary Получить текущую аватарку пользователя
// @Description Возвращает текущую активную аватарку пользователя.
// @Tags users
// @Produce octet-stream
// @Param user_id path string true "ID пользователя"
// @Success 200 {file} file
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /users/{user_id}/avatar [get]
func (h *Handler) GetUserAvatar(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userID := strings.TrimSpace(chi.URLParam(r, "user_id"))
	if !validateUserID(userID, w) {
		return
	}

	result, err := h.avatarService.GetUserAvatar(ctx, userID)

	if handleServiceError(err, w) {
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{
			Error:   "failed to get user avatar",
			Details: err.Error(),
		})
		return
	}
	defer func() {
		_ = result.Reader.Close()
	}()

	writeFileResponse(w, result)
}

// GetUserAvatars godoc
// @Summary Получить список аватарок пользователя
// @Description Возвращает список всех аватарок пользователя.
// @Tags users
// @Produce json
// @Param user_id path string true "ID пользователя"
// @Success 200 {array} AvatarItem
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /users/{user_id}/avatars [get]
func (h *Handler) GetUserAvatars(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userID := strings.TrimSpace(chi.URLParam(r, "user_id"))
	if !validateUserID(userID, w) {
		return
	}

	result, err := h.avatarService.GetListUserAvatar(ctx, userID)
	if err != nil {
		if handleServiceError(err, w) {
			return
		}
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{
			Error:   "failed to get user avatars",
			Details: err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// UpdateCurrentAvatar godoc
// @Summary Сделать аватарку текущей
// @Description Устанавливает выбранную аватарку как основную для пользователя.
// @Tags avatars
// @Accept json
// @Produce json
// @Param avatar_id path string true "UUID аватарки"
// @Param X-User-ID header string true "ID пользователя"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /avatars/{avatar_id}/current [patch]
func (h *Handler) UpdateCurrentAvatar(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	avatarID := strings.TrimSpace(chi.URLParam(r, "avatar_id"))
	if !validateAvatarID(avatarID, w) {
		return
	}

	userID, ok := extractUserID(r, w)
	if !ok {
		return
	}

	err := h.avatarService.UpdateCurrentAvatar(ctx, userID, avatarID)

	if handleServiceError(err, w) {
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{
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

// DeleteUserCurrentAvatar godoc
// @Summary Убрать статус текущей аватарки
// @Description Удаляет статус текущей аватарки пользователя без удаления самой записи аватарки.
// @Tags users
// @Accept json
// @Produce json
// @Param user_id path string true "ID пользователя"
// @Param X-User-ID header string true "ID авторизованного пользователя"
// @Success 200 {object} map[string]string
// @Failure 400 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /users/{user_id}/avatar [delete]
func (h *Handler) DeleteUserCurrentAvatar(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userID := strings.TrimSpace(chi.URLParam(r, "user_id"))
	if !validateUserID(userID, w) {
		return
	}

	// Получаем авторизованного пользователя из заголовка
	authUserID, ok := extractUserID(r, w)
	if !ok {
		return
	}

	// Проверяем, что пользователь удаляет свою собственную аватарку
	if authUserID != userID {
		writeJSON(w, http.StatusForbidden, ErrorResponse{
			Error:   "Forbidden",
			Details: "You can only delete your own current avatar",
		})
		return
	}

	err := h.avatarService.DeleteCurrentUserAvatar(ctx, userID)
	if err != nil {
		if handleServiceError(err, w) {
			return
		}
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{
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

// DeleteAvatarByID godoc
// @Summary Удалить аватарку по ID
// @Description Ставит аватарку в очередь на асинхронное удаление.
// @Tags avatars
// @Accept json
// @Produce json
// @Param avatar_id path string true "UUID аватарки"
// @Param X-User-ID header string true "ID пользователя"
// @Success 202 {object} map[string]string
// @Failure 400 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 410 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /avatars/{avatar_id} [delete]
func (h *Handler) DeleteAvatarByID(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	avatarID := strings.TrimSpace(chi.URLParam(r, "avatar_id"))
	if !validateAvatarID(avatarID, w) {
		return
	}

	userID, ok := extractUserID(r, w)
	if !ok {
		return
	}

	err := h.avatarService.DeleteAvatarByID(ctx, avatarID, userID)
	if err != nil {
		// Проверяем ошибку авторизации (аватарка не принадлежит пользователю)
		if strings.Contains(err.Error(), "unauthorized") || strings.Contains(err.Error(), "does not belong") {
			writeJSON(w, http.StatusForbidden, ErrorResponse{
				Error:   "Forbidden",
				Details: "You don't have permission to delete this avatar",
			})
			return
		}

		// Проверяем, не удалена ли уже аватарка
		if strings.Contains(err.Error(), "already deleted") {
			writeJSON(w, http.StatusGone, ErrorResponse{
				Error:   "Avatar already deleted",
				Details: "This avatar has already been deleted",
			})
			return
		}

		// Проверяем, не находится ли уже в очереди на удаление
		if errors.Is(err, constErr.ErrAvatarDeletionAlreadyQueued) {
			writeJSON(w, http.StatusAccepted, map[string]string{
				"status":  "already_queued",
				"message": "Avatar is already in deletion queue",
			})
			return
		}

		// Обрабатываем стандартные ошибки (context canceled, deadline, not found)
		if handleServiceError(err, w) {
			return
		}

		// Все остальные ошибки
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to delete avatar",
			Details: err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusAccepted, map[string]string{
		"status":  "queued",
		"message": "Avatar has been added to deletion queue",
	})
}

// int64ToString вспомогательная функция перевода числа64 в строку
func int64ToString(v int64) string {
	return strconv.FormatInt(v, 10)
}

// writeJSON кусок кода вынесен в общую реализацию, функция возвращает ответ json
func writeJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// detectAndValidateAvatarMime:
// 1. читает первые байты файла
// 2. определяет MIME по magic bytes
// 3. проверяет whitelist допустимых типов
// 4. возвращает указатель чтения в начало файла
func detectAndValidateAvatarMime(file multipart.File) (string, error) {
	seeker, ok := file.(io.Seeker)
	if !ok {
		return "", errors.New("uploaded file is not seekable")
	}

	buf := make([]byte, 512)
	n, err := file.Read(buf)
	if err != nil && !errors.Is(err, io.EOF) {
		return "", errors.New("failed to read file header")
	}
	if n == 0 {
		return "", errors.New("file is empty")
	}

	detectedContentType := http.DetectContentType(buf[:n])

	if _, allowed := allowedAvatarMIMETypes[detectedContentType]; !allowed {
		return "", errors.New("allowed types: image/jpeg, image/png, image/webp")
	}

	if _, err := seeker.Seek(0, io.SeekStart); err != nil {
		return "", errors.New("failed to reset file pointer")
	}

	return detectedContentType, nil
}
