package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kastoras/images-api/internal/server"
	"github.com/kastoras/images-api/internal/server/authentication"
)

const testToken = "test-secret-token"

// makeAPIServer returns an APIServer configured for simple bearer-token auth
// (Zitadel disabled) with the given API token.
func makeAPIServer(token string) *server.APIServer {
	return &server.APIServer{
		Auth:      authentication.NewBearerAuthenticator(token),
		Semaphore: make(chan struct{}, 1),
	}
}

// okHandler is a trivial next handler that writes 200 OK.
var okHandler = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
})

func applyAuth(s *server.APIServer, req *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	Auth(s)(okHandler).ServeHTTP(rec, req)
	return rec
}

func TestAuth_ValidToken_PassesThrough(t *testing.T) {
	s := makeAPIServer(testToken)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+testToken)

	rec := applyAuth(s, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestAuth_MissingHeader_Returns401(t *testing.T) {
	s := makeAPIServer(testToken)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	// No Authorization header set.

	rec := applyAuth(s, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestAuth_WrongToken_Returns401(t *testing.T) {
	s := makeAPIServer(testToken)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer wrong-token")

	rec := applyAuth(s, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestAuth_MissingBearerPrefix_Returns401(t *testing.T) {
	s := makeAPIServer(testToken)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	// Token sent without "Bearer " prefix.
	req.Header.Set("Authorization", testToken)

	rec := applyAuth(s, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestAuth_EmptyBearerValue_Returns401(t *testing.T) {
	s := makeAPIServer(testToken)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer ")

	rec := applyAuth(s, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

// applyRequireRole runs RequireRole(role) over a request carrying the given
// principal in context (nil = no principal, as if Auth never ran).
func applyRequireRole(role string, p *authentication.Principal) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if p != nil {
		req = req.WithContext(authentication.WithPrincipal(context.Background(), p))
	}
	rec := httptest.NewRecorder()
	RequireRole(role)(okHandler).ServeHTTP(rec, req)
	return rec
}

func TestRequireRole(t *testing.T) {
	tests := []struct {
		name      string
		required  string
		principal *authentication.Principal
		want      int
	}{
		{
			name:      "principal has required role passes through",
			required:  "resize",
			principal: &authentication.Principal{Subject: "client-a", Roles: []string{"resize"}},
			want:      http.StatusOK,
		},
		{
			name:      "wildcard role passes any check",
			required:  "jobs",
			principal: &authentication.Principal{Subject: "local-dev", Roles: []string{"*"}},
			want:      http.StatusOK,
		},
		{
			name:      "principal lacks required role is forbidden",
			required:  "jobs",
			principal: &authentication.Principal{Subject: "client-a", Roles: []string{"resize"}},
			want:      http.StatusForbidden,
		},
		{
			name:      "principal with no roles is forbidden",
			required:  "resize",
			principal: &authentication.Principal{Subject: "client-a", Roles: nil},
			want:      http.StatusForbidden,
		},
		{
			name:      "missing principal is forbidden",
			required:  "resize",
			principal: nil,
			want:      http.StatusForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := applyRequireRole(tt.required, tt.principal)
			if rec.Code != tt.want {
				t.Errorf("expected %d, got %d", tt.want, rec.Code)
			}
		})
	}
}
