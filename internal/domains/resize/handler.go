package resize

import (
	"net/http"

	"github.com/kastoras/images-api/internal/imageprocessing"
	"github.com/kastoras/images-api/internal/utils/files"
	"github.com/kastoras/images-api/internal/utils/responses"
)

func NewHandler(svc *Service) *Handler {
	return &Handler{service: svc}
}

func (h *Handler) Resize(w http.ResponseWriter, r *http.Request) {

	err := validateRequest(r)
	if err != nil {
		responses.ErrorResponse(w, err)
		return
	}

	req, err := parseResizeRequest(r)
	if err != nil {
		responses.ErrorResponse(w, err)
		return
	}
	defer files.SafeClose(req.File)

	resOptions := imageprocessing.ResizeOptions{
		Width:  req.Width,
		Height: req.Height,
		Mode:   req.Mode,
	}
	result, err := h.service.Resize(r.Context(), req.File, resOptions)
	if err != nil {
		responses.ErrorResponse(w, err)
		return
	}

	if result.Queued {
		responses.Accepted(w, result.JobID)
		return
	}

	if result.ImageData != nil {
		responses.JPEGImage(w, result.ImageData)
		return
	}

	responses.Success(w, resizeResult{URL: result.URL})
}
