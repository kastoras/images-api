package resize

import (
	"github.com/gorilla/mux"
	"github.com/kastoras/images-api/internal/server"
)

func Register(router *mux.Router, s *server.APIServer) {
	svc := NewService(s)
	h := NewHandler(svc)
	s.RegisterProcessor("resize", svc.Process)
	router.HandleFunc("/resize", h.Resize).Methods("POST")
}
