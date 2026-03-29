// Логирование HTTP-запросов
package middleware

import (
	"net/http"
	"time"

	logger "goph-profile-avatars/internal/logging"
)

// ResponseWriter — обёртка над http.ResponseWriter,
// позволяющая перехватывать HTTP-статус и размер ответа.
//
// Используется в middleware для логирования и метрик.
type ResponseWriter struct {
	http.ResponseWriter
	Status int // HTTP статус ответа
	Size   int // Количество записанных байт
}

// WriteHeader перехватывает установку HTTP-статуса.
// Если статус не был установлен явно, он будет определён при Write().
func (w *ResponseWriter) WriteHeader(Status int) {
	w.Status = Status
	w.ResponseWriter.WriteHeader(Status)
}

// Write записывает тело ответа и учитывает размер.
// Если статус не был установлен ранее, используется 200 OK.
func (w *ResponseWriter) Write(b []byte) (int, error) {
	if w.Status == 0 {
		w.Status = http.StatusOK
	}
	Size, err := w.ResponseWriter.Write(b)
	w.Size += Size
	return Size, err
}

// LoggerMiddleware возвращает HTTP middleware,
// логирующий входящие HTTP-запросы.
//
// Логируются:
//   - HTTP метод
//   - URI
//   - статус ответа
//   - размер ответа
//   - время обработки запроса (в миллисекундах)
func LoggerMiddleware() func(http.Handler) http.Handler {
	loggerHTTP := logger.NewHTTPLogger()

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			wr := &ResponseWriter{ResponseWriter: w}
			next.ServeHTTP(wr, r)

			duration := time.Since(start).Seconds() * 1000
			loggerHTTP.LogRequest(r.Method, r.RequestURI, wr.Status, wr.Size, duration)
		})
	}
}
