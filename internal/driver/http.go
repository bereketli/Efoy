package driver

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/blinge12/efoy/internal/vehicle"
	"github.com/blinge12/efoy/pkg/authz"
	"github.com/blinge12/efoy/pkg/errs"
	"github.com/blinge12/efoy/pkg/httpx"
)

// Handler exposes the driver, document and document review endpoints.
type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) Routes(r chi.Router) {
	r.Group(func(r chi.Router) {
		r.Use(authz.RequireAuth)
		r.Post("/drivers", h.register)
		r.Get("/drivers/me", h.myProfile)
		r.Get("/drivers/{id}", h.profile)
		r.Post("/documents/upload-url", h.uploadURL)
		r.Post("/documents", h.submit)
	})
	r.Route("/admin/documents", func(r chi.Router) {
		r.Use(authz.RequireRole(authz.RoleSuperAdmin, authz.RoleDispatcher))
		r.Get("/", h.queue)
		r.Get("/{id}", h.document)
		r.Post("/{id}/review", h.review)
	})
}

// ---- bodies (schemas in efoy.yaml) ----

type driverInputBody struct {
	Source                string  `json:"source"`
	LicenceNumber         string  `json:"licence_number"`
	LicenceCategory       string  `json:"licence_category"`
	LicenceExpiresOn      *string `json:"licence_expires_on"`
	EmergencyContactName  *string `json:"emergency_contact_name"`
	EmergencyContactPhone *string `json:"emergency_contact_phone"`
}

type driverBody struct {
	ID                    string    `json:"id"`
	UserID                string    `json:"user_id"`
	FullName              string    `json:"full_name"`
	PhoneE164             *string   `json:"phone_e164"`
	Source                string    `json:"source"`
	LicenceNumber         string    `json:"licence_number"`
	LicenceCategory       string    `json:"licence_category"`
	LicenceExpiresOn      string    `json:"licence_expires_on"`
	EmergencyContactName  *string   `json:"emergency_contact_name"`
	EmergencyContactPhone *string   `json:"emergency_contact_phone"`
	Status                string    `json:"status"`
	StudentCertified      bool      `json:"student_certified"`
	CreatedAt             time.Time `json:"created_at"`
}

type documentBody struct {
	ID              string     `json:"id"`
	OwnerType       string     `json:"owner_type"`
	OwnerID         string     `json:"owner_id"`
	OwnerLabel      string     `json:"owner_label"`
	DocType         string     `json:"doc_type"`
	DocNumber       *string    `json:"doc_number"`
	IssuedOn        *string    `json:"issued_on"`
	ExpiresOn       *string    `json:"expires_on"`
	Status          string     `json:"status"`
	ContentType     string     `json:"content_type"`
	RejectionReason *string    `json:"rejection_reason"`
	ReviewedAt      *time.Time `json:"reviewed_at"`
	CreatedAt       time.Time  `json:"created_at"`
	ViewURL         *string    `json:"view_url,omitempty"`
}

type profileBody struct {
	Driver    driverBody     `json:"driver"`
	Vehicles  []vehicle.Body `json:"vehicles"`
	Documents []documentBody `json:"documents"`
}

type uploadRequestBody struct {
	OwnerType   string `json:"owner_type"`
	OwnerID     string `json:"owner_id"`
	DocType     string `json:"doc_type"`
	ContentType string `json:"content_type"`
}

type uploadBody struct {
	UploadURL string    `json:"upload_url"`
	FileKey   string    `json:"file_key"`
	ExpiresAt time.Time `json:"expires_at"`
	MaxBytes  int64     `json:"max_bytes"`
}

type documentInputBody struct {
	OwnerType string  `json:"owner_type"`
	OwnerID   string  `json:"owner_id"`
	DocType   string  `json:"doc_type"`
	FileKey   string  `json:"file_key"`
	DocNumber *string `json:"doc_number"`
	IssuedOn  *string `json:"issued_on"`
	ExpiresOn *string `json:"expires_on"`
}

type pageBody struct {
	Items      []documentBody `json:"items"`
	NextCursor *string        `json:"next_cursor"`
}

type reviewBody struct {
	Decision string `json:"decision"`
	Reason   string `json:"reason"`
}

func toDriverBody(d Driver) driverBody {
	return driverBody{
		ID:                    d.ID.String(),
		UserID:                d.UserID.String(),
		FullName:              d.FullName,
		PhoneE164:             d.Phone,
		Source:                d.Source,
		LicenceNumber:         d.LicenceNumber,
		LicenceCategory:       d.LicenceCategory,
		LicenceExpiresOn:      d.LicenceExpiresOn.Format(httpx.DateLayout),
		EmergencyContactName:  d.EmergencyContactName,
		EmergencyContactPhone: d.EmergencyContactPhone,
		Status:                d.Status,
		StudentCertified:      d.StudentCertified,
		CreatedAt:             d.CreatedAt,
	}
}

func toDocumentBody(d Document) documentBody {
	return documentBody{
		ID:              d.ID.String(),
		OwnerType:       d.OwnerType,
		OwnerID:         d.OwnerID.String(),
		OwnerLabel:      d.OwnerLabel,
		DocType:         d.DocType,
		DocNumber:       d.DocNumber,
		IssuedOn:        httpx.FormatDate(d.IssuedOn),
		ExpiresOn:       httpx.FormatDate(d.ExpiresOn),
		Status:          d.Status,
		ContentType:     d.ContentType,
		RejectionReason: d.RejectionReason,
		ReviewedAt:      d.ReviewedAt,
		CreatedAt:       d.CreatedAt,
		ViewURL:         d.ViewURL,
	}
}

func toDocumentBodies(docs []Document) []documentBody {
	out := make([]documentBody, 0, len(docs))
	for _, d := range docs {
		out = append(out, toDocumentBody(d))
	}
	return out
}

func toProfileBody(p Profile) profileBody {
	vehicles := make([]vehicle.Body, 0, len(p.Vehicles))
	for _, v := range p.Vehicles {
		vehicles = append(vehicles, vehicle.ToBody(v))
	}
	return profileBody{Driver: toDriverBody(p.Driver), Vehicles: vehicles, Documents: toDocumentBodies(p.Documents)}
}

var errInvalidID = errs.Invalid("INVALID_ID", "The id must be a UUID.")

func parseID(s string) (uuid.UUID, error) {
	id, err := uuid.Parse(s)
	if err != nil {
		return uuid.Nil, errInvalidID
	}
	return id, nil
}

// ---- handlers ----

func (h *Handler) register(w http.ResponseWriter, r *http.Request) {
	actor, _ := authz.ActorFrom(r.Context())
	var body driverInputBody
	if err := httpx.DecodeJSON(w, r, &body); err != nil {
		errs.Write(w, r, err)
		return
	}
	expires, err := httpx.ParseDate("licence_expires_on", body.LicenceExpiresOn)
	if err != nil {
		errs.Write(w, r, err)
		return
	}
	if expires == nil {
		errs.Write(w, r, ErrLicenceExpired)
		return
	}
	d, err := h.svc.Register(r.Context(), actor, Input{
		Source:                body.Source,
		LicenceNumber:         body.LicenceNumber,
		LicenceCategory:       body.LicenceCategory,
		LicenceExpiresOn:      *expires,
		EmergencyContactName:  body.EmergencyContactName,
		EmergencyContactPhone: body.EmergencyContactPhone,
	})
	if err != nil {
		errs.Write(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, toDriverBody(d))
}

func (h *Handler) myProfile(w http.ResponseWriter, r *http.Request) {
	actor, _ := authz.ActorFrom(r.Context())
	p, err := h.svc.MyProfile(r.Context(), actor)
	if err != nil {
		errs.Write(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toProfileBody(p))
}

func (h *Handler) profile(w http.ResponseWriter, r *http.Request) {
	actor, _ := authz.ActorFrom(r.Context())
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		errs.Write(w, r, err)
		return
	}
	p, err := h.svc.ProfileByID(r.Context(), actor, id)
	if err != nil {
		errs.Write(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toProfileBody(p))
}

func (h *Handler) uploadURL(w http.ResponseWriter, r *http.Request) {
	actor, _ := authz.ActorFrom(r.Context())
	var body uploadRequestBody
	if err := httpx.DecodeJSON(w, r, &body); err != nil {
		errs.Write(w, r, err)
		return
	}
	ownerID, err := parseID(body.OwnerID)
	if err != nil {
		errs.Write(w, r, err)
		return
	}
	up, err := h.svc.RequestUpload(r.Context(), actor, UploadRequest{
		OwnerType:   body.OwnerType,
		OwnerID:     ownerID,
		DocType:     body.DocType,
		ContentType: body.ContentType,
	})
	if err != nil {
		errs.Write(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, uploadBody{UploadURL: up.URL, FileKey: up.FileKey, ExpiresAt: up.ExpiresAt, MaxBytes: up.MaxBytes})
}

func (h *Handler) submit(w http.ResponseWriter, r *http.Request) {
	actor, _ := authz.ActorFrom(r.Context())
	var body documentInputBody
	if err := httpx.DecodeJSON(w, r, &body); err != nil {
		errs.Write(w, r, err)
		return
	}
	ownerID, err := parseID(body.OwnerID)
	if err != nil {
		errs.Write(w, r, err)
		return
	}
	issued, err := httpx.ParseDate("issued_on", body.IssuedOn)
	if err != nil {
		errs.Write(w, r, err)
		return
	}
	expires, err := httpx.ParseDate("expires_on", body.ExpiresOn)
	if err != nil {
		errs.Write(w, r, err)
		return
	}
	doc, err := h.svc.Submit(r.Context(), actor, DocumentInput{
		OwnerType: body.OwnerType,
		OwnerID:   ownerID,
		DocType:   body.DocType,
		FileKey:   body.FileKey,
		DocNumber: body.DocNumber,
		IssuedOn:  issued,
		ExpiresOn: expires,
	})
	if err != nil {
		errs.Write(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, toDocumentBody(doc))
}

func (h *Handler) queue(w http.ResponseWriter, r *http.Request) {
	actor, _ := authz.ActorFrom(r.Context())
	q := r.URL.Query()
	var statuses []string
	if s := q.Get("status"); s != "" {
		statuses = strings.Split(s, ",")
	}
	limit, _ := strconv.Atoi(q.Get("limit"))
	page, err := h.svc.ReviewQueue(r.Context(), actor, statuses, limit, q.Get("cursor"))
	if err != nil {
		errs.Write(w, r, err)
		return
	}
	body := pageBody{Items: toDocumentBodies(page.Items)}
	if page.NextCursor != "" {
		body.NextCursor = &page.NextCursor
	}
	httpx.WriteJSON(w, http.StatusOK, body)
}

func (h *Handler) document(w http.ResponseWriter, r *http.Request) {
	actor, _ := authz.ActorFrom(r.Context())
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		errs.Write(w, r, err)
		return
	}
	doc, err := h.svc.DocumentForReview(r.Context(), actor, id)
	if err != nil {
		errs.Write(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toDocumentBody(doc))
}

func (h *Handler) review(w http.ResponseWriter, r *http.Request) {
	actor, _ := authz.ActorFrom(r.Context())
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		errs.Write(w, r, err)
		return
	}
	var body reviewBody
	if err := httpx.DecodeJSON(w, r, &body); err != nil {
		errs.Write(w, r, err)
		return
	}
	doc, err := h.svc.Review(r.Context(), actor, id, body.Decision, body.Reason)
	if err != nil {
		errs.Write(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toDocumentBody(doc))
}
