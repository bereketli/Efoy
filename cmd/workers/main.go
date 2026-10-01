// Command workers runs River background jobs (trip generation, billing,
// payouts, reminders, notifications) and the transactional outbox relay that
// publishes domain events to NATS JetStream.
package main

import (
	"context"
	"time"

	"github.com/blinge12/efoy/internal/jobs"
	"github.com/blinge12/efoy/internal/platform/bootstrap"
	"github.com/blinge12/efoy/internal/platform/postgres"
	"github.com/blinge12/efoy/pkg/natsx"
	"github.com/blinge12/efoy/pkg/outbox"
)

func main() {
	bootstrap.Run("workers", setup)
}

func setup(ctx context.Context, app *bootstrap.App) error {
	cfg := app.Config

	pool, err := postgres.Connect(ctx, cfg.Database)
	if err != nil {
		return err
	}
	app.OnShutdown(pool.Close)
	app.Health.AddCheck("postgres", pool.Ping)

	js, err := natsx.Connect(cfg.NATS.URL, "efoy-workers")
	if err != nil {
		return err
	}
	app.OnShutdown(js.Close)
	app.Health.AddCheck("nats", js.Ping)

	streamCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if err := js.EnsureStreams(streamCtx, cfg.NATS.Replicas); err != nil {
		return err
	}

	// Outbox relay: runs until shutdown starts (ctx is cancelled on SIGTERM).
	relay := outbox.NewRelay(pool, js, app.Log)
	relayDone := make(chan struct{})
	go func() {
		defer close(relayDone)
		relay.Run(ctx)
	}()

	queue, err := jobs.NewClient(pool, app.Log)
	if err != nil {
		return err
	}
	if err := queue.Start(ctx); err != nil {
		return err
	}
	// Closers run in reverse: stop River, wait for the relay, then close NATS
	// and the pool.
	app.OnShutdown(func() { <-relayDone })
	app.OnShutdown(func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		if err := queue.Stop(stopCtx); err != nil {
			app.Log.Error("river stop", "error", err)
		}
	})
	return nil
}
