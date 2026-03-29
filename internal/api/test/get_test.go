package test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"goph-profile-avatars/internal/api"
	"goph-profile-avatars/internal/api/mocks"
	"goph-profile-avatars/internal/repository"
)

func TestGetAvatar(t *testing.T) {
	validUUID := uuid.New().String()

	tests := []struct {
		name           string
		avatarID       string
		size           string
		setupMock      func(*mocks.AvatarService)
		expectedStatus int
		expectedError  string
	}{
		{
			name:     "успешное получение аватара",
			avatarID: validUUID,
			size:     "original",
			setupMock: func(m *mocks.AvatarService) {
				m.On("GetAvatarByID", validUUID, "original").Return(&api.GetAvatarResult{
					ID:        validUUID,
					UserID:    "user123",
					FileName:  "avatar.jpg",
					MimeType:  "image/jpeg",
					SizeBytes: 100,
					Reader:    mockReadCloser{Reader: bytes.NewReader([]byte("data"))},
				}, nil)
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:           "отсутствует avatar_id",
			avatarID:       "",
			setupMock:      func(m *mocks.AvatarService) {},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "avatar_id is required",
		},
		{
			name:           "невалидный UUID",
			avatarID:       "invalid",
			setupMock:      func(m *mocks.AvatarService) {},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "invalid avatar_id",
		},
		{
			name:     "аватар не найден",
			avatarID: validUUID,
			size:     "original",
			setupMock: func(m *mocks.AvatarService) {
				m.On("GetAvatarByID", validUUID, "original").Return(nil, repository.ErrAvatarNotFound)
			},
			expectedStatus: http.StatusNotFound,
			expectedError:  "avatar not found",
		},
		{
			name:           "невалидный параметр size",
			avatarID:       validUUID,
			size:           "invalid",
			setupMock:      func(m *mocks.AvatarService) {}, // не ожидаем вызова, т.к. валидация до сервиса
			expectedStatus: http.StatusBadRequest,
			expectedError:  "invalid size",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockSvc := new(mocks.AvatarService)
			tt.setupMock(mockSvc)

			handler := api.NewHandler(nil, mockSvc)

			req := httptest.NewRequest(http.MethodGet, "/api/avatar/"+tt.avatarID, nil)
			if tt.size != "" {
				req.URL.RawQuery = "size=" + tt.size
			}

			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("avatar_id", tt.avatarID)
			req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

			rr := httptest.NewRecorder()
			handler.GetAvatar(rr, req)

			assert.Equal(t, tt.expectedStatus, rr.Code)

			if tt.expectedError != "" && tt.expectedStatus != http.StatusOK {
				var resp errorResponse
				err := json.NewDecoder(rr.Body).Decode(&resp)
				require.NoError(t, err)
				assert.Contains(t, resp.Error, tt.expectedError)
			}

			mockSvc.AssertExpectations(t)
		})
	}
}

func TestGetUserAvatar(t *testing.T) {
	tests := []struct {
		name           string
		userID         string
		setupMock      func(*mocks.AvatarService)
		expectedStatus int
		expectedError  string
	}{
		{
			name:   "успешное получение аватара пользователя",
			userID: "user123",
			setupMock: func(m *mocks.AvatarService) {
				m.On("GetUserAvatar", "user123").Return(&api.GetAvatarResult{
					ID:        uuid.New().String(),
					UserID:    "user123",
					FileName:  "avatar.jpg",
					MimeType:  "image/jpeg",
					SizeBytes: 100,
					Reader:    mockReadCloser{Reader: bytes.NewReader([]byte("data"))},
				}, nil)
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:           "отсутствует user_id",
			userID:         "",
			setupMock:      func(m *mocks.AvatarService) {},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "user_id is required",
		},
		{
			name:   "аватар не найден",
			userID: "user123",
			setupMock: func(m *mocks.AvatarService) {
				m.On("GetUserAvatar", "user123").Return(nil, repository.ErrAvatarNotFound)
			},
			expectedStatus: http.StatusNotFound,
			expectedError:  "avatar not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockSvc := new(mocks.AvatarService)
			tt.setupMock(mockSvc)

			handler := api.NewHandler(nil, mockSvc)

			req := httptest.NewRequest(http.MethodGet, "/api/user/"+tt.userID+"/avatar", nil)

			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("user_id", tt.userID)
			req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

			rr := httptest.NewRecorder()
			handler.GetUserAvatar(rr, req)

			assert.Equal(t, tt.expectedStatus, rr.Code)

			if tt.expectedError != "" && tt.expectedStatus != http.StatusOK {
				var resp errorResponse
				err := json.NewDecoder(rr.Body).Decode(&resp)
				require.NoError(t, err)
				assert.Contains(t, resp.Error, tt.expectedError)
			}

			mockSvc.AssertExpectations(t)
		})
	}
}

func TestGetUserAvatars(t *testing.T) {
	tests := []struct {
		name           string
		userID         string
		setupMock      func(*mocks.AvatarService)
		expectedStatus int
	}{
		{
			name:   "успешное получение списка аватаров",
			userID: "user123",
			setupMock: func(m *mocks.AvatarService) {
				m.On("GetListUserAvatar", "user123").Return([]api.AvatarItem{}, nil)
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:           "отсутствует user_id",
			userID:         "",
			setupMock:      func(m *mocks.AvatarService) {},
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockSvc := new(mocks.AvatarService)
			tt.setupMock(mockSvc)

			handler := api.NewHandler(nil, mockSvc)

			req := httptest.NewRequest(http.MethodGet, "/api/user/"+tt.userID+"/avatars", nil)

			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("user_id", tt.userID)
			req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

			rr := httptest.NewRecorder()
			handler.GetUserAvatars(rr, req)

			assert.Equal(t, tt.expectedStatus, rr.Code)
			mockSvc.AssertExpectations(t)
		})
	}
}
