// Package bootstrap wires the pieces every Efoy binary shares: config,
// logging, the chi router with health endpoints, and graceful shutdown.
package bootstrap

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/go-chi/chi/v5"

	"github.com/blinge12/efoy/internal/buildinfo"
	"github.com/blinge12/efoy/internal/config"
	"github.com/blinge12/efoy/internal/platform/logging"
	"github.com/blinge12/efoy/pkg/httpx"
)

// App is handed to each service's setup function.
type App struct {
	Name    string
	Config  *config.Config
	Log     *slog.Logger
	Router  chi.Router
	Health  *httpx.Health
	closers []func()
}

// OnShutdown registers fn to run after the HTTP server stops, in reverse order.
func (a *App) OnShutdown(fn func()) {
	a.closers = append(a.closers, fn)
}

func (a *App) close() {
	for i := len(a.closers) - 1; i >= 0; i-- {
		a.closers[i]()
	}
}

// SetupFunc registers a service's routes, health checks and dependencies.
type SetupFunc func(ctx context.Context, app *App) error

// Run starts the named service and blocks until SIGINT/SIGTERM.
func Run(name string, setup SetupFunc) {
	if err := run(name, setup); err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", name, err)
		os.Exit(1)
	}
}

func run(name string, setup SetupFunc) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	log, err := logging.New(cfg.Log, name, cfg.Env, os.Stdout)
	if err != nil {
		return err
	}
	slog.SetDefault(log)

	health := httpx.NewHealth(name, buildinfo.Version, log)
	app := &App{
		Name:   name,
		Config: cfg,
		Log:    log,
		Router: httpx.NewRouter(log, health),
		Health: health,
	}
	defer app.close()

	if setup != nil {
		if err := setup(ctx, app); err != nil {
			return fmt.Errorf("setup: %w", err)
		}
	}

	log.Info("starting", slog.String("commit", buildinfo.Commit))
	return httpx.Serve(ctx, log, httpx.ServerConfig{
		Addr:            cfg.HTTP.Addr,
		ReadTimeout:     cfg.HTTP.ReadTimeout,
		WriteTimeout:    cfg.HTTP.WriteTimeout,
		IdleTimeout:     cfg.HTTP.IdleTimeout,
		ShutdownTimeout: cfg.HTTP.ShutdownTimeout,
	}, app.Router)
}
