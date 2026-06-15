package authentication

import (
	"errors"
	"net/http"
)

// errInvalidToken is returned when the bearer token is missing or wrong.
var errInvalidToken = errors.New("invalid bearer token")

// BearerAuthenticator validates a static bearer token.
// Use this for local development (AUTHENTICATION_TYPE=bearer). The token is treated
// as a superuser: the returned principal holds the wildcard role and therefore
// passes every authorization check.
type BearerAuthenticator struct {
	token string
}

func NewBearerAuthenticator(token string) *BearerAuthenticator {
	return &BearerAuthenticator{token: token}
}

func (a *BearerAuthenticator) Authenticate(r *http.Request) (*Principal, error) {
	raw := extractBearer(r)
	if raw == "" || raw != a.token {
		return nil, errInvalidToken
	}
	return &Principal{Subject: "local-dev", Roles: []string{wildcardRole}}, nil
}
