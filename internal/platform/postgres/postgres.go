// Package postgres opens the pgx connection pool.
package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/blinge12/efoy/internal/config"
)

// Connect creates a pool without dialing; the readiness probe reports whether
// the database is reachable, so a service can start before Postgres is up.
func Connect(ctx context.Context, cfg config.Database) (*pgxpool.Pool, error) {
	pcfg, err := pgxpool.ParseConfig(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("postgres: parse url: %w", err)
	}
	if cfg.MaxConns > 0 {
		pcfg.MaxConns = cfg.MaxConns
	}
	pool, err := pgxpool.NewWithConfig(ctx, pcfg)
	if err != nil {
		return nil, fmt.Errorf("postgres: new pool: %w", err)
	}
	return pool, nil
}
