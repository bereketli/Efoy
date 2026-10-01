package vehicle

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/blinge12/efoy/pkg/authz"
	"github.com/blinge12/efoy/pkg/clock"
	"github.com/blinge12/efoy/pkg/idgen"
	"github.com/blinge12/efoy/pkg/outbox"
)

type memRepo struct {
	vehicles map[uuid.UUID]Vehicle
	events   []string
}

func (m *memRepo) WithTx(_ context.Context, fn func(Repo) error) error { return fn(m) }

func (m *memRepo) Create(_ context.Context, v Vehicle) error {
	for _, o := range m.vehicles {
		if o.PlateNumber == v.PlateNumber {
			return ErrPlateExists
		}
	}
	m.vehicles[v.ID] = v
	return nil
}

func (m *memRepo) Get(_ context.Context, id uuid.UUID) (Vehicle, error) {
	v, ok := m.vehicles[id]
	if !ok {
		return Vehicle{}, ErrNotFound
	}
	return v, nil
}

func (m *memRepo) ListByOwnerDriver(_ context.Context, driverID uuid.UUID) ([]Vehicle, error) {
	out := []Vehicle{}
	for _, v := range m.vehicles {
		if v.OwnerDriverID != nil && *v.OwnerDriverID == driverID {
			out = append(out, v)
		}
	}
	return out, nil
}

func (m *memRepo) AddEvent(_ context.Context, e outbox.Event) error {
	m.events = append(m.events, e.Subject)
	return nil
}

func minibus() Input {
	return Input{PlateNumber: " aa-3-12345 ", Class: "MINIBUS", Make: "Toyota", Model: "HiAce", Year: 2018, Colour: "White", PassengerSeats: 12}
}

func TestRegisterVehicle(t *testing.T) {
	repo := &memRepo{vehicles: map[uuid.UUID]Vehicle{}}
	driverUser, driverID := idgen.New(), idgen.New()
	lookup := func(_ context.Context, userID uuid.UUID) (uuid.UUID, error) {
		if userID == driverUser {
			return driverID, nil
		}
		return uuid.Nil, nil
	}
	svc := NewService(repo, lookup, clock.NewFake(time.Date(2026, 10, 1, 6, 0, 0, 0, time.UTC)))
	ctx := context.Background()
	driver := authz.Actor{UserID: driverUser}

	v, err := svc.Register(ctx, driver, minibus())
	require.NoError(t, err)
	assert.Equal(t, "AA-3-12345", v.PlateNumber)
	assert.Equal(t, &driverID, v.OwnerDriverID)
	assert.True(t, v.HasSeatbelts, "seatbelts default to true")
	assert.Equal(t, "PENDING", v.Status)
	assert.Equal(t, []string{"efoy.vehicle.registered"}, repo.events)

	owner, err := svc.OwnerDriverID(ctx, v.ID)
	require.NoError(t, err)
	assert.Equal(t, &driverID, owner)

	_, err = svc.Register(ctx, driver, minibus())
	assert.ErrorIs(t, err, ErrPlateTaken, "plates are compared after normalisation")

	_, err = svc.Register(ctx, authz.Actor{UserID: idgen.New()}, minibus())
	assert.ErrorIs(t, err, ErrNotADriver)
}

func TestVehicleValidation(t *testing.T) {
	now := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	for name, mutate := range map[string]func(*Input){
		"17 seats":       func(in *Input) { in.PassengerSeats = 17 },
		"no seats":       func(in *Input) { in.PassengerSeats = 0 },
		"unknown class":  func(in *Input) { in.Class = "BUS" },
		"bad plate":      func(in *Input) { in.PlateNumber = "A!" },
		"future year":    func(in *Input) { in.Year = 2028 },
		"too old":        func(in *Input) { in.Year = 1985 },
		"missing make":   func(in *Input) { in.Make = " " },
		"missing colour": func(in *Input) { in.Colour = "" },
	} {
		in := minibus()
		mutate(&in)
		assert.Error(t, in.validate(now), name)
	}
	in := minibus()
	in.PassengerSeats = MaxPassengerSeats
	assert.NoError(t, in.validate(now), "16 seats is the maximum")
}
