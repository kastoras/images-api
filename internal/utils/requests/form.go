package requests

import (
	"fmt"
	"mime/multipart"
	"net/http"
	"strconv"

	internal_errors "github.com/kastoras/images-api/internal/utils/errors"
	"github.com/kastoras/images-api/internal/utils/files"
)

func ParseImageFile(r *http.Request, allowedMIMETypes map[string]bool) (multipart.File, *multipart.FileHeader, error) {
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		return nil, nil, internal_errors.NewValidationError("invalid multipart form")
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		return nil, nil, internal_errors.NewValidationError("missing file field")
	}
	if !allowedMIMETypes[header.Header.Get("Content-Type")] {
		files.SafeClose(file)
		return nil, nil, internal_errors.ErrUnsupportedFormat
	}
	return file, header, nil
}

func ParseString(r *http.Request, field string) string {
	return r.FormValue(field)
}

func ParseInt(r *http.Request, field string) (int, error) {
	s := r.FormValue(field)
	if s == "" {
		return 0, internal_errors.NewValidationError(fmt.Sprintf("%s must be a positive integer", field))
	}
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return 0, internal_errors.NewValidationError(fmt.Sprintf("%s must be a positive integer", field))
	}
	return n, nil
}

// ParseOptionalInt parses a named form field as a positive integer.
// Returns 0 without error if the field is absent.
func ParseOptionalInt(r *http.Request, field string) (int, error) {
	s := r.FormValue(field)
	if s == "" {
		return 0, nil
	}
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return 0, internal_errors.NewValidationError(fmt.Sprintf("%s must be a positive integer", field))
	}
	return n, nil
}
