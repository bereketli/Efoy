package httpx

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// NewRouter returns a chi router with the standard middleware stack and the
// /healthz (liveness) and /readyz (readiness) probes mounted.
func NewRouter(log *slog.Logger, health *Health) chi.Router {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(RequestLogger(log))
	r.Use(middleware.Recoverer)

	r.Get("/healthz", health.Liveness)
	r.Get("/readyz", health.Readiness)
	return r
}

// RequestLogger logs one line per request. Probe requests are logged at
// debug level so they do not flood the logs.
func RequestLogger(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			defer func() {
				status := ww.Status()
				if status == 0 {
					status = http.StatusOK
				}
				level := slog.LevelInfo
				switch {
				case status >= 500:
					level = slog.LevelError
				case r.URL.Path == "/healthz" || r.URL.Path == "/readyz":
					level = slog.LevelDebug
				}
				log.LogAttrs(r.Context(), level, "http request",
					slog.String("method", r.Method),
					slog.String("path", r.URL.Path),
					slog.Int("status", status),
					slog.Int("bytes", ww.BytesWritten()),
					slog.Duration("duration", time.Since(start)),
					slog.String("request_id", middleware.GetReqID(r.Context())),
				)
			}()
			next.ServeHTTP(ww, r)
		})
	}
}

// WriteJSON writes v as a JSON response with the given status code.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
