package files

import "testing"

func TestSniffVideo(t *testing.T) {
	cases := []struct {
		name string
		head []byte
		want string
	}{
		{"mp4 with mp42 brand", mp4Bytes, "video/mp4"},
		{"iso-only mp4", []byte("\x00\x00\x00\x14ftypisom\x00\x00\x02\x00isom"), "video/mp4"},
		{"quicktime", movBytes, "video/quicktime"},
		{"quicktime as compatible brand", []byte("\x00\x00\x00\x18ftypM4V \x00\x00\x00\x00qt  M4V "), "video/quicktime"},
		{"other ftyp brand (HEIC image)", []byte("\x00\x00\x00\x18ftypheic\x00\x00\x00\x00mif1heic"), ""},
		{"png", pngBytes, ""},
		{"too short", []byte("\x00\x00\x00\x08ftyp"), ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := sniffVideo(tc.head); got != tc.want {
				t.Fatalf("sniffVideo = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestIsMedia(t *testing.T) {
	for ct, want := range map[string]bool{"image/png": true, "video/mp4": true, "video/quicktime": true, "application/pdf": false, "text/plain; charset=utf-8": false} {
		if got := isMedia(ct); got != want {
			t.Errorf("isMedia(%q) = %v, want %v", ct, got, want)
		}
	}
}
