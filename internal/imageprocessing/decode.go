package imageprocessing

import (
	"image"
	"io"

	"github.com/disintegration/imaging"
	internal_errors "github.com/kastoras/images-api/internal/utils/errors"
)

var SupportedMIMETypes = map[string]bool{
	"image/jpeg": true,
	"image/png":  true,
	"image/gif":  true,
	"image/tiff": true,
}

func Decode(r io.Reader) (image.Image, error) {
	img, err := imaging.Decode(r)
	if err != nil {
		return nil, internal_errors.ErrUnsupportedFormat
	}
	return img, nil
}

func EncodeJPEG(w io.Writer, img image.Image) error {
	return imaging.Encode(w, img, imaging.JPEG)
}
