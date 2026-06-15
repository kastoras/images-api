package authentication

import (
	"context"
	"fmt"

	"github.com/kastoras/images-api/internal/config"
	"github.com/rs/zerolog"
)

// NewAuthenticator selects and initialises the appropriate Authenticator based
// on config. Add new IDP implementations here — the Authenticator interface and
// all callers remain unchanged.
func NewAuthenticator(ctx context.Context, cfg *config.Config, log zerolog.Logger) (Authenticator, error) {

	switch cfg.AuthenticationType {
	case "zitadel":
		auth, err := NewZitadelAuthenticator(ctx, cfg.ZitadelIssuer, cfg.ZitadelAudience)
		if err != nil {
			return nil, fmt.Errorf("zitadel auth: %w", err)
		}
		log.Info().Str("issuer", cfg.ZitadelIssuer).Msg("zitadel auth initialized")
		return auth, nil
	default:
		log.Info().Msg("bearer token auth initialized")
		return NewBearerAuthenticator(cfg.APIToken), nil
	}
}
