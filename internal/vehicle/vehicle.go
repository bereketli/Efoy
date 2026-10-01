// Package vehicle registers vehicles (FR-DRV-2). A vehicle belongs to a
// driver (taxi and Yango drivers bring their own car) or, from day 4, to a
// fleet owner. It stays PENDING until its documents are approved.
package vehicle

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/blinge12/efoy/pkg/authz"
	"github.com/blinge12/efoy/pkg/clock"
	"github.com/blinge12/efoy/pkg/errs"
	"github.com/blinge12/efoy/pkg/idgen"
	"github.com/blinge12/efoy/pkg/outbox"
)

// MaxPassengerSeats is the platform limit (FR-DRV-2, FR-SUB-2).
const MaxPassengerSeats = 16

var Classes = []string{"MINIBUS", "MIDIBUS", "SEDAN", "SUV"}

type Vehicle struct {
	ID             uuid.UUID
	OwnerDriverID  *uuid.UUID
	FleetOwnerID   *uuid.UUID
	PlateNumber    string
	Class          string
	Make           string
	Model          string
	Year           int
	Colour         string
	PassengerSeats int
	HasSeatbelts   bool
	HasFirstAid    bool
	Status         string
	CreatedAt      time.Time
}

// Input is a registration request.
type Input struct {
	PlateNumber    string
	Class          string
	Make           string
	Model          string
	Year           int
	Colour         string
	PassengerSeats int
	HasSeatbelts   *bool // defaults to true
	HasFirstAid    bool
}

var (
	ErrNotFound    = errors.New("vehicle: not found")
	ErrPlateExists = errors.New("vehicle: plate already registered")

	ErrPlateTaken = errs.Conflict("PLATE_TAKEN", "A vehicle with this plate number is already registered.")
	ErrNotADriver = errs.Forbidden("NOT_A_DRIVER", "Register as a driver before adding a vehicle.")
)

// Ethiopian plates look like "AA-3-12345" or "3-A12345": letters, digits,
// dashes and spaces.
var plateRE = regexp.MustCompile(`^[A-Z0-9][A-Z0-9 -]{2,14}$`)

// NormalizePlate upper-cases and collapses whitespace so the unique index
// catches the same plate typed differently.
func NormalizePlate(p string) string {
	return strings.ToUpper(strings.Join(strings.Fields(p), " "))
}

func invalid(code, detail string) error { return errs.Invalid(code, detail) }

func (in Input) validate(now time.Time) error {
	switch {
	case !plateRE.MatchString(NormalizePlate(in.PlateNumber)):
		return invalid("INVALID_PLATE", "plate_number must be 3–15 letters, digits, dashes or spaces, e.g. AA-3-12345.")
	case !slices.Contains(Classes, in.Class):
		return invalid("INVALID_VEHICLE_CLASS", "vehicle_class must be MINIBUS, MIDIBUS, SEDAN or SUV.")
	case strings.TrimSpace(in.Make) == "" || len(in.Make) > 60:
		return invalid("INVALID_MAKE", "make is required (at most 60 characters).")
	case strings.TrimSpace(in.Model) == "" || len(in.Model) > 60:
		return invalid("INVALID_MODEL", "model is required (at most 60 characters).")
	case strings.TrimSpace(in.Colour) == "" || len(in.Colour) > 30:
		return invalid("INVALID_COLOUR", "colour is required (at most 30 characters).")
	case in.Year < 1990 || in.Year > now.Year()+1:
		return invalid("INVALID_YEAR", fmt.Sprintf("year must be between 1990 and %d.", now.Year()+1))
	case in.PassengerSeats < 1 || in.PassengerSeats > MaxPassengerSeats:
		return invalid("INVALID_SEATS", fmt.Sprintf("passenger_seats must be between 1 and %d.", MaxPassengerSeats))
	}
	return nil
}

// Repo is the persistence port. Create returns ErrPlateExists on a duplicate
// plate; lookups return ErrNotFound.
type Repo interface {
	WithTx(ctx context.Context, fn func(Repo) error) error
	Create(ctx context.Context, v Vehicle) error
	Get(ctx context.Context, id uuid.UUID) (Vehicle, error)
	ListByOwnerDriver(ctx context.Context, driverID uuid.UUID) ([]Vehicle, error)
	AddEvent(ctx context.Context, e outbox.Event) error
}

// DriverLookup returns the driver id of a user, or uuid.Nil if the user is
// not a driver. The driver domain provides it.
type DriverLookup func(ctx context.Context, userID uuid.UUID) (uuid.UUID, error)

type Service struct {
	repo    Repo
	drivers DriverLookup
	clock   clock.Clock
}

func NewService(repo Repo, drivers DriverLookup, clk clock.Clock) *Service {
	return &Service{repo: repo, drivers: drivers, clock: clk}
}

// Register adds a vehicle owned by the calling driver.
func (s *Service) Register(ctx context.Context, actor authz.Actor, in Input) (Vehicle, error) {
	driverID, err := s.drivers(ctx, actor.UserID)
	if err != nil {
		return Vehicle{}, err
	}
	if driverID == uuid.Nil {
		return Vehicle{}, ErrNotADriver
	}
	now := s.clock.Now()
	if err := in.validate(now); err != nil {
		return Vehicle{}, err
	}
	seatbelts := true
	if in.HasSeatbelts != nil {
		seatbelts = *in.HasSeatbelts
	}
	v := Vehicle{
		ID:             idgen.New(),
		OwnerDriverID:  &driverID,
		PlateNumber:    NormalizePlate(in.PlateNumber),
		Class:          in.Class,
		Make:           strings.TrimSpace(in.Make),
		Model:          strings.TrimSpace(in.Model),
		Year:           in.Year,
		Colour:         strings.TrimSpace(in.Colour),
		PassengerSeats: in.PassengerSeats,
		HasSeatbelts:   seatbelts,
		HasFirstAid:    in.HasFirstAid,
		Status:         "PENDING",
		CreatedAt:      now,
	}
	err = s.repo.WithTx(ctx, func(r Repo) error {
		if err := r.Create(ctx, v); err != nil {
			if errors.Is(err, ErrPlateExists) {
				return ErrPlateTaken
			}
			return err
		}
		return r.AddEvent(ctx, outbox.Event{
			AggregateType: "vehicle",
			AggregateID:   v.ID,
			Subject:       "efoy.vehicle.registered",
			Data:          map[string]any{"vehicle_id": v.ID, "driver_id": driverID, "plate_number": v.PlateNumber},
			OccurredAt:    now,
		})
	})
	if err != nil {
		return Vehicle{}, err
	}
	return s.repo.Get(ctx, v.ID)
}

// OwnedByDriver lists a driver's vehicles, oldest first.
func (s *Service) OwnedByDriver(ctx context.Context, driverID uuid.UUID) ([]Vehicle, error) {
	return s.repo.ListByOwnerDriver(ctx, driverID)
}

// OwnerDriverID returns the driver owning the vehicle (nil for fleet
// vehicles) or ErrNotFound.
func (s *Service) OwnerDriverID(ctx context.Context, vehicleID uuid.UUID) (*uuid.UUID, error) {
	v, err := s.repo.Get(ctx, vehicleID)
	if err != nil {
		return nil, err
	}
	return v.OwnerDriverID, nil
}
