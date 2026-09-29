package assets

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kastoras/images-api/internal/imageprocessing"
)

func TestConsumerFromContext(t *testing.T) {
	base := httptest.NewRequest(http.MethodGet, "/", nil)

	if _, err := consumerFromContext(base); !isValidationError(err) {
		t.Errorf("no principal: got %v", err)
	}
	if _, err := consumerFromContext(withPrincipal(base, "")); !isValidationError(err) {
		t.Errorf("empty subject: got %v", err)
	}
	if _, err := consumerFromContext(withPrincipal(base, "bad/consumer")); !isValidationError(err) {
		t.Errorf("invalid subject: got %v", err)
	}
	if got, err := consumerFromContext(withPrincipal(base, "acme")); err != nil || got != "acme" {
		t.Errorf("got %q, %v", got, err)
	}
}

func TestParseCreateRequest(t *testing.T) {
	req := withPrincipal(multipartRequest(t, " t1 ", []byte("x"), "image/png"), "acme")
	if err := req.ParseMultipartForm(1 << 20); err != nil {
		t.Fatal(err)
	}
	parsed, err := parseCreateRequest(req)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	defer func() { _ = parsed.File.Close() }()
	if parsed.Consumer != "acme" || parsed.Tenant != "t1" {
		t.Errorf("unexpected: %+v", parsed)
	}

	bad := withPrincipal(multipartRequest(t, "bad tenant", []byte("x"), "image/png"), "acme")
	_ = bad.ParseMultipartForm(1 << 20)
	if _, err := parseCreateRequest(bad); !isValidationError(err) {
		t.Errorf("invalid tenant: got %v", err)
	}

	noAuth := multipartRequest(t, "t1", []byte("x"), "image/png")
	_ = noAuth.ParseMultipartForm(1 << 20)
	if _, err := parseCreateRequest(noAuth); !isValidationError(err) {
		t.Errorf("no principal: got %v", err)
	}
}

func TestParseRenderRequest_Defaults(t *testing.T) {
	req := routedRequest(http.MethodGet, renderTarget("?width=100&height=50"), renderPattern)
	got, err := parseRenderRequest(req, 10)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got.Consumer != "acme" || got.Tenant != "t1" || got.Hash != validHash || got.Width != 100 || got.Height != 50 {
		t.Errorf("unexpected: %+v", got)
	}
	if got.Mode != imageprocessing.ResizeModeFit {
		t.Errorf("default mode = %s, want fit", got.Mode)
	}
	if got.Format != "" {
		t.Errorf("format = %q, want empty", got.Format)
	}
}

func TestParseRenderRequest_Cases(t *testing.T) {
	cases := []struct {
		name    string
		query   string
		target  string
		maxMP   int
		wantErr bool
		mode    imageprocessing.ResizeMode
		format  string
	}{
		{name: "width only exact", query: "?width=100&mode=exact", maxMP: 10, mode: imageprocessing.ResizeModeExact},
		{name: "width only default fit rejected", query: "?width=100", maxMP: 10, wantErr: true},
		{name: "fill needs both", query: "?width=100&mode=fill", maxMP: 10, wantErr: true},
		{name: "no dimensions", query: "", maxMP: 10, wantErr: true},
		{name: "bad mode", query: "?width=10&height=10&mode=stretch", maxMP: 10, wantErr: true},
		{name: "bad format", query: "?width=10&height=10&format=gif", maxMP: 10, wantErr: true},
		{name: "jpg alias", query: "?width=10&height=10&format=jpg", maxMP: 10, mode: imageprocessing.ResizeModeFit, format: "jpeg"},
		{name: "png", query: "?width=10&height=10&format=png", maxMP: 10, mode: imageprocessing.ResizeModeFit, format: "png"},
		{name: "non-numeric width", query: "?width=abc&height=10", maxMP: 10, wantErr: true},
		{name: "too many pixels", query: "?width=5000&height=5000", maxMP: 10, wantErr: true},
		{name: "unlimited pixels", query: "?width=5000&height=5000", maxMP: 0, mode: imageprocessing.ResizeModeFit},
		{name: "bad hash", query: "?width=10&height=10", target: "/a/acme/t1/nothex/render", maxMP: 10, wantErr: true},
		{name: "bad consumer", query: "?width=10&height=10", target: "/a/bad.consumer/t1/" + validHash + "/render", maxMP: 10, wantErr: true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			target := c.target
			if target == "" {
				target = renderTarget("")
			}
			req := routedRequest(http.MethodGet, target+c.query, renderPattern)
			if req == nil {
				t.Fatal("route did not match")
			}
			got, err := parseRenderRequest(req, c.maxMP)
			if c.wantErr {
				if !isValidationError(err) {
					t.Fatalf("expected ValidationError, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Mode != c.mode || got.Format != c.format {
				t.Errorf("mode=%s format=%q, want mode=%s format=%q", got.Mode, got.Format, c.mode, c.format)
			}
		})
	}
}

func TestParseTenantAndHash(t *testing.T) {
	req := routedRequest(http.MethodDelete, "/assets/t1/"+validHash, "/assets/{tenant}/{hash}")
	tenant, hash, err := parseTenantAndHash(req)
	if err != nil || tenant != "t1" || hash != validHash {
		t.Errorf("got %q %q %v", tenant, hash, err)
	}
	bad := routedRequest(http.MethodDelete, "/assets/t1/short", "/assets/{tenant}/{hash}")
	if _, _, err := parseTenantAndHash(bad); !isValidationError(err) {
		t.Errorf("bad hash: got %v", err)
	}
}

func TestParseTenant(t *testing.T) {
	if got, err := parseTenant(httptest.NewRequest(http.MethodGet, "/?tenant=t1", nil)); err != nil || got != "t1" {
		t.Errorf("got %q %v", got, err)
	}
	if _, err := parseTenant(httptest.NewRequest(http.MethodGet, "/", nil)); !isValidationError(err) {
		t.Errorf("missing tenant: got %v", err)
	}
}
