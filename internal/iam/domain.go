// Package iam handles identity and access: phone OTP login for the apps,
// email + password + TOTP login for the web portals, access tokens, rotating
// refresh tokens and the current user's profile (FR-IAM-1 to FR-IAM-4).
package iam

import (
	"errors"
	"regexp"
	"time"

	"github.com/google/uuid"

	"github.com/blinge12/efoy/pkg/authz"
	"github.com/blinge12/efoy/pkg/errs"
)

type UserStatus string

const (
	UserActive    UserStatus = "ACTIVE"
	UserSuspended UserStatus = "SUSPENDED"
	UserDeleted   UserStatus = "DELETED"
)

// User is a row of users. Secrets never leave this package.
type User struct {
	ID                uuid.UUID
	Phone             *string
	Email             *string
	PasswordHash      *string
	TOTPSecretEnc     []byte
	FullName          string
	FullNameAm        *string
	PreferredLanguage string
	PhotoKey          *string
	Status            UserStatus
	CreatedAt         time.Time
}

// Device is a phone or browser the user signed in from.
type Device struct {
	ID         uuid.UUID
	UserID     uuid.UUID
	Platform   string // ANDROID | IOS | WEB
	PushToken  *string
	AppVersion *string
	OSVersion  *string
}

type RevokeReason string

const (
	RevokeRotated       RevokeReason = "ROTATED"
	RevokeLogout        RevokeReason = "LOGOUT"
	RevokeReuseDetected RevokeReason = "REUSE_DETECTED"
)

// Session is one refresh token. Rotation revokes it and creates a successor in
// the same family; presenting a revoked token revokes the whole family.
type Session struct {
	ID            uuid.UUID
	UserID        uuid.UUID
	DeviceID      *uuid.UUID
	TokenHash     []byte
	FamilyID      uuid.UUID
	IP            string
	UserAgent     string
	ExpiresAt     time.Time
	RevokedAt     *time.Time
	RevokedReason *RevokeReason
	Sliding       bool // mobile sessions slide; staff portal sessions are absolute
}

// Profile is what GET /me returns.
type Profile struct {
	User           User
	Grants         []authz.Grant
	LinkedRiderIDs []uuid.UUID
}

// TokenPair is the result of every successful login or refresh.
type TokenPair struct {
	AccessToken  string
	ExpiresIn    time.Duration
	RefreshToken string
	Profile      Profile
}

// ClientMeta describes the caller for session records.
type ClientMeta struct {
	IP        string
	UserAgent string
}

// DeviceInput is the optional device sent with OTP verification.
type DeviceInput struct {
	Platform   string
	PushToken  *string
	AppVersion *string
	OSVersion  *string
}

// OTPRequested tells the app how long the code lives and when to offer resend.
type OTPRequested struct {
	ExpiresIn   time.Duration
	ResendAfter time.Duration
}

var (
	phoneRE    = regexp.MustCompile(`^\+[1-9][0-9]{7,14}$`)
	otpCodeRE  = regexp.MustCompile(`^[0-9]{6}$`)
	totpCodeRE = regexp.MustCompile(`^[0-9]{6}$`)
)

// Repository sentinel errors.
var (
	ErrNotFound = errors.New("iam: not found")
	ErrConflict = errors.New("iam: already exists")
)

// Errors returned to clients (codes are part of the API contract).
var (
	ErrInvalidPhone       = errs.Invalid("INVALID_PHONE", "phone_e164 must be in E.164 format, e.g. +251911234567.")
	ErrInvalidOTPFormat   = errs.Invalid("INVALID_CODE", "code must be 6 digits.")
	ErrInvalidDevice      = errs.Invalid("INVALID_DEVICE", "device.platform must be ANDROID, IOS or WEB.")
	ErrMissingCredentials = errs.Invalid("MISSING_CREDENTIALS", "email and password are required.")
	ErrOTPInvalid         = errs.Unauthorized("OTP_INVALID", "The code is incorrect or has expired.")
	ErrOTPAttempts        = errs.Unauthorized("OTP_TOO_MANY_ATTEMPTS", "Too many incorrect codes. Request a new code.")
	ErrInvalidCredentials = errs.Unauthorized("INVALID_CREDENTIALS", "Email or password is incorrect.")
	ErrTOTPRequired       = errs.Unauthorized("TOTP_REQUIRED", "Enter the 6-digit code from your authenticator app.")
	ErrTOTPInvalid        = errs.Unauthorized("TOTP_INVALID", "The authenticator code is incorrect.")
	ErrTOTPNotEnrolled    = errs.Forbidden("TOTP_NOT_ENROLLED", "This account must set up two-factor authentication. Contact an administrator.")
	ErrAccountDisabled    = errs.Forbidden("ACCOUNT_DISABLED", "This account is suspended. Contact support.")
	ErrRefreshInvalid     = errs.Unauthorized("REFRESH_TOKEN_INVALID", "Your session has ended. Sign in again.")
	ErrRefreshReused      = errs.Unauthorized("REFRESH_TOKEN_REUSED", "Your session was ended for your security. Sign in again.")
	ErrEmailTaken         = errs.Conflict("EMAIL_TAKEN", "A user with this email already exists.")
)
