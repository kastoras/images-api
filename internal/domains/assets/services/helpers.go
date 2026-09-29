package assets_services

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"image"
	"io"
	"path/filepath"
	"strings"

	"github.com/disintegration/imaging"
	internal_errors "github.com/kastoras/images-api/internal/utils/errors"
)

func masterKey(consumer, tenant, hash, ext string) string {
	return fmt.Sprintf("masters/%s/%s/%s.%s", consumer, tenant, hash, ext)
}

func masterPrefix(consumer, tenant string) string {
	return fmt.Sprintf("masters/%s/%s/", consumer, tenant)
}

func derivativeKey(consumer, tenant, hash string, width, height int, mode, ext string) string {
	return fmt.Sprintf("derivatives/%s/%s/%s/%dx%d-%s.%s", consumer, tenant, hash, width, height, mode, ext)
}

func derivativePrefix(consumer, tenant, hash string) string {
	return fmt.Sprintf("derivatives/%s/%s/%s/", consumer, tenant, hash)
}

func hashFromMasterKey(key string) string {
	base := key[strings.LastIndex(key, "/")+1:]
	return strings.TrimSuffix(base, filepath.Ext(base))
}

func extForFormat(format string) string {
	if format == "png" {
		return "png"
	}
	return "jpg"
}

func formatFromExt(ext string) string {
	if ext == "png" {
		return "png"
	}
	return "jpeg"
}

func contentTypeForFormat(format string) string {
	if format == "png" {
		return "image/png"
	}
	return "image/jpeg"
}

func encodeAs(w io.Writer, img image.Image, format string, quality int) (string, error) {
	if format == "png" {
		return "png", imaging.Encode(w, img, imaging.PNG)
	}
	return "jpeg", imaging.Encode(w, img, imaging.JPEG, imaging.JPEGQuality(quality))
}

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func checkSourceMegapixels(img image.Image, maxMegapixels int) error {
	if maxMegapixels <= 0 {
		return nil
	}
	b := img.Bounds()
	pixels := int64(b.Dx()) * int64(b.Dy())
	if pixels > int64(maxMegapixels)*1_000_000 {
		return internal_errors.NewValidationError("image exceeds maximum allowed resolution")
	}
	return nil
}
