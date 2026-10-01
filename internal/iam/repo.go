package iam

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/blinge12/efoy/pkg/authz"
)

// Repo is the persistence port of the iam domain. Lookups return ErrNotFound
// when nothing matches; inserts that violate a unique key return ErrConflict.
type Repo interface {
	// WithTx runs fn in one database transaction.
	WithTx(ctx context.Context, fn func(Repo) error) error

	GetUserByID(ctx context.Context, id uuid.UUID) (User, error)
	GetUserByPhone(ctx context.Context, phone string) (User, error)
	GetUserByEmail(ctx context.Context, email string) (User, error)
	// EnsureUserByPhone inserts u unless a user with u.Phone exists, and
	// returns the stored user either way.
	EnsureUserByPhone(ctx context.Context, u User) (User, error)
	CreateUser(ctx context.Context, u User) error
	TouchLastLogin(ctx context.Context, userID uuid.UUID, at time.Time) error
	UpdateProfile(ctx context.Context, userID uuid.UUID, fullName string, fullNameAm *string, language string) error

	ListGrants(ctx context.Context, userID uuid.UUID) ([]authz.Grant, error)
	GrantRole(ctx context.Context, id, userID uuid.UUID, g authz.Grant, grantedBy *uuid.UUID) error
	ListLinkedRiderIDs(ctx context.Context, userID uuid.UUID) ([]uuid.UUID, error)

	// UpsertDevice stores d, or refreshes the existing row with the same
	// push token, and returns the device id.
	UpsertDevice(ctx context.Context, d Device) (uuid.UUID, error)

	CreateSession(ctx context.Context, s Session) error
	// GetSessionByHashForUpdate locks the session row until the transaction ends.
	GetSessionByHashForUpdate(ctx context.Context, hash []byte) (Session, error)
	RevokeSession(ctx context.Context, id uuid.UUID, at time.Time, reason RevokeReason) error
	RevokeFamily(ctx context.Context, familyID uuid.UUID, at time.Time, reason RevokeReason) error
	// RevokeFamilyOfSession revokes every session in the family of sessionID,
	// provided that session belongs to userID.
	RevokeFamilyOfSession(ctx context.Context, userID, sessionID uuid.UUID, at time.Time, reason RevokeReason) error
	HasActiveSessionInFamily(ctx context.Context, familyID uuid.UUID, now time.Time) (bool, error)
}
