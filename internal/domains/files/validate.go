package files

import (
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"
	internal_errors "github.com/kastoras/images-api/internal/utils/errors"
)

// allowedContentTypes is matched against the type sniffed from the bytes,
// never the client's header. SVG/HTML are deliberately absent: served from
// the public route they would be stored XSS.
var allowedContentTypes = map[string]bool{
	"image/jpeg":                true,
	"image/png":                 true,
	"image/webp":                true,
	"image/gif":                 true,
	"application/pdf":           true,
	"text/plain; charset=utf-8": true,
}

// uploadMemoryBytes is how much of an upload stays in RAM; the rest spills to
// a temp file so concurrent large uploads don't exhaust memory. The size cap
// itself is enforced by http.MaxBytesReader in the handler.
const uploadMemoryBytes = 1 << 20

func validateCreateRequest(r *http.Request) error {
	if err := r.ParseMultipartForm(uploadMemoryBytes); err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			return internal_errors.NewValidationError("file exceeds maximum upload size")
		}
		return internal_errors.NewValidationError("invalid multipart form")
	}

	if strings.TrimSpace(r.FormValue("tenant")) == "" {
		return internal_errors.NewValidationError("tenant is required")
	}

	if v := r.FormValue("visibility"); v != "" && v != visibilityPrivate && v != visibilityPublic {
		return internal_errors.NewValidationError("visibility must be private or public")
	}

	files := r.MultipartForm.File["file"]
	if len(files) == 0 {
		return internal_errors.NewValidationError("missing file field")
	}
	if files[0].Size == 0 {
		return internal_errors.NewValidationError("file is empty")
	}

	return nil
}

func validateID(id string) error {
	if _, err := uuid.Parse(id); err != nil {
		return internal_errors.NewValidationError("id must be a uuid")
	}
	return nil
}
