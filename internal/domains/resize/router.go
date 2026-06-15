package resize

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/kastoras/images-api/internal/middleware"
	"github.com/kastoras/images-api/internal/server"
)

func Register(router *mux.Router, s *server.APIServer) {
	svc := NewService(s)
	h := NewHandler(svc)
	s.RegisterProcessor("resize", svc.Process)
	router.Handle("/resize",
		middleware.RequireRole("resize")(http.HandlerFunc(h.Resize)),
	).Methods("POST")
}
