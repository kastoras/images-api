package files

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/kastoras/images-api/internal/middleware"
	"github.com/kastoras/images-api/internal/server"
)

// Register wires the authenticated /api/v1/files routes.
func Register(router *mux.Router, s *server.APIServer) {
	h := NewHandler(s, NewService(s))
	role := middleware.RequireRole("files")

	router.Handle("/files", role(http.HandlerFunc(h.Create))).Methods("POST")
	router.Handle("/files/{tenant}/{id}", role(http.HandlerFunc(h.Get))).Methods("GET")
	router.Handle("/files/{tenant}/{id}", role(http.HandlerFunc(h.Delete))).Methods("DELETE")
}

// RegisterPublic wires the unauthenticated download route for files uploaded
// with visibility=public.
func RegisterPublic(router *mux.Router, s *server.APIServer) {
	h := NewHandler(s, NewService(s))
	router.HandleFunc("/f/{consumer}/{tenant}/{id}", h.GetPublic).Methods("GET")
}
