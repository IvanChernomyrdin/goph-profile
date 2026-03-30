package middleware

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

func TracingMiddleware(serviceName string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return otelhttp.NewHandler(
			next,
			serviceName,
			otelhttp.WithSpanNameFormatter(func(_ string, r *http.Request) string {
				route := r.URL.Path
				if routeCtx := chi.RouteContext(r.Context()); routeCtx != nil {
					if rp := routeCtx.RoutePattern(); rp != "" {
						route = rp
					}
				}
				return r.Method + " " + route
			}),
		)
	}
}
