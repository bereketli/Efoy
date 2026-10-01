package iam

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"regexp"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"

	"github.com/blinge12/efoy/internal/config"
	"github.com/blinge12/efoy/pkg/authz"
	"github.com/blinge12/efoy/pkg/clock"
)

// memRepo is an in-memory Repo. WithTx does not roll back on error, which
// the tests below do not rely on.
type memRepo struct {
	mu       sync.Mutex
	users    map[uuid.UUID]User
	grants   map[uuid.UUID][]authz.Grant
	devices  map[uuid.UUID]Device
	sessions map[uuid.UUID]Session
	riders   map[uuid.UUID][]uuid.UUID
}

func newMemRepo() *memRepo {
	return &memRepo{
		users:    map[uuid.UUID]User{},
		grants:   map[uuid.UUID][]authz.Grant{},
		devices:  map[uuid.UUID]Device{},
		sessions: map[uuid.UUID]Session{},
		riders:   map[uuid.UUID][]uuid.UUID{},
	}
}

func (m *memRepo) WithTx(_ context.Context, fn func(Repo) error) error { return fn(m) }

func (m *memRepo) GetUserByID(_ context.Context, id uuid.UUID) (User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	u, ok := m.users[id]
	if !ok {
		return User{}, ErrNotFound
	}
	return u, nil
}

func (m *memRepo) findUser(match func(User) bool) (User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, u := range m.users {
		if match(u) {
			return u, nil
		}
	}
	return User{}, ErrNotFound
}

func (m *memRepo) GetUserByPhone(_ context.Context, phone string) (User, error) {
	return m.findUser(func(u User) bool { return u.Phone != nil && *u.Phone == phone })
}

func (m *memRepo) GetUserByEmail(_ context.Context, email string) (User, error) {
	return m.findUser(func(u User) bool { return u.Email != nil && strings.EqualFold(*u.Email, email) })
}

func (m *memRepo) EnsureUserByPhone(ctx context.Context, u User) (User, error) {
	if existing, err := m.GetUserByPhone(ctx, *u.Phone); err == nil {
		return existing, nil
	}
	if err := m.CreateUser(ctx, u); err != nil {
		return User{}, err
	}
	return m.GetUserByID(ctx, u.ID)
}

func (m *memRepo) CreateUser(_ context.Context, u User) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, other := range m.users {
		if (u.Email != nil && other.Email != nil && strings.EqualFold(*u.Email, *other.Email)) ||
			(u.Phone != nil && other.Phone != nil && *u.Phone == *other.Phone) {
			return ErrConflict
		}
	}
	m.users[u.ID] = u
	return nil
}

func (m *memRepo) TouchLastLogin(context.Context, uuid.UUID, time.Time) error { return nil }

func (m *memRepo) UpdateProfile(_ context.Context, userID uuid.UUID, fullName string, fullNameAm *string, language string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	u, ok := m.users[userID]
	if !ok {
		return ErrNotFound
	}
	u.FullName, u.FullNameAm, u.PreferredLanguage = fullName, fullNameAm, language
	m.users[userID] = u
	return nil
}

func (m *memRepo) ListGrants(_ context.Context, userID uuid.UUID) ([]authz.Grant, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]authz.Grant{}, m.grants[userID]...), nil
}

func (m *memRepo) GrantRole(_ context.Context, _, userID uuid.UUID, g authz.Grant, _ *uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if slices.Contains(m.grants[userID], g) {
		return ErrConflict
	}
	m.grants[userID] = append(m.grants[userID], g)
	return nil
}

func (m *memRepo) ListLinkedRiderIDs(_ context.Context, userID uuid.UUID) ([]uuid.UUID, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]uuid.UUID{}, m.riders[userID]...), nil
}

func (m *memRepo) UpsertDevice(_ context.Context, d Device) (uuid.UUID, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, other := range m.devices {
		if other.UserID == d.UserID && other.PushToken != nil && d.PushToken != nil && *other.PushToken == *d.PushToken {
			return id, nil
		}
	}
	m.devices[d.ID] = d
	return d.ID, nil
}

func (m *memRepo) CreateSession(_ context.Context, s Session) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessions[s.ID] = s
	return nil
}

func (m *memRepo) GetSessionByHashForUpdate(_ context.Context, hash []byte) (Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, s := range m.sessions {
		if bytes.Equal(s.TokenHash, hash) {
			return s, nil
		}
	}
	return Session{}, ErrNotFound
}

func (m *memRepo) revokeWhere(at time.Time, reason RevokeReason, match func(Session) bool) {
	for id, s := range m.sessions {
		if s.RevokedAt == nil && match(s) {
			t, r := at, reason
			s.RevokedAt, s.RevokedReason = &t, &r
			m.sessions[id] = s
		}
	}
}

func (m *memRepo) RevokeSession(_ context.Context, id uuid.UUID, at time.Time, reason RevokeReason) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.revokeWhere(at, reason, func(s Session) bool { return s.ID == id })
	return nil
}

func (m *memRepo) RevokeFamily(_ context.Context, familyID uuid.UUID, at time.Time, reason RevokeReason) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.revokeWhere(at, reason, func(s Session) bool { return s.FamilyID == familyID })
	return nil
}

func (m *memRepo) RevokeFamilyOfSession(_ context.Context, userID, sessionID uuid.UUID, at time.Time, reason RevokeReason) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sessions[sessionID]
	if !ok || s.UserID != userID {
		return nil
	}
	m.revokeWhere(at, reason, func(other Session) bool { return other.FamilyID == s.FamilyID })
	return nil
}

func (m *memRepo) HasActiveSessionInFamily(_ context.Context, familyID uuid.UUID, now time.Time) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, s := range m.sessions {
		if s.FamilyID == familyID && s.RevokedAt == nil && now.Before(s.ExpiresAt) {
			return true, nil
		}
	}
	return false, nil
}

// captureSMS records sent messages instead of delivering them.
type captureSMS struct {
	mu   sync.Mutex
	last map[string]string
}

func (c *captureSMS) SendSMS(_ context.Context, to, body string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.last[to] = body
	return nil
}

var sixDigits = regexp.MustCompile(`\d{6}`)

// code returns the OTP from the last SMS sent to phone.
func (c *captureSMS) code(t *testing.T, phone string) string {
	t.Helper()
	c.mu.Lock()
	defer c.mu.Unlock()
	code := sixDigits.FindString(c.last[phone])
	require.NotEmpty(t, code, "no OTP sent to %s", phone)
	return code
}

type env struct {
	svc   *Service
	repo  *memRepo
	sms   *captureSMS
	clock *clock.Fake
	redis *miniredis.Miniredis
	keys  *Keyring
	cfg   config.Auth
}

func discardLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func testAuthConfig() config.Auth {
	return config.Auth{
		Issuer:          "efoy",
		Audience:        "efoy-api",
		AccessTTL:       15 * time.Minute,
		RefreshTTL:      30 * 24 * time.Hour,
		StaffSessionTTL: 12 * time.Hour,
		SigningKeyID:    "test",
		OTP: config.OTP{
			TTL:         5 * time.Minute,
			MaxAttempts: 5,
			RateLimit:   3,
			RateWindow:  10 * time.Minute,
			ResendAfter: time.Minute,
		},
	}
}

func newEnv(t *testing.T) *env {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	clk := clock.NewFake(time.Date(2026, 10, 1, 6, 0, 0, 0, time.UTC))
	log := discardLogger()
	cfg := testAuthConfig()
	keys, err := LoadKeyring(cfg, "dev", clk, log)
	require.NoError(t, err)

	repo := newMemRepo()
	sms := &captureSMS{last: map[string]string{}}
	svc := NewService(Deps{
		Repo:    repo,
		OTP:     NewOTPStore(rdb, cfg.OTP, keys.OTPPepper),
		Limiter: NewLimiter(rdb),
		Tokens:  keys.Tokens,
		TOTP:    keys.TOTP,
		SMS:     sms,
		Clock:   clk,
		Config:  cfg,
		Log:     log,
	})
	return &env{svc: svc, repo: repo, sms: sms, clock: clk, redis: mr, keys: keys, cfg: cfg}
}

// otpLogin signs phone in through the OTP flow.
func (e *env) otpLogin(t *testing.T, phone string) TokenPair {
	t.Helper()
	ctx := context.Background()
	_, err := e.svc.RequestOTP(ctx, phone, "en")
	require.NoError(t, err)
	pair, err := e.svc.VerifyOTP(ctx, phone, e.sms.code(t, phone), nil, ClientMeta{IP: "196.188.0.1"})
	require.NoError(t, err)
	return pair
}
