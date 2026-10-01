// Package postgres implements vehicle.Repo with pgx.
package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/blinge12/efoy/internal/vehicle"
	"github.com/blinge12/efoy/pkg/outbox"
)

type querier interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type Repo struct {
	pool *pgxpool.Pool
	q    querier
	inTx bool
}

func New(pool *pgxpool.Pool) *Repo { return &Repo{pool: pool, q: pool} }

var _ vehicle.Repo = (*Repo)(nil)

func (r *Repo) WithTx(ctx context.Context, fn func(vehicle.Repo) error) error {
	if r.inTx {
		return fn(r)
	}
	return pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		return fn(&Repo{pool: r.pool, q: tx, inTx: true})
	})
}

const columns = `id, owner_driver_id, fleet_owner_id, plate_number, vehicle_class::text, make, model,
	year, colour, passenger_seats, has_seatbelts, has_first_aid, status::text, created_at`

func scan(row pgx.Row) (vehicle.Vehicle, error) {
	var v vehicle.Vehicle
	err := row.Scan(&v.ID, &v.OwnerDriverID, &v.FleetOwnerID, &v.PlateNumber, &v.Class, &v.Make, &v.Model,
		&v.Year, &v.Colour, &v.PassengerSeats, &v.HasSeatbelts, &v.HasFirstAid, &v.Status, &v.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return vehicle.Vehicle{}, vehicle.ErrNotFound
	}
	if err != nil {
		return vehicle.Vehicle{}, fmt.Errorf("vehicle/postgres: scan: %w", err)
	}
	return v, nil
}

func (r *Repo) Create(ctx context.Context, v vehicle.Vehicle) error {
	_, err := r.q.Exec(ctx, `
		INSERT INTO vehicles (id, owner_driver_id, fleet_owner_id, plate_number, vehicle_class, make, model,
		                      year, colour, passenger_seats, has_seatbelts, has_first_aid, status, created_at)
		VALUES ($1, $2, $3, $4, $5::text::vehicle_class, $6, $7, $8, $9, $10, $11, $12,
		        $13::text::onboarding_status, $14)`,
		v.ID, v.OwnerDriverID, v.FleetOwnerID, v.PlateNumber, v.Class, v.Make, v.Model,
		v.Year, v.Colour, v.PassengerSeats, v.HasSeatbelts, v.HasFirstAid, v.Status, v.CreatedAt)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return vehicle.ErrPlateExists
	}
	if err != nil {
		return fmt.Errorf("vehicle/postgres: create: %w", err)
	}
	return nil
}

func (r *Repo) Get(ctx context.Context, id uuid.UUID) (vehicle.Vehicle, error) {
	return scan(r.q.QueryRow(ctx, `SELECT `+columns+` FROM vehicles WHERE id = $1`, id))
}

func (r *Repo) ListByOwnerDriver(ctx context.Context, driverID uuid.UUID) ([]vehicle.Vehicle, error) {
	rows, err := r.q.Query(ctx, `SELECT `+columns+` FROM vehicles WHERE owner_driver_id = $1 ORDER BY created_at, id`, driverID)
	if err != nil {
		return nil, fmt.Errorf("vehicle/postgres: list: %w", err)
	}
	vs, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (vehicle.Vehicle, error) { return scan(row) })
	if err != nil {
		return nil, fmt.Errorf("vehicle/postgres: list: %w", err)
	}
	if vs == nil {
		vs = []vehicle.Vehicle{}
	}
	return vs, nil
}

func (r *Repo) AddEvent(ctx context.Context, e outbox.Event) error {
	return outbox.Add(ctx, r.q, e)
}
