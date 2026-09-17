package authentication

import (
	"errors"
	"net/http"
)

// errInvalidToken is returned when the bearer token is missing or wrong.
var errInvalidToken = errors.New("invalid bearer token")

// BearerAuthenticator validates a static bearer token against a map of
// token -> consumer name. Each token is treated as a superuser within its
// own consumer namespace: the returned principal holds the wildcard role and
// therefore passes every authorization check, but its Subject (the consumer
// name) is what namespaces that caller's data from every other consumer's —
// see the `assets` domain, which never trusts a client-supplied consumer id.
type BearerAuthenticator struct {
	tokens map[string]string // token -> consumer name
}

// NewBearerAuthenticator takes the fully-resolved token->consumer map
// (config.go already applies the legacy single-API_TOKEN fallback).
func NewBearerAuthenticator(tokens map[string]string) *BearerAuthenticator {
	return &BearerAuthenticator{tokens: tokens}
}

func (a *BearerAuthenticator) Authenticate(r *http.Request) (*Principal, error) {
	raw := extractBearer(r)
	if raw == "" {
		return nil, errInvalidToken
	}
	consumer, ok := a.tokens[raw]
	if !ok {
		return nil, errInvalidToken
	}
	return &Principal{Subject: consumer, Roles: []string{wildcardRole}}, nil
}
