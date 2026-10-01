package iam

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/blinge12/efoy/internal/config"
	"github.com/blinge12/efoy/pkg/authz"
	"github.com/blinge12/efoy/pkg/clock"
	"github.com/blinge12/efoy/pkg/errs"
	"github.com/blinge12/efoy/pkg/idgen"
)

// refreshReuseGrace tolerates two requests refreshing with the same token at
// almost the same time (a browser loading a page and its data in parallel).
// Reuse after this window is treated as token theft.
const refreshReuseGrace = 30 * time.Second

// Staff login attempts per email per window (on top of OTP and IP limits).
const (
	loginRateLimit  = 10
	loginRateWindow = 15 * time.Minute
)

// SMSSender delivers text messages. notification.ConsoleSMS is the dev fake.
type SMSSender interface {
	SendSMS(ctx context.Context, to, body string) error
}

// Service implements the iam use cases.
type Service struct {
	repo    Repo
	otp     *OTPStore
	limiter *Limiter
	tokens  *Tokens
	totp    *SecretCipher
	sms     SMSSender
	clock   clock.Clock
	cfg     config.Auth
	log     *slog.Logger
}

// Deps are the Service dependencies.
type Deps struct {
	Repo    Repo
	OTP     *OTPStore
	Limiter *Limiter
	Tokens  *Tokens
	TOTP    *SecretCipher
	SMS     SMSSender
	Clock   clock.Clock
	Config  config.Auth
	Log     *slog.Logger
}

func NewService(d Deps) *Service {
	return &Service{
		repo:    d.Repo,
		otp:     d.OTP,
		limiter: d.Limiter,
		tokens:  d.Tokens,
		totp:    d.TOTP,
		sms:     d.SMS,
		clock:   d.Clock,
		cfg:     d.Config,
		log:     d.Log,
	}
}

// RequestOTP sends a login code by SMS (FR-IAM-1).
func (s *Service) RequestOTP(ctx context.Context, phone, lang string) (OTPRequested, error) {
	if !phoneRE.MatchString(phone) {
		return OTPRequested{}, ErrInvalidPhone
	}
	ok, retry, err := s.limiter.Allow(ctx, "otp", phone, s.cfg.OTP.RateLimit, s.cfg.OTP.RateWindow)
	if err != nil {
		return OTPRequested{}, err
	}
	if !ok {
		return OTPRequested{}, errs.TooManyRequests("OTP_RATE_LIMITED", "Too many codes requested. Try again later.", retry)
	}
	code, err := s.otp.Issue(ctx, phone)
	if err != nil {
		return OTPRequested{}, err
	}
	if err := s.sms.SendSMS(ctx, phone, otpMessage(lang, code, s.cfg.OTP.TTL)); err != nil {
		_ = s.otp.Discard(ctx, phone)
		return OTPRequested{}, fmt.Errorf("iam: send otp sms: %w", err)
	}
	return OTPRequested{ExpiresIn: s.cfg.OTP.TTL, ResendAfter: s.cfg.OTP.ResendAfter}, nil
}

// otpMessage renders the OTP SMS. These strings move to the
// notification_templates table with the notification worker (day 5).
func otpMessage(lang, code string, ttl time.Duration) string {
	minutes := int(ttl.Minutes())
	switch lang {
	case "en":
		return fmt.Sprintf("Your Efoy code is %s. It expires in %d minutes.", code, minutes)
	case "om":
		return fmt.Sprintf("Koodiin Efoy keessan %s. Daqiiqaa %d booda ni dhumata.", code, minutes)
	default:
		return fmt.Sprintf("የእፋይ ማረጋገጫ ኮድዎ %s ነው። በ%d ደቂቃ ውስጥ ጊዜው ያልፋል።", code, minutes)
	}
}

// VerifyOTP checks the code and signs the user in, creating the account on
// first login (FR-IAM-1).
func (s *Service) VerifyOTP(ctx context.Context, phone, code string, device *DeviceInput, meta ClientMeta) (TokenPair, error) {
	if !phoneRE.MatchString(phone) {
		return TokenPair{}, ErrInvalidPhone
	}
	if !otpCodeRE.MatchString(code) {
		return TokenPair{}, ErrInvalidOTPFormat
	}
	if device != nil && !validPlatform(device.Platform) {
		return TokenPair{}, ErrInvalidDevice
	}
	if err := s.otp.Verify(ctx, phone, code); err != nil {
		return TokenPair{}, err
	}

	var pair TokenPair
	err := s.repo.WithTx(ctx, func(r Repo) error {
		user, err := r.EnsureUserByPhone(ctx, User{
			ID:                idgen.New(),
			Phone:             &phone,
			PreferredLanguage: "am",
			Status:            UserActive,
		})
		if err != nil {
			return err
		}
		if user.Status != UserActive {
			return ErrAccountDisabled
		}
		var deviceID *uuid.UUID
		if device != nil {
			id, err := r.UpsertDevice(ctx, Device{
				ID:         idgen.New(),
				UserID:     user.ID,
				Platform:   device.Platform,
				PushToken:  device.PushToken,
				AppVersion: device.AppVersion,
				OSVersion:  device.OSVersion,
			})
			if err != nil {
				return err
			}
			deviceID = &id
		}
		pair, err = s.startSession(ctx, r, user, deviceID, true, s.cfg.RefreshTTL, meta)
		return err
	})
	return pair, err
}

func validPlatform(p string) bool {
	return p == "ANDROID" || p == "IOS" || p == "WEB"
}

// Login signs a staff or portal user in with email, password and, when
// enrolled or required by role, a TOTP code (FR-IAM-2).
func (s *Service) Login(ctx context.Context, email, password, totpCode string, meta ClientMeta) (TokenPair, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" || password == "" {
		return TokenPair{}, ErrMissingCredentials
	}
	ok, retry, err := s.limiter.Allow(ctx, "login", email, loginRateLimit, loginRateWindow)
	if err != nil {
		return TokenPair{}, err
	}
	if !ok {
		return TokenPair{}, errs.TooManyRequests("LOGIN_RATE_LIMITED", "Too many sign-in attempts. Try again later.", retry)
	}

	user, err := s.repo.GetUserByEmail(ctx, email)
	if errors.Is(err, ErrNotFound) || (err == nil && user.PasswordHash == nil) {
		burnPasswordCheck(password)
		return TokenPair{}, ErrInvalidCredentials
	}
	if err != nil {
		return TokenPair{}, err
	}
	match, err := VerifyPassword(password, *user.PasswordHash)
	if err != nil {
		return TokenPair{}, err
	}
	if !match {
		return TokenPair{}, ErrInvalidCredentials
	}
	if user.Status != UserActive {
		return TokenPair{}, ErrAccountDisabled
	}

	grants, err := s.repo.ListGrants(ctx, user.ID)
	if err != nil {
		return TokenPair{}, err
	}
	if len(user.TOTPSecretEnc) == 0 {
		if authz.RequiresTOTP(grants) {
			return TokenPair{}, ErrTOTPNotEnrolled
		}
	} else if err := s.checkTOTP(ctx, user, totpCode); err != nil {
		return TokenPair{}, err
	}

	var pair TokenPair
	err = s.repo.WithTx(ctx, func(r Repo) error {
		var err error
		pair, err = s.startSession(ctx, r, user, nil, false, s.cfg.StaffSessionTTL, meta)
		return err
	})
	return pair, err
}

func (s *Service) checkTOTP(ctx context.Context, user User, code string) error {
	if code == "" {
		return ErrTOTPRequired
	}
	if !totpCodeRE.MatchString(code) {
		return ErrTOTPInvalid
	}
	secret, err := s.totp.Decrypt(user.TOTPSecretEnc)
	if err != nil {
		return fmt.Errorf("iam: decrypt totp secret: %w", err)
	}
	if !ValidateTOTP(code, string(secret), s.clock.Now()) {
		return ErrTOTPInvalid
	}
	fresh, err := s.otp.MarkTOTPUsed(ctx, user.ID, code)
	if err != nil {
		return err
	}
	if !fresh {
		return ErrTOTPInvalid // the same code was already used to sign in
	}
	return nil
}

// Refresh rotates a refresh token (FR-IAM-3). Presenting a token that was
// already rotated more than refreshReuseGrace ago revokes the whole family.
func (s *Service) Refresh(ctx context.Context, rawToken string, meta ClientMeta) (TokenPair, error) {
	if rawToken == "" {
		return TokenPair{}, ErrRefreshInvalid
	}
	hash := hashToken(rawToken)
	now := s.clock.Now()

	var (
		pair  TokenPair
		reuse bool
	)
	err := s.repo.WithTx(ctx, func(r Repo) error {
		sess, err := r.GetSessionByHashForUpdate(ctx, hash)
		if errors.Is(err, ErrNotFound) {
			return ErrRefreshInvalid
		}
		if err != nil {
			return err
		}
		if !now.Before(sess.ExpiresAt) {
			return ErrRefreshInvalid
		}

		if sess.RevokedAt != nil {
			if sess.RevokedReason == nil || *sess.RevokedReason != RevokeRotated {
				return ErrRefreshInvalid // logged out, or the family is already revoked
			}
			benign := now.Sub(*sess.RevokedAt) <= refreshReuseGrace
			if benign {
				benign, err = r.HasActiveSessionInFamily(ctx, sess.FamilyID, now)
				if err != nil {
					return err
				}
			}
			if !benign {
				// Commit the family revocation, then report the reuse.
				reuse = true
				return r.RevokeFamily(ctx, sess.FamilyID, now, RevokeReuseDetected)
			}
		} else if err := r.RevokeSession(ctx, sess.ID, now, RevokeRotated); err != nil {
			return err
		}

		user, err := r.GetUserByID(ctx, sess.UserID)
		if err != nil {
			return err
		}
		if user.Status != UserActive {
			return ErrAccountDisabled
		}

		expires := sess.ExpiresAt
		if sess.Sliding {
			expires = now.Add(s.cfg.RefreshTTL)
		}
		raw, next := s.newSession(user.ID, sess.DeviceID, sess.FamilyID, sess.Sliding, expires, meta)
		if err := r.CreateSession(ctx, next); err != nil {
			return err
		}
		pair, err = s.issue(ctx, r, user, next, raw)
		return err
	})
	if err != nil {
		return TokenPair{}, err
	}
	if reuse {
		s.log.WarnContext(ctx, "refresh token reuse detected: session family revoked")
		return TokenPair{}, ErrRefreshReused
	}
	return pair, nil
}

// Logout ends the caller's login: every refresh token in the session family
// is revoked. The access token itself stays valid until it expires (at most
// 15 minutes), so clients must also drop it.
func (s *Service) Logout(ctx context.Context, actor authz.Actor) error {
	return s.repo.RevokeFamilyOfSession(ctx, actor.UserID, actor.SessionID, s.clock.Now(), RevokeLogout)
}

// Me returns the caller's profile, roles and linked riders.
func (s *Service) Me(ctx context.Context, actor authz.Actor) (Profile, error) {
	user, err := s.repo.GetUserByID(ctx, actor.UserID)
	if errors.Is(err, ErrNotFound) {
		return Profile{}, authz.ErrUnauthenticated
	}
	if err != nil {
		return Profile{}, err
	}
	return s.profile(ctx, s.repo, user)
}

// ProfileUpdate changes fields of the caller's profile; nil fields are kept.
type ProfileUpdate struct {
	FullName          *string
	FullNameAm        *string
	ClearFullNameAm   bool
	PreferredLanguage *string
}

var (
	ErrInvalidName     = errs.Invalid("INVALID_NAME", "full_name must be 1–120 characters.")
	ErrInvalidLanguage = errs.Invalid("INVALID_LANGUAGE", "preferred_language must be am, en or om.")
)

// UpdateProfile implements PATCH /me.
func (s *Service) UpdateProfile(ctx context.Context, actor authz.Actor, in ProfileUpdate) (Profile, error) {
	user, err := s.repo.GetUserByID(ctx, actor.UserID)
	if errors.Is(err, ErrNotFound) {
		return Profile{}, authz.ErrUnauthenticated
	}
	if err != nil {
		return Profile{}, err
	}
	if in.FullName != nil {
		name := strings.TrimSpace(*in.FullName)
		if name == "" || len(name) > 120 {
			return Profile{}, ErrInvalidName
		}
		user.FullName = name
	}
	if in.ClearFullNameAm {
		user.FullNameAm = nil
	} else if in.FullNameAm != nil {
		name := strings.TrimSpace(*in.FullNameAm)
		if len(name) > 120 {
			return Profile{}, ErrInvalidName
		}
		user.FullNameAm = &name
	}
	if in.PreferredLanguage != nil {
		switch *in.PreferredLanguage {
		case "am", "en", "om":
			user.PreferredLanguage = *in.PreferredLanguage
		default:
			return Profile{}, ErrInvalidLanguage
		}
	}
	if err := s.repo.UpdateProfile(ctx, user.ID, user.FullName, user.FullNameAm, user.PreferredLanguage); err != nil {
		return Profile{}, err
	}
	return s.profile(ctx, s.repo, user)
}

// GrantRole gives a user a role; granting a role the user already holds is a
// no-op. Other domains use it, e.g. driver registration grants DRIVER.
func (s *Service) GrantRole(ctx context.Context, userID uuid.UUID, g authz.Grant) error {
	if err := g.Validate(); err != nil {
		return err
	}
	err := s.repo.GrantRole(ctx, idgen.New(), userID, g, nil)
	if errors.Is(err, ErrConflict) {
		return nil
	}
	return err
}

func (s *Service) startSession(ctx context.Context, r Repo, user User, deviceID *uuid.UUID, sliding bool, ttl time.Duration, meta ClientMeta) (TokenPair, error) {
	now := s.clock.Now()
	raw, sess := s.newSession(user.ID, deviceID, idgen.New(), sliding, now.Add(ttl), meta)
	if err := r.CreateSession(ctx, sess); err != nil {
		return TokenPair{}, err
	}
	if err := r.TouchLastLogin(ctx, user.ID, now); err != nil {
		return TokenPair{}, err
	}
	return s.issue(ctx, r, user, sess, raw)
}

func (s *Service) newSession(userID uuid.UUID, deviceID *uuid.UUID, familyID uuid.UUID, sliding bool, expires time.Time, meta ClientMeta) (string, Session) {
	raw := newRefreshToken()
	return raw, Session{
		ID:        idgen.New(),
		UserID:    userID,
		DeviceID:  deviceID,
		TokenHash: hashToken(raw),
		FamilyID:  familyID,
		IP:        meta.IP,
		UserAgent: meta.UserAgent,
		ExpiresAt: expires,
		Sliding:   sliding,
	}
}

func (s *Service) issue(ctx context.Context, r Repo, user User, sess Session, raw string) (TokenPair, error) {
	profile, err := s.profile(ctx, r, user)
	if err != nil {
		return TokenPair{}, err
	}
	access, ttl, err := s.tokens.Issue(user.ID, sess.ID, profile.Grants, user.PreferredLanguage)
	if err != nil {
		return TokenPair{}, err
	}
	return TokenPair{AccessToken: access, ExpiresIn: ttl, RefreshToken: raw, Profile: profile}, nil
}

func (s *Service) profile(ctx context.Context, r Repo, user User) (Profile, error) {
	grants, err := r.ListGrants(ctx, user.ID)
	if err != nil {
		return Profile{}, err
	}
	riders, err := r.ListLinkedRiderIDs(ctx, user.ID)
	if err != nil {
		return Profile{}, err
	}
	return Profile{User: user, Grants: grants, LinkedRiderIDs: riders}, nil
}

// newRefreshToken returns a 256-bit opaque token (design doc 13.2).
func newRefreshToken() string {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		panic(fmt.Sprintf("iam: random source failed: %v", err))
	}
	return base64.RawURLEncoding.EncodeToString(buf)
}

// hashToken is what sessions.refresh_token_hash stores.
func hashToken(raw string) []byte {
	sum := sha256.Sum256([]byte(raw))
	return sum[:]
}

// CreateStaffInput describes a new web portal account.
type CreateStaffInput struct {
	Email    string
	FullName string
	Grant    authz.Grant
	Password string // generated when empty
}

// CreateStaffResult carries the one-time secrets to hand to the new user.
type CreateStaffResult struct {
	UserID            uuid.UUID
	GeneratedPassword string // empty when the caller chose the password
	TOTPSecret        string
	TOTPURL           string
}

// CreateStaff creates a portal user with a password, an enrolled TOTP secret
// and one role grant. Used by `core-api create-staff`; the user management
// screen (day 5) will call it too.
func CreateStaff(ctx context.Context, repo Repo, cipher *SecretCipher, in CreateStaffInput) (CreateStaffResult, error) {
	email := strings.ToLower(strings.TrimSpace(in.Email))
	if !strings.Contains(email, "@") {
		return CreateStaffResult{}, errs.Invalid("INVALID_EMAIL", "A valid email is required.")
	}
	if strings.TrimSpace(in.FullName) == "" {
		return CreateStaffResult{}, errs.Invalid("INVALID_NAME", "A full name is required.")
	}
	if err := in.Grant.Validate(); err != nil {
		return CreateStaffResult{}, errs.Invalid("INVALID_ROLE", err.Error())
	}

	var result CreateStaffResult
	password := in.Password
	if password == "" {
		generated, err := GeneratePassword()
		if err != nil {
			return CreateStaffResult{}, err
		}
		password, result.GeneratedPassword = generated, generated
	} else if len(password) < MinPasswordLength {
		return CreateStaffResult{}, errs.Invalid("WEAK_PASSWORD",
			fmt.Sprintf("Passwords need at least %d characters.", MinPasswordLength))
	}
	hash, err := HashPassword(password)
	if err != nil {
		return CreateStaffResult{}, err
	}
	secret, url, err := NewTOTPSecret(email)
	if err != nil {
		return CreateStaffResult{}, err
	}
	encrypted, err := cipher.Encrypt([]byte(secret))
	if err != nil {
		return CreateStaffResult{}, err
	}

	user := User{
		ID:                idgen.New(),
		Email:             &email,
		PasswordHash:      &hash,
		TOTPSecretEnc:     encrypted,
		FullName:          strings.TrimSpace(in.FullName),
		PreferredLanguage: "en",
		Status:            UserActive,
	}
	err = repo.WithTx(ctx, func(r Repo) error {
		if err := r.CreateUser(ctx, user); err != nil {
			if errors.Is(err, ErrConflict) {
				return ErrEmailTaken
			}
			return err
		}
		return r.GrantRole(ctx, idgen.New(), user.ID, in.Grant, nil)
	})
	if err != nil {
		return CreateStaffResult{}, err
	}
	result.UserID, result.TOTPSecret, result.TOTPURL = user.ID, secret, url
	return result, nil
}
