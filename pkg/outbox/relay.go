package outbox

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Publisher sends one message; msgID lets the broker drop duplicates when the
// relay retries after a crash (NATS JetStream Nats-Msg-Id).
type Publisher interface {
	Publish(ctx context.Context, subject string, data []byte, msgID string) error
}

// Relay publishes unpublished outbox rows in id order. Several relays may run
// at once: rows are claimed with FOR UPDATE SKIP LOCKED.
type Relay struct {
	pool     *pgxpool.Pool
	pub      Publisher
	log      *slog.Logger
	batch    int
	interval time.Duration
}

func NewRelay(pool *pgxpool.Pool, pub Publisher, log *slog.Logger) *Relay {
	return &Relay{pool: pool, pub: pub, log: log, batch: 100, interval: 500 * time.Millisecond}
}

// Run relays until ctx is cancelled. A full batch is followed immediately by
// the next one; otherwise the relay polls every interval.
func (r *Relay) Run(ctx context.Context) {
	r.log.Info("outbox relay started")
	for {
		n, err := r.RelayOnce(ctx)
		if err != nil && ctx.Err() == nil {
			r.log.Error("outbox relay", slog.Any("error", err))
		}
		if n == r.batch && err == nil {
			continue
		}
		select {
		case <-ctx.Done():
			r.log.Info("outbox relay stopped")
			return
		case <-time.After(r.interval):
		}
	}
}

type pending struct {
	id      int64
	subject string
	payload []byte
	eventID string
}

// RelayOnce publishes up to one batch. Rows published before a failure are
// still marked as published; the failed row and the rest are retried later.
func (r *Relay) RelayOnce(ctx context.Context) (int, error) {
	published := 0
	var pubErr error
	err := pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT id, subject, payload::text, COALESCE(headers->>'event_id', id::text)
			  FROM outbox_events
			 WHERE published_at IS NULL
			 ORDER BY id
			 LIMIT $1
			   FOR UPDATE SKIP LOCKED`, r.batch)
		if err != nil {
			return err
		}
		batch, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (pending, error) {
			var p pending
			var payload string
			err := row.Scan(&p.id, &p.subject, &payload, &p.eventID)
			p.payload = []byte(payload)
			return p, err
		})
		if err != nil {
			return err
		}

		ids := make([]int64, 0, len(batch))
		for _, p := range batch {
			if err := r.pub.Publish(ctx, p.subject, p.payload, p.eventID); err != nil {
				pubErr = fmt.Errorf("publish %s (outbox id %d): %w", p.subject, p.id, err)
				break
			}
			ids = append(ids, p.id)
		}
		if len(ids) == 0 {
			return nil
		}
		if _, err := tx.Exec(ctx, `UPDATE outbox_events SET published_at = now() WHERE id = ANY($1)`, ids); err != nil {
			return err
		}
		published = len(ids)
		return nil
	})
	if err != nil {
		return 0, fmt.Errorf("outbox relay: %w", err)
	}
	return published, pubErr
}
