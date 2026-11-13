package files

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/mux"
	"github.com/kastoras/images-api/internal/server"
	"github.com/rs/zerolog"
)

// newTestRouter wires the real handlers to a service backed by fake storage,
// with a fixed principal standing in for the Auth middleware.
func newTestRouter(st *fakeStorage, subject string) *mux.Router {
	s := &server.APIServer{Storage: &server.ObjectStorage{}, MaxUploadSizeBytes: 1 << 20, Log: zerolog.Nop()}
	h := NewHandler(s, &Service{storage: st})

	r := mux.NewRouter()
	r.HandleFunc("/f/{consumer}/{tenant}/{id}", h.GetPublic).Methods("GET")
	api := r.PathPrefix("/api/v1").Subrouter()
	api.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			next.ServeHTTP(w, withPrincipal(req, subject))
		})
	})
	api.HandleFunc("/files", h.Create).Methods("POST")
	api.HandleFunc("/files/{tenant}/{id}", h.Get).Methods("GET")
	api.HandleFunc("/files/{tenant}/{id}", h.Delete).Methods("DELETE")
	return r
}

func do(r http.Handler, req *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func TestHandlerDownloadHeaders(t *testing.T) {
	st := newFakeStorage()
	svc := &Service{storage: st}
	f, _ := svc.Create(context.Background(), "acme", Upload{Tenant: "t1", Filename: "a.pdf", ContentType: "application/pdf", Visibility: visibilityPrivate, Size: int64(len(pdfBytes)), Body: bytes.NewReader(pdfBytes)})

	rec := do(newTestRouter(st, "acme"), httptest.NewRequest(http.MethodGet, "/api/v1/files/t1/"+f.ID, nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	h := rec.Header()
	if h.Get("Content-Type") != "application/pdf" || h.Get("X-Content-Type-Options") != "nosniff" ||
		h.Get("Content-Disposition") != `attachment; filename="a.pdf"` || h.Get("Cache-Control") != "private, no-store" {
		t.Fatalf("headers %v", h)
	}
	if !bytes.Equal(rec.Body.Bytes(), pdfBytes) {
		t.Fatalf("body %q", rec.Body.Bytes())
	}

	// Private file is a 404 on the public route.
	if rec := do(newTestRouter(st, "acme"), httptest.NewRequest(http.MethodGet, "/f/acme/t1/"+f.ID, nil)); rec.Code != http.StatusNotFound {
		t.Fatalf("public route on private file: %d", rec.Code)
	}
}

func TestHandlerUploadThenPublicDownload(t *testing.T) {
	st := newFakeStorage()
	r := newTestRouter(st, "acme")

	rec := do(r, uploadRequest(t, map[string]string{"tenant": "t1", "visibility": "public"}, "pic.png", "image/png", pngBytes))
	if rec.Code != http.StatusCreated {
		t.Fatalf("upload status %d: %s", rec.Code, rec.Body.String())
	}
	if len(st.objects) != 1 {
		t.Fatalf("expected 1 stored object, got %d", len(st.objects))
	}
	var key string
	for k := range st.objects {
		key = k
	}
	id := key[len("files/acme/t1/"):]

	rec = do(r, httptest.NewRequest(http.MethodGet, "/f/acme/t1/"+id, nil))
	if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "image/png" || rec.Header().Get("Content-Disposition") != "" {
		t.Fatalf("public download: %d %v", rec.Code, rec.Header())
	}
	body, _ := io.ReadAll(rec.Body)
	if !bytes.Equal(body, pngBytes) {
		t.Fatalf("body %q", body)
	}
}

func TestHandlerConsumerComesFromPrincipal(t *testing.T) {
	st := newFakeStorage()
	do(newTestRouter(st, "acme"), uploadRequest(t, map[string]string{"tenant": "t1", "consumer": "evil"}, "a.txt", "text/plain", txtBytes))
	for k := range st.objects {
		if k[:len("files/acme/t1/")] != "files/acme/t1/" {
			t.Fatalf("stored under %q", k)
		}
	}
}

func TestHandlerRejectsOversizedUpload(t *testing.T) {
	big := bytes.Repeat([]byte("a"), 3<<20) // limit is 1MB + 1MB slack
	rec := do(newTestRouter(newFakeStorage(), "acme"), uploadRequest(t, map[string]string{"tenant": "t1"}, "a.txt", "text/plain", big))
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "maximum upload size") {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandlerBadID(t *testing.T) {
	rec := do(newTestRouter(newFakeStorage(), "acme"), httptest.NewRequest(http.MethodGet, "/api/v1/files/t1/not-a-uuid", nil))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestHandlerDelete(t *testing.T) {
	rec := do(newTestRouter(newFakeStorage(), "acme"), httptest.NewRequest(http.MethodDelete, "/api/v1/files/t1/"+testID, nil))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status %d", rec.Code)
	}
}
