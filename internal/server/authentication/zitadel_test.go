package authentication

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestJWKSURL(t *testing.T) {
	for _, tc := range []struct{ issuer, want string }{
		{"https://idp.example.com", "https://idp.example.com/oauth/v2/keys"},
		{"https://idp.example.com/", "https://idp.example.com/oauth/v2/keys"},
		{"https://idp.example.com///", "https://idp.example.com/oauth/v2/keys"},
	} {
		if got := JWKSURL(tc.issuer); got != tc.want {
			t.Errorf("JWKSURL(%q) = %q, want %q", tc.issuer, got, tc.want)
		}
	}
}

// PingJWKS is what stands between a running-but-useless service and a loud
// failure, so each way the issuer can be wrong is pinned here.
func TestPingJWKS(t *testing.T) {
	const oneKey = `{"keys":[{"kty":"RSA","use":"sig","alg":"RS256","kid":"k1","n":"xGOr","e":"AQAB"}]}`

	tests := []struct {
		name    string
		status  int
		body    string
		wantErr string // substring; "" means success expected
	}{
		{name: "healthy key set", status: 200, body: oneKey},
		{name: "two keys during rotation", status: 200,
			body: `{"keys":[{"kid":"a"},{"kid":"b"}]}`},
		// A 200 alone is not proof: an IdP mid-boot, a proxy error page or a
		// login redirect all return 200 with no usable keys.
		{name: "empty key set", status: 200, body: `{"keys":[]}`, wantErr: "contains no keys"},
		{name: "keys field absent", status: 200, body: `{}`, wantErr: "contains no keys"},
		{name: "html login page", status: 200, body: `<html>Sign in</html>`, wantErr: "parse JWKS"},
		{name: "truncated json", status: 200, body: `{"keys":[`, wantErr: "parse JWKS"},
		{name: "not found", status: 404, body: `nope`, wantErr: "returned HTTP 404"},
		{name: "gateway down", status: 502, body: ``, wantErr: "returned HTTP 502"},
		{name: "unauthorized", status: 401, body: ``, wantErr: "returned HTTP 401"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var gotPath string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotPath = r.URL.Path
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer srv.Close()

			err := PingJWKS(context.Background(), srv.URL)

			if gotPath != "/oauth/v2/keys" {
				t.Errorf("requested %q, want /oauth/v2/keys", gotPath)
			}
			switch {
			case tc.wantErr == "" && err != nil:
				t.Fatalf("unexpected error: %v", err)
			case tc.wantErr != "" && err == nil:
				t.Fatalf("expected error containing %q, got nil", tc.wantErr)
			case tc.wantErr != "" && !strings.Contains(err.Error(), tc.wantErr):
				t.Fatalf("error %q does not contain %q", err, tc.wantErr)
			}
		})
	}
}

func TestPingJWKSUnreachable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	url := srv.URL
	srv.Close() // nothing is listening now

	if err := PingJWKS(context.Background(), url); err == nil {
		t.Fatal("expected an error for an unreachable issuer, got nil")
	}
}

// The probe must honour its context, otherwise a retry loop inherits the
// jwkset default of a full minute per attempt.
func TestPingJWKSRespectsContext(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	start := time.Now()
	err := PingJWKS(ctx, srv.URL)
	if err == nil {
		t.Fatal("expected an error when the context expires")
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("PingJWKS ignored the context: took %v", elapsed)
	}
}
