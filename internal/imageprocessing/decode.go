package imageprocessing

import (
	"image"
	"io"

	"github.com/disintegration/imaging"
	internal_errors "github.com/kastoras/images-api/internal/utils/errors"

	// Registers webp decoding with the stdlib image package (self-registers
	// via init()) so imaging.Decode can read webp input. Encode-only formats
	// never need this — masters/derivatives are always re-encoded as
	// JPEG/PNG, so no webp encoder is needed.
	_ "golang.org/x/image/webp"
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

// EncodeMaster re-encodes img as PNG (if it has an alpha channel, so
// transparency survives) or JPEG at the given quality otherwise, and reports
// which format it chose ("png" or "jpeg").
func EncodeMaster(w io.Writer, img image.Image, jpegQuality int) (format string, err error) {
	if hasAlphaChannel(img) {
		return "png", imaging.Encode(w, img, imaging.PNG)
	}
	return "jpeg", imaging.Encode(w, img, imaging.JPEG, imaging.JPEGQuality(jpegQuality))
}

// hasAlphaChannel reports whether img has any non-opaque pixel. img is
// always *image.NRGBA here: every imaging resize/fit call returns that
// concrete type regardless of the source format, so a color-model check
// alone can't tell a JPEG from a transparent PNG post-resize — only a real
// scan of the alpha byte can. Resizing preserves opacity (opaque stays
// opaque), so scanning the already-resized buffer is still correct.
func hasAlphaChannel(img image.Image) bool {
	nrgba, ok := img.(*image.NRGBA)
	if !ok {
		return false
	}
	for i := 3; i < len(nrgba.Pix); i += 4 {
		if nrgba.Pix[i] != 0xff {
			return true
		}
	}
	return false
}
