package authz

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	schoolA = uuid.MustParse("01926f3a-7c1e-7d2b-9a41-5f0c3b8e2d11")
	schoolB = uuid.MustParse("01926f3a-7c1e-7d2b-9a41-5f0c3b8e2d22")
)

func TestGrantRoundTrip(t *testing.T) {
	for _, g := range []Grant{
		{Role: RoleDispatcher, Scope: ScopeGlobal},
		{Role: RoleInstitutionAdmin, Scope: ScopeInstitution, ScopeID: schoolA},
	} {
		parsed, err := ParseGrant(g.String())
		require.NoError(t, err)
		assert.Equal(t, g, parsed)
	}
	assert.Equal(t, "DISPATCHER/GLOBAL", Grant{Role: RoleDispatcher, Scope: ScopeGlobal}.String())
}

func TestParseGrantRejectsInvalid(t *testing.T) {
	for _, s := range []string{
		"",
		"DISPATCHER",
		"PILOT/GLOBAL",
		"DISPATCHER/PLANET",
		"INSTITUTION_ADMIN/INSTITUTION",         // scoped grant without id
		"DISPATCHER/GLOBAL/" + schoolA.String(), // global grant with id
		"INSTITUTION_ADMIN/INSTITUTION/not-a-uuid",
	} {
		_, err := ParseGrant(s)
		assert.Error(t, err, s)
	}
}

func TestActorRoleChecks(t *testing.T) {
	admin := Actor{Grants: []Grant{{Role: RoleInstitutionAdmin, Scope: ScopeInstitution, ScopeID: schoolA}}}

	assert.True(t, admin.HasRole(RoleInstitutionAdmin))
	assert.False(t, admin.HasGlobalRole(RoleInstitutionAdmin))
	assert.True(t, admin.HasRoleIn(RoleInstitutionAdmin, ScopeInstitution, schoolA))
	assert.False(t, admin.HasRoleIn(RoleInstitutionAdmin, ScopeInstitution, schoolB))

	require.NoError(t, CanManageInstitution(admin, schoolA))
	assert.ErrorIs(t, CanManageInstitution(admin, schoolB), ErrForbidden)
	assert.ErrorIs(t, CanManageUsers(admin), ErrForbidden)

	super := Actor{Grants: []Grant{{Role: RoleSuperAdmin, Scope: ScopeGlobal}}}
	require.NoError(t, CanManageInstitution(super, schoolB))
	require.NoError(t, CanManageUsers(super))
}

func TestRolesAreDistinctAndSorted(t *testing.T) {
	a := Actor{Grants: []Grant{
		{Role: RoleInstitutionAdmin, Scope: ScopeInstitution, ScopeID: schoolA},
		{Role: RoleGuardian, Scope: ScopeGlobal},
		{Role: RoleInstitutionAdmin, Scope: ScopeInstitution, ScopeID: schoolB},
	}}
	assert.Equal(t, []Role{RoleGuardian, RoleInstitutionAdmin}, a.Roles())
}

func TestRequiresTOTP(t *testing.T) {
	assert.True(t, RequiresTOTP([]Grant{{Role: RoleSupportAgent, Scope: ScopeGlobal}}))
	assert.False(t, RequiresTOTP([]Grant{{Role: RoleInstitutionAdmin, Scope: ScopeInstitution, ScopeID: schoolA}}))
}

type fakeVerifier map[string]Actor

func (f fakeVerifier) Verify(token string) (Actor, error) {
	a, ok := f[token]
	if !ok {
		return Actor{}, errors.New("bad token")
	}
	return a, nil
}

func TestMiddleware(t *testing.T) {
	verifier := fakeVerifier{
		"dispatcher": {Grants: []Grant{{Role: RoleDispatcher, Scope: ScopeGlobal}}},
		"school-a":   {Grants: []Grant{{Role: RoleInstitutionAdmin, Scope: ScopeInstitution, ScopeID: schoolA}}},
	}
	ok := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })

	r := chi.NewRouter()
	r.Use(Authenticate(verifier))
	r.Get("/public", ok)
	r.With(RequireAuth).Get("/me", ok)
	r.With(RequireRole(RoleDispatcher)).Get("/ops", ok)
	r.With(RequireRoleIn(RoleInstitutionAdmin, ScopeInstitution, "id")).Get("/institutions/{id}", ok)

	tests := []struct {
		name, path, token string
		want              int
	}{
		{"public without token", "/public", "", 204},
		{"bad token is rejected even on public routes", "/public", "nope", 401},
		{"auth required", "/me", "", 401},
		{"auth ok", "/me", "school-a", 204},
		{"role missing", "/ops", "school-a", 403},
		{"role ok", "/ops", "dispatcher", 204},
		{"scope ok", "/institutions/" + schoolA.String(), "school-a", 204},
		{"other institution", "/institutions/" + schoolB.String(), "school-a", 403},
		{"malformed id", "/institutions/abc", "school-a", 403},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			if tt.token != "" {
				req.Header.Set("Authorization", "Bearer "+tt.token)
			}
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)
			assert.Equal(t, tt.want, rec.Code)
		})
	}
}
