package authentication

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/golang-jwt/jwt/v5"
)

// ZitadelAuthenticator validates OIDC JWT access tokens issued by Zitadel.
// Token signatures are verified offline using Zitadel's JWKS endpoint;
// no per-request round-trip to Zitadel is made.
type ZitadelAuthenticator struct {
	issuer   string
	audience string
	kf       keyfunc.Keyfunc
}

// NewZitadelAuthenticator creates an authenticator that verifies JWTs against
// the JWKS at <issuer>/oauth/v2/keys. The context controls the background
// key-refresh goroutine — cancel it on server shutdown.
func NewZitadelAuthenticator(ctx context.Context, issuer, audience string) (*ZitadelAuthenticator, error) {
	jwksURL := strings.TrimRight(issuer, "/") + "/oauth/v2/keys"
	kf, err := keyfunc.NewDefaultCtx(ctx, []string{jwksURL})
	if err != nil {
		return nil, fmt.Errorf("init JWKS from %s: %w", jwksURL, err)
	}
	return &ZitadelAuthenticator{issuer: issuer, audience: audience, kf: kf}, nil
}

func (a *ZitadelAuthenticator) Authenticate(r *http.Request) (*Principal, error) {
	raw := extractBearer(r)
	if raw == "" {
		return nil, errors.New("missing bearer token")
	}
	claims := jwt.MapClaims{}
	token, err := jwt.ParseWithClaims(raw, claims, a.kf.Keyfunc,
		jwt.WithExpirationRequired(),
		jwt.WithIssuer(a.issuer),
		jwt.WithAudience(a.audience),
	)
	if err != nil {
		return nil, fmt.Errorf("token validation: %w", err)
	}
	if !token.Valid {
		return nil, errors.New("invalid token")
	}

	sub, _ := claims.GetSubject()
	return &Principal{Subject: sub, Roles: extractZitadelRoles(claims)}, nil
}

// extractZitadelRoles collects role names from the token.
// Zitadel emits roles under two key variants:
//   - "urn:zitadel:iam:org:project:roles"           — human/OIDC flow
//   - "urn:zitadel:iam:org:project:<projectId>:roles" — M2M / jwt-bearer flow
//
// Both map role name → {orgId: orgDomain}. We accept either form.
func extractZitadelRoles(claims jwt.MapClaims) []string {
	var roles []string
	for key, val := range claims {
		if !isZitadelRolesClaim(key) {
			continue
		}
		roleMap, ok := val.(map[string]interface{})
		if !ok {
			continue
		}
		for role := range roleMap {
			roles = append(roles, role)
		}
	}
	return roles
}

func isZitadelRolesClaim(key string) bool {
	const prefix = "urn:zitadel:iam:org:project:"
	return strings.HasPrefix(key, prefix) && strings.HasSuffix(key, "roles")
}
