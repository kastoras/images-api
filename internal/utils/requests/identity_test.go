package requests

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/kastoras/images-api/internal/server/authentication"
	internal_errors "github.com/kastoras/images-api/internal/utils/errors"
)

func isValidationError(err error) bool {
	var v *internal_errors.ValidationError
	return errors.As(err, &v)
}

func withPrincipal(r *http.Request, subject string) *http.Request {
	return r.WithContext(authentication.WithPrincipal(r.Context(), &authentication.Principal{Subject: subject}))
}

func TestValidateIdentifier(t *testing.T) {
	valid := []string{"acme", "Tenant_1", "a-b", strings.Repeat("x", 128)}
	invalid := []string{"", "a/b", "a b", "../x", "a.b", strings.Repeat("x", 129)}
	for _, v := range valid {
		if err := ValidateIdentifier("tenant", v); err != nil {
			t.Errorf("%q should be valid: %v", v, err)
		}
	}
	for _, v := range invalid {
		if err := ValidateIdentifier("tenant", v); !isValidationError(err) {
			t.Errorf("%q should be rejected, got %v", v, err)
		}
	}
}

func TestConsumerFromContext(t *testing.T) {
	base := httptest.NewRequest(http.MethodGet, "/", nil)

	if _, err := ConsumerFromContext(base); !isValidationError(err) {
		t.Errorf("no principal: got %v", err)
	}
	if _, err := ConsumerFromContext(withPrincipal(base, "")); !isValidationError(err) {
		t.Errorf("empty subject: got %v", err)
	}
	if _, err := ConsumerFromContext(withPrincipal(base, "bad/consumer")); !isValidationError(err) {
		t.Errorf("invalid subject: got %v", err)
	}
	if got, err := ConsumerFromContext(withPrincipal(base, "acme")); err != nil || got != "acme" {
		t.Errorf("got %q, %v", got, err)
	}
}
