// Package logging builds the service-wide slog logger.
package logging

import (
	"fmt"
	"io"
	"log/slog"

	"github.com/blinge12/efoy/internal/buildinfo"
	"github.com/blinge12/efoy/internal/config"
)

// New returns a JSON (or text, for local dev) logger tagged with the service,
// environment and version. Log lines must never carry PII fields.
func New(cfg config.Log, service, env string, w io.Writer) (*slog.Logger, error) {
	var level slog.Level
	if err := level.UnmarshalText([]byte(cfg.Level)); err != nil {
		return nil, fmt.Errorf("logging: invalid level %q: %w", cfg.Level, err)
	}
	opts := &slog.HandlerOptions{Level: level}

	var h slog.Handler
	if cfg.Format == "text" {
		h = slog.NewTextHandler(w, opts)
	} else {
		h = slog.NewJSONHandler(w, opts)
	}
	return slog.New(h).With(
		slog.String("service", service),
		slog.String("env", env),
		slog.String("version", buildinfo.Version),
	), nil
}
