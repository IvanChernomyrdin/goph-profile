package test

import (
	"bytes"
	"mime/multipart"
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

	validUserID := "123"
	validAvatarID := "550e8400-e29b-41d4-a716-446655440001"

	tests := []struct {
		name       string
		method     string
		path       string
		setupMock  func()
		buildBody  func() (*bytes.Buffer, string)
		headers    map[string]string
		statusCode int
	}{
		{
			name:   "POST /api/v1/avatars",
			method: http.MethodPost,
			path:   "/api/v1/avatars",
			setupMock: func() {
				mockService.On("UploadAvatar", mock.Anything, mock.Anything).
					Return(&api.UploadAvatarResult{ID: "avatar-id"}, nil).Once()
			},
			buildBody: func() (*bytes.Buffer, string) {
				body := &bytes.Buffer{}
				writer := multipart.NewWriter(body)

				part, err := writer.CreateFormFile("file", "avatar.png")
				if err != nil {
					panic(err)
				}

				if _, err = part.Write([]byte{
					0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A,
					0x00, 0x00, 0x00, 0x0D, 0x49, 0x48, 0x44, 0x52,
				}); err != nil {
					panic(err)
				}

				if err = writer.Close(); err != nil {
					panic(err)
				}

				return body, writer.FormDataContentType()
			},
			headers: map[string]string{
				"X-User-ID": validUserID,
			},
			statusCode: http.StatusCreated,
		},
		{
			name:   "GET /api/v1/users/{user_id}/avatars",
			method: http.MethodGet,
			path:   "/api/v1/users/123/avatars",
			setupMock: func() {
				mockService.On("GetListUserAvatar", mock.Anything, "123").
					Return([]api.AvatarItem{}, nil).Once()
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
				mockService.On("DeleteCurrentUserAvatar", mock.Anything, "123").
					Return(nil).Once()
			},
			headers: map[string]string{
				"X-User-ID": "123",
			},
			statusCode: http.StatusOK,
		},
		{
			name:   "PATCH /api/v1/avatars/{avatar_id}/current success",
			method: http.MethodPatch,
			path:   "/api/v1/avatars/" + validAvatarID + "/current",
			setupMock: func() {
				mockService.On("UpdateCurrentAvatar", mock.Anything, "123", validAvatarID).
					Return(nil).Once()
			},
			headers: map[string]string{
				"X-User-ID": "123",
			},
			statusCode: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupMock()

			var req *http.Request
			if tt.buildBody != nil {
				body, contentType := tt.buildBody()
				req = httptest.NewRequest(tt.method, tt.path, body)
				req.Header.Set("Content-Type", contentType)
			} else {
				req = httptest.NewRequest(tt.method, tt.path, nil)
			}

			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}

			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)

			assert.Equal(t, tt.statusCode, rr.Code)
		})
	}

	mockService.AssertExpectations(t)
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
