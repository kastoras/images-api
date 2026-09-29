package assets

import (
	"net/http"
	"regexp"
	"strings"

	internal_errors "github.com/kastoras/images-api/internal/utils/errors"
)

// Input allow-list for POST /api/v1/assets, wider than resize's
// imageprocessing.SupportedMIMETypes (adds webp) — scoped to this domain
// only so resize's documented contract doesn't change.
var allowedMIMETypes = map[string]bool{
	"image/jpeg": true,
	"image/png":  true,
	"image/webp": true,
}

// identifierPattern bounds both consumer and tenant identifiers. tenant is
// caller-supplied, so this is a real security/robustness check (it becomes
// part of a storage key and a URL path segment), not just hygiene.
var identifierPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)

// hashPattern matches the sha256 hex digest asset ids are addressed by.
var hashPattern = regexp.MustCompile(`^[a-f0-9]{64}$`)

func validateIdentifier(name, value string) error {
	if !identifierPattern.MatchString(value) {
		return internal_errors.NewValidationError(name + " must match ^[A-Za-z0-9_-]{1,128}$")
	}
	return nil
}

func validateHash(hash string) error {
	if !hashPattern.MatchString(hash) {
		return internal_errors.NewValidationError("hash must be a 64-character sha256 hex digest")
	}
	return nil
}

func validateCreateRequest(r *http.Request, maxUploadSizeBytes int) error {
	if err := r.ParseMultipartForm(int64(maxUploadSizeBytes)); err != nil {
		return internal_errors.NewValidationError("invalid multipart form")
	}

	if strings.TrimSpace(r.FormValue("tenant")) == "" {
		return internal_errors.NewValidationError("tenant is required")
	}

	files := r.MultipartForm.File["file"]
	if len(files) == 0 {
		return internal_errors.NewValidationError("missing file field")
	}
	if !allowedMIMETypes[files[0].Header.Get("Content-Type")] {
		return internal_errors.ErrUnsupportedFormat
	}

	return nil
}
