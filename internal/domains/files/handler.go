package files

import (
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/kastoras/images-api/internal/server"
	utilfiles "github.com/kastoras/images-api/internal/utils/files"
	form "github.com/kastoras/images-api/internal/utils/requests"
	"github.com/kastoras/images-api/internal/utils/responses"
)

func NewHandler(s *server.APIServer, svc Servicer) *Handler {
	return &Handler{server: s, service: svc}
}

// Create handles POST /api/v1/files.
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	if h.server.Storage == nil {
		responses.ServiceUnavailable(w, "file storage is not configured")
		return
	}

	// Hard cap on the body; ParseMultipartForm's argument only limits memory.
	// 1MB of slack for the multipart framing and form fields.
	r.Body = http.MaxBytesReader(w, r.Body, int64(h.server.MaxUploadSizeBytes)+1<<20)

	if err := validateCreateRequest(r); err != nil {
		responses.ErrorResponse(w, err)
		return
	}

	req, err := parseCreateRequest(r)
	if err != nil {
		responses.ErrorResponse(w, err)
		return
	}
	defer utilfiles.SafeClose(req.File)

	file, err := h.service.Create(r.Context(), req.Consumer, req.Upload)
	if err != nil {
		responses.ErrorResponse(w, err)
		return
	}

	responses.Created(w, file)
}

// Get handles GET /api/v1/files/{tenant}/{id}: any visibility, own consumer only.
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	if h.server.Storage == nil {
		responses.ServiceUnavailable(w, "file storage is not configured")
		return
	}

	consumer, err := form.ConsumerFromContext(r)
	if err != nil {
		responses.ErrorResponse(w, err)
		return
	}

	tenant, id, err := parseTenantAndID(r)
	if err != nil {
		responses.ErrorResponse(w, err)
		return
	}

	h.serve(w, r, consumer, tenant, id, false)
}

// GetPublic handles GET /f/{consumer}/{tenant}/{id}. No auth: only files
// uploaded with visibility=public are served; anything else is a 404.
func (h *Handler) GetPublic(w http.ResponseWriter, r *http.Request) {
	if h.server.Storage == nil {
		responses.ServiceUnavailable(w, "file storage is not configured")
		return
	}

	consumer, tenant, id, err := parsePublicPath(r)
	if err != nil {
		responses.ErrorResponse(w, err)
		return
	}

	h.serve(w, r, consumer, tenant, id, true)
}

func (h *Handler) serve(w http.ResponseWriter, r *http.Request, consumer, tenant, id string, publicOnly bool) {
	obj, err := h.service.Open(r.Context(), consumer, tenant, id, publicOnly)
	if err != nil {
		if errors.Is(err, server.ErrNotFound) {
			responses.NotFound(w)
			return
		}
		responses.ErrorResponse(w, err)
		return
	}
	defer func() { _ = obj.Body.Close() }()

	w.Header().Set("Content-Type", obj.ContentType)
	w.Header().Set("Content-Length", strconv.FormatInt(obj.Size, 10))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if !strings.HasPrefix(obj.ContentType, "image/") {
		w.Header().Set("Content-Disposition", `attachment; filename="`+obj.Filename+`"`)
	}
	if publicOnly {
		w.Header().Set("Cache-Control", "public, max-age=3600")
	} else {
		w.Header().Set("Cache-Control", "private, no-store")
	}
	w.WriteHeader(http.StatusOK)

	if _, err := io.Copy(w, obj.Body); err != nil {
		// Headers are already sent; all we can do is log.
		h.server.Log.Warn().Err(err).Str("tenant", tenant).Str("id", id).Msg("file stream interrupted")
	}
}

// Delete handles DELETE /api/v1/files/{tenant}/{id}.
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	if h.server.Storage == nil {
		responses.ServiceUnavailable(w, "file storage is not configured")
		return
	}

	consumer, err := form.ConsumerFromContext(r)
	if err != nil {
		responses.ErrorResponse(w, err)
		return
	}

	tenant, id, err := parseTenantAndID(r)
	if err != nil {
		responses.ErrorResponse(w, err)
		return
	}

	if err := h.service.Delete(r.Context(), consumer, tenant, id); err != nil {
		responses.ErrorResponse(w, err)
		return
	}

	responses.NoContent(w)
}
