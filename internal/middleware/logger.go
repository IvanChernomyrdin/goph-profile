package middleware

import (
	"net/http"
	"strconv"
	"time"

	"goph-profile-avatars/internal/logging"
	"goph-profile-avatars/internal/metrics"

	"github.com/go-chi/chi/v5"
)

type ResponseWriter struct {
	http.ResponseWriter
	Status int
	Size   int
}

func (w *ResponseWriter) WriteHeader(status int) {
	w.Status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *ResponseWriter) Write(b []byte) (int, error) {
	if w.Status == 0 {
		w.Status = http.StatusOK
	}
	size, err := w.ResponseWriter.Write(b)
	w.Size += size
	return size, err
}

func LoggerMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			wr := &ResponseWriter{ResponseWriter: w}

			next.ServeHTTP(wr, r)

			if wr.Status == 0 {
				wr.Status = http.StatusOK
			}

			routePattern := "unknown"
			if routeCtx := chi.RouteContext(r.Context()); routeCtx != nil {
				if rp := routeCtx.RoutePattern(); rp != "" {
					routePattern = rp
				}
			}
			if routePattern == "unknown" {
				routePattern = r.URL.Path
			}

			statusCode := strconv.Itoa(wr.Status)
			duration := time.Since(start).Seconds()

			metrics.HTTPRequestsTotal.WithLabelValues(
				r.Method,
				routePattern,
				statusCode,
			).Inc()

			metrics.HTTPRequestDuration.WithLabelValues(
				r.Method,
				routePattern,
				statusCode,
			).Observe(duration)

			logging.HTTPLoggerFromContext(r.Context()).Info(
				"http request completed",
				"method", r.Method,
				"route", routePattern,
				"uri", r.RequestURI,
				"status", wr.Status,
				"response_size", wr.Size,
				"duration_ms", duration*1000,
				"remote_addr", r.RemoteAddr,
			)
		})
	}
}
