package driver

import (
	"context"
	"encoding/base64"
	"errors"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/blinge12/efoy/internal/vehicle"
	"github.com/blinge12/efoy/pkg/authz"
	"github.com/blinge12/efoy/pkg/clock"
	"github.com/blinge12/efoy/pkg/idgen"
	"github.com/blinge12/efoy/pkg/objstore"
	"github.com/blinge12/efoy/pkg/outbox"
)

const (
	uploadURLTTL = 15 * time.Minute
	viewURLTTL   = 10 * time.Minute
)

// Repo is the persistence port. Lookups return ErrNotFound.
type Repo interface {
	WithTx(ctx context.Context, fn func(Repo) error) error

	Create(ctx context.Context, d Driver) error // ErrDuplicateUser, ErrDuplicate
	Get(ctx context.Context, id uuid.UUID) (Driver, error)
	GetByUser(ctx context.Context, userID uuid.UUID) (Driver, error)

	CreateDocument(ctx context.Context, d Document) error // ErrDuplicateUpload
	GetDocument(ctx context.Context, id uuid.UUID) (Document, error)
	GetDocumentForUpdate(ctx context.Context, id uuid.UUID) (Document, error)
	SetDocumentReview(ctx context.Context, id uuid.UUID, status string, reviewer uuid.UUID, at time.Time, reason *string) error
	ListDocumentsByStatus(ctx context.Context, statuses []string, after Cursor, limit int) ([]Document, error)
	ListDocumentsByOwners(ctx context.Context, ownerIDs []uuid.UUID) ([]Document, error)

	AddEvent(ctx context.Context, e outbox.Event) error
}

// Vehicles is what this domain needs from the vehicle domain.
type Vehicles interface {
	OwnedByDriver(ctx context.Context, driverID uuid.UUID) ([]vehicle.Vehicle, error)
	OwnerDriverID(ctx context.Context, vehicleID uuid.UUID) (*uuid.UUID, error)
}

// Files signs and inspects uploads (objstore.Store).
type Files interface {
	PresignPut(ctx context.Context, key string, ttl time.Duration) (string, error)
	PresignGet(ctx context.Context, key string, ttl time.Duration) (string, error)
	Stat(ctx context.Context, key string) (objstore.Info, error)
	SHA256(ctx context.Context, key string) ([]byte, error)
}

// RoleGranter gives a user a role (the iam domain).
type RoleGranter interface {
	GrantRole(ctx context.Context, userID uuid.UUID, g authz.Grant) error
}

type Service struct {
	repo     Repo
	vehicles Vehicles
	files    Files
	roles    RoleGranter
	clock    clock.Clock
}

func NewService(repo Repo, vehicles Vehicles, files Files, roles RoleGranter, clk clock.Clock) *Service {
	return &Service{repo: repo, vehicles: vehicles, files: files, roles: roles, clock: clk}
}

// ---- drivers ----

func (in Input) validate(today time.Time) error {
	switch {
	case !slices.Contains(sources, in.Source):
		return ErrInvalidSource
	case len(strings.TrimSpace(in.LicenceNumber)) < 3 || len(in.LicenceNumber) > 40,
		strings.TrimSpace(in.LicenceCategory) == "" || len(in.LicenceCategory) > 40:
		return ErrInvalidLicence
	case !in.LicenceExpiresOn.After(today):
		return ErrLicenceExpired
	case in.EmergencyContactPhone != nil && *in.EmergencyContactPhone != "" && !phoneRE.MatchString(*in.EmergencyContactPhone),
		in.EmergencyContactName != nil && len(*in.EmergencyContactName) > 120:
		return ErrInvalidContactInfo
	}
	return nil
}

// Register makes the calling user a driver (status PENDING) and grants the
// DRIVER role.
func (s *Service) Register(ctx context.Context, actor authz.Actor, in Input) (Driver, error) {
	if err := in.validate(clock.Today(s.clock)); err != nil {
		return Driver{}, err
	}
	if _, err := s.repo.GetByUser(ctx, actor.UserID); err == nil {
		// Repair a missing role grant from an earlier partial registration.
		if err := s.grantDriverRole(ctx, actor.UserID); err != nil {
			return Driver{}, err
		}
		return Driver{}, ErrDriverExists
	} else if !errors.Is(err, ErrNotFound) {
		return Driver{}, err
	}

	now := s.clock.Now()
	d := Driver{
		ID:                    idgen.New(),
		UserID:                actor.UserID,
		Source:                in.Source,
		LicenceNumber:         strings.ToUpper(strings.TrimSpace(in.LicenceNumber)),
		LicenceCategory:       strings.TrimSpace(in.LicenceCategory),
		LicenceExpiresOn:      in.LicenceExpiresOn,
		EmergencyContactName:  in.EmergencyContactName,
		EmergencyContactPhone: in.EmergencyContactPhone,
		Status:                StatusPending,
		CreatedAt:             now,
	}
	err := s.repo.WithTx(ctx, func(r Repo) error {
		if err := r.Create(ctx, d); err != nil {
			switch {
			case errors.Is(err, ErrDuplicateUser):
				return ErrDriverExists
			case errors.Is(err, ErrDuplicate):
				return ErrLicenceTaken
			}
			return err
		}
		return r.AddEvent(ctx, outbox.Event{
			AggregateType: "driver",
			AggregateID:   d.ID,
			Subject:       "efoy.driver.registered",
			Data:          map[string]any{"driver_id": d.ID, "user_id": d.UserID, "source": d.Source},
			OccurredAt:    now,
		})
	})
	if err != nil {
		return Driver{}, err
	}
	if err := s.grantDriverRole(ctx, actor.UserID); err != nil {
		return Driver{}, err
	}
	return s.repo.Get(ctx, d.ID)
}

func (s *Service) grantDriverRole(ctx context.Context, userID uuid.UUID) error {
	return s.roles.GrantRole(ctx, userID, authz.Grant{Role: authz.RoleDriver, Scope: authz.ScopeGlobal})
}

// DriverIDForUser returns the user's driver id, or uuid.Nil if the user is
// not a driver (vehicle.DriverLookup).
func (s *Service) DriverIDForUser(ctx context.Context, userID uuid.UUID) (uuid.UUID, error) {
	d, err := s.repo.GetByUser(ctx, userID)
	if errors.Is(err, ErrNotFound) {
		return uuid.Nil, nil
	}
	if err != nil {
		return uuid.Nil, err
	}
	return d.ID, nil
}

// Profile is a driver with their vehicles and every related document.
type Profile struct {
	Driver    Driver
	Vehicles  []vehicle.Vehicle
	Documents []Document
}

// MyProfile returns the caller's driver profile.
func (s *Service) MyProfile(ctx context.Context, actor authz.Actor) (Profile, error) {
	d, err := s.repo.GetByUser(ctx, actor.UserID)
	if errors.Is(err, ErrNotFound) {
		return Profile{}, ErrDriverNotFound
	}
	if err != nil {
		return Profile{}, err
	}
	return s.profile(ctx, d)
}

// ProfileByID returns a driver profile to the driver or to operations staff.
func (s *Service) ProfileByID(ctx context.Context, actor authz.Actor, id uuid.UUID) (Profile, error) {
	d, err := s.repo.Get(ctx, id)
	if errors.Is(err, ErrNotFound) {
		return Profile{}, ErrDriverNotFound
	}
	if err != nil {
		return Profile{}, err
	}
	if d.UserID != actor.UserID && !actor.HasGlobalRole(authz.RoleSuperAdmin, authz.RoleDispatcher, authz.RoleSupportAgent) {
		return Profile{}, authz.ErrForbidden
	}
	return s.profile(ctx, d)
}

func (s *Service) profile(ctx context.Context, d Driver) (Profile, error) {
	vehicles, err := s.vehicles.OwnedByDriver(ctx, d.ID)
	if err != nil {
		return Profile{}, err
	}
	owners := []uuid.UUID{d.ID}
	for _, v := range vehicles {
		owners = append(owners, v.ID)
	}
	docs, err := s.repo.ListDocumentsByOwners(ctx, owners)
	if err != nil {
		return Profile{}, err
	}
	return Profile{Driver: d, Vehicles: vehicles, Documents: docs}, nil
}

// ---- document uploads ----

// UploadRequest asks for a pre-signed upload URL.
type UploadRequest struct {
	OwnerType   string
	OwnerID     uuid.UUID
	DocType     string
	ContentType string
}

// Upload is a pre-signed upload.
type Upload struct {
	URL       string
	FileKey   string
	ExpiresAt time.Time
	MaxBytes  int64
}

// DocumentInput records an uploaded file as a document.
type DocumentInput struct {
	OwnerType string
	OwnerID   uuid.UUID
	DocType   string
	FileKey   string
	DocNumber *string
	IssuedOn  *time.Time
	ExpiresOn *time.Time
}

func checkTypes(ownerType, docType string) error {
	if ownerType != OwnerDriver && ownerType != OwnerVehicle {
		return ErrInvalidOwnerType
	}
	if !docTypeAllowed(ownerType, docType) {
		return ErrInvalidDocType
	}
	return nil
}

// authorizeOwner allows drivers to manage their own documents and those of
// vehicles they own.
func (s *Service) authorizeOwner(ctx context.Context, actor authz.Actor, ownerType string, ownerID uuid.UUID) error {
	driverID, err := s.DriverIDForUser(ctx, actor.UserID)
	if err != nil {
		return err
	}
	if driverID == uuid.Nil {
		return ErrNotYourDocument
	}
	switch ownerType {
	case OwnerDriver:
		if ownerID != driverID {
			return ErrNotYourDocument
		}
	case OwnerVehicle:
		owner, err := s.vehicles.OwnerDriverID(ctx, ownerID)
		if errors.Is(err, vehicle.ErrNotFound) {
			return ErrVehicleNotFound
		}
		if err != nil {
			return err
		}
		if owner == nil || *owner != driverID {
			return ErrNotYourDocument
		}
	}
	return nil
}

func keyPrefix(ownerType string, ownerID uuid.UUID) string {
	return "documents/" + strings.ToLower(ownerType) + "/" + ownerID.String() + "/"
}

// RequestUpload returns a pre-signed URL for one document file.
func (s *Service) RequestUpload(ctx context.Context, actor authz.Actor, in UploadRequest) (Upload, error) {
	if err := checkTypes(in.OwnerType, in.DocType); err != nil {
		return Upload{}, err
	}
	ext, ok := contentTypes[in.ContentType]
	if !ok {
		return Upload{}, ErrUnsupportedFile
	}
	if err := s.authorizeOwner(ctx, actor, in.OwnerType, in.OwnerID); err != nil {
		return Upload{}, err
	}
	key := keyPrefix(in.OwnerType, in.OwnerID) + idgen.New().String() + ext
	url, err := s.files.PresignPut(ctx, key, uploadURLTTL)
	if err != nil {
		return Upload{}, err
	}
	return Upload{URL: url, FileKey: key, ExpiresAt: s.clock.Now().Add(uploadURLTTL), MaxBytes: MaxFileBytes}, nil
}

func (in DocumentInput) validateDates(today time.Time) error {
	if in.ExpiresOn == nil && slices.Contains(expiringDocTypes, in.DocType) {
		return ErrExpiryRequired
	}
	if in.ExpiresOn != nil && !in.ExpiresOn.After(today) {
		return ErrExpired
	}
	if in.IssuedOn != nil && (in.IssuedOn.After(today) || (in.ExpiresOn != nil && !in.IssuedOn.Before(*in.ExpiresOn))) {
		return ErrIssuedInFuture
	}
	return nil
}

// Submit records an uploaded file as a PENDING document after checking the
// file exists, is small enough and has an accepted type.
func (s *Service) Submit(ctx context.Context, actor authz.Actor, in DocumentInput) (Document, error) {
	if err := checkTypes(in.OwnerType, in.DocType); err != nil {
		return Document{}, err
	}
	if !strings.HasPrefix(in.FileKey, keyPrefix(in.OwnerType, in.OwnerID)) || strings.Contains(in.FileKey, "..") {
		return Document{}, ErrFileKeyMismatch
	}
	if err := in.validateDates(clock.Today(s.clock)); err != nil {
		return Document{}, err
	}
	if err := s.authorizeOwner(ctx, actor, in.OwnerType, in.OwnerID); err != nil {
		return Document{}, err
	}

	info, err := s.files.Stat(ctx, in.FileKey)
	if errors.Is(err, objstore.ErrNotFound) {
		return Document{}, ErrUploadMissing
	}
	if err != nil {
		return Document{}, err
	}
	if info.Size > MaxFileBytes {
		return Document{}, ErrFileTooLarge
	}
	contentType, _, _ := strings.Cut(info.ContentType, ";")
	if _, ok := contentTypes[strings.TrimSpace(contentType)]; !ok {
		return Document{}, ErrUnsupportedFile
	}
	sum, err := s.files.SHA256(ctx, in.FileKey)
	if err != nil {
		return Document{}, err
	}

	var docNumber *string
	if in.DocNumber != nil && strings.TrimSpace(*in.DocNumber) != "" {
		n := strings.TrimSpace(*in.DocNumber)
		docNumber = &n
	}
	now := s.clock.Now()
	doc := Document{
		ID:          idgen.New(),
		OwnerType:   in.OwnerType,
		OwnerID:     in.OwnerID,
		DocType:     in.DocType,
		FileKey:     in.FileKey,
		SHA256:      sum,
		ContentType: strings.TrimSpace(contentType),
		SizeBytes:   info.Size,
		DocNumber:   docNumber,
		IssuedOn:    in.IssuedOn,
		ExpiresOn:   in.ExpiresOn,
		Status:      StatusPending,
		CreatedAt:   now,
	}
	err = s.repo.WithTx(ctx, func(r Repo) error {
		if err := r.CreateDocument(ctx, doc); err != nil {
			if errors.Is(err, ErrDuplicateUpload) {
				return ErrUploadReused
			}
			return err
		}
		return r.AddEvent(ctx, outbox.Event{
			AggregateType: "document",
			AggregateID:   doc.ID,
			Subject:       "efoy.document.submitted",
			Data:          documentEvent(doc),
			OccurredAt:    now,
		})
	})
	if err != nil {
		return Document{}, err
	}
	return s.repo.GetDocument(ctx, doc.ID)
}

func documentEvent(d Document) map[string]any {
	return map[string]any{
		"document_id": d.ID,
		"owner_type":  d.OwnerType,
		"owner_id":    d.OwnerID,
		"doc_type":    d.DocType,
		"status":      d.Status,
	}
}

// ---- admin review ----

// Page is one page of the review queue.
type Page struct {
	Items      []Document
	NextCursor string
}

const (
	defaultPageSize = 50
	maxPageSize     = 100
)

func encodeCursor(c Cursor) string {
	return base64.RawURLEncoding.EncodeToString([]byte(strconv.FormatInt(c.CreatedAt.UnixNano(), 10) + ":" + c.ID.String()))
}

func decodeCursor(s string) (Cursor, error) {
	if s == "" {
		return Cursor{}, nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return Cursor{}, ErrInvalidCursor
	}
	ts, id, ok := strings.Cut(string(raw), ":")
	nanos, err1 := strconv.ParseInt(ts, 10, 64)
	uid, err2 := uuid.Parse(id)
	if !ok || err1 != nil || err2 != nil {
		return Cursor{}, ErrInvalidCursor
	}
	return Cursor{CreatedAt: time.Unix(0, nanos).UTC(), ID: uid}, nil
}

// ReviewQueue lists documents with the given statuses (default: waiting for
// review), oldest first.
func (s *Service) ReviewQueue(ctx context.Context, actor authz.Actor, statuses []string, limit int, cursor string) (Page, error) {
	if err := authz.CanReviewDocuments(actor); err != nil {
		return Page{}, err
	}
	if len(statuses) == 0 {
		statuses = []string{StatusPending, StatusUnderReview}
	}
	for _, st := range statuses {
		if !slices.Contains(documentStatuses, st) {
			return Page{}, ErrInvalidStatus
		}
	}
	if limit <= 0 {
		limit = defaultPageSize
	}
	limit = min(limit, maxPageSize)
	after, err := decodeCursor(cursor)
	if err != nil {
		return Page{}, err
	}

	docs, err := s.repo.ListDocumentsByStatus(ctx, statuses, after, limit+1)
	if err != nil {
		return Page{}, err
	}
	page := Page{Items: docs}
	if len(docs) > limit {
		page.Items = docs[:limit]
		last := page.Items[limit-1]
		page.NextCursor = encodeCursor(Cursor{CreatedAt: last.CreatedAt, ID: last.ID})
	}
	return page, nil
}

// DocumentForReview returns a document with a short-lived URL to its file.
func (s *Service) DocumentForReview(ctx context.Context, actor authz.Actor, id uuid.UUID) (Document, error) {
	if err := authz.CanReviewDocuments(actor); err != nil {
		return Document{}, err
	}
	doc, err := s.repo.GetDocument(ctx, id)
	if errors.Is(err, ErrNotFound) {
		return Document{}, ErrDocumentNotFound
	}
	if err != nil {
		return Document{}, err
	}
	return s.withViewURL(ctx, doc)
}

func (s *Service) withViewURL(ctx context.Context, doc Document) (Document, error) {
	url, err := s.files.PresignGet(ctx, doc.FileKey, viewURLTTL)
	if err != nil {
		return Document{}, err
	}
	doc.ViewURL = &url
	return doc, nil
}

// Review approves or rejects a document (FR-DRV-3). Rejections need a reason,
// which is shown to the driver.
func (s *Service) Review(ctx context.Context, actor authz.Actor, id uuid.UUID, decision, reason string) (Document, error) {
	if err := authz.CanReviewDocuments(actor); err != nil {
		return Document{}, err
	}
	var status string
	switch decision {
	case "APPROVE":
		status = StatusApproved
	case "REJECT":
		status = StatusRejected
	default:
		return Document{}, ErrInvalidDecision
	}
	reason = strings.TrimSpace(reason)
	if status == StatusRejected && reason == "" {
		return Document{}, ErrRejectNeedsReason
	}
	if len(reason) > 500 {
		reason = reason[:500]
	}
	var rejection *string
	if status == StatusRejected {
		rejection = &reason
	}

	now := s.clock.Now()
	err := s.repo.WithTx(ctx, func(r Repo) error {
		doc, err := r.GetDocumentForUpdate(ctx, id)
		if errors.Is(err, ErrNotFound) {
			return ErrDocumentNotFound
		}
		if err != nil {
			return err
		}
		if doc.Status != StatusPending && doc.Status != StatusUnderReview {
			return ErrAlreadyReviewed
		}
		if err := r.SetDocumentReview(ctx, id, status, actor.UserID, now, rejection); err != nil {
			return err
		}
		doc.Status = status
		event := documentEvent(doc)
		event["reviewed_by"] = actor.UserID
		if rejection != nil {
			event["rejection_reason"] = *rejection
		}
		return r.AddEvent(ctx, outbox.Event{
			AggregateType: "document",
			AggregateID:   doc.ID,
			Subject:       "efoy.document.reviewed",
			Data:          event,
			OccurredAt:    now,
		})
	})
	if err != nil {
		return Document{}, err
	}
	doc, err := s.repo.GetDocument(ctx, id)
	if err != nil {
		return Document{}, err
	}
	return s.withViewURL(ctx, doc)
}
