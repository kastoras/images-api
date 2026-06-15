package jobs

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/kastoras/images-api/internal/middleware"
	"github.com/kastoras/images-api/internal/server"
)

func Register(router *mux.Router, s *server.APIServer) {
	h := NewHandler(s)
	router.Handle("/jobs/{id}",
		middleware.RequireRole("jobs")(http.HandlerFunc(h.GetJob)),
	).Methods("GET")
}
