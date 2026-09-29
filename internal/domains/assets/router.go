package assets

import (
	"net/http"

	"github.com/gorilla/mux"
	assets_services "github.com/kastoras/images-api/internal/domains/assets/services"
	"github.com/kastoras/images-api/internal/middleware"
	"github.com/kastoras/images-api/internal/server"
)

// Register wires the authenticated /api/v1/assets routes.
func Register(router *mux.Router, s *server.APIServer) {
	svc := assets_services.NewService(s)
	h := NewHandler(s, svc)

	router.Handle("/assets",
		middleware.RequireRole("assets")(http.HandlerFunc(h.Create)),
	).Methods("POST")

	router.Handle("/assets",
		middleware.RequireRole("assets")(http.HandlerFunc(h.List)),
	).Methods("GET")

	router.Handle("/assets/{tenant}/{hash}",
		middleware.RequireRole("assets")(http.HandlerFunc(h.Delete)),
	).Methods("DELETE")
}

// RegisterPublic wires the unauthenticated render route on the bare router
// (the same tier as /health) — assets are served directly by images-api,
// not proxied through a consumer's own backend.
func RegisterPublic(router *mux.Router, s *server.APIServer) {
	svc := assets_services.NewService(s)
	h := NewHandler(s, svc)

	router.HandleFunc("/a/{consumer}/{tenant}/{hash}/render", h.Render).Methods("GET")
}
