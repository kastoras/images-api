package requests

import (
	"bytes"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"net/url"
	"strings"
	"testing"

	internal_errors "github.com/kastoras/images-api/internal/utils/errors"
)

var allowedMIME = map[string]bool{
	"image/jpeg": true,
	"image/png":  true,
}

// buildFileRequest creates a multipart POST request containing a single file part.
// Pass nil fileContent to omit the file part entirely (simulates missing field).
func buildFileRequest(t *testing.T, fileContent []byte, contentType string) *http.Request {
	t.Helper()
	body := &bytes.Buffer{}
	w := multipart.NewWriter(body)
	if fileContent != nil {
		h := textproto.MIMEHeader{}
		h.Set("Content-Disposition", `form-data; name="file"; filename="test.jpg"`)
		if contentType != "" {
			h.Set("Content-Type", contentType)
		}
		part, err := w.CreatePart(h)
		if err != nil {
			t.Fatalf("create part: %v", err)
		}
		if _, err := part.Write(fileContent); err != nil {
			t.Fatalf("write part: %v", err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/", body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	return req
}

// formRequest creates a URL-encoded POST request with the given fields.
func formRequest(t *testing.T, fields map[string]string) *http.Request {
	t.Helper()
	vals := url.Values{}
	for k, v := range fields {
		vals.Set(k, v)
	}
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(vals.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return req
}

// --- ParseImageFile ---

func TestParseImageFile_Valid(t *testing.T) {
	req := buildFileRequest(t, []byte("fake image bytes"), "image/jpeg")
	file, header, err := ParseImageFile(req, allowedMIME)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if file == nil {
		t.Fatal("expected non-nil file")
	}
	file.Close()
	if header == nil {
		t.Fatal("expected non-nil header")
	}
	if got := header.Header.Get("Content-Type"); got != "image/jpeg" {
		t.Errorf("expected Content-Type image/jpeg, got %q", got)
	}
}

func TestParseImageFile_InvalidMultipartForm(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("not multipart"))
	req.Header.Set("Content-Type", "text/plain")

	_, _, err := ParseImageFile(req, allowedMIME)
	if err == nil {
		t.Fatal("expected error for non-multipart request")
	}
	var ve *internal_errors.ValidationError
	if !errors.As(err, &ve) {
		t.Errorf("expected *ValidationError, got %T: %v", err, err)
	}
}

func TestParseImageFile_MissingFileField(t *testing.T) {
	req := buildFileRequest(t, nil, "")

	_, _, err := ParseImageFile(req, allowedMIME)
	if err == nil {
		t.Fatal("expected error for missing file field")
	}
	var ve *internal_errors.ValidationError
	if !errors.As(err, &ve) {
		t.Errorf("expected *ValidationError, got %T: %v", err, err)
	}
}

func TestParseImageFile_UnsupportedMIMEType(t *testing.T) {
	req := buildFileRequest(t, []byte("fake image bytes"), "image/bmp")

	_, _, err := ParseImageFile(req, allowedMIME)
	if err == nil {
		t.Fatal("expected error for unsupported MIME type")
	}
	if !errors.Is(err, internal_errors.ErrUnsupportedFormat) {
		t.Errorf("expected ErrUnsupportedFormat, got %v", err)
	}
}

func TestParseImageFile_AllAllowedTypes(t *testing.T) {
	types := []string{"image/jpeg", "image/png"}
	for _, ct := range types {
		req := buildFileRequest(t, []byte("fake image bytes"), ct)
		file, _, err := ParseImageFile(req, allowedMIME)
		if err != nil {
			t.Errorf("content-type %q should be allowed, got: %v", ct, err)
			continue
		}
		file.Close()
	}
}

// --- ParseString ---

func TestParseString_FieldPresent(t *testing.T) {
	req := formRequest(t, map[string]string{"color": "red"})
	if got := ParseString(req, "color"); got != "red" {
		t.Errorf("expected %q, got %q", "red", got)
	}
}

func TestParseString_FieldAbsent(t *testing.T) {
	req := formRequest(t, map[string]string{})
	if got := ParseString(req, "color"); got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

// --- ParseInt ---

func TestParseInt(t *testing.T) {
	cases := []struct {
		name    string
		value   string
		want    int
		wantErr bool
	}{
		{"valid", "42", 42, false},
		{"empty", "", 0, true},
		{"zero", "0", 0, true},
		{"negative", "-1", 0, true},
		{"non-integer", "abc", 0, true},
		{"float", "1.5", 0, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fields := map[string]string{}
			if tc.value != "" {
				fields["count"] = tc.value
			}
			req := formRequest(t, fields)
			got, err := ParseInt(req, "count")
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil (value=%q)", tc.value)
				}
				var ve *internal_errors.ValidationError
				if !errors.As(err, &ve) {
					t.Errorf("expected *ValidationError, got %T: %v", err, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("expected %d, got %d", tc.want, got)
			}
		})
	}
}

func TestParseInt_ErrorMessageContainsFieldName(t *testing.T) {
	req := formRequest(t, map[string]string{"limit": "bad"})
	_, err := ParseInt(req, "limit")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "limit") {
		t.Errorf("expected error message to contain field name %q, got %q", "limit", err.Error())
	}
}

// --- ParseOptionalInt ---

func TestParseOptionalInt(t *testing.T) {
	cases := []struct {
		name    string
		value   string
		want    int
		wantErr bool
	}{
		{"valid", "42", 42, false},
		{"empty returns zero", "", 0, false},
		{"zero is invalid", "0", 0, true},
		{"negative", "-1", 0, true},
		{"non-integer", "abc", 0, true},
		{"float", "1.5", 0, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fields := map[string]string{}
			if tc.value != "" {
				fields["size"] = tc.value
			}
			req := formRequest(t, fields)
			got, err := ParseOptionalInt(req, "size")
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil (value=%q)", tc.value)
				}
				var ve *internal_errors.ValidationError
				if !errors.As(err, &ve) {
					t.Errorf("expected *ValidationError, got %T: %v", err, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("expected %d, got %d", tc.want, got)
			}
		})
	}
}

func TestParseOptionalInt_ErrorMessageContainsFieldName(t *testing.T) {
	req := formRequest(t, map[string]string{"width": "bad"})
	_, err := ParseOptionalInt(req, "width")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "width") {
		t.Errorf("expected error message to contain field name %q, got %q", "width", err.Error())
	}
}
