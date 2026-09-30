// Package httpx contains domain-free HTTP helpers shared by all services:
// the server lifecycle, common middleware, JSON responses and health probes.
package httpx

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

// ServerConfig controls the HTTP server and its graceful shutdown.
type ServerConfig struct {
	Addr            string
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	IdleTimeout     time.Duration
	ShutdownTimeout time.Duration
}

// Serve runs h on cfg.Addr until ctx is cancelled, then drains in-flight
// requests for up to cfg.ShutdownTimeout.
func Serve(ctx context.Context, log *slog.Logger, cfg ServerConfig, h http.Handler) error {
	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           h,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       cfg.ReadTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       cfg.IdleTimeout,
		ErrorLog:          slog.NewLogLogger(log.Handler(), slog.LevelWarn),
	}

	errCh := make(chan error, 1)
	go func() {
		log.Info("http server listening", slog.String("addr", cfg.Addr))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case err := <-errCh:
		if err != nil {
			return fmt.Errorf("http server: %w", err)
		}
		return nil
	case <-ctx.Done():
	}

	log.Info("shutting down http server")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("http shutdown: %w", err)
	}
	return nil
}
