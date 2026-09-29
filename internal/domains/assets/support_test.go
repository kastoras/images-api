package assets

import (
	"bytes"
	"context"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"strings"
	"testing"

	"github.com/gorilla/mux"
	assets_services "github.com/kastoras/images-api/internal/domains/assets/services"
	"github.com/kastoras/images-api/internal/imageprocessing"
	"github.com/kastoras/images-api/internal/server"
	"github.com/kastoras/images-api/internal/server/authentication"
	internal_errors "github.com/kastoras/images-api/internal/utils/errors"
)

var validHash = strings.Repeat("a", 64)

// stubService records the arguments it was called with.
type stubService struct {
	master *assets_services.Master
	data   []byte
	ct     string
	list   []assets_services.ListedAsset
	err    error

	consumer, tenant, hash string
	opts                   imageprocessing.ResizeOptions
	format                 string
	called                 bool
}

func (s *stubService) Create(_ context.Context, consumer, tenant string, _ io.Reader) (*assets_services.Master, error) {
	s.called, s.consumer, s.tenant = true, consumer, tenant
	return s.master, s.err
}

func (s *stubService) Render(_ context.Context, consumer, tenant, hash string, opts imageprocessing.ResizeOptions, format string) ([]byte, string, error) {
	s.called, s.consumer, s.tenant, s.hash, s.opts, s.format = true, consumer, tenant, hash, opts, format
	return s.data, s.ct, s.err
}

func (s *stubService) Delete(_ context.Context, consumer, tenant, hash string) error {
	s.called, s.consumer, s.tenant, s.hash = true, consumer, tenant, hash
	return s.err
}

func (s *stubService) List(_ context.Context, consumer, tenant string) ([]assets_services.ListedAsset, error) {
	s.called, s.consumer, s.tenant = true, consumer, tenant
	return s.list, s.err
}

func newTestHandler(svc Servicer) *Handler {
	return NewHandler(&server.APIServer{
		Storage:             &server.ObjectStorage{}, // zero value: only its presence is checked, stub service never touches it
		MaxUploadSizeBytes:  1 << 20,
		MaxSourceMegapixels: 10,
	}, svc)
}

func withPrincipal(r *http.Request, subject string) *http.Request {
	return r.WithContext(authentication.WithPrincipal(r.Context(), &authentication.Principal{Subject: subject}))
}

func assertStatus(t *testing.T, rec *httptest.ResponseRecorder, want int) {
	t.Helper()
	if rec.Code != want {
		t.Fatalf("expected HTTP %d, got %d (body: %s)", want, rec.Code, rec.Body.String())
	}
}

func multipartRequest(t *testing.T, tenant string, file []byte, contentType string) *http.Request {
	t.Helper()
	body := &bytes.Buffer{}
	w := multipart.NewWriter(body)
	if tenant != "" {
		_ = w.WriteField("tenant", tenant)
	}
	if file != nil {
		h := textproto.MIMEHeader{}
		h.Set("Content-Disposition", `form-data; name="file"; filename="a.png"`)
		if contentType != "" {
			h.Set("Content-Type", contentType)
		}
		part, err := w.CreatePart(h)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = part.Write(file)
	}
	_ = w.Close()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/assets", body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	return req
}

func routedRequest(method, target, pattern string) *http.Request {
	var got *http.Request
	r := mux.NewRouter()
	r.HandleFunc(pattern, func(_ http.ResponseWriter, req *http.Request) { got = req })
	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(method, target, nil))
	return got
}

func isValidationError(err error) bool {
	var v *internal_errors.ValidationError
	return errors.As(err, &v)
}

const renderPattern = "/a/{consumer}/{tenant}/{hash}/render"

func renderTarget(query string) string {
	return "/a/acme/t1/" + validHash + "/render" + query
}
