package test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"goph-profile-avatars/internal/api"
	"goph-profile-avatars/internal/api/mocks"
	constErr "goph-profile-avatars/internal/errors"
)

func TestUploadAvatar(t *testing.T) {
	validUUID := uuid.New().String()

	tests := []struct {
		name           string
		userID         string
		fileName       string
		fileContent    []byte
		setupMock      func(*mocks.AvatarService)
		expectedStatus int
		expectedError  string
	}{
		{
			name:        "успешная загрузка JPEG",
			userID:      "user123",
			fileName:    "avatar.jpg",
			fileContent: validJPEG(),
			setupMock: func(m *mocks.AvatarService) {
				m.On("UploadAvatar", mock.MatchedBy(func(input api.UploadAvatarInput) bool {
					return input.UserID == "user123" && input.FileName == "avatar.jpg"
				})).Return(&api.UploadAvatarResult{
					ID:        validUUID,
					UserID:    "user123",
					URL:       "http://example.com/avatar.jpg",
					Status:    "uploaded",
					CreatedAt: "2024-01-01T00:00:00Z",
				}, nil)
			},
			expectedStatus: http.StatusCreated,
		},
		{
			name:        "успешная загрузка PNG",
			userID:      "user123",
			fileName:    "avatar.png",
			fileContent: validPNG(),
			setupMock: func(m *mocks.AvatarService) {
				m.On("UploadAvatar", mock.MatchedBy(func(input api.UploadAvatarInput) bool {
					return input.UserID == "user123" && input.FileName == "avatar.png"
				})).Return(&api.UploadAvatarResult{
					ID:        validUUID,
					UserID:    "user123",
					URL:       "http://example.com/avatar.png",
					Status:    "uploaded",
					CreatedAt: "2024-01-01T00:00:00Z",
				}, nil)
			},
			expectedStatus: http.StatusCreated,
		},
		{
			name:           "отсутствует X-User-ID",
			userID:         "",
			fileName:       "avatar.jpg",
			fileContent:    validJPEG(),
			setupMock:      func(m *mocks.AvatarService) {},
			expectedStatus: http.StatusBadRequest,
			expectedError:  constErr.ErrXUserID.Error(),
		},
		{
			name:           "файл слишком большой",
			userID:         "user123",
			fileName:       "large.jpg",
			fileContent:    make([]byte, maxAvatarSize+1),
			setupMock:      func(m *mocks.AvatarService) {},
			expectedStatus: http.StatusRequestEntityTooLarge,
			expectedError:  "File too large",
		},
		{
			name:           "неподдерживаемый тип файла",
			userID:         "user123",
			fileName:       "avatar.txt",
			fileContent:    []byte("text content"),
			setupMock:      func(m *mocks.AvatarService) {},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Unsupported file type",
		},
		{
			name:           "пустой файл",
			userID:         "user123",
			fileName:       "empty.jpg",
			fileContent:    []byte{},
			setupMock:      func(m *mocks.AvatarService) {},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "invalid file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockSvc := new(mocks.AvatarService)
			tt.setupMock(mockSvc)

			handler := api.NewHandler(nil, mockSvc)

			var body *bytes.Buffer
			var contentType string

			// Для пустого имени файла используем специальную функцию
			if tt.name == "пустое имя файла" {
				body, contentType = createMultipartFormEmptyFilename(t, tt.fileContent)
			} else {
				body, contentType = createMultipartForm(t, tt.fileName, tt.fileContent)
			}

			req := httptest.NewRequest(http.MethodPost, "/api/avatar", body)
			req.Header.Set("Content-Type", contentType)
			if tt.userID != "" {
				req.Header.Set("X-User-ID", tt.userID)
			}

			rr := httptest.NewRecorder()
			handler.UploadAvatar(rr, req)

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
