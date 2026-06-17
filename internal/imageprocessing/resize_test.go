package imageprocessing

import (
	"testing"
)

func TestResize_ExactMode(t *testing.T) {
	src := newTestImage(100, 100)
	out := Resize(src, ResizeOptions{Width: 50, Height: 30, Mode: ResizeModeExact})
	b := out.Bounds()
	if b.Dx() != 50 || b.Dy() != 30 {
		t.Errorf("expected 50x30, got %dx%d", b.Dx(), b.Dy())
	}
}

func TestResize_FitMode(t *testing.T) {
	src := newTestImage(100, 100)
	out := Resize(src, ResizeOptions{Width: 50, Height: 30, Mode: ResizeModeFit})
	b := out.Bounds()
	if b.Dx() > 50 || b.Dy() > 30 {
		t.Errorf("fit result %dx%d exceeds bounds 50x30", b.Dx(), b.Dy())
	}
}

func TestResize_FillMode(t *testing.T) {
	src := newTestImage(100, 100)
	out := Resize(src, ResizeOptions{Width: 50, Height: 30, Mode: ResizeModeFill})
	b := out.Bounds()
	if b.Dx() != 50 || b.Dy() != 30 {
		t.Errorf("expected 50x30, got %dx%d", b.Dx(), b.Dy())
	}
}

func TestResize_WidthOnly(t *testing.T) {
	src := newTestImage(100, 80)
	out := Resize(src, ResizeOptions{Width: 50, Height: 0, Mode: ResizeModeExact})
	b := out.Bounds()
	if b.Dx() != 50 {
		t.Errorf("expected width 50, got %d", b.Dx())
	}
}

func TestResize_HeightOnly(t *testing.T) {
	src := newTestImage(100, 80)
	out := Resize(src, ResizeOptions{Width: 0, Height: 40, Mode: ResizeModeExact})
	b := out.Bounds()
	if b.Dy() != 40 {
		t.Errorf("expected height 40, got %d", b.Dy())
	}
}

func TestResize_DefaultModeIsExact(t *testing.T) {
	src := newTestImage(100, 100)
	out := Resize(src, ResizeOptions{Width: 60, Height: 40})
	b := out.Bounds()
	if b.Dx() != 60 || b.Dy() != 40 {
		t.Errorf("expected 60x40, got %dx%d", b.Dx(), b.Dy())
	}
}
