package httpx

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/blinge12/efoy/pkg/errs"
)

const maxBodyBytes = 64 << 10

var errInvalidJSON = errs.Invalid("INVALID_JSON", "The request body must be valid JSON.")

// DecodeJSON reads a JSON request body of at most 64 KiB into v.
func DecodeJSON(w http.ResponseWriter, r *http.Request, v any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		return errInvalidJSON
	}
	return nil
}

// DateLayout is the API's calendar date format (OpenAPI format: date).
const DateLayout = "2006-01-02"

// ParseDate parses an optional date field; field names the field in errors.
func ParseDate(field string, s *string) (*time.Time, error) {
	if s == nil || *s == "" {
		return nil, nil
	}
	t, err := time.Parse(DateLayout, *s)
	if err != nil {
		return nil, errs.Invalid("INVALID_DATE", field+" must be a date like 2027-06-30.")
	}
	return &t, nil
}

// FormatDate formats an optional date for a response.
func FormatDate(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.Format(DateLayout)
	return &s
}
