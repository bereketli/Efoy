package httpx

import (
	"context"
	"log/slog"
	"net/http"
	"sort"
	"sync"
	"time"
)

// Check reports whether a dependency (database, Redis, NATS...) is usable.
type Check func(ctx context.Context) error

// HealthStatus is the body of /healthz and /readyz (HealthStatus in efoy.yaml).
type HealthStatus struct {
	Status  string            `json:"status"`
	Service string            `json:"service"`
	Version string            `json:"version"`
	Checks  map[string]string `json:"checks,omitempty"`
}

// Health serves liveness and readiness probes.
type Health struct {
	service string
	version string
	log     *slog.Logger
	timeout time.Duration

	mu     sync.RWMutex
	checks map[string]Check
}

func NewHealth(service, version string, log *slog.Logger) *Health {
	return &Health{
		service: service,
		version: version,
		log:     log,
		timeout: 2 * time.Second,
		checks:  map[string]Check{},
	}
}

// AddCheck registers a readiness check under name.
func (h *Health) AddCheck(name string, c Check) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.checks[name] = c
}

// Liveness answers 200 while the process can serve HTTP.
func (h *Health) Liveness(w http.ResponseWriter, _ *http.Request) {
	WriteJSON(w, http.StatusOK, HealthStatus{Status: "ok", Service: h.service, Version: h.version})
}

// Readiness runs every registered check and answers 503 if any fails.
// Failure details are logged, not returned, so internal hosts never leak.
func (h *Health) Readiness(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), h.timeout)
	defer cancel()

	h.mu.RLock()
	names := make([]string, 0, len(h.checks))
	for name := range h.checks {
		names = append(names, name)
	}
	checks := make(map[string]Check, len(h.checks))
	for name, c := range h.checks {
		checks[name] = c
	}
	h.mu.RUnlock()
	sort.Strings(names)

	body := HealthStatus{Status: "ok", Service: h.service, Version: h.version, Checks: map[string]string{}}
	code := http.StatusOK
	for _, name := range names {
		if err := checks[name](ctx); err != nil {
			h.log.WarnContext(ctx, "readiness check failed", slog.String("check", name), slog.Any("error", err))
			body.Checks[name] = "fail"
			body.Status = "unavailable"
			code = http.StatusServiceUnavailable
			continue
		}
		body.Checks[name] = "ok"
	}
	WriteJSON(w, code, body)
}
