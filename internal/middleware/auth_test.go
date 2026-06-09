package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kastoras/images-api/internal/server"
)

const testToken = "test-secret-token"

// makeAPIServer returns an APIServer configured for simple bearer-token auth
// (Zitadel disabled) with the given API token.
func makeAPIServer(token string) *server.APIServer {
	return &server.APIServer{
		Auth:      server.NewAuthServer(token, false),
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
