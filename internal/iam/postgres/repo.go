// Package postgres implements iam.Repo with pgx.
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

	"github.com/blinge12/efoy/internal/iam"
	"github.com/blinge12/efoy/pkg/authz"
)

// querier is satisfied by both *pgxpool.Pool and pgx.Tx.
type querier interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// Repo is the Postgres-backed iam repository.
type Repo struct {
	pool *pgxpool.Pool
	q    querier
	inTx bool
}

func New(pool *pgxpool.Pool) *Repo { return &Repo{pool: pool, q: pool} }

var _ iam.Repo = (*Repo)(nil)

func (r *Repo) WithTx(ctx context.Context, fn func(iam.Repo) error) error {
	if r.inTx {
		return fn(r)
	}
	return pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		return fn(&Repo{pool: r.pool, q: tx, inTx: true})
	})
}

const uniqueViolation = "23505"

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == uniqueViolation
}

// ---- users ----

const userColumns = `id, phone_e164, email::text, password_hash, totp_secret_enc, full_name,
	full_name_am, preferred_language::text, photo_key, status::text, created_at`

func scanUser(row pgx.Row) (iam.User, error) {
	var u iam.User
	var status string
	err := row.Scan(&u.ID, &u.Phone, &u.Email, &u.PasswordHash, &u.TOTPSecretEnc, &u.FullName,
		&u.FullNameAm, &u.PreferredLanguage, &u.PhotoKey, &status, &u.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return iam.User{}, iam.ErrNotFound
	}
	if err != nil {
		return iam.User{}, fmt.Errorf("iam/postgres: scan user: %w", err)
	}
	u.Status = iam.UserStatus(status)
	return u, nil
}

func (r *Repo) GetUserByID(ctx context.Context, id uuid.UUID) (iam.User, error) {
	return scanUser(r.q.QueryRow(ctx, `SELECT `+userColumns+` FROM users WHERE id = $1`, id))
}

func (r *Repo) GetUserByPhone(ctx context.Context, phone string) (iam.User, error) {
	return scanUser(r.q.QueryRow(ctx, `SELECT `+userColumns+` FROM users WHERE phone_e164 = $1`, phone))
}

func (r *Repo) GetUserByEmail(ctx context.Context, email string) (iam.User, error) {
	return scanUser(r.q.QueryRow(ctx, `SELECT `+userColumns+` FROM users WHERE email = $1::text::citext`, email))
}

func (r *Repo) EnsureUserByPhone(ctx context.Context, u iam.User) (iam.User, error) {
	if u.Phone == nil {
		return iam.User{}, errors.New("iam/postgres: ensure user: phone is required")
	}
	_, err := r.q.Exec(ctx, `
		INSERT INTO users (id, phone_e164, full_name, preferred_language, status)
		VALUES ($1, $2, $3, $4::text::app_language, $5::text::user_status)
		ON CONFLICT (phone_e164) DO NOTHING`,
		u.ID, u.Phone, u.FullName, u.PreferredLanguage, string(u.Status))
	if err != nil {
		return iam.User{}, fmt.Errorf("iam/postgres: ensure user: %w", err)
	}
	return r.GetUserByPhone(ctx, *u.Phone)
}

func (r *Repo) CreateUser(ctx context.Context, u iam.User) error {
	_, err := r.q.Exec(ctx, `
		INSERT INTO users (id, phone_e164, email, password_hash, totp_secret_enc, full_name,
		                   full_name_am, preferred_language, photo_key, status)
		VALUES ($1, $2, $3::text::citext, $4, $5, $6, $7, $8::text::app_language, $9, $10::text::user_status)`,
		u.ID, u.Phone, u.Email, u.PasswordHash, u.TOTPSecretEnc, u.FullName,
		u.FullNameAm, u.PreferredLanguage, u.PhotoKey, string(u.Status))
	if isUniqueViolation(err) {
		return iam.ErrConflict
	}
	if err != nil {
		return fmt.Errorf("iam/postgres: create user: %w", err)
	}
	return nil
}

func (r *Repo) TouchLastLogin(ctx context.Context, userID uuid.UUID, at time.Time) error {
	_, err := r.q.Exec(ctx, `UPDATE users SET last_login_at = $2 WHERE id = $1`, userID, at)
	if err != nil {
		return fmt.Errorf("iam/postgres: touch last login: %w", err)
	}
	return nil
}

func (r *Repo) UpdateProfile(ctx context.Context, userID uuid.UUID, fullName string, fullNameAm *string, language string) error {
	_, err := r.q.Exec(ctx, `
		UPDATE users
		   SET full_name = $2, full_name_am = $3, preferred_language = $4::text::app_language
		 WHERE id = $1`, userID, fullName, fullNameAm, language)
	if err != nil {
		return fmt.Errorf("iam/postgres: update profile: %w", err)
	}
	return nil
}

// ---- roles and links ----

func (r *Repo) ListGrants(ctx context.Context, userID uuid.UUID) ([]authz.Grant, error) {
	rows, err := r.q.Query(ctx, `
		SELECT role::text, scope::text, scope_id FROM user_roles
		 WHERE user_id = $1
		 ORDER BY role, scope, scope_id`, userID)
	if err != nil {
		return nil, fmt.Errorf("iam/postgres: list grants: %w", err)
	}
	defer rows.Close()

	grants := []authz.Grant{}
	for rows.Next() {
		var role, scope string
		var scopeID *uuid.UUID
		if err := rows.Scan(&role, &scope, &scopeID); err != nil {
			return nil, fmt.Errorf("iam/postgres: scan grant: %w", err)
		}
		g := authz.Grant{Role: authz.Role(role), Scope: authz.ScopeType(scope)}
		if scopeID != nil {
			g.ScopeID = *scopeID
		}
		grants = append(grants, g)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iam/postgres: list grants: %w", err)
	}
	return grants, nil
}

func (r *Repo) GrantRole(ctx context.Context, id, userID uuid.UUID, g authz.Grant, grantedBy *uuid.UUID) error {
	var scopeID *uuid.UUID
	if g.Scope != authz.ScopeGlobal {
		scopeID = &g.ScopeID
	}
	_, err := r.q.Exec(ctx, `
		INSERT INTO user_roles (id, user_id, role, scope, scope_id, granted_by)
		VALUES ($1, $2, $3::text::role_name, $4::text::scope_type, $5, $6)`,
		id, userID, string(g.Role), string(g.Scope), scopeID, grantedBy)
	if isUniqueViolation(err) {
		return iam.ErrConflict
	}
	if err != nil {
		return fmt.Errorf("iam/postgres: grant role: %w", err)
	}
	return nil
}

func (r *Repo) ListLinkedRiderIDs(ctx context.Context, userID uuid.UUID) ([]uuid.UUID, error) {
	rows, err := r.q.Query(ctx, `
		SELECT rider_id FROM guardian_links WHERE guardian_user_id = $1
		UNION
		SELECT id FROM riders WHERE user_id = $1
		ORDER BY 1`, userID)
	if err != nil {
		return nil, fmt.Errorf("iam/postgres: linked riders: %w", err)
	}
	defer rows.Close()

	ids := []uuid.UUID{}
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("iam/postgres: scan rider id: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iam/postgres: linked riders: %w", err)
	}
	return ids, nil
}

// ---- devices ----

func (r *Repo) UpsertDevice(ctx context.Context, d iam.Device) (uuid.UUID, error) {
	var id uuid.UUID
	err := r.q.QueryRow(ctx, `
		INSERT INTO devices (id, user_id, platform, push_token, app_version, os_version)
		VALUES ($1, $2, $3::text::device_platform, $4, $5, $6)
		ON CONFLICT (user_id, push_token) DO UPDATE
		   SET app_version = EXCLUDED.app_version,
		       os_version = EXCLUDED.os_version,
		       last_seen_at = now()
		RETURNING id`,
		d.ID, d.UserID, d.Platform, d.PushToken, d.AppVersion, d.OSVersion).Scan(&id)
	if err != nil {
		return uuid.Nil, fmt.Errorf("iam/postgres: upsert device: %w", err)
	}
	return id, nil
}

// ---- sessions ----

func (r *Repo) CreateSession(ctx context.Context, s iam.Session) error {
	_, err := r.q.Exec(ctx, `
		INSERT INTO sessions (id, user_id, device_id, refresh_token_hash, family_id, ip, user_agent, expires_at, sliding)
		VALUES ($1, $2, $3, $4, $5, NULLIF($6::text, '')::inet, NULLIF($7::text, ''), $8, $9)`,
		s.ID, s.UserID, s.DeviceID, s.TokenHash, s.FamilyID, s.IP, s.UserAgent, s.ExpiresAt, s.Sliding)
	if err != nil {
		return fmt.Errorf("iam/postgres: create session: %w", err)
	}
	return nil
}

func (r *Repo) GetSessionByHashForUpdate(ctx context.Context, hash []byte) (iam.Session, error) {
	var s iam.Session
	var reason *string
	err := r.q.QueryRow(ctx, `
		SELECT id, user_id, device_id, refresh_token_hash, family_id, expires_at, revoked_at, revoked_reason, sliding
		  FROM sessions
		 WHERE refresh_token_hash = $1
		   FOR UPDATE`, hash).
		Scan(&s.ID, &s.UserID, &s.DeviceID, &s.TokenHash, &s.FamilyID, &s.ExpiresAt, &s.RevokedAt, &reason, &s.Sliding)
	if errors.Is(err, pgx.ErrNoRows) {
		return iam.Session{}, iam.ErrNotFound
	}
	if err != nil {
		return iam.Session{}, fmt.Errorf("iam/postgres: get session: %w", err)
	}
	if reason != nil {
		rr := iam.RevokeReason(*reason)
		s.RevokedReason = &rr
	}
	return s, nil
}

func (r *Repo) RevokeSession(ctx context.Context, id uuid.UUID, at time.Time, reason iam.RevokeReason) error {
	_, err := r.q.Exec(ctx, `
		UPDATE sessions SET revoked_at = $2, revoked_reason = $3
		 WHERE id = $1 AND revoked_at IS NULL`, id, at, string(reason))
	if err != nil {
		return fmt.Errorf("iam/postgres: revoke session: %w", err)
	}
	return nil
}

func (r *Repo) RevokeFamily(ctx context.Context, familyID uuid.UUID, at time.Time, reason iam.RevokeReason) error {
	_, err := r.q.Exec(ctx, `
		UPDATE sessions SET revoked_at = $2, revoked_reason = $3
		 WHERE family_id = $1 AND revoked_at IS NULL`, familyID, at, string(reason))
	if err != nil {
		return fmt.Errorf("iam/postgres: revoke family: %w", err)
	}
	return nil
}

func (r *Repo) RevokeFamilyOfSession(ctx context.Context, userID, sessionID uuid.UUID, at time.Time, reason iam.RevokeReason) error {
	_, err := r.q.Exec(ctx, `
		UPDATE sessions SET revoked_at = $3, revoked_reason = $4
		 WHERE revoked_at IS NULL
		   AND family_id = (SELECT family_id FROM sessions WHERE id = $1 AND user_id = $2)`,
		sessionID, userID, at, string(reason))
	if err != nil {
		return fmt.Errorf("iam/postgres: revoke session family: %w", err)
	}
	return nil
}

func (r *Repo) HasActiveSessionInFamily(ctx context.Context, familyID uuid.UUID, now time.Time) (bool, error) {
	var ok bool
	err := r.q.QueryRow(ctx, `
		SELECT EXISTS (
		  SELECT 1 FROM sessions
		   WHERE family_id = $1 AND revoked_at IS NULL AND expires_at > $2)`, familyID, now).Scan(&ok)
	if err != nil {
		return false, fmt.Errorf("iam/postgres: active family session: %w", err)
	}
	return ok, nil
}
