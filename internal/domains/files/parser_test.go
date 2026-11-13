package files

import (
	"io"
	"strings"
	"testing"
)

func TestValidateAndParseCreateRequest(t *testing.T) {
	cases := []struct {
		name       string
		fields     map[string]string
		header     string
		file       []byte
		wantErr    bool
		wantType   string
		wantVisibl string
	}{
		{"pdf", map[string]string{"tenant": "t1"}, "application/pdf", pdfBytes, false, "application/pdf", "private"},
		{"png public", map[string]string{"tenant": "t1", "visibility": "public"}, "image/png", pngBytes, false, "image/png", "public"},
		{"txt", map[string]string{"tenant": "t1"}, "text/plain", txtBytes, false, "text/plain; charset=utf-8", "private"},
		{"html spoofed as png", map[string]string{"tenant": "t1"}, "image/png", htmlBytes, true, "", ""},
		{"svg", map[string]string{"tenant": "t1"}, "image/svg+xml", []byte(`<?xml version="1.0"?><svg xmlns="http://www.w3.org/2000/svg"/>`), true, "", ""},
		{"bad visibility", map[string]string{"tenant": "t1", "visibility": "world"}, "application/pdf", pdfBytes, true, "", ""},
		{"missing tenant", map[string]string{}, "application/pdf", pdfBytes, true, "", ""},
		{"bad tenant", map[string]string{"tenant": "../x"}, "application/pdf", pdfBytes, true, "", ""},
		{"missing file", map[string]string{"tenant": "t1"}, "", nil, true, "", ""},
		{"empty file", map[string]string{"tenant": "t1"}, "text/plain", []byte{}, true, "", ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := withPrincipal(uploadRequest(t, tc.fields, "doc.bin", tc.header, tc.file), "acme")

			err := validateCreateRequest(r)
			var req *createRequest
			if err == nil {
				req, err = parseCreateRequest(r)
			}
			if tc.wantErr {
				if !isValidationError(err) {
					t.Fatalf("want validation error, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if req.Upload.ContentType != tc.wantType || req.Upload.Visibility != tc.wantVisibl || req.Consumer != "acme" {
				t.Fatalf("got %+v consumer=%q", req.Upload, req.Consumer)
			}
			// Sniffing must rewind: the full file is what gets uploaded.
			got, _ := io.ReadAll(req.Upload.Body)
			if string(got) != string(tc.file) {
				t.Fatalf("body not rewound: got %q", got)
			}
		})
	}
}

func TestSanitizeFilename(t *testing.T) {
	cases := map[string]string{
		"report.pdf":                      "report.pdf",
		`..\..\evil".pdf`:                 "evil_.pdf",
		"../../etc/passwd":                "passwd",
		"résumé 2026.pdf":                 "r_sum__2026.pdf",
		"":                                "file",
		"..":                              "file",
		strings.Repeat("a", 200) + ".pdf": strings.Repeat("a", 124) + ".pdf",
	}
	for in, want := range cases {
		if got := sanitizeFilename(in); got != want {
			t.Errorf("sanitizeFilename(%q) = %q, want %q", in, got, want)
		}
	}
}
