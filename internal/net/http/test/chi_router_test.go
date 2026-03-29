package test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"goph-profile-avatars/internal/api"
	"goph-profile-avatars/internal/api/mocks"
	myHttp "goph-profile-avatars/internal/net/http"
)

func TestNewRouter(t *testing.T) {
	mockService := new(mocks.AvatarService)
	handler := api.NewHandler(nil, mockService)

	router := myHttp.NewRouter(handler, 100)
	assert.NotNil(t, router)
}

func TestRouter_APIRoutes(t *testing.T) {
	mockService := new(mocks.AvatarService)
	handler := api.NewHandler(nil, mockService)
	router := myHttp.NewRouter(handler, 100)

	tests := []struct {
		name       string
		method     string
		path       string
		setupMock  func()
		statusCode int
	}{
		{
			name:   "POST /api/v1/avatars",
			method: http.MethodPost,
			path:   "/api/v1/avatars",
			setupMock: func() {
				mockService.On("UploadAvatar", mock.Anything, mock.Anything).
					Return(&api.AvatarItem{ID: "avatar-id"}, nil)
			},
			statusCode: http.StatusOK,
		},
		{
			name:   "GET /api/v1/users/{user_id}/avatars",
			method: http.MethodGet,
			path:   "/api/v1/users/123/avatars",
			setupMock: func() {
				mockService.On("GetListUserAvatar", mock.Anything, "123").Return([]api.AvatarItem{}, nil)
			},
			statusCode: http.StatusOK,
		},
		{
			name:       "GET /api/v1/avatars/{avatar_id}",
			method:     http.MethodGet,
			path:       "/api/v1/avatars/invalid-id",
			setupMock:  func() {},
			statusCode: http.StatusBadRequest,
		},
		{
			name:       "PATCH /api/v1/avatars/{avatar_id}/current",
			method:     http.MethodPatch,
			path:       "/api/v1/avatars/invalid-id/current",
			setupMock:  func() {},
			statusCode: http.StatusBadRequest,
		},
		{
			name:       "DELETE /api/v1/avatars/{avatar_id}",
			method:     http.MethodDelete,
			path:       "/api/v1/avatars/invalid-id",
			setupMock:  func() {},
			statusCode: http.StatusBadRequest,
		},
		{
			name:   "DELETE /api/v1/users/{user_id}/avatar",
			method: http.MethodDelete,
			path:   "/api/v1/users/123/avatar",
			setupMock: func() {
				mockService.On("DeleteCurrentUserAvatar", mock.Anything, "123").Return(nil)
			},
			statusCode: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupMock()

			req := httptest.NewRequest(tt.method, tt.path, nil)
			rr := httptest.NewRecorder()

			router.ServeHTTP(rr, req)

			assert.Equal(t, tt.statusCode, rr.Code)
		})
	}
}

func TestRouter_NotFound(t *testing.T) {
	mockService := new(mocks.AvatarService)
	handler := api.NewHandler(nil, mockService)
	router := myHttp.NewRouter(handler, 100)

	req := httptest.NewRequest(http.MethodGet, "/non-existent-path", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestRouter_MethodNotAllowed(t *testing.T) {
	mockService := new(mocks.AvatarService)
	handler := api.NewHandler(nil, mockService)
	router := myHttp.NewRouter(handler, 100)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/avatars/invalid-id", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusMethodNotAllowed, rr.Code)
}
