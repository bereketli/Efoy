package vehicle

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/blinge12/efoy/pkg/authz"
	"github.com/blinge12/efoy/pkg/errs"
	"github.com/blinge12/efoy/pkg/httpx"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) Routes(r chi.Router) {
	r.With(authz.RequireAuth).Post("/vehicles", h.register)
}

type inputBody struct {
	PlateNumber    string `json:"plate_number"`
	VehicleClass   string `json:"vehicle_class"`
	Make           string `json:"make"`
	Model          string `json:"model"`
	Year           int    `json:"year"`
	Colour         string `json:"colour"`
	PassengerSeats int    `json:"passenger_seats"`
	HasSeatbelts   *bool  `json:"has_seatbelts"`
	HasFirstAid    bool   `json:"has_first_aid"`
}

// Body is the Vehicle schema; the driver domain embeds it in profiles.
type Body struct {
	ID             string    `json:"id"`
	OwnerDriverID  *string   `json:"owner_driver_id"`
	FleetOwnerID   *string   `json:"fleet_owner_id"`
	PlateNumber    string    `json:"plate_number"`
	VehicleClass   string    `json:"vehicle_class"`
	Make           string    `json:"make"`
	Model          string    `json:"model"`
	Year           int       `json:"year"`
	Colour         string    `json:"colour"`
	PassengerSeats int       `json:"passenger_seats"`
	HasSeatbelts   bool      `json:"has_seatbelts"`
	HasFirstAid    bool      `json:"has_first_aid"`
	Status         string    `json:"status"`
	CreatedAt      time.Time `json:"created_at"`
}

func idString(id interface{ String() string }) *string {
	s := id.String()
	return &s
}

// ToBody converts a vehicle to its API representation.
func ToBody(v Vehicle) Body {
	b := Body{
		ID:             v.ID.String(),
		PlateNumber:    v.PlateNumber,
		VehicleClass:   v.Class,
		Make:           v.Make,
		Model:          v.Model,
		Year:           v.Year,
		Colour:         v.Colour,
		PassengerSeats: v.PassengerSeats,
		HasSeatbelts:   v.HasSeatbelts,
		HasFirstAid:    v.HasFirstAid,
		Status:         v.Status,
		CreatedAt:      v.CreatedAt,
	}
	if v.OwnerDriverID != nil {
		b.OwnerDriverID = idString(v.OwnerDriverID)
	}
	if v.FleetOwnerID != nil {
		b.FleetOwnerID = idString(v.FleetOwnerID)
	}
	return b
}

func (h *Handler) register(w http.ResponseWriter, r *http.Request) {
	actor, err := authz.Require(r.Context())
	if err != nil {
		errs.Write(w, r, err)
		return
	}
	var body inputBody
	if err := httpx.DecodeJSON(w, r, &body); err != nil {
		errs.Write(w, r, err)
		return
	}
	v, err := h.svc.Register(r.Context(), actor, Input{
		PlateNumber:    body.PlateNumber,
		Class:          body.VehicleClass,
		Make:           body.Make,
		Model:          body.Model,
		Year:           body.Year,
		Colour:         body.Colour,
		PassengerSeats: body.PassengerSeats,
		HasSeatbelts:   body.HasSeatbelts,
		HasFirstAid:    body.HasFirstAid,
	})
	if err != nil {
		errs.Write(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, ToBody(v))
}
