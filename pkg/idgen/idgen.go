// Package idgen generates time-ordered UUIDv7 identifiers (design doc 9.5).
package idgen

import "github.com/google/uuid"

// New returns a new UUIDv7. It panics only if the system's random source fails.
func New() uuid.UUID {
	return uuid.Must(uuid.NewV7())
}
