package health

import (
	"github.com/gorilla/mux"
	"github.com/kastoras/images-api/internal/server"
)

func Register(router *mux.Router, s *server.APIServer) {
	h := NewHandler(s)
	router.HandleFunc("/health", h.Health).Methods("GET")
}
