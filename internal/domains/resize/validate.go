package resize

import (
	"net/http"

	"github.com/kastoras/images-api/internal/imageprocessing"
	internal_errors "github.com/kastoras/images-api/internal/utils/errors"
	form "github.com/kastoras/images-api/internal/utils/requests"
)

func validateRequest(r *http.Request) error {
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		return internal_errors.NewValidationError("invalid multipart form")
	}

	files := r.MultipartForm.File["file"]
	if len(files) == 0 {
		return internal_errors.NewValidationError("missing file field")
	}
	if !imageprocessing.SupportedMIMETypes[files[0].Header.Get("Content-Type")] {
		return internal_errors.ErrUnsupportedFormat
	}

	width, err := form.ParseOptionalInt(r, "width")
	if err != nil {
		return err
	}
	height, err := form.ParseOptionalInt(r, "height")
	if err != nil {
		return err
	}
	mode, err := parseMode(r.FormValue("mode"))
	if err != nil {
		return err
	}

	return validateDimensions(width, height, mode)
}
