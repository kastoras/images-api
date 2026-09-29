package assets_services

import (
	"image"
	"testing"
)

func TestKeyBuilders(t *testing.T) {
	if got := masterKey("c", "t", "h", "jpg"); got != "masters/c/t/h.jpg" {
		t.Errorf("masterKey = %s", got)
	}
	if got := masterPrefix("c", "t"); got != "masters/c/t/" {
		t.Errorf("masterPrefix = %s", got)
	}
	if got := derivativeKey("c", "t", "h", 10, 20, "fit", "png"); got != "derivatives/c/t/h/10x20-fit.png" {
		t.Errorf("derivativeKey = %s", got)
	}
	if got := derivativePrefix("c", "t", "h"); got != "derivatives/c/t/h/" {
		t.Errorf("derivativePrefix = %s", got)
	}
	if got := hashFromMasterKey("masters/c/t/abc123.jpg"); got != "abc123" {
		t.Errorf("hashFromMasterKey = %s", got)
	}
}

func TestFormatHelpers(t *testing.T) {
	cases := []struct{ format, ext, ct string }{
		{"png", "png", "image/png"},
		{"jpeg", "jpg", "image/jpeg"},
		{"", "jpg", "image/jpeg"},
	}
	for _, c := range cases {
		if got := extForFormat(c.format); got != c.ext {
			t.Errorf("extForFormat(%q) = %s", c.format, got)
		}
		if got := contentTypeForFormat(c.format); got != c.ct {
			t.Errorf("contentTypeForFormat(%q) = %s", c.format, got)
		}
	}
	if formatFromExt("png") != "png" || formatFromExt("jpg") != "jpeg" || formatFromExt("") != "jpeg" {
		t.Error("formatFromExt mapping wrong")
	}
}

func TestSha256Hex(t *testing.T) {
	// sha256("abc")
	want := "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"
	if got := sha256Hex([]byte("abc")); got != want {
		t.Errorf("sha256Hex = %s", got)
	}
}

func TestCheckSourceMegapixels(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 2000, 1000)) // 2 MP
	if err := checkSourceMegapixels(img, 0); err != nil {
		t.Errorf("limit 0 disables the check, got %v", err)
	}
	if err := checkSourceMegapixels(img, 2); err != nil {
		t.Errorf("exactly at limit should pass, got %v", err)
	}
	if err := checkSourceMegapixels(img, 1); err == nil {
		t.Error("expected error above limit")
	}
}
