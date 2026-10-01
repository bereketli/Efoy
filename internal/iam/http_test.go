package iam

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/blinge12/efoy/pkg/authz"
)

func newRouter(e *env) http.Handler {
	r := chi.NewRouter()
	r.Route("/v1", func(r chi.Router) {
		r.Use(authz.Authenticate(e.keys.Tokens))
		NewHandler(e.svc).Routes(r)
	})
	return r
}

func call(t *testing.T, h http.Handler, method, path, token string, body any) (int, map[string]any) {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		require.NoError(t, json.NewEncoder(&buf).Encode(body))
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	out := map[string]any{}
	if rec.Body.Len() > 0 {
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out), rec.Body.String())
	}
	return rec.Code, out
}

// TestHTTPLoginFlow walks the day-2 "done when" path through the real routes:
// OTP request -> verify -> /me -> refresh -> logout.
func TestHTTPLoginFlow(t *testing.T) {
	e := newEnv(t)
	h := newRouter(e)

	code, body := call(t, h, http.MethodPost, "/v1/auth/otp/request", "", map[string]any{"phone_e164": phone})
	require.Equal(t, http.StatusAccepted, code, body)
	assert.EqualValues(t, 300, body["expires_in_s"])
	assert.EqualValues(t, 60, body["resend_after_s"])
	assert.Contains(t, e.sms.last[phone], "Your Efoy code", "Accept-Language picks the English template")

	code, body = call(t, h, http.MethodPost, "/v1/auth/otp/verify", "", map[string]any{
		"phone_e164": phone,
		"code":       e.sms.code(t, phone),
		"device":     map[string]any{"platform": "ANDROID", "app_version": "1.0.0"},
	})
	require.Equal(t, http.StatusOK, code, body)
	assert.Equal(t, "Bearer", body["token_type"])
	assert.EqualValues(t, 900, body["expires_in"])
	access, _ := body["access_token"].(string)
	refresh, _ := body["refresh_token"].(string)
	require.NotEmpty(t, access)
	require.NotEmpty(t, refresh)
	user, _ := body["user"].(map[string]any)
	assert.Equal(t, phone, user["phone_e164"])
	assert.Equal(t, []any{}, user["roles"])
	assert.Equal(t, []any{}, user["linked_rider_ids"])

	code, body = call(t, h, http.MethodGet, "/v1/me", access, nil)
	require.Equal(t, http.StatusOK, code, body)
	assert.Equal(t, user["id"], body["id"])

	code, body = call(t, h, http.MethodGet, "/v1/me", "", nil)
	assert.Equal(t, http.StatusUnauthorized, code)
	assert.Equal(t, "UNAUTHENTICATED", body["code"])

	code, body = call(t, h, http.MethodGet, "/v1/me", "not-a-jwt", nil)
	assert.Equal(t, http.StatusUnauthorized, code)
	assert.Equal(t, "INVALID_TOKEN", body["code"])

	code, body = call(t, h, http.MethodPost, "/v1/auth/refresh", "", map[string]any{"refresh_token": refresh})
	require.Equal(t, http.StatusOK, code, body)
	newAccess, _ := body["access_token"].(string)
	newRefresh, _ := body["refresh_token"].(string)
	assert.NotEqual(t, refresh, newRefresh)

	code, _ = call(t, h, http.MethodPost, "/v1/auth/logout", newAccess, nil)
	assert.Equal(t, http.StatusNoContent, code)

	code, body = call(t, h, http.MethodPost, "/v1/auth/refresh", "", map[string]any{"refresh_token": newRefresh})
	assert.Equal(t, http.StatusUnauthorized, code)
	assert.Equal(t, "REFRESH_TOKEN_INVALID", body["code"])
}

func TestHTTPStaffLoginNeedsTOTP(t *testing.T) {
	e := newEnv(t)
	h := newRouter(e)
	email, secret := e.createStaff(t, "admin@efoy.et", authz.RoleSuperAdmin)
	creds := map[string]any{"email": email, "password": "correct horse battery staple"}

	code, body := call(t, h, http.MethodPost, "/v1/auth/login", "", creds)
	assert.Equal(t, http.StatusUnauthorized, code)
	assert.Equal(t, "TOTP_REQUIRED", body["code"])
	assert.Equal(t, "https://docs.efoy.et/errors/totp-required", body["type"])

	creds["totp_code"] = e.totpCode(t, secret)
	code, body = call(t, h, http.MethodPost, "/v1/auth/login", "", creds)
	require.Equal(t, http.StatusOK, code, body)
	user, _ := body["user"].(map[string]any)
	assert.Equal(t, []any{map[string]any{"role": "SUPER_ADMIN", "scope": "GLOBAL", "scope_id": nil}}, user["roles"])
}

func TestHTTPRejectsInvalidJSON(t *testing.T) {
	e := newEnv(t)
	req := httptest.NewRequest(http.MethodPost, "/v1/auth/otp/request", bytes.NewBufferString("{"))
	rec := httptest.NewRecorder()
	newRouter(e).ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Equal(t, "application/problem+json", rec.Header().Get("Content-Type"))
}
