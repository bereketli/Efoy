// Package jobs holds the River background jobs run by the workers binary
// (design doc 7.1). Business jobs (trip generation, renewals, reminders...)
// are added here as their domains land.
package jobs

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
)

// Queues and their concurrency (design doc 14.1).
const (
	QueueNotifyCritical = "notify_critical"
	QueueNotifyDefault  = "notify_default"
)

// OutboxRetention matches the 7-day retention of the NATS DOMAIN stream:
// older published events could no longer be replayed from NATS anyway.
const OutboxRetention = 7 * 24 * time.Hour

// OutboxCleanupArgs deletes published outbox rows past retention.
type OutboxCleanupArgs struct{}

func (OutboxCleanupArgs) Kind() string { return "outbox_cleanup" }

type OutboxCleanupWorker struct {
	river.WorkerDefaults[OutboxCleanupArgs]
	Pool *pgxpool.Pool
	Log  *slog.Logger
}

func (w *OutboxCleanupWorker) Work(ctx context.Context, _ *river.Job[OutboxCleanupArgs]) error {
	tag, err := w.Pool.Exec(ctx, `
		DELETE FROM outbox_events
		 WHERE published_at IS NOT NULL AND published_at < now() - $1::interval`,
		fmt.Sprintf("%d seconds", int(OutboxRetention.Seconds())))
	if err != nil {
		return fmt.Errorf("outbox cleanup: %w", err)
	}
	w.Log.InfoContext(ctx, "outbox cleanup", slog.Int64("deleted", tag.RowsAffected()))
	return nil
}

// NewClient builds the River client with every worker and periodic job.
func NewClient(pool *pgxpool.Pool, log *slog.Logger) (*river.Client[pgx.Tx], error) {
	workers := river.NewWorkers()
	river.AddWorker(workers, &OutboxCleanupWorker{Pool: pool, Log: log})

	return river.NewClient(riverpgxv5.New(pool), &river.Config{
		Logger: log,
		Queues: map[string]river.QueueConfig{
			river.QueueDefault:  {MaxWorkers: 5},
			QueueNotifyCritical: {MaxWorkers: 20},
			QueueNotifyDefault:  {MaxWorkers: 5},
		},
		Workers: workers,
		PeriodicJobs: []*river.PeriodicJob{
			river.NewPeriodicJob(
				river.PeriodicInterval(time.Hour),
				func() (river.JobArgs, *river.InsertOpts) { return OutboxCleanupArgs{}, nil },
				&river.PeriodicJobOpts{RunOnStart: true},
			),
		},
	})
}
