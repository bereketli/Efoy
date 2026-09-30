package httpx

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestRouter(checks map[string]Check) http.Handler {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := NewHealth("core-api", "v0.0.1", log)
	for name, c := range checks {
		h.AddCheck(name, c)
	}
	return NewRouter(log, h)
}

func get(t *testing.T, h http.Handler, path string) (int, HealthStatus) {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

	var body HealthStatus
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	return rec.Code, body
}

func TestLiveness(t *testing.T) {
	code, body := get(t, newTestRouter(nil), "/healthz")

	assert.Equal(t, http.StatusOK, code)
	assert.Equal(t, HealthStatus{Status: "ok", Service: "core-api", Version: "v0.0.1"}, body)
}

func TestReadinessAllChecksPass(t *testing.T) {
	ok := func(context.Context) error { return nil }
	code, body := get(t, newTestRouter(map[string]Check{"postgres": ok, "redis": ok}), "/readyz")

	assert.Equal(t, http.StatusOK, code)
	assert.Equal(t, "ok", body.Status)
	assert.Equal(t, map[string]string{"postgres": "ok", "redis": "ok"}, body.Checks)
}

func TestReadinessFailingCheck(t *testing.T) {
	checks := map[string]Check{
		"postgres": func(context.Context) error { return errors.New("dial tcp 10.0.0.5:5432: connection refused") },
		"redis":    func(context.Context) error { return nil },
	}
	code, body := get(t, newTestRouter(checks), "/readyz")

	assert.Equal(t, http.StatusServiceUnavailable, code)
	assert.Equal(t, "unavailable", body.Status)
	assert.Equal(t, map[string]string{"postgres": "fail", "redis": "ok"}, body.Checks)
}
