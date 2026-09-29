package assets

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	assets_services "github.com/kastoras/images-api/internal/domains/assets/services"
	"github.com/kastoras/images-api/internal/imageprocessing"
	"github.com/kastoras/images-api/internal/server"
)

func TestHandler_StorageNotConfigured(t *testing.T) {
	h := NewHandler(&server.APIServer{}, &stubService{})
	handlers := map[string]func(http.ResponseWriter, *http.Request){
		"create": h.Create, "render": h.Render, "delete": h.Delete, "list": h.List,
	}
	for name, fn := range handlers {
		rec := httptest.NewRecorder()
		fn(rec, httptest.NewRequest(http.MethodGet, "/", nil))
		if rec.Code != http.StatusServiceUnavailable {
			t.Errorf("%s: got %d, want 503", name, rec.Code)
		}
	}
}

func TestHandler_Create(t *testing.T) {
	t.Run("success uses principal as consumer", func(t *testing.T) {
		stub := &stubService{master: &assets_services.Master{ID: validHash, Consumer: "acme", Tenant: "t1"}}
		rec := httptest.NewRecorder()
		newTestHandler(stub).Create(rec, withPrincipal(multipartRequest(t, "t1", []byte("img"), "image/png"), "acme"))

		assertStatus(t, rec, http.StatusCreated)
		if stub.consumer != "acme" || stub.tenant != "t1" {
			t.Errorf("service got consumer=%q tenant=%q", stub.consumer, stub.tenant)
		}
		var body struct {
			Data assets_services.Master `json:"data"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || body.Data.ID != validHash {
			t.Errorf("body = %s (%v)", rec.Body.String(), err)
		}
	})

	t.Run("client-supplied consumer is ignored", func(t *testing.T) {
		stub := &stubService{master: &assets_services.Master{}}
		req := multipartRequest(t, "t1", []byte("img"), "image/png")
		q := req.URL.Query()
		q.Set("consumer", "victim")
		req.URL.RawQuery = q.Encode()
		newTestHandler(stub).Create(httptest.NewRecorder(), withPrincipal(req, "acme"))
		if stub.consumer != "acme" {
			t.Errorf("consumer = %q, want acme", stub.consumer)
		}
	})

	t.Run("validation error", func(t *testing.T) {
		stub := &stubService{}
		rec := httptest.NewRecorder()
		newTestHandler(stub).Create(rec, withPrincipal(multipartRequest(t, "", []byte("img"), "image/png"), "acme"))
		assertStatus(t, rec, http.StatusBadRequest)
		if stub.called {
			t.Error("service must not be called on invalid input")
		}
	})

	t.Run("unsupported media type", func(t *testing.T) {
		rec := httptest.NewRecorder()
		newTestHandler(&stubService{}).Create(rec, withPrincipal(multipartRequest(t, "t1", []byte("img"), "image/gif"), "acme"))
		assertStatus(t, rec, http.StatusUnsupportedMediaType)
	})

	t.Run("service error", func(t *testing.T) {
		rec := httptest.NewRecorder()
		newTestHandler(&stubService{err: errors.New("boom")}).Create(rec, withPrincipal(multipartRequest(t, "t1", []byte("img"), "image/png"), "acme"))
		assertStatus(t, rec, http.StatusInternalServerError)
	})

	t.Run("unauthenticated", func(t *testing.T) {
		stub := &stubService{}
		rec := httptest.NewRecorder()
		newTestHandler(stub).Create(rec, multipartRequest(t, "t1", []byte("img"), "image/png"))
		assertStatus(t, rec, http.StatusBadRequest)
		if stub.called {
			t.Error("service must not be called without a principal")
		}
	})
}

func TestHandler_Render(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		stub := &stubService{data: []byte("IMG"), ct: "image/png"}
		rec := httptest.NewRecorder()
		newTestHandler(stub).Render(rec, routedRequest(http.MethodGet, renderTarget("?width=10&height=20&mode=fill&format=png"), renderPattern))

		assertStatus(t, rec, http.StatusOK)
		if rec.Body.String() != "IMG" || rec.Header().Get("Content-Type") != "image/png" {
			t.Errorf("body=%q ct=%q", rec.Body.String(), rec.Header().Get("Content-Type"))
		}
		if cc := rec.Header().Get("Cache-Control"); !strings.Contains(cc, "immutable") {
			t.Errorf("Cache-Control = %q", cc)
		}
		if stub.consumer != "acme" || stub.tenant != "t1" || stub.hash != validHash || stub.format != "png" ||
			stub.opts != (imageprocessing.ResizeOptions{Width: 10, Height: 20, Mode: imageprocessing.ResizeModeFill}) {
			t.Errorf("service args: %+v", stub)
		}
	})

	t.Run("not found", func(t *testing.T) {
		rec := httptest.NewRecorder()
		newTestHandler(&stubService{err: server.ErrNotFound}).Render(rec, routedRequest(http.MethodGet, renderTarget("?width=10&height=10"), renderPattern))
		assertStatus(t, rec, http.StatusNotFound)
	})

	t.Run("wrapped not found", func(t *testing.T) {
		rec := httptest.NewRecorder()
		err := errors.Join(errors.New("ctx"), server.ErrNotFound)
		newTestHandler(&stubService{err: err}).Render(rec, routedRequest(http.MethodGet, renderTarget("?width=10&height=10"), renderPattern))
		assertStatus(t, rec, http.StatusNotFound)
	})

	t.Run("bad params", func(t *testing.T) {
		stub := &stubService{}
		rec := httptest.NewRecorder()
		newTestHandler(stub).Render(rec, routedRequest(http.MethodGet, renderTarget(""), renderPattern))
		assertStatus(t, rec, http.StatusBadRequest)
		if stub.called {
			t.Error("service must not be called")
		}
	})

	t.Run("service error", func(t *testing.T) {
		rec := httptest.NewRecorder()
		newTestHandler(&stubService{err: errors.New("boom")}).Render(rec, routedRequest(http.MethodGet, renderTarget("?width=10&height=10"), renderPattern))
		assertStatus(t, rec, http.StatusInternalServerError)
	})
}

func TestHandler_Delete(t *testing.T) {
	pattern := "/assets/{tenant}/{hash}"

	t.Run("success", func(t *testing.T) {
		stub := &stubService{}
		rec := httptest.NewRecorder()
		newTestHandler(stub).Delete(rec, withPrincipal(routedRequest(http.MethodDelete, "/assets/t1/"+validHash, pattern), "acme"))
		assertStatus(t, rec, http.StatusNoContent)
		if stub.consumer != "acme" || stub.tenant != "t1" || stub.hash != validHash {
			t.Errorf("service args: %+v", stub)
		}
	})

	t.Run("no principal", func(t *testing.T) {
		stub := &stubService{}
		rec := httptest.NewRecorder()
		newTestHandler(stub).Delete(rec, routedRequest(http.MethodDelete, "/assets/t1/"+validHash, pattern))
		assertStatus(t, rec, http.StatusBadRequest)
		if stub.called {
			t.Error("service must not be called")
		}
	})

	t.Run("bad hash", func(t *testing.T) {
		rec := httptest.NewRecorder()
		newTestHandler(&stubService{}).Delete(rec, withPrincipal(routedRequest(http.MethodDelete, "/assets/t1/zzz", pattern), "acme"))
		assertStatus(t, rec, http.StatusBadRequest)
	})

	t.Run("service error", func(t *testing.T) {
		rec := httptest.NewRecorder()
		newTestHandler(&stubService{err: errors.New("boom")}).Delete(rec, withPrincipal(routedRequest(http.MethodDelete, "/assets/t1/"+validHash, pattern), "acme"))
		assertStatus(t, rec, http.StatusInternalServerError)
	})
}

func TestHandler_List(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		stub := &stubService{list: []assets_services.ListedAsset{{ID: validHash, Bytes: 5, LastModified: "2026-01-02T03:04:05Z"}}}
		rec := httptest.NewRecorder()
		newTestHandler(stub).List(rec, withPrincipal(httptest.NewRequest(http.MethodGet, "/assets?tenant=t1", nil), "acme"))

		assertStatus(t, rec, http.StatusOK)
		var body struct {
			Data assetsList `json:"data"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if body.Data.Tenant != "t1" || len(body.Data.Assets) != 1 || body.Data.Assets[0].ID != validHash {
			t.Errorf("body = %s", rec.Body.String())
		}
		if stub.consumer != "acme" || stub.tenant != "t1" {
			t.Errorf("service args: %+v", stub)
		}
	})

	t.Run("missing tenant", func(t *testing.T) {
		rec := httptest.NewRecorder()
		newTestHandler(&stubService{}).List(rec, withPrincipal(httptest.NewRequest(http.MethodGet, "/assets", nil), "acme"))
		assertStatus(t, rec, http.StatusBadRequest)
	})

	t.Run("no principal", func(t *testing.T) {
		rec := httptest.NewRecorder()
		newTestHandler(&stubService{}).List(rec, httptest.NewRequest(http.MethodGet, "/assets?tenant=t1", nil))
		assertStatus(t, rec, http.StatusBadRequest)
	})

	t.Run("service error", func(t *testing.T) {
		rec := httptest.NewRecorder()
		newTestHandler(&stubService{err: errors.New("boom")}).List(rec, withPrincipal(httptest.NewRequest(http.MethodGet, "/assets?tenant=t1", nil), "acme"))
		assertStatus(t, rec, http.StatusInternalServerError)
	})
}
