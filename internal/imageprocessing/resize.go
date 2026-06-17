package imageprocessing

import (
	"image"

	"github.com/disintegration/imaging"
)

type ResizeMode string

const (
	ResizeModeExact ResizeMode = "exact"
	ResizeModeFit   ResizeMode = "fit"
	ResizeModeFill  ResizeMode = "fill"
)

type ResizeOptions struct {
	Width  int
	Height int
	Mode   ResizeMode
}

func Resize(src image.Image, opts ResizeOptions) image.Image {
	switch opts.Mode {
	case ResizeModeFit:
		return imaging.Fit(src, opts.Width, opts.Height, imaging.Lanczos)
	case ResizeModeFill:
		return imaging.Fill(src, opts.Width, opts.Height, imaging.Center, imaging.Lanczos)
	default:
		return imaging.Resize(src, opts.Width, opts.Height, imaging.Lanczos)
	}
}
