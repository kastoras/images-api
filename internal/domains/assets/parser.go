package assets

import (
	"mime/multipart"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	"github.com/kastoras/images-api/internal/imageprocessing"
	"github.com/kastoras/images-api/internal/server/authentication"
	internal_errors "github.com/kastoras/images-api/internal/utils/errors"
	"github.com/kastoras/images-api/internal/utils/files"
	form "github.com/kastoras/images-api/internal/utils/requests"
)

// consumerFromContext resolves the calling consumer from the principal Auth
// middleware placed in the request context. This is the whole point of the
// two-level tenancy model: consumer is never taken from the request itself,
// so no caller can address another consumer's namespace.
func consumerFromContext(r *http.Request) (string, error) {
	principal, ok := authentication.PrincipalFrom(r.Context())
	if !ok || principal.Subject == "" {
		return "", internal_errors.NewValidationError("no authenticated consumer identity")
	}
	if err := validateIdentifier("consumer", principal.Subject); err != nil {
		return "", err
	}
	return principal.Subject, nil
}

type createRequest struct {
	Consumer string
	Tenant   string
	File     multipart.File
}

func parseCreateRequest(r *http.Request) (*createRequest, error) {
	consumer, err := consumerFromContext(r)
	if err != nil {
		return nil, err
	}

	tenant := strings.TrimSpace(r.FormValue("tenant"))
	if err := validateIdentifier("tenant", tenant); err != nil {
		return nil, err
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		return nil, internal_errors.NewValidationError("missing file field")
	}
	if !allowedMIMETypes[header.Header.Get("Content-Type")] {
		files.SafeClose(file)
		return nil, internal_errors.ErrUnsupportedFormat
	}

	return &createRequest{Consumer: consumer, Tenant: tenant, File: file}, nil
}

type renderRequest struct {
	Consumer string
	Tenant   string
	Hash     string
	Width    int
	Height   int
	Mode     imageprocessing.ResizeMode
	Format   string
}

// defaultRenderMode is "fit" rather than resize's "exact" default: this is a
// public, general-purpose render endpoint, and fitting within bounds without
// distortion is the safer default when a caller omits mode.
const defaultRenderMode = imageprocessing.ResizeModeFit

func parseRenderRequest(r *http.Request, maxMegapixels int) (*renderRequest, error) {
	vars := mux.Vars(r)
	consumer, tenant, hash := vars["consumer"], vars["tenant"], vars["hash"]

	if err := validateIdentifier("consumer", consumer); err != nil {
		return nil, err
	}
	if err := validateIdentifier("tenant", tenant); err != nil {
		return nil, err
	}
	if err := validateHash(hash); err != nil {
		return nil, err
	}

	width, err := form.ParseOptionalInt(r, "width")
	if err != nil {
		return nil, err
	}
	height, err := form.ParseOptionalInt(r, "height")
	if err != nil {
		return nil, err
	}
	if width == 0 && height == 0 {
		return nil, internal_errors.NewValidationError("at least one of width or height must be provided")
	}

	mode, err := parseRenderMode(r.FormValue("mode"))
	if err != nil {
		return nil, err
	}
	if (width == 0 || height == 0) && (mode == imageprocessing.ResizeModeFit || mode == imageprocessing.ResizeModeFill) {
		return nil, internal_errors.NewValidationError("mode=fit and mode=fill require both width and height")
	}

	if err := checkRequestedDimensions(width, height, maxMegapixels); err != nil {
		return nil, err
	}

	format, err := parseFormat(r.FormValue("format"))
	if err != nil {
		return nil, err
	}

	return &renderRequest{Consumer: consumer, Tenant: tenant, Hash: hash, Width: width, Height: height, Mode: mode, Format: format}, nil
}

func parseRenderMode(s string) (imageprocessing.ResizeMode, error) {
	mode := imageprocessing.ResizeMode(s)
	if mode == "" {
		mode = defaultRenderMode
	}
	switch mode {
	case imageprocessing.ResizeModeExact, imageprocessing.ResizeModeFit, imageprocessing.ResizeModeFill:
		return mode, nil
	default:
		return "", internal_errors.NewValidationError("mode must be one of: exact, fit, fill")
	}
}

// parseFormat returns "" when unspecified, letting the service fall back to
// the master's own stored format.
func parseFormat(s string) (string, error) {
	switch s {
	case "":
		return "", nil
	case "jpeg", "jpg":
		return "jpeg", nil
	case "png":
		return "png", nil
	default:
		return "", internal_errors.NewValidationError("format must be jpeg or png")
	}
}

func checkRequestedDimensions(width, height, maxMegapixels int) error {
	if maxMegapixels <= 0 {
		return nil
	}
	w, h := width, height
	if w == 0 {
		w = h
	}
	if h == 0 {
		h = w
	}
	if int64(w)*int64(h) > int64(maxMegapixels)*1_000_000 {
		return internal_errors.NewValidationError("requested render size exceeds maximum allowed resolution")
	}
	return nil
}

func parseTenantAndHash(r *http.Request) (tenant, hash string, err error) {
	vars := mux.Vars(r)
	tenant, hash = vars["tenant"], vars["hash"]
	if err := validateIdentifier("tenant", tenant); err != nil {
		return "", "", err
	}
	if err := validateHash(hash); err != nil {
		return "", "", err
	}
	return tenant, hash, nil
}

func parseTenant(r *http.Request) (string, error) {
	tenant := strings.TrimSpace(r.URL.Query().Get("tenant"))
	if err := validateIdentifier("tenant", tenant); err != nil {
		return "", err
	}
	return tenant, nil
}
