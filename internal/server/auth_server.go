package server

import (
	"net/http"
	"strings"

	"github.com/kastoras/images-api/internal/config"
)

type AuthServer struct {
	apiToken       string
	zitadelEnabled bool
}

func initAuthServer(cfg *config.Config) *AuthServer {
	return &AuthServer{
		apiToken:       cfg.APIToken,
		zitadelEnabled: cfg.ZitadelEnabled,
	}
}

// NewAuthServer constructs an AuthServer directly, without requiring a Config.
// Intended for tests and lightweight wiring.
func NewAuthServer(apiToken string, zitadelEnabled bool) *AuthServer {
	return &AuthServer{
		apiToken:       apiToken,
		zitadelEnabled: zitadelEnabled,
	}
}

// Validate checks the Authorization: Bearer <token> header.
// Zitadel JWT validation is wired in Phase 5.
func (a *AuthServer) Validate(r *http.Request) bool {
	header := r.Header.Get("Authorization")
	if !strings.HasPrefix(header, "Bearer ") {
		return false
	}
	token := strings.TrimPrefix(header, "Bearer ")
	if a.zitadelEnabled {
		return false
	}
	return token == a.apiToken
}
