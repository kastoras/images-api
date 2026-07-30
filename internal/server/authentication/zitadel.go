package authentication

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

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

// JWKSURL returns the key-set endpoint for an issuer.
func JWKSURL(issuer string) string {
	return strings.TrimRight(issuer, "/") + "/oauth/v2/keys"
}

// PingJWKS reports whether the issuer is serving a usable key set right now.
//
// This exists because NewZitadelAuthenticator cannot fail on an unreachable
// issuer: the underlying jwkset client is built with NoErrorReturnFirstHTTPReq,
// so the first fetch failing is swallowed and the constructor returns an
// authenticator holding an EMPTY key set. That process starts cleanly, answers
// /health with 200, and rejects every request with "key not found" until a
// background refresh eventually succeeds — up to an hour later. Checking the
// endpoint separately is the only way to turn that into a visible failure.
func PingJWKS(ctx context.Context, issuer string) error {
	url := JWKSURL(issuer)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("build JWKS request for %s: %w", url, err)
	}

	resp, err := jwksProbeClient.Do(req)
	if err != nil {
		return fmt.Errorf("fetch JWKS from %s: %w", url, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("JWKS at %s returned HTTP %d", url, resp.StatusCode)
	}

	// A 200 is not enough — a login page or an error document would also be a
	// 200. Require at least one key, which is what verification actually needs.
	var body struct {
		Keys []json.RawMessage `json:"keys"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, jwksProbeMaxBytes)).Decode(&body); err != nil {
		return fmt.Errorf("parse JWKS from %s: %w", url, err)
	}
	if len(body.Keys) == 0 {
		return fmt.Errorf("JWKS at %s contains no keys", url)
	}

	return nil
}

const jwksProbeMaxBytes = 1 << 20

// Bounded on purpose: the default jwkset client allows a full minute per
// attempt, which is far too long for a readiness probe that retries.
var jwksProbeClient = &http.Client{Timeout: 10 * time.Second}

// NewZitadelAuthenticator creates an authenticator that verifies JWTs against
// the JWKS at <issuer>/oauth/v2/keys. The context controls the background
// key-refresh goroutine — cancel it on server shutdown.
//
// Note this does NOT fail when the issuer is unreachable — see PingJWKS.
func NewZitadelAuthenticator(ctx context.Context, issuer, audience string) (*ZitadelAuthenticator, error) {
	jwksURL := JWKSURL(issuer)
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
