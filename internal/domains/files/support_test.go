package files

import (
	"bytes"
	"context"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"sync"
	"testing"

	"github.com/kastoras/images-api/internal/server"
	"github.com/kastoras/images-api/internal/server/authentication"
	internal_errors "github.com/kastoras/images-api/internal/utils/errors"
)

var (
	pdfBytes = []byte("%PDF-1.4\n%fake pdf body\n")
	pngBytes = []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR")
	// Leading ftyp boxes as cameras and phones write them.
	mp4Bytes = []byte("\x00\x00\x00\x18ftypmp42\x00\x00\x00\x00mp42isom\x00\x00\x00\x08free")
	movBytes = []byte("\x00\x00\x00\x14ftypqt  \x00\x00\x00\x00qt  \x00\x00\x00\x08wide")
	txtBytes = []byte("hello world\n")
	// HTML disguised behind an image extension and header.
	htmlBytes = []byte("<!DOCTYPE html><html><script>alert(1)</script></html>")
)

const testID = "0b6f4d8e-2c1a-4f5b-9d3e-7a8b9c0d1e2f"

type fakeStorage struct {
	mu       sync.Mutex
	objects  map[string][]byte
	metadata map[string]map[string]string
}

func newFakeStorage() *fakeStorage {
	return &fakeStorage{objects: map[string][]byte{}, metadata: map[string]map[string]string{}}
}

func (f *fakeStorage) UploadWithMetadata(_ context.Context, key string, body io.Reader, _ int64, metadata map[string]string) error {
	data, err := io.ReadAll(body)
	if err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.objects[key], f.metadata[key] = data, metadata
	return nil
}

func (f *fakeStorage) Download(_ context.Context, key string) (io.ReadCloser, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	data, ok := f.objects[key]
	if !ok {
		return nil, server.ErrNotFound
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}

func (f *fakeStorage) HeadObject(_ context.Context, key string) (map[string]string, int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	data, ok := f.objects[key]
	if !ok {
		return nil, 0, server.ErrNotFound
	}
	return f.metadata[key], int64(len(data)), nil
}

func (f *fakeStorage) Delete(_ context.Context, key string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.objects, key)
	return nil
}

func withPrincipal(r *http.Request, subject string) *http.Request {
	return r.WithContext(authentication.WithPrincipal(r.Context(), &authentication.Principal{Subject: subject}))
}

func isValidationError(err error) bool {
	var v *internal_errors.ValidationError
	return errors.As(err, &v)
}

func uploadRequest(t *testing.T, fields map[string]string, filename, headerType string, file []byte) *http.Request {
	t.Helper()
	body := &bytes.Buffer{}
	w := multipart.NewWriter(body)
	for k, v := range fields {
		_ = w.WriteField(k, v)
	}
	if file != nil {
		h := textproto.MIMEHeader{}
		h.Set("Content-Disposition", `form-data; name="file"; filename="`+filename+`"`)
		h.Set("Content-Type", headerType)
		part, err := w.CreatePart(h)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = part.Write(file)
	}
	_ = w.Close()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/files", body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	return req
}
