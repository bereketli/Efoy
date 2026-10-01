package errs

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func write(t *testing.T, err error) (*httptest.ResponseRecorder, Problem) {
	t.Helper()
	rec := httptest.NewRecorder()
	Write(rec, httptest.NewRequest(http.MethodGet, "/x", nil), err)

	var p Problem
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &p))
	return rec, p
}

func TestWriteDomainError(t *testing.T) {
	rec, p := write(t, fmt.Errorf("wrapped: %w", Invalid("INVALID_PHONE", "Use E.164.")))

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Equal(t, "application/problem+json", rec.Header().Get("Content-Type"))
	assert.Equal(t, Problem{
		Type:   "https://docs.efoy.et/errors/invalid-phone",
		Title:  "Invalid request",
		Status: 400,
		Code:   "INVALID_PHONE",
		Detail: "Use E.164.",
	}, p)
}

func TestWriteRetryAfterRoundsUp(t *testing.T) {
	rec, _ := write(t, TooManyRequests("RATE_LIMITED", "", 1500*time.Millisecond))

	assert.Equal(t, http.StatusTooManyRequests, rec.Code)
	assert.Equal(t, "2", rec.Header().Get("Retry-After"))
}

func TestWriteUnknownErrorHidesDetails(t *testing.T) {
	rec, p := write(t, errors.New("pq: password authentication failed for user efoy"))

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Equal(t, "INTERNAL", p.Code)
	assert.Empty(t, p.Detail)
}
