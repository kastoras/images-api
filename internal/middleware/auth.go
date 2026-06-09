package middleware

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/kastoras/images-api/internal/server"
	"github.com/kastoras/images-api/internal/utils/responses"
)

func Auth(s *server.APIServer) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !s.Auth.Validate(r) {
				responses.Unauthorized(w)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
