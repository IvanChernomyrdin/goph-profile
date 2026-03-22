package test

import (
	"bytes"
	"mime/multipart"
	"testing"

	"github.com/stretchr/testify/require"
)

const maxAvatarSize = 10 << 20

type errorResponse struct {
	Error   string `json:"error"`
	Details string `json:"details,omitempty"`
	MaxSize int64  `json:"max_size,omitempty"`
}

type mockReadCloser struct {
	*bytes.Reader
}

func (m mockReadCloser) Close() error {
	return nil
}

func createMultipartForm(t *testing.T, fileName string, content []byte) (*bytes.Buffer, string) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("file", fileName)
	require.NoError(t, err)

	_, err = part.Write(content)
	require.NoError(t, err)

	err = writer.Close()
	require.NoError(t, err)

	return body, writer.FormDataContentType()
}

func validJPEG() []byte {
	return []byte{
		0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46,
		0x49, 0x46, 0x00, 0x01, 0x01, 0x00, 0x00, 0x01,
		0x00, 0x01, 0x00, 0x00, 0xFF, 0xDB,
	}
}

func validPNG() []byte {
	return []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
}

// createMultipartFormEmptyFilename создает multipart form с файлом без имени
func createMultipartFormEmptyFilename(t *testing.T, content []byte) (*bytes.Buffer, string) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Создаем part без имени файла
	part, err := writer.CreateFormFile("file", "")
	require.NoError(t, err)

	_, err = part.Write(content)
	require.NoError(t, err)

	err = writer.Close()
	require.NoError(t, err)

	return body, writer.FormDataContentType()
}
