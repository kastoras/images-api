package assets

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	internal_errors "github.com/kastoras/images-api/internal/utils/errors"
)

func TestValidateIdentifier(t *testing.T) {
	valid := []string{"acme", "Tenant_1", "a-b", strings.Repeat("x", 128)}
	invalid := []string{"", "a/b", "a b", "../x", "a.b", strings.Repeat("x", 129)}
	for _, v := range valid {
		if err := validateIdentifier("tenant", v); err != nil {
			t.Errorf("%q should be valid: %v", v, err)
		}
	}
	for _, v := range invalid {
		if err := validateIdentifier("tenant", v); !isValidationError(err) {
			t.Errorf("%q should be rejected, got %v", v, err)
		}
	}
}

func TestValidateHash(t *testing.T) {
	if err := validateHash(validHash); err != nil {
		t.Errorf("valid hash rejected: %v", err)
	}
	for _, h := range []string{"", "abc", strings.Repeat("A", 64), strings.Repeat("g", 64), validHash + "a"} {
		if err := validateHash(h); !isValidationError(err) {
			t.Errorf("%q should be rejected, got %v", h, err)
		}
	}
}

func TestValidateCreateRequest(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		if err := validateCreateRequest(multipartRequest(t, "t1", []byte("x"), "image/webp"), 1<<20); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
	t.Run("missing tenant", func(t *testing.T) {
		if err := validateCreateRequest(multipartRequest(t, "", []byte("x"), "image/png"), 1<<20); !isValidationError(err) {
			t.Errorf("got %v", err)
		}
	})
	t.Run("missing file", func(t *testing.T) {
		if err := validateCreateRequest(multipartRequest(t, "t1", nil, ""), 1<<20); !isValidationError(err) {
			t.Errorf("got %v", err)
		}
	})
	t.Run("unsupported type", func(t *testing.T) {
		if err := validateCreateRequest(multipartRequest(t, "t1", []byte("x"), "image/gif"), 1<<20); !errors.Is(err, internal_errors.ErrUnsupportedFormat) {
			t.Errorf("got %v", err)
		}
	})
	t.Run("not multipart", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("x"))
		if err := validateCreateRequest(req, 1<<20); !isValidationError(err) {
			t.Errorf("got %v", err)
		}
	})
}
