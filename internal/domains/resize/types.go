package resize

import (
	"context"
	"io"

	"github.com/kastoras/images-api/internal/imageprocessing"
	internal_errors "github.com/kastoras/images-api/internal/utils/errors"
)

var (
	ErrUnsupportedFormat = internal_errors.ErrUnsupportedFormat
	ErrTooManyRequests   = internal_errors.ErrTooManyRequests
	ErrQueueFull         = internal_errors.ErrQueueFull
)

type Handler struct {
	service Resizer
}

type ProcessResult struct {
	URL       string // presigned URL when S3 is available
	ImageData []byte // raw JPEG when S3 is unavailable
	JobID     string // set when the request was queued
	Queued    bool
}

type resizeResult struct {
	URL string `json:"url"`
}

// Resizer is the interface the Handler depends on. *Service satisfies it.
type Resizer interface {
	Resize(ctx context.Context, file io.Reader, opts imageprocessing.ResizeOptions) (*ProcessResult, error)
}
