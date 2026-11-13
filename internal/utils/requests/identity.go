package requests

import (
	"net/http"
	"regexp"

	"github.com/kastoras/images-api/internal/server/authentication"
	internal_errors "github.com/kastoras/images-api/internal/utils/errors"
)

// identifierPattern bounds both consumer and tenant identifiers. tenant is
// caller-supplied, so this is a real security/robustness check (it becomes
// part of a storage key and a URL path segment), not just hygiene.
var identifierPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)

func ValidateIdentifier(name, value string) error {
	if !identifierPattern.MatchString(value) {
		return internal_errors.NewValidationError(name + " must match ^[A-Za-z0-9_-]{1,128}$")
	}
	return nil
}

// ConsumerFromContext resolves the calling consumer from the principal Auth
// middleware placed in the request context. consumer is never taken from the
// request itself, so no caller can address another consumer's namespace.
func ConsumerFromContext(r *http.Request) (string, error) {
	principal, ok := authentication.PrincipalFrom(r.Context())
	if !ok || principal.Subject == "" {
		return "", internal_errors.NewValidationError("no authenticated consumer identity")
	}
	if err := ValidateIdentifier("consumer", principal.Subject); err != nil {
		return "", err
	}
	return principal.Subject, nil
}
