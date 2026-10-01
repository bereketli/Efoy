package iam

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/pquerna/otp/totp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/blinge12/efoy/pkg/authz"
	"github.com/blinge12/efoy/pkg/errs"
	"github.com/blinge12/efoy/pkg/idgen"
)

const phone = "+251911234567"

func TestOTPLoginCreatesUserOnce(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()

	res, err := e.svc.RequestOTP(ctx, phone, "en")
	require.NoError(t, err)
	assert.Equal(t, 5*time.Minute, res.ExpiresIn)
	assert.Contains(t, e.sms.last[phone], "Your Efoy code is")

	_, err = e.svc.VerifyOTP(ctx, phone, "000000", nil, ClientMeta{})
	if e.sms.code(t, phone) != "000000" {
		assert.ErrorIs(t, err, ErrOTPInvalid)
	}

	pair, err := e.svc.VerifyOTP(ctx, phone, e.sms.code(t, phone), &DeviceInput{Platform: "ANDROID"}, ClientMeta{})
	require.NoError(t, err)
	require.NotNil(t, pair.Profile.User.Phone)
	assert.Equal(t, phone, *pair.Profile.User.Phone)
	assert.Equal(t, 15*time.Minute, pair.ExpiresIn)
	assert.NotEmpty(t, pair.RefreshToken)

	actor, err := e.keys.Tokens.Verify(pair.AccessToken)
	require.NoError(t, err)
	assert.Equal(t, pair.Profile.User.ID, actor.UserID)

	again := e.otpLogin(t, phone)
	assert.Equal(t, pair.Profile.User.ID, again.Profile.User.ID, "second login reuses the account")
}

func TestOTPCodeIsSingleUse(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	_, err := e.svc.RequestOTP(ctx, phone, "am")
	require.NoError(t, err)
	code := e.sms.code(t, phone)

	_, err = e.svc.VerifyOTP(ctx, phone, code, nil, ClientMeta{})
	require.NoError(t, err)
	_, err = e.svc.VerifyOTP(ctx, phone, code, nil, ClientMeta{})
	assert.ErrorIs(t, err, ErrOTPInvalid)
}

func TestOTPExpires(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	_, err := e.svc.RequestOTP(ctx, phone, "en")
	require.NoError(t, err)

	e.redis.FastForward(5*time.Minute + time.Second)
	_, err = e.svc.VerifyOTP(ctx, phone, e.sms.code(t, phone), nil, ClientMeta{})
	assert.ErrorIs(t, err, ErrOTPInvalid)
}

func TestOTPFiveAttempts(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	_, err := e.svc.RequestOTP(ctx, phone, "en")
	require.NoError(t, err)
	code := e.sms.code(t, phone)
	wrong := "123456"
	if code == wrong {
		wrong = "654321"
	}

	for i := 1; i <= 4; i++ {
		_, err = e.svc.VerifyOTP(ctx, phone, wrong, nil, ClientMeta{})
		assert.ErrorIs(t, err, ErrOTPInvalid, "attempt %d", i)
	}
	_, err = e.svc.VerifyOTP(ctx, phone, wrong, nil, ClientMeta{})
	assert.ErrorIs(t, err, ErrOTPAttempts, "fifth wrong attempt burns the code")

	_, err = e.svc.VerifyOTP(ctx, phone, code, nil, ClientMeta{})
	assert.ErrorIs(t, err, ErrOTPInvalid, "the right code no longer works")
}

func TestOTPRateLimit(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		_, err := e.svc.RequestOTP(ctx, phone, "en")
		require.NoError(t, err)
	}
	_, err := e.svc.RequestOTP(ctx, phone, "en")
	var appErr *errs.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, http.StatusTooManyRequests, appErr.Status)
	assert.Greater(t, appErr.RetryAfter, time.Duration(0))

	e.redis.FastForward(10 * time.Minute)
	_, err = e.svc.RequestOTP(ctx, phone, "en")
	assert.NoError(t, err, "the window resets")
}

func TestOTPValidation(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()

	_, err := e.svc.RequestOTP(ctx, "0911234567", "en")
	assert.ErrorIs(t, err, ErrInvalidPhone)
	_, err = e.svc.VerifyOTP(ctx, phone, "12ab56", nil, ClientMeta{})
	assert.ErrorIs(t, err, ErrInvalidOTPFormat)
	_, err = e.svc.VerifyOTP(ctx, phone, "123456", &DeviceInput{Platform: "WINDOWS"}, ClientMeta{})
	assert.ErrorIs(t, err, ErrInvalidDevice)
}

func TestRefreshRotatesAndDetectsReuse(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	first := e.otpLogin(t, phone)

	second, err := e.svc.Refresh(ctx, first.RefreshToken, ClientMeta{})
	require.NoError(t, err)
	assert.NotEqual(t, first.RefreshToken, second.RefreshToken)

	// Replaying the rotated token after the grace window looks like theft:
	// the whole family is revoked, including the token issued to the
	// legitimate client.
	e.clock.Advance(time.Minute)
	_, err = e.svc.Refresh(ctx, first.RefreshToken, ClientMeta{})
	assert.ErrorIs(t, err, ErrRefreshReused)
	_, err = e.svc.Refresh(ctx, second.RefreshToken, ClientMeta{})
	assert.ErrorIs(t, err, ErrRefreshInvalid)
}

func TestRefreshToleratesParallelRefresh(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	first := e.otpLogin(t, phone)

	a, err := e.svc.Refresh(ctx, first.RefreshToken, ClientMeta{})
	require.NoError(t, err)
	e.clock.Advance(2 * time.Second)
	b, err := e.svc.Refresh(ctx, first.RefreshToken, ClientMeta{})
	require.NoError(t, err, "a second refresh within the grace window is a race, not theft")

	_, err = e.svc.Refresh(ctx, a.RefreshToken, ClientMeta{})
	assert.NoError(t, err)
	_, err = e.svc.Refresh(ctx, b.RefreshToken, ClientMeta{})
	assert.NoError(t, err)
}

func TestMobileSessionsSlideStaffSessionsDoNot(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()

	mobile := e.otpLogin(t, phone)
	e.clock.Advance(29 * 24 * time.Hour)
	mobile, err := e.svc.Refresh(ctx, mobile.RefreshToken, ClientMeta{})
	require.NoError(t, err)
	e.clock.Advance(29 * 24 * time.Hour)
	_, err = e.svc.Refresh(ctx, mobile.RefreshToken, ClientMeta{})
	require.NoError(t, err, "each refresh extends a mobile session by 30 days")

	staff, secret := e.createStaff(t, "dispatcher@efoy.et", authz.RoleDispatcher)
	pair := e.staffLogin(t, staff, secret)
	e.clock.Advance(11 * time.Hour)
	pair, err = e.svc.Refresh(ctx, pair.RefreshToken, ClientMeta{})
	require.NoError(t, err)
	e.clock.Advance(2 * time.Hour)
	_, err = e.svc.Refresh(ctx, pair.RefreshToken, ClientMeta{})
	assert.ErrorIs(t, err, ErrRefreshInvalid, "staff sessions end 12 h after login")
}

func TestLogoutEndsTheWholeSession(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	first := e.otpLogin(t, phone)
	second, err := e.svc.Refresh(ctx, first.RefreshToken, ClientMeta{})
	require.NoError(t, err)

	// Log out with the older access token: the newer refresh token dies too.
	actor, err := e.keys.Tokens.Verify(first.AccessToken)
	require.NoError(t, err)
	require.NoError(t, e.svc.Logout(ctx, actor))

	_, err = e.svc.Refresh(ctx, second.RefreshToken, ClientMeta{})
	assert.ErrorIs(t, err, ErrRefreshInvalid)
}

func TestMeReturnsRolesAndLinkedRiders(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	pair := e.otpLogin(t, phone)
	userID := pair.Profile.User.ID
	rider := idgen.New()
	e.repo.riders[userID] = []uuid.UUID{rider}
	require.NoError(t, e.repo.GrantRole(ctx, idgen.New(), userID, authz.Grant{Role: authz.RoleGuardian, Scope: authz.ScopeGlobal}, nil))

	actor, err := e.keys.Tokens.Verify(pair.AccessToken)
	require.NoError(t, err)
	profile, err := e.svc.Me(ctx, actor)
	require.NoError(t, err)
	assert.Equal(t, []uuid.UUID{rider}, profile.LinkedRiderIDs)
	assert.Equal(t, []authz.Grant{{Role: authz.RoleGuardian, Scope: authz.ScopeGlobal}}, profile.Grants)
}

// ---- staff login ----

func (e *env) createStaff(t *testing.T, email string, role authz.Role) (string, string) {
	t.Helper()
	res, err := CreateStaff(context.Background(), e.repo, e.keys.TOTP, CreateStaffInput{
		Email:    email,
		FullName: "Selam Tesfaye",
		Grant:    authz.Grant{Role: role, Scope: authz.ScopeGlobal},
		Password: "correct horse battery staple",
	})
	require.NoError(t, err)
	assert.Empty(t, res.GeneratedPassword, "a chosen password is not echoed back")
	assert.True(t, strings.HasPrefix(res.TOTPURL, "otpauth://totp/"))
	assert.Contains(t, res.TOTPURL, "issuer=Efoy")
	return email, res.TOTPSecret
}

func (e *env) totpCode(t *testing.T, secret string) string {
	t.Helper()
	code, err := totp.GenerateCodeCustom(secret, e.clock.Now(), totpOpts)
	require.NoError(t, err)
	return code
}

func (e *env) staffLogin(t *testing.T, email, secret string) TokenPair {
	t.Helper()
	pair, err := e.svc.Login(context.Background(), email, "correct horse battery staple", e.totpCode(t, secret), ClientMeta{})
	require.NoError(t, err)
	return pair
}

func TestStaffLoginRequiresTOTP(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	email, secret := e.createStaff(t, "Dispatcher@Efoy.et", authz.RoleDispatcher)
	password := "correct horse battery staple"

	_, err := e.svc.Login(ctx, email, password, "", ClientMeta{})
	assert.ErrorIs(t, err, ErrTOTPRequired)

	_, err = e.svc.Login(ctx, email, password, "000000", ClientMeta{})
	if e.totpCode(t, secret) != "000000" {
		assert.ErrorIs(t, err, ErrTOTPInvalid)
	}

	code := e.totpCode(t, secret)
	pair, err := e.svc.Login(ctx, " DISPATCHER@efoy.et ", password, code, ClientMeta{})
	require.NoError(t, err)
	actor, err := e.keys.Tokens.Verify(pair.AccessToken)
	require.NoError(t, err)
	assert.True(t, actor.HasGlobalRole(authz.RoleDispatcher))

	_, err = e.svc.Login(ctx, email, password, code, ClientMeta{})
	assert.ErrorIs(t, err, ErrTOTPInvalid, "a TOTP code cannot be replayed")
}

func TestStaffLoginBadCredentials(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	email, secret := e.createStaff(t, "admin@efoy.et", authz.RoleSuperAdmin)

	_, err := e.svc.Login(ctx, email, "wrong password!", e.totpCode(t, secret), ClientMeta{})
	assert.ErrorIs(t, err, ErrInvalidCredentials)
	_, err = e.svc.Login(ctx, "nobody@efoy.et", "whatever password", "", ClientMeta{})
	assert.ErrorIs(t, err, ErrInvalidCredentials)
	_, err = e.svc.Login(ctx, "", "", "", ClientMeta{})
	assert.ErrorIs(t, err, ErrMissingCredentials)
}

func TestStaffRoleWithoutTOTPIsRefused(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	hash, err := HashPassword("correct horse battery staple")
	require.NoError(t, err)
	email := "support@efoy.et"
	user := User{ID: idgen.New(), Email: &email, PasswordHash: &hash, FullName: "Support", PreferredLanguage: "en", Status: UserActive}
	require.NoError(t, e.repo.CreateUser(ctx, user))
	require.NoError(t, e.repo.GrantRole(ctx, idgen.New(), user.ID, authz.Grant{Role: authz.RoleSupportAgent, Scope: authz.ScopeGlobal}, nil))

	_, err = e.svc.Login(ctx, email, "correct horse battery staple", "", ClientMeta{})
	assert.ErrorIs(t, err, ErrTOTPNotEnrolled)
}

func TestCreateStaffValidation(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	in := CreateStaffInput{Email: "ops@efoy.et", FullName: "Ops", Grant: authz.Grant{Role: authz.RoleDispatcher, Scope: authz.ScopeGlobal}}

	res, err := CreateStaff(ctx, e.repo, e.keys.TOTP, in)
	require.NoError(t, err)
	assert.Len(t, res.GeneratedPassword, 22, "16 random bytes, base64url")

	_, err = CreateStaff(ctx, e.repo, e.keys.TOTP, in)
	assert.ErrorIs(t, err, ErrEmailTaken)

	weak := in
	weak.Email, weak.Password = "weak@efoy.et", "short"
	_, err = CreateStaff(ctx, e.repo, e.keys.TOTP, weak)
	require.Error(t, err)

	scoped := in
	scoped.Email, scoped.Grant = "school@efoy.et", authz.Grant{Role: authz.RoleInstitutionAdmin, Scope: authz.ScopeInstitution}
	_, err = CreateStaff(ctx, e.repo, e.keys.TOTP, scoped)
	assert.Error(t, err, "a scoped role needs a scope id")
}

func TestUpdateProfile(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	pair := e.otpLogin(t, phone)
	actor, err := e.keys.Tokens.Verify(pair.AccessToken)
	require.NoError(t, err)

	name, am, lang := "  Abebe Kebede ", "አበበ ከበደ", "en"
	profile, err := e.svc.UpdateProfile(ctx, actor, ProfileUpdate{FullName: &name, FullNameAm: &am, PreferredLanguage: &lang})
	require.NoError(t, err)
	assert.Equal(t, "Abebe Kebede", profile.User.FullName)
	assert.Equal(t, &am, profile.User.FullNameAm)
	assert.Equal(t, "en", profile.User.PreferredLanguage)

	profile, err = e.svc.UpdateProfile(ctx, actor, ProfileUpdate{ClearFullNameAm: true})
	require.NoError(t, err)
	assert.Nil(t, profile.User.FullNameAm)
	assert.Equal(t, "Abebe Kebede", profile.User.FullName, "unset fields are kept")

	empty, bad := " ", "fr"
	_, err = e.svc.UpdateProfile(ctx, actor, ProfileUpdate{FullName: &empty})
	assert.ErrorIs(t, err, ErrInvalidName)
	_, err = e.svc.UpdateProfile(ctx, actor, ProfileUpdate{PreferredLanguage: &bad})
	assert.ErrorIs(t, err, ErrInvalidLanguage)
}

func TestGrantRoleIsIdempotent(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	user := e.otpLogin(t, phone).Profile.User.ID
	driverRole := authz.Grant{Role: authz.RoleDriver, Scope: authz.ScopeGlobal}

	require.NoError(t, e.svc.GrantRole(ctx, user, driverRole))
	require.NoError(t, e.svc.GrantRole(ctx, user, driverRole))
	grants, err := e.repo.ListGrants(ctx, user)
	require.NoError(t, err)
	assert.Equal(t, []authz.Grant{driverRole}, grants)

	assert.Error(t, e.svc.GrantRole(ctx, user, authz.Grant{Role: authz.RoleDriver, Scope: authz.ScopeFleet}))
}
