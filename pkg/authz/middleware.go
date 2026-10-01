package authz

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/blinge12/efoy/pkg/errs"
)

// Verifier turns a bearer token into an Actor.
type Verifier interface {
	Verify(token string) (Actor, error)
}

var errInvalidToken = errs.Unauthorized("INVALID_TOKEN", "The access token is invalid or expired.")

// Authenticate verifies the bearer token when one is sent and stores the
// actor in the request context. Requests without a token pass through, so
// public endpoints keep working; RequireAuth and RequireRole guard the rest.
func Authenticate(v Verifier) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			if header == "" {
				next.ServeHTTP(w, r)
				return
			}
			scheme, token, ok := strings.Cut(header, " ")
			if !ok || !strings.EqualFold(scheme, "Bearer") || token == "" {
				errs.Write(w, r, errInvalidToken)
				return
			}
			actor, err := v.Verify(token)
			if err != nil {
				errs.Write(w, r, errInvalidToken)
				return
			}
			next.ServeHTTP(w, r.WithContext(WithActor(r.Context(), actor)))
		})
	}
}

// RequireAuth answers 401 unless the request carries a valid access token.
func RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := ActorFrom(r.Context()); !ok {
			errs.Write(w, r, ErrUnauthenticated)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// RequireRole answers 401 without a token and 403 unless the actor holds at
// least one of roles (in any scope).
func RequireRole(roles ...Role) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			a, ok := ActorFrom(r.Context())
			if !ok {
				errs.Write(w, r, ErrUnauthenticated)
				return
			}
			if !a.HasRole(roles...) {
				errs.Write(w, r, ErrForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequireRoleIn guards routes such as /institutions/{id}/...: the actor needs
// role scoped to the entity named by the chi URL parameter param, or the same
// role globally. SUPER_ADMIN always passes.
func RequireRoleIn(role Role, scope ScopeType, param string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			a, ok := ActorFrom(r.Context())
			if !ok {
				errs.Write(w, r, ErrUnauthenticated)
				return
			}
			id, err := uuid.Parse(chi.URLParam(r, param))
			if err != nil || !(a.HasGlobalRole(RoleSuperAdmin) || a.HasRoleIn(role, scope, id)) {
				errs.Write(w, r, ErrForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
