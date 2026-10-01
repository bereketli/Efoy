// Package errs maps domain errors to RFC 9457 problem+json responses.
//
// Services return *Error values for expected failures (validation, auth,
// conflicts). Anything else is treated as an internal error: it is logged and
// the client gets a generic 500 without details.
package errs

import (
	"encoding/json"
	"errors"
	"log/slog"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const typeBase = "https://docs.efoy.et/errors/"

// Error is an expected failure with a stable, machine-readable code.
type Error struct {
	Status     int
	Code       string
	Title      string
	Detail     string
	RetryAfter time.Duration
	Err        error
}

func (e *Error) Error() string {
	msg := e.Code
	if e.Detail != "" {
		msg += ": " + e.Detail
	}
	if e.Err != nil {
		msg += ": " + e.Err.Error()
	}
	return msg
}

func (e *Error) Unwrap() error { return e.Err }

// New builds an Error. Prefer the helpers below.
func New(status int, code, title, detail string) *Error {
	return &Error{Status: status, Code: code, Title: title, Detail: detail}
}

func Invalid(code, detail string) *Error {
	return New(http.StatusBadRequest, code, "Invalid request", detail)
}

func Unauthorized(code, detail string) *Error {
	return New(http.StatusUnauthorized, code, "Unauthorized", detail)
}

func Forbidden(code, detail string) *Error {
	return New(http.StatusForbidden, code, "Forbidden", detail)
}

func NotFound(code, detail string) *Error {
	return New(http.StatusNotFound, code, "Not found", detail)
}

func Conflict(code, detail string) *Error {
	return New(http.StatusConflict, code, "Conflict", detail)
}

func TooManyRequests(code, detail string, retryAfter time.Duration) *Error {
	e := New(http.StatusTooManyRequests, code, "Too many requests", detail)
	e.RetryAfter = retryAfter
	return e
}

// Problem is the RFC 9457 response body (Problem in efoy.yaml).
type Problem struct {
	Type   string `json:"type"`
	Title  string `json:"title"`
	Status int    `json:"status"`
	Code   string `json:"code"`
	Detail string `json:"detail,omitempty"`
}

// Write sends err as problem+json. Unknown errors are logged with
// slog.Default and answered with a generic 500.
func Write(w http.ResponseWriter, r *http.Request, err error) {
	var e *Error
	if !errors.As(err, &e) {
		slog.Default().ErrorContext(r.Context(), "internal error",
			slog.Any("error", err), slog.String("method", r.Method), slog.String("path", r.URL.Path))
		e = New(http.StatusInternalServerError, "INTERNAL", "Internal error", "")
	}
	if e.RetryAfter > 0 {
		w.Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(e.RetryAfter.Seconds()))))
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(e.Status)
	_ = json.NewEncoder(w).Encode(Problem{
		Type:   typeBase + strings.ReplaceAll(strings.ToLower(e.Code), "_", "-"),
		Title:  e.Title,
		Status: e.Status,
		Code:   e.Code,
		Detail: e.Detail,
	})
}
