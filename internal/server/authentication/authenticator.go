package authentication

import (
	"context"
	"net/http"
	"strings"
)

// Principal is the authenticated caller behind a request. For Zitadel M2M
// clients, Subject is the service user ID (the JWT `sub`) and Roles are the
// project roles granted to that service user.
type Principal struct {
	Subject string
	Roles   []string
}

// wildcardRole, when present in Roles, satisfies every authorization check.
// Used by the local-dev bearer authenticator so the static token is a superuser.
const wildcardRole = "*"

// HasRole reports whether the principal may act under the given role. A
// principal holding the wildcard role passes every check.
func (p *Principal) HasRole(role string) bool {
	for _, r := range p.Roles {
		if r == wildcardRole || r == role {
			return true
		}
	}
	return false
}

// Authenticator validates inbound HTTP requests and resolves the caller.
// A non-nil error means the request is unauthenticated (HTTP 401).
type Authenticator interface {
	Authenticate(r *http.Request) (*Principal, error)
}

type principalCtxKey struct{}

// WithPrincipal returns a copy of ctx carrying the principal.
func WithPrincipal(ctx context.Context, p *Principal) context.Context {
	return context.WithValue(ctx, principalCtxKey{}, p)
}

// PrincipalFrom extracts the principal placed in ctx by the auth middleware.
func PrincipalFrom(ctx context.Context) (*Principal, bool) {
	p, ok := ctx.Value(principalCtxKey{}).(*Principal)
	return p, ok
}

func extractBearer(r *http.Request) string {
	header := r.Header.Get("Authorization")
	if !strings.HasPrefix(header, "Bearer ") {
		return ""
	}
	return strings.TrimPrefix(header, "Bearer ")
}
