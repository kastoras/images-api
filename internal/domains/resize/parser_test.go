package resize

import (
	"bytes"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"testing"

	"github.com/kastoras/images-api/internal/imageprocessing"
	internal_errors "github.com/kastoras/images-api/internal/utils/errors"
	"github.com/kastoras/images-api/internal/utils/files"
)

// buildMultipartRequest constructs an *http.Request with a multipart/form-data body
// containing an optional file part and optional width/height/mode form fields.
// Pass contentType="" to skip writing the part header (simulates missing file).
// Pass filename="" to omit the file part entirely.
// Pass mode="" to omit the mode field.
func buildMultipartRequest(t *testing.T, fileContent []byte, fileContentType, width, height, mode string) *http.Request {
	t.Helper()
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	if fileContent != nil {
		// Build the file part with an explicit Content-Type header.
		h := textproto.MIMEHeader{}
		h.Set("Content-Disposition", `form-data; name="file"; filename="test.jpg"`)
		if fileContentType != "" {
			h.Set("Content-Type", fileContentType)
		}
		part, err := writer.CreatePart(h)
		if err != nil {
			t.Fatalf("create part: %v", err)
		}
		if _, err := part.Write(fileContent); err != nil {
			t.Fatalf("write part: %v", err)
		}
	}

	if width != "" {
		if err := writer.WriteField("width", width); err != nil {
			t.Fatalf("write width: %v", err)
		}
	}
	if height != "" {
		if err := writer.WriteField("height", height); err != nil {
			t.Fatalf("write height: %v", err)
		}
	}
	if mode != "" {
		if err := writer.WriteField("mode", mode); err != nil {
			t.Fatalf("write mode: %v", err)
		}
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/resize", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req
}

// minimalJPEG is a 4×4 red JPEG used across tests that need a parseable image file.
var minimalJPEG = []byte{
	0xff, 0xd8, 0xff, 0xdb, 0x00, 0x84, 0x00, 0x08, 0x06, 0x06, 0x07, 0x06, 0x05, 0x08, 0x07, 0x07,
	0x07, 0x09, 0x09, 0x08, 0x0a, 0x0c, 0x14, 0x0d, 0x0c, 0x0b, 0x0b, 0x0c, 0x19, 0x12, 0x13, 0x0f,
	0x14, 0x1d, 0x1a, 0x1f, 0x1e, 0x1d, 0x1a, 0x1c, 0x1c, 0x20, 0x24, 0x2e, 0x27, 0x20, 0x22, 0x2c,
	0x23, 0x1c, 0x1c, 0x28, 0x37, 0x29, 0x2c, 0x30, 0x31, 0x34, 0x34, 0x34, 0x1f, 0x27, 0x39, 0x3d,
	0x38, 0x32, 0x3c, 0x2e, 0x33, 0x34, 0x32, 0x01, 0x09, 0x09, 0x09, 0x0c, 0x0b, 0x0c, 0x18, 0x0d,
	0x0d, 0x18, 0x32, 0x21, 0x1c, 0x21, 0x32, 0x32, 0x32, 0x32, 0x32, 0x32, 0x32, 0x32, 0x32, 0x32,
	0x32, 0x32, 0x32, 0x32, 0x32, 0x32, 0x32, 0x32, 0x32, 0x32, 0x32, 0x32, 0x32, 0x32, 0x32, 0x32,
	0x32, 0x32, 0x32, 0x32, 0x32, 0x32, 0x32, 0x32, 0x32, 0x32, 0x32, 0x32, 0x32, 0x32, 0x32, 0x32,
	0x32, 0x32, 0x32, 0x32, 0x32, 0x32, 0x32, 0x32, 0xff, 0xc0, 0x00, 0x11, 0x08, 0x00, 0x04, 0x00,
	0x04, 0x03, 0x01, 0x22, 0x00, 0x02, 0x11, 0x01, 0x03, 0x11, 0x01, 0xff, 0xc4, 0x01, 0xa2, 0x00,
	0x00, 0x01, 0x05, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x10, 0x00, 0x02, 0x01,
	0x03, 0x03, 0x02, 0x04, 0x03, 0x05, 0x05, 0x04, 0x04, 0x00, 0x00, 0x01, 0x7d, 0x01, 0x02, 0x03,
	0x00, 0x04, 0x11, 0x05, 0x12, 0x21, 0x31, 0x41, 0x06, 0x13, 0x51, 0x61, 0x07, 0x22, 0x71, 0x14,
	0x32, 0x81, 0x91, 0xa1, 0x08, 0x23, 0x42, 0xb1, 0xc1, 0x15, 0x52, 0xd1, 0xf0, 0x24, 0x33, 0x62,
	0x72, 0x82, 0x09, 0x0a, 0x16, 0x17, 0x18, 0x19, 0x1a, 0x25, 0x26, 0x27, 0x28, 0x29, 0x2a, 0x34,
	0x35, 0x36, 0x37, 0x38, 0x39, 0x3a, 0x43, 0x44, 0x45, 0x46, 0x47, 0x48, 0x49, 0x4a, 0x53, 0x54,
	0x55, 0x56, 0x57, 0x58, 0x59, 0x5a, 0x63, 0x64, 0x65, 0x66, 0x67, 0x68, 0x69, 0x6a, 0x73, 0x74,
	0x75, 0x76, 0x77, 0x78, 0x79, 0x7a, 0x83, 0x84, 0x85, 0x86, 0x87, 0x88, 0x89, 0x8a, 0x92, 0x93,
	0x94, 0x95, 0x96, 0x97, 0x98, 0x99, 0x9a, 0xa2, 0xa3, 0xa4, 0xa5, 0xa6, 0xa7, 0xa8, 0xa9, 0xaa,
	0xb2, 0xb3, 0xb4, 0xb5, 0xb6, 0xb7, 0xb8, 0xb9, 0xba, 0xc2, 0xc3, 0xc4, 0xc5, 0xc6, 0xc7, 0xc8,
	0xc9, 0xca, 0xd2, 0xd3, 0xd4, 0xd5, 0xd6, 0xd7, 0xd8, 0xd9, 0xda, 0xe1, 0xe2, 0xe3, 0xe4, 0xe5,
	0xe6, 0xe7, 0xe8, 0xe9, 0xea, 0xf1, 0xf2, 0xf3, 0xf4, 0xf5, 0xf6, 0xf7, 0xf8, 0xf9, 0xfa, 0x01,
	0x00, 0x03, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x11, 0x00, 0x02, 0x01,
	0x02, 0x04, 0x04, 0x03, 0x04, 0x07, 0x05, 0x04, 0x04, 0x00, 0x01, 0x02, 0x77, 0x00, 0x01, 0x02,
	0x03, 0x11, 0x04, 0x05, 0x21, 0x31, 0x06, 0x12, 0x41, 0x51, 0x07, 0x61, 0x71, 0x13, 0x22, 0x32,
	0x81, 0x08, 0x14, 0x42, 0x91, 0xa1, 0xb1, 0xc1, 0x09, 0x23, 0x33, 0x52, 0xf0, 0x15, 0x62, 0x72,
	0xd1, 0x0a, 0x16, 0x24, 0x34, 0xe1, 0x25, 0xf1, 0x17, 0x18, 0x19, 0x1a, 0x26, 0x27, 0x28, 0x29,
	0x2a, 0x35, 0x36, 0x37, 0x38, 0x39, 0x3a, 0x43, 0x44, 0x45, 0x46, 0x47, 0x48, 0x49, 0x4a, 0x53,
	0x54, 0x55, 0x56, 0x57, 0x58, 0x59, 0x5a, 0x63, 0x64, 0x65, 0x66, 0x67, 0x68, 0x69, 0x6a, 0x73,
	0x74, 0x75, 0x76, 0x77, 0x78, 0x79, 0x7a, 0x82, 0x83, 0x84, 0x85, 0x86, 0x87, 0x88, 0x89, 0x8a,
	0x92, 0x93, 0x94, 0x95, 0x96, 0x97, 0x98, 0x99, 0x9a, 0xa2, 0xa3, 0xa4, 0xa5, 0xa6, 0xa7, 0xa8,
	0xa9, 0xaa, 0xb2, 0xb3, 0xb4, 0xb5, 0xb6, 0xb7, 0xb8, 0xb9, 0xba, 0xc2, 0xc3, 0xc4, 0xc5, 0xc6,
	0xc7, 0xc8, 0xc9, 0xca, 0xd2, 0xd3, 0xd4, 0xd5, 0xd6, 0xd7, 0xd8, 0xd9, 0xda, 0xe2, 0xe3, 0xe4,
	0xe5, 0xe6, 0xe7, 0xe8, 0xe9, 0xea, 0xf2, 0xf3, 0xf4, 0xf5, 0xf6, 0xf7, 0xf8, 0xf9, 0xfa, 0xff,
	0xda, 0x00, 0x0c, 0x03, 0x01, 0x00, 0x02, 0x11, 0x03, 0x11, 0x00, 0x3f, 0x00, 0xe2, 0xe8, 0xa2,
	0x8a, 0xf9, 0x93, 0xf7, 0x13, 0xff, 0xd9,
}

func TestParseResizeRequest_ValidJPEG(t *testing.T) {
	req := buildMultipartRequest(t, minimalJPEG, "image/jpeg", "100", "80", "")
	result, err := parseResizeRequest(req)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if result.Width != 100 {
		t.Errorf("expected Width=100, got %d", result.Width)
	}
	if result.Height != 80 {
		t.Errorf("expected Height=80, got %d", result.Height)
	}
	if result.File == nil {
		t.Error("expected non-nil File")
	} else {
		files.SafeClose(result.File)
	}
}

func TestParseResizeRequest_MissingFileField(t *testing.T) {
	req := buildMultipartRequest(t, nil, "", "100", "80", "")
	_, err := parseResizeRequest(req)
	if err == nil {
		t.Fatal("expected error for missing file field")
	}
	var ve *internal_errors.ValidationError
	if !errors.As(err, &ve) {
		t.Errorf("expected *validationErr, got %T: %v", err, err)
	}
}

func TestParseResizeRequest_UnsupportedContentType(t *testing.T) {
	req := buildMultipartRequest(t, []byte("fake data"), "image/bmp", "100", "80", "")
	_, err := parseResizeRequest(req)
	if err == nil {
		t.Fatal("expected error for unsupported content type")
	}
	if !errors.Is(err, internal_errors.ErrUnsupportedFormat) {
		t.Errorf("expected ErrUnsupportedFormat, got %v", err)
	}
}

func TestParseResizeRequest_NonIntegerWidth(t *testing.T) {
	req := buildMultipartRequest(t, minimalJPEG, "image/jpeg", "abc", "80", "")
	_, err := parseResizeRequest(req)
	if err == nil {
		t.Fatal("expected error for non-integer width")
	}
	var ve *internal_errors.ValidationError
	if !errors.As(err, &ve) {
		t.Errorf("expected *validationErr, got %T: %v", err, err)
	}
}

func TestParseResizeRequest_ZeroWidth(t *testing.T) {
	req := buildMultipartRequest(t, minimalJPEG, "image/jpeg", "0", "80", "")
	_, err := parseResizeRequest(req)
	if err == nil {
		t.Fatal("expected error for zero width")
	}
	var ve *internal_errors.ValidationError
	if !errors.As(err, &ve) {
		t.Errorf("expected *validationErr, got %T: %v", err, err)
	}
}

func TestParseResizeRequest_NegativeHeight(t *testing.T) {
	req := buildMultipartRequest(t, minimalJPEG, "image/jpeg", "100", "-5", "")
	_, err := parseResizeRequest(req)
	if err == nil {
		t.Fatal("expected error for negative height")
	}
	var ve *internal_errors.ValidationError
	if !errors.As(err, &ve) {
		t.Errorf("expected *validationErr, got %T: %v", err, err)
	}
}

func TestParseResizeRequest_BothMissing(t *testing.T) {
	req := buildMultipartRequest(t, minimalJPEG, "image/jpeg", "", "", "")
	_, err := parseResizeRequest(req)
	if err == nil {
		t.Fatal("expected error when both width and height are absent")
	}
	var ve *internal_errors.ValidationError
	if !errors.As(err, &ve) {
		t.Errorf("expected *validationErr, got %T: %v", err, err)
	}
}

func TestParseResizeRequest_WidthOnly(t *testing.T) {
	req := buildMultipartRequest(t, minimalJPEG, "image/jpeg", "100", "", "")
	result, err := parseResizeRequest(req)
	if err != nil {
		t.Fatalf("expected no error for width-only request, got: %v", err)
	}
	defer files.SafeClose(result.File)
	if result.Width != 100 {
		t.Errorf("expected Width=100, got %d", result.Width)
	}
	if result.Height != 0 {
		t.Errorf("expected Height=0 (absent), got %d", result.Height)
	}
}

func TestParseResizeRequest_HeightOnly(t *testing.T) {
	req := buildMultipartRequest(t, minimalJPEG, "image/jpeg", "", "80", "")
	result, err := parseResizeRequest(req)
	if err != nil {
		t.Fatalf("expected no error for height-only request, got: %v", err)
	}
	defer files.SafeClose(result.File)
	if result.Width != 0 {
		t.Errorf("expected Width=0 (absent), got %d", result.Width)
	}
	if result.Height != 80 {
		t.Errorf("expected Height=80, got %d", result.Height)
	}
}

func TestParseResizeRequest_FitRequiresBothDimensions(t *testing.T) {
	req := buildMultipartRequest(t, minimalJPEG, "image/jpeg", "100", "", "fit")
	_, err := parseResizeRequest(req)
	if err == nil {
		t.Fatal("expected error: mode=fit requires both dimensions")
	}
	var ve *internal_errors.ValidationError
	if !errors.As(err, &ve) {
		t.Errorf("expected *validationErr, got %T: %v", err, err)
	}
}

func TestParseResizeRequest_FillRequiresBothDimensions(t *testing.T) {
	req := buildMultipartRequest(t, minimalJPEG, "image/jpeg", "", "80", "fill")
	_, err := parseResizeRequest(req)
	if err == nil {
		t.Fatal("expected error: mode=fill requires both dimensions")
	}
	var ve *internal_errors.ValidationError
	if !errors.As(err, &ve) {
		t.Errorf("expected *validationErr, got %T: %v", err, err)
	}
}

func TestParseResizeRequest_SupportedTypes(t *testing.T) {
	types := []string{"image/jpeg", "image/png", "image/gif", "image/tiff"}
	for _, ct := range types {
		req := buildMultipartRequest(t, minimalJPEG, ct, "10", "10", "")
		result, err := parseResizeRequest(req)
		if err != nil {
			if errors.Is(err, internal_errors.ErrUnsupportedFormat) {
				t.Errorf("content-type %q should be supported but got ErrUnsupportedFormat", ct)
			}
			continue
		}
		files.SafeClose(result.File)
	}
}

func TestParseResizeRequest_DefaultModeIsExact(t *testing.T) {
	req := buildMultipartRequest(t, minimalJPEG, "image/jpeg", "100", "80", "")
	result, err := parseResizeRequest(req)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	defer files.SafeClose(result.File)
	if result.Mode != imageprocessing.ResizeModeExact {
		t.Errorf("expected default Mode=%q, got %q", imageprocessing.ResizeModeExact, result.Mode)
	}
}

func TestParseResizeRequest_ModeFit(t *testing.T) {
	req := buildMultipartRequest(t, minimalJPEG, "image/jpeg", "100", "80", "fit")
	result, err := parseResizeRequest(req)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	defer files.SafeClose(result.File)
	if result.Mode != imageprocessing.ResizeModeFit {
		t.Errorf("expected Mode=%q, got %q", imageprocessing.ResizeModeFit, result.Mode)
	}
}

func TestParseResizeRequest_ModeFill(t *testing.T) {
	req := buildMultipartRequest(t, minimalJPEG, "image/jpeg", "100", "80", "fill")
	result, err := parseResizeRequest(req)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	defer files.SafeClose(result.File)
	if result.Mode != imageprocessing.ResizeModeFill {
		t.Errorf("expected Mode=%q, got %q", imageprocessing.ResizeModeFill, result.Mode)
	}
}

func TestParseResizeRequest_InvalidMode(t *testing.T) {
	req := buildMultipartRequest(t, minimalJPEG, "image/jpeg", "100", "80", "stretch")
	_, err := parseResizeRequest(req)
	if err == nil {
		t.Fatal("expected error for invalid mode")
	}
	var ve *internal_errors.ValidationError
	if !errors.As(err, &ve) {
		t.Errorf("expected *validationErr, got %T: %v", err, err)
	}
}
