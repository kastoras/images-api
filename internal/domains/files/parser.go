package files

import (
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/gorilla/mux"
	internal_errors "github.com/kastoras/images-api/internal/utils/errors"
	utilfiles "github.com/kastoras/images-api/internal/utils/files"
	form "github.com/kastoras/images-api/internal/utils/requests"
)

type createRequest struct {
	Consumer string
	Upload   Upload
	File     multipart.File
}

func parseCreateRequest(r *http.Request) (*createRequest, error) {
	consumer, err := form.ConsumerFromContext(r)
	if err != nil {
		return nil, err
	}

	tenant := strings.TrimSpace(r.FormValue("tenant"))
	if err := form.ValidateIdentifier("tenant", tenant); err != nil {
		return nil, err
	}

	visibility := r.FormValue("visibility")
	if visibility == "" {
		visibility = visibilityPrivate
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		return nil, internal_errors.NewValidationError("missing file field")
	}

	contentType, err := sniffContentType(file)
	if err != nil {
		utilfiles.SafeClose(file)
		return nil, err
	}
	if !allowedContentTypes[contentType] {
		utilfiles.SafeClose(file)
		return nil, internal_errors.NewValidationError("unsupported file type " + contentType + ": use JPEG, PNG, WebP, GIF, PDF or plain text")
	}

	return &createRequest{
		Consumer: consumer,
		File:     file,
		Upload: Upload{
			Tenant:      tenant,
			Filename:    sanitizeFilename(header.Filename),
			ContentType: contentType,
			Visibility:  visibility,
			Size:        header.Size,
			Body:        file,
		},
	}, nil
}

// sniffContentType detects the type from the first 512 bytes, then rewinds
// so the full file is uploaded.
func sniffContentType(f io.ReadSeeker) (string, error) {
	buf := make([]byte, 512)
	n, err := io.ReadFull(f, buf)
	if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
		return "", err
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return "", err
	}
	return http.DetectContentType(buf[:n]), nil
}

// sanitizeFilename keeps the name safe for S3 metadata (ASCII only) and for
// a quoted Content-Disposition header.
func sanitizeFilename(name string) string {
	name = filepath.Base(strings.ReplaceAll(name, `\`, "/"))
	var b strings.Builder
	for _, c := range name {
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9', c == '.', c == '-', c == '_':
			b.WriteRune(c)
		default:
			b.WriteRune('_')
		}
	}
	out := b.String()
	if len(out) > 128 {
		out = out[len(out)-128:] // keep the extension
	}
	if strings.Trim(out, "._") == "" {
		return "file"
	}
	return out
}

func parseTenantAndID(r *http.Request) (tenant, id string, err error) {
	vars := mux.Vars(r)
	tenant, id = vars["tenant"], vars["id"]
	if err := form.ValidateIdentifier("tenant", tenant); err != nil {
		return "", "", err
	}
	if err := validateID(id); err != nil {
		return "", "", err
	}
	return tenant, id, nil
}

func parsePublicPath(r *http.Request) (consumer, tenant, id string, err error) {
	consumer = mux.Vars(r)["consumer"]
	if err := form.ValidateIdentifier("consumer", consumer); err != nil {
		return "", "", "", err
	}
	tenant, id, err = parseTenantAndID(r)
	return consumer, tenant, id, err
}
