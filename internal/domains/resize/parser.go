package resize

import (
	"mime/multipart"
	"net/http"

	"github.com/kastoras/images-api/internal/imageprocessing"
	internal_errors "github.com/kastoras/images-api/internal/utils/errors"
	"github.com/kastoras/images-api/internal/utils/files"
	form "github.com/kastoras/images-api/internal/utils/requests"
)

type resizeRequest struct {
	File   multipart.File
	Width  int
	Height int
	Mode   imageprocessing.ResizeMode
}

func parseResizeRequest(r *http.Request) (*resizeRequest, error) {
	file, _, err := form.ParseImageFile(r, imageprocessing.SupportedMIMETypes)
	if err != nil {
		return nil, err
	}

	width, err := form.ParseOptionalInt(r, "width")
	if err != nil {
		defer files.SafeClose(file)
		return nil, err
	}

	height, err := form.ParseOptionalInt(r, "height")
	if err != nil {
		defer files.SafeClose(file)
		return nil, err
	}

	mode, err := parseMode(r.FormValue("mode"))
	if err != nil {
		defer files.SafeClose(file)
		return nil, err
	}

	if err := validateDimensions(width, height, mode); err != nil {
		defer files.SafeClose(file)
		return nil, err
	}

	return &resizeRequest{File: file, Width: width, Height: height, Mode: mode}, nil
}

func parseMode(s string) (imageprocessing.ResizeMode, error) {
	mode := imageprocessing.ResizeMode(s)
	if mode == "" {
		mode = imageprocessing.ResizeModeExact
	}
	switch mode {
	case imageprocessing.ResizeModeExact, imageprocessing.ResizeModeFit, imageprocessing.ResizeModeFill:
		return mode, nil
	default:
		return "", internal_errors.NewValidationError("mode must be one of: exact, fit, fill")
	}
}

func validateDimensions(width, height int, mode imageprocessing.ResizeMode) error {
	if width == 0 && height == 0 {
		return internal_errors.NewValidationError("at least one of width or height must be provided")
	}
	if (width == 0 || height == 0) && (mode == imageprocessing.ResizeModeFit || mode == imageprocessing.ResizeModeFill) {
		return internal_errors.NewValidationError("mode=fit and mode=fill require both width and height")
	}
	return nil
}
