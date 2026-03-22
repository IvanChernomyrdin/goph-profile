package test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"goph-profile-avatars/internal/middleware"

	"github.com/stretchr/testify/require"
)

// Статус по умолчанию и размер
func TestResponseWriter_Write_DefaultStatus(t *testing.T) {
	rr := httptest.NewRecorder()
	w := &middleware.ResponseWriter{ResponseWriter: rr}

	body := []byte("hello")
	n, err := w.Write(body)

	require.NoError(t, err)
	require.Equal(t, len(body), n)
	require.Equal(t, http.StatusOK, w.Status)
	require.Equal(t, len(body), w.Size)
}

// вспомогательная функция
func testHandler(status int, body string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	})
}

// проверка корректного прохода статуса и тела через мидлу
func TestLoggerMiddleware(t *testing.T) {
	mw := middleware.LoggerMiddleware()

	handler := mw(testHandler(http.StatusTeapot, "tea"))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	require.Equal(t, http.StatusTeapot, rr.Code)
	require.Equal(t, "tea", rr.Body.String())
}
