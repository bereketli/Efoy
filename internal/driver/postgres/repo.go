// Package postgres implements driver.Repo with pgx.
package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/blinge12/efoy/internal/driver"
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

var _ driver.Repo = (*Repo)(nil)

func (r *Repo) WithTx(ctx context.Context, fn func(driver.Repo) error) error {
	if r.inTx {
		return fn(r)
	}
	return pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		return fn(&Repo{pool: r.pool, q: tx, inTx: true})
	})
}

func uniqueConstraint(err error) (string, bool) {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return pgErr.ConstraintName, true
	}
	return "", false
}

// ---- drivers ----

const driverSelect = `
	SELECT d.id, d.user_id, u.full_name, u.phone_e164, d.source::text, d.licence_number, d.licence_category,
	       d.licence_expires_on, d.emergency_contact_name, d.emergency_contact_phone, d.status::text,
	       d.student_certified, d.created_at
	  FROM drivers d
	  JOIN users u ON u.id = d.user_id`

func scanDriver(row pgx.Row) (driver.Driver, error) {
	var d driver.Driver
	err := row.Scan(&d.ID, &d.UserID, &d.FullName, &d.Phone, &d.Source, &d.LicenceNumber, &d.LicenceCategory,
		&d.LicenceExpiresOn, &d.EmergencyContactName, &d.EmergencyContactPhone, &d.Status,
		&d.StudentCertified, &d.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return driver.Driver{}, driver.ErrNotFound
	}
	if err != nil {
		return driver.Driver{}, fmt.Errorf("driver/postgres: scan driver: %w", err)
	}
	return d, nil
}

func (r *Repo) Create(ctx context.Context, d driver.Driver) error {
	_, err := r.q.Exec(ctx, `
		INSERT INTO drivers (id, user_id, source, licence_number, licence_category, licence_expires_on,
		                     emergency_contact_name, emergency_contact_phone, status, created_at)
		VALUES ($1, $2, $3::text::driver_source, $4, $5, $6, $7, $8, $9::text::onboarding_status, $10)`,
		d.ID, d.UserID, d.Source, d.LicenceNumber, d.LicenceCategory, d.LicenceExpiresOn,
		d.EmergencyContactName, d.EmergencyContactPhone, d.Status, d.CreatedAt)
	if name, ok := uniqueConstraint(err); ok {
		if name == "drivers_user_id_key" {
			return driver.ErrDuplicateUser
		}
		return driver.ErrDuplicate
	}
	if err != nil {
		return fmt.Errorf("driver/postgres: create driver: %w", err)
	}
	return nil
}

func (r *Repo) Get(ctx context.Context, id uuid.UUID) (driver.Driver, error) {
	return scanDriver(r.q.QueryRow(ctx, driverSelect+` WHERE d.id = $1`, id))
}

func (r *Repo) GetByUser(ctx context.Context, userID uuid.UUID) (driver.Driver, error) {
	return scanDriver(r.q.QueryRow(ctx, driverSelect+` WHERE d.user_id = $1`, userID))
}

// ---- documents ----

// owner_label is the driver's name or the vehicle's plate.
const documentSelect = `
	SELECT doc.id, doc.owner_type::text, doc.owner_id, COALESCE(NULLIF(u.full_name, ''), v.plate_number, ''),
	       doc.doc_type::text, doc.file_key, doc.sha256, doc.content_type, COALESCE(doc.size_bytes, 0),
	       doc.doc_number, doc.issued_on, doc.expires_on, doc.status::text, doc.reviewed_by,
	       doc.reviewed_at, doc.rejection_reason, doc.created_at
	  FROM documents doc
	  LEFT JOIN drivers dr ON doc.owner_type = 'DRIVER' AND dr.id = doc.owner_id
	  LEFT JOIN users u ON u.id = dr.user_id
	  LEFT JOIN vehicles v ON doc.owner_type = 'VEHICLE' AND v.id = doc.owner_id`

func scanDocument(row pgx.Row) (driver.Document, error) {
	var d driver.Document
	err := row.Scan(&d.ID, &d.OwnerType, &d.OwnerID, &d.OwnerLabel,
		&d.DocType, &d.FileKey, &d.SHA256, &d.ContentType, &d.SizeBytes,
		&d.DocNumber, &d.IssuedOn, &d.ExpiresOn, &d.Status, &d.ReviewedBy,
		&d.ReviewedAt, &d.RejectionReason, &d.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return driver.Document{}, driver.ErrNotFound
	}
	if err != nil {
		return driver.Document{}, fmt.Errorf("driver/postgres: scan document: %w", err)
	}
	return d, nil
}

func collectDocuments(rows pgx.Rows, err error) ([]driver.Document, error) {
	if err != nil {
		return nil, fmt.Errorf("driver/postgres: list documents: %w", err)
	}
	docs, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (driver.Document, error) { return scanDocument(row) })
	if err != nil {
		return nil, fmt.Errorf("driver/postgres: list documents: %w", err)
	}
	if docs == nil {
		docs = []driver.Document{}
	}
	return docs, nil
}

func (r *Repo) CreateDocument(ctx context.Context, d driver.Document) error {
	_, err := r.q.Exec(ctx, `
		INSERT INTO documents (id, owner_type, owner_id, doc_type, file_key, sha256, content_type, size_bytes,
		                       doc_number, issued_on, expires_on, status, created_at)
		VALUES ($1, $2::text::doc_owner_type, $3, $4::text::doc_type, $5, $6, $7, $8,
		        $9, $10, $11, $12::text::doc_status, $13)`,
		d.ID, d.OwnerType, d.OwnerID, d.DocType, d.FileKey, d.SHA256, d.ContentType, d.SizeBytes,
		d.DocNumber, d.IssuedOn, d.ExpiresOn, d.Status, d.CreatedAt)
	if _, ok := uniqueConstraint(err); ok {
		return driver.ErrDuplicateUpload
	}
	if err != nil {
		return fmt.Errorf("driver/postgres: create document: %w", err)
	}
	return nil
}

func (r *Repo) GetDocument(ctx context.Context, id uuid.UUID) (driver.Document, error) {
	return scanDocument(r.q.QueryRow(ctx, documentSelect+` WHERE doc.id = $1`, id))
}

func (r *Repo) GetDocumentForUpdate(ctx context.Context, id uuid.UUID) (driver.Document, error) {
	return scanDocument(r.q.QueryRow(ctx, documentSelect+` WHERE doc.id = $1 FOR UPDATE OF doc`, id))
}

func (r *Repo) SetDocumentReview(ctx context.Context, id uuid.UUID, status string, reviewer uuid.UUID, at time.Time, reason *string) error {
	_, err := r.q.Exec(ctx, `
		UPDATE documents
		   SET status = $2::text::doc_status, reviewed_by = $3, reviewed_at = $4, rejection_reason = $5
		 WHERE id = $1`, id, status, reviewer, at, reason)
	if err != nil {
		return fmt.Errorf("driver/postgres: review document: %w", err)
	}
	return nil
}

func (r *Repo) ListDocumentsByStatus(ctx context.Context, statuses []string, after driver.Cursor, limit int) ([]driver.Document, error) {
	rows, err := r.q.Query(ctx, documentSelect+`
		 WHERE doc.status::text = ANY($1::text[])
		   AND (doc.created_at, doc.id) > ($2, $3)
		 ORDER BY doc.created_at, doc.id
		 LIMIT $4`, statuses, after.CreatedAt, after.ID, limit)
	return collectDocuments(rows, err)
}

func (r *Repo) ListDocumentsByOwners(ctx context.Context, ownerIDs []uuid.UUID) ([]driver.Document, error) {
	ids := make([]string, 0, len(ownerIDs))
	for _, id := range ownerIDs {
		ids = append(ids, id.String())
	}
	rows, err := r.q.Query(ctx, documentSelect+`
		 WHERE doc.owner_id = ANY($1::uuid[])
		 ORDER BY doc.created_at DESC, doc.id DESC`, ids)
	return collectDocuments(rows, err)
}

func (r *Repo) AddEvent(ctx context.Context, e outbox.Event) error {
	return outbox.Add(ctx, r.q, e)
}
