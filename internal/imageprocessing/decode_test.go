package imageprocessing

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"testing"

	internal_errors "github.com/kastoras/images-api/internal/utils/errors"
)

func newTestImage(w, h int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, color.RGBA{R: 255, G: 100, B: 50, A: 255})
		}
	}
	return img
}

func encodeToJPEGBytes(t *testing.T, img image.Image) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := EncodeJPEG(&buf, img); err != nil {
		t.Fatalf("EncodeJPEG failed: %v", err)
	}
	return buf.Bytes()
}

func TestDecode_ValidJPEG(t *testing.T) {
	src := newTestImage(64, 64)
	jpegBytes := encodeToJPEGBytes(t, src)

	img, err := Decode(bytes.NewReader(jpegBytes))
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if img == nil {
		t.Fatal("expected non-nil image")
	}
}

func TestDecode_InvalidBytes(t *testing.T) {
	_, err := Decode(bytes.NewReader([]byte("not an image")))
	if err == nil {
		t.Fatal("expected error for invalid bytes")
	}
	if !errors.Is(err, internal_errors.ErrUnsupportedFormat) {
		t.Errorf("expected ErrUnsupportedFormat, got %v", err)
	}
}

func TestDecode_EmptyReader(t *testing.T) {
	_, err := Decode(bytes.NewReader(nil))
	if err == nil {
		t.Fatal("expected error for empty reader")
	}
	if !errors.Is(err, internal_errors.ErrUnsupportedFormat) {
		t.Errorf("expected ErrUnsupportedFormat, got %v", err)
	}
}

func TestEncodeJPEG_WritesBytes(t *testing.T) {
	var buf bytes.Buffer
	if err := EncodeJPEG(&buf, newTestImage(32, 32)); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if buf.Len() == 0 {
		t.Error("expected non-empty output")
	}
}

func TestEncodeJPEG_OutputIsJPEG(t *testing.T) {
	var buf bytes.Buffer
	if err := EncodeJPEG(&buf, newTestImage(32, 32)); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	b := buf.Bytes()
	if len(b) < 2 || b[0] != 0xFF || b[1] != 0xD8 {
		t.Errorf("output does not start with JPEG magic bytes, got: %x", b[:min(4, len(b))])
	}
}
