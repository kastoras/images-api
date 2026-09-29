package assets

import (
	"errors"
	"net/http"

	"github.com/kastoras/images-api/internal/imageprocessing"
	"github.com/kastoras/images-api/internal/server"
	"github.com/kastoras/images-api/internal/utils/files"
	"github.com/kastoras/images-api/internal/utils/responses"
)

func NewHandler(s *server.APIServer, svc Servicer) *Handler {
	return &Handler{server: s, service: svc}
}

// Create handles POST /api/v1/assets.
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	if h.server.Storage == nil {
		responses.ServiceUnavailable(w, "asset storage is not configured")
		return
	}

	if err := validateCreateRequest(r, h.server.MaxUploadSizeBytes); err != nil {
		responses.ErrorResponse(w, err)
		return
	}

	req, err := parseCreateRequest(r)
	if err != nil {
		responses.ErrorResponse(w, err)
		return
	}
	defer files.SafeClose(req.File)

	master, err := h.service.Create(r.Context(), req.Consumer, req.Tenant, req.File)
	if err != nil {
		responses.ErrorResponse(w, err)
		return
	}

	responses.Created(w, master)
}

// Render handles GET /a/{consumer}/{tenant}/{hash}/render. Public: no auth.
func (h *Handler) Render(w http.ResponseWriter, r *http.Request) {
	if h.server.Storage == nil {
		responses.ServiceUnavailable(w, "asset storage is not configured")
		return
	}

	req, err := parseRenderRequest(r, h.server.MaxSourceMegapixels)
	if err != nil {
		responses.ErrorResponse(w, err)
		return
	}

	data, contentType, err := h.service.Render(r.Context(), req.Consumer, req.Tenant, req.Hash, imageprocessing.ResizeOptions{
		Width:  req.Width,
		Height: req.Height,
		Mode:   req.Mode,
	}, req.Format)
	if err != nil {
		if errors.Is(err, server.ErrNotFound) {
			responses.NotFound(w)
			return
		}
		responses.ErrorResponse(w, err)
		return
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

// Delete handles DELETE /api/v1/assets/{tenant}/{hash}. consumer is resolved
// from the authenticated principal, never accepted from the path.
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	if h.server.Storage == nil {
		responses.ServiceUnavailable(w, "asset storage is not configured")
		return
	}

	consumer, err := consumerFromContext(r)
	if err != nil {
		responses.ErrorResponse(w, err)
		return
	}

	tenant, hash, err := parseTenantAndHash(r)
	if err != nil {
		responses.ErrorResponse(w, err)
		return
	}

	if err := h.service.Delete(r.Context(), consumer, tenant, hash); err != nil {
		responses.ErrorResponse(w, err)
		return
	}

	responses.NoContent(w)
}

// List handles GET /api/v1/assets?tenant=. consumer is resolved from the
// authenticated principal, never accepted as a query parameter.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	if h.server.Storage == nil {
		responses.ServiceUnavailable(w, "asset storage is not configured")
		return
	}

	consumer, err := consumerFromContext(r)
	if err != nil {
		responses.ErrorResponse(w, err)
		return
	}

	tenant, err := parseTenant(r)
	if err != nil {
		responses.ErrorResponse(w, err)
		return
	}

	list, err := h.service.List(r.Context(), consumer, tenant)
	if err != nil {
		responses.ErrorResponse(w, err)
		return
	}

	responses.Success(w, assetsList{Tenant: tenant, Assets: list})
}
