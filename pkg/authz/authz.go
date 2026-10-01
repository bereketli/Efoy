// Package authz holds role-based access control: roles, scoped grants, the
// authenticated actor, and one policy function per use case (design doc 13.2).
// Services call the policies; handlers never make access decisions.
package authz

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/google/uuid"

	"github.com/blinge12/efoy/pkg/errs"
)

// Role is one of the nine Efoy roles (design doc section 2).
type Role string

const (
	RoleStudentRider      Role = "STUDENT_RIDER"
	RoleCivilServantRider Role = "CIVIL_SERVANT_RIDER"
	RoleGuardian          Role = "GUARDIAN"
	RoleDriver            Role = "DRIVER"
	RoleFleetOwner        Role = "FLEET_OWNER"
	RoleInstitutionAdmin  Role = "INSTITUTION_ADMIN"
	RoleDispatcher        Role = "DISPATCHER"
	RoleSupportAgent      Role = "SUPPORT_AGENT"
	RoleSuperAdmin        Role = "SUPER_ADMIN"
)

var allRoles = []Role{
	RoleStudentRider, RoleCivilServantRider, RoleGuardian, RoleDriver, RoleFleetOwner,
	RoleInstitutionAdmin, RoleDispatcher, RoleSupportAgent, RoleSuperAdmin,
}

func (r Role) Valid() bool { return slices.Contains(allRoles, r) }

// totpRoles must use two-factor login on the web portals (FR-IAM-2).
var totpRoles = []Role{RoleDispatcher, RoleSupportAgent, RoleSuperAdmin}

// ScopeType limits a grant to one institution, fleet or zone.
type ScopeType string

const (
	ScopeGlobal      ScopeType = "GLOBAL"
	ScopeInstitution ScopeType = "INSTITUTION"
	ScopeFleet       ScopeType = "FLEET"
	ScopeZone        ScopeType = "ZONE"
)

func (s ScopeType) Valid() bool {
	switch s {
	case ScopeGlobal, ScopeInstitution, ScopeFleet, ScopeZone:
		return true
	}
	return false
}

// Grant is one row of user_roles: a role, optionally scoped to an entity.
type Grant struct {
	Role    Role
	Scope   ScopeType
	ScopeID uuid.UUID // uuid.Nil for GLOBAL
}

// Validate mirrors the user_roles check: GLOBAL grants have no scope id and
// every other scope has one.
func (g Grant) Validate() error {
	if !g.Role.Valid() {
		return fmt.Errorf("authz: unknown role %q", g.Role)
	}
	if !g.Scope.Valid() {
		return fmt.Errorf("authz: unknown scope %q", g.Scope)
	}
	if (g.Scope == ScopeGlobal) != (g.ScopeID == uuid.Nil) {
		return fmt.Errorf("authz: scope %s requires a scope id only when not GLOBAL", g.Scope)
	}
	return nil
}

// String encodes the grant for the JWT "scopes" claim:
// "DISPATCHER/GLOBAL" or "INSTITUTION_ADMIN/INSTITUTION/<uuid>".
func (g Grant) String() string {
	if g.Scope == ScopeGlobal {
		return string(g.Role) + "/" + string(g.Scope)
	}
	return string(g.Role) + "/" + string(g.Scope) + "/" + g.ScopeID.String()
}

// ParseGrant decodes Grant.String.
func ParseGrant(s string) (Grant, error) {
	parts := strings.Split(s, "/")
	var g Grant
	switch len(parts) {
	case 2:
		g = Grant{Role: Role(parts[0]), Scope: ScopeType(parts[1])}
	case 3:
		id, err := uuid.Parse(parts[2])
		if err != nil {
			return Grant{}, fmt.Errorf("authz: grant %q: %w", s, err)
		}
		g = Grant{Role: Role(parts[0]), Scope: ScopeType(parts[1]), ScopeID: id}
	default:
		return Grant{}, fmt.Errorf("authz: malformed grant %q", s)
	}
	return g, g.Validate()
}

// RequiresTOTP reports whether any grant makes two-factor login mandatory.
func RequiresTOTP(grants []Grant) bool {
	for _, g := range grants {
		if slices.Contains(totpRoles, g.Role) {
			return true
		}
	}
	return false
}

// Actor is the authenticated caller, built from a verified access token.
type Actor struct {
	UserID    uuid.UUID
	SessionID uuid.UUID
	Grants    []Grant
	Language  string
}

// Roles returns the distinct roles of the actor, sorted.
func (a Actor) Roles() []Role {
	roles := make([]Role, 0, len(a.Grants))
	for _, g := range a.Grants {
		if !slices.Contains(roles, g.Role) {
			roles = append(roles, g.Role)
		}
	}
	slices.Sort(roles)
	return roles
}

// HasRole reports whether the actor holds any of roles, in any scope.
func (a Actor) HasRole(roles ...Role) bool {
	for _, g := range a.Grants {
		if slices.Contains(roles, g.Role) {
			return true
		}
	}
	return false
}

// HasGlobalRole reports whether the actor holds any of roles platform-wide.
func (a Actor) HasGlobalRole(roles ...Role) bool {
	for _, g := range a.Grants {
		if g.Scope == ScopeGlobal && slices.Contains(roles, g.Role) {
			return true
		}
	}
	return false
}

// HasRoleIn reports whether the actor holds role for the given entity, either
// through a grant scoped to it or a GLOBAL grant of the same role.
func (a Actor) HasRoleIn(role Role, scope ScopeType, id uuid.UUID) bool {
	for _, g := range a.Grants {
		if g.Role != role {
			continue
		}
		if g.Scope == ScopeGlobal || (g.Scope == scope && g.ScopeID == id) {
			return true
		}
	}
	return false
}

type actorKey struct{}

// WithActor stores the authenticated actor in ctx.
func WithActor(ctx context.Context, a Actor) context.Context {
	return context.WithValue(ctx, actorKey{}, a)
}

// ActorFrom returns the authenticated actor, if any.
func ActorFrom(ctx context.Context) (Actor, bool) {
	a, ok := ctx.Value(actorKey{}).(Actor)
	return a, ok
}

var (
	ErrUnauthenticated = errs.Unauthorized("UNAUTHENTICATED", "Sign in to continue.")
	ErrForbidden       = errs.Forbidden("FORBIDDEN", "You do not have permission to do this.")
)

// Require returns the authenticated actor or ErrUnauthenticated.
func Require(ctx context.Context) (Actor, error) {
	a, ok := ActorFrom(ctx)
	if !ok {
		return Actor{}, ErrUnauthenticated
	}
	return a, nil
}

func allow(ok bool) error {
	if !ok {
		return ErrForbidden
	}
	return nil
}

// Policies: one function per use case. Data-dependent policies (for example
// CanViewRiderLocation, which needs guardian links and active trips) live
// with their domain once it exists.

// CanManageUsers covers user and role management and the audit log.
func CanManageUsers(a Actor) error { return allow(a.HasGlobalRole(RoleSuperAdmin)) }

// CanReviewDocuments covers approving or rejecting driver documents.
func CanReviewDocuments(a Actor) error {
	return allow(a.HasGlobalRole(RoleSuperAdmin, RoleDispatcher))
}

// CanDispatch covers the live map, incidents and replacements.
func CanDispatch(a Actor) error { return allow(a.HasGlobalRole(RoleSuperAdmin, RoleDispatcher)) }

// CanManageInstitution covers an institution's riders, schedules and invoices.
func CanManageInstitution(a Actor, institutionID uuid.UUID) error {
	return allow(a.HasGlobalRole(RoleSuperAdmin) ||
		a.HasRoleIn(RoleInstitutionAdmin, ScopeInstitution, institutionID))
}

// CanManageFleet covers a fleet owner's vehicles, drivers and payouts.
func CanManageFleet(a Actor, fleetOwnerID uuid.UUID) error {
	return allow(a.HasGlobalRole(RoleSuperAdmin) ||
		a.HasRoleIn(RoleFleetOwner, ScopeFleet, fleetOwnerID))
}
