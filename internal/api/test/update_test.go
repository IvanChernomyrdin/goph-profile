package test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"goph-profile-avatars/internal/api"
	"goph-profile-avatars/internal/api/mocks"
	constErr "goph-profile-avatars/internal/errors"
	"goph-profile-avatars/internal/repository"
)

func TestUpdateCurrentAvatar(t *testing.T) {
	validUUID := uuid.New().String()

	tests := []struct {
		name           string
		avatarID       string
		userID         string
		setupMock      func(*mocks.AvatarService)
		expectedStatus int
		expectedError  string
	}{
		{
			name:     "успешное обновление текущего аватара",
			avatarID: validUUID,
			userID:   "user123",
			setupMock: func(m *mocks.AvatarService) {
				m.On("UpdateCurrentAvatar", mock.Anything, "user123", validUUID).Return(nil)
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:           "отсутствует avatar_id",
			avatarID:       "",
			userID:         "user123",
			setupMock:      func(m *mocks.AvatarService) {},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "avatar_id is required",
		},
		{
			name:           "невалидный avatar_id",
			avatarID:       "invalid",
			userID:         "user123",
			setupMock:      func(m *mocks.AvatarService) {},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "invalid avatar_id",
		},
		{
			name:           "отсутствует X-User-ID",
			avatarID:       validUUID,
			userID:         "",
			setupMock:      func(m *mocks.AvatarService) {},
			expectedStatus: http.StatusBadRequest,
			expectedError:  constErr.ErrXUserID.Error(),
		},
		{
			name:     "аватар не найден",
			avatarID: validUUID,
			userID:   "user123",
			setupMock: func(m *mocks.AvatarService) {
				m.On("UpdateCurrentAvatar", mock.Anything, "user123", validUUID).Return(repository.ErrAvatarNotFound)
			},
			expectedStatus: http.StatusNotFound,
			expectedError:  "avatar not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockSvc := new(mocks.AvatarService)
			tt.setupMock(mockSvc)

			handler := api.NewHandler(nil, mockSvc, &atomic.Bool{})

			req := httptest.NewRequest(http.MethodPut, "/api/avatar/"+tt.avatarID+"/current", nil)
			if tt.userID != "" {
				req.Header.Set("X-User-ID", tt.userID)
			}

			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("avatar_id", tt.avatarID)
			req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

			rr := httptest.NewRecorder()
			handler.UpdateCurrentAvatar(rr, req)

			assert.Equal(t, tt.expectedStatus, rr.Code)

			if tt.expectedError != "" {
				var resp errorResponse
				err := json.NewDecoder(rr.Body).Decode(&resp)
				require.NoError(t, err)
				assert.Contains(t, resp.Error, tt.expectedError)
			}

			mockSvc.AssertExpectations(t)
		})
	}
}

func TestDeleteUserCurrentAvatar(t *testing.T) {
	tests := []struct {
		name           string
		userID         string
		setupMock      func(*mocks.AvatarService)
		expectedStatus int
	}{
		{
			name:   "успешное удаление статуса текущего аватара",
			userID: "user123",
			setupMock: func(m *mocks.AvatarService) {
				m.On("DeleteCurrentUserAvatar",
					mock.MatchedBy(func(ctx context.Context) bool { return ctx != nil }),
					"user123",
				).Return(nil)
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

			handler := api.NewHandler(nil, mockSvc, &atomic.Bool{})

			req := httptest.NewRequest(http.MethodDelete, "/api/user/"+tt.userID+"/avatar/current", nil)

			// добавляем заголовок X-User-ID, чтобы авторизация прошла
			if tt.userID != "" {
				req.Header.Set("X-User-ID", tt.userID)
			}

			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("user_id", tt.userID)
			req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

			rr := httptest.NewRecorder()
			handler.DeleteUserCurrentAvatar(rr, req)

			assert.Equal(t, tt.expectedStatus, rr.Code)
			mockSvc.AssertExpectations(t)
		})
	}
}
