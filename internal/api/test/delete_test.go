package test

import (
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
	constErr "goph-profile-avatars/internal/errors"
)

func TestDeleteAvatarByID(t *testing.T) {
	validUUID := uuid.New().String()

	tests := []struct {
		name           string
		avatarID       string
		userID         string
		setupMock      func(*mocks.AvatarService)
		expectedStatus int
	}{
		{
			name:     "успешное добавление в очередь удаления",
			avatarID: validUUID,
			userID:   "user123",
			setupMock: func(m *mocks.AvatarService) {
				m.On("DeleteAvatarByID", validUUID, "user123").Return(nil)
			},
			expectedStatus: http.StatusAccepted,
		},
		{
			name:     "аватар уже в очереди на удаление",
			avatarID: validUUID,
			userID:   "user123",
			setupMock: func(m *mocks.AvatarService) {
				m.On("DeleteAvatarByID", validUUID, "user123").Return(constErr.ErrAvatarDeletionAlreadyQueued)
			},
			expectedStatus: http.StatusAccepted,
		},
		{
			name:           "отсутствует avatar_id",
			avatarID:       "",
			userID:         "user123",
			setupMock:      func(m *mocks.AvatarService) {},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "отсутствует X-User-ID",
			avatarID:       validUUID,
			userID:         "",
			setupMock:      func(m *mocks.AvatarService) {},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "невалидный avatar_id",
			avatarID:       "invalid",
			userID:         "user123",
			setupMock:      func(m *mocks.AvatarService) {},
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockSvc := new(mocks.AvatarService)
			tt.setupMock(mockSvc)

			handler := api.NewHandler(nil, mockSvc)

			req := httptest.NewRequest(http.MethodDelete, "/api/avatar/"+tt.avatarID, nil)
			if tt.userID != "" {
				req.Header.Set("X-User-ID", tt.userID)
			}

			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("avatar_id", tt.avatarID)
			req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

			rr := httptest.NewRecorder()
			handler.DeleteAvatarByID(rr, req)

			assert.Equal(t, tt.expectedStatus, rr.Code)

			if tt.expectedStatus == http.StatusAccepted {
				var resp map[string]string
				err := json.NewDecoder(rr.Body).Decode(&resp)
				require.NoError(t, err)
				assert.Contains(t, resp, "status")
			}

			mockSvc.AssertExpectations(t)
		})
	}
}
