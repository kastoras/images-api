package middleware

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/kastoras/images-api/internal/server"
	"github.com/kastoras/images-api/internal/server/authentication"
	"github.com/kastoras/images-api/internal/utils/responses"
)

// Auth authenticates the request and, on success, stores the resolved
// Principal in the request context for downstream authorization and logging.
func Auth(s *server.APIServer) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			principal, err := s.Auth.Authenticate(r)
			if err != nil {
				responses.Unauthorized(w)
				return
			}
			ctx := authentication.WithPrincipal(r.Context(), principal)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireRole gates a handler on the caller holding the given role. It must run
// after Auth (which populates the principal). Missing principal or role -> 403.
func RequireRole(role string) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			principal, ok := authentication.PrincipalFrom(r.Context())
			if !ok || !principal.HasRole(role) {
				responses.Forbidden(w)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
