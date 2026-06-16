package resize

import (
	"mime/multipart"
	"net/http"
	"strconv"

	internal_errors "github.com/kastoras/images-api/internal/utils/errors"
	"github.com/kastoras/images-api/internal/utils/files"
)

var supportedTypes = map[string]bool{
	"image/jpeg": true,
	"image/png":  true,
	"image/gif":  true,
	"image/tiff": true,
}

type resizeRequest struct {
	File   multipart.File
	Width  int
	Height int
	Mode   ResizeMode
}

func parseResizeRequest(r *http.Request) (*resizeRequest, error) {
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		return nil, internal_errors.NewValidationError("invalid multipart form")
	}

	file, fileHeader, err := r.FormFile("file")
	if err != nil {
		return nil, internal_errors.NewValidationError("missing file field")
	}
	if !supportedTypes[fileHeader.Header.Get("Content-Type")] {
		defer files.SafeClose(file)
		return nil, internal_errors.ErrUnsupportedFormat
	}

	width, err := parseWidth(r.FormValue("width"))
	if err != nil {
		defer files.SafeClose(file)
		return nil, err
	}

	height, err := parseHeight(r.FormValue("height"))
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

func parseWidth(s string) (int, error) {
	if s == "" {
		return 0, nil
	}
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return 0, internal_errors.NewValidationError("width must be a positive integer")
	}
	return n, nil
}

func parseHeight(s string) (int, error) {
	if s == "" {
		return 0, nil
	}
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return 0, internal_errors.NewValidationError("height must be a positive integer")
	}
	return n, nil
}

func parseMode(s string) (ResizeMode, error) {
	mode := ResizeMode(s)
	if mode == "" {
		mode = ResizeModeExact
	}
	switch mode {
	case ResizeModeExact, ResizeModeFit, ResizeModeFill:
		return mode, nil
	default:
		return "", internal_errors.NewValidationError("mode must be one of: exact, fit, fill")
	}
}

func validateDimensions(width, height int, mode ResizeMode) error {
	if width == 0 && height == 0 {
		return internal_errors.NewValidationError("at least one of width or height must be provided")
	}
	if (width == 0 || height == 0) && (mode == ResizeModeFit || mode == ResizeModeFill) {
		return internal_errors.NewValidationError("mode=fit and mode=fill require both width and height")
	}
	return nil
}
