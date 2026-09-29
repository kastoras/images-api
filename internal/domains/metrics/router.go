package metrics

import (
	"github.com/gorilla/mux"
	"github.com/kastoras/images-api/internal/server"
)

// Register wires the unauthenticated /metrics route — same trust tier as
// /health, so it relies on network placement rather than application auth.
// Do not route it through a public ingress; expose it only to your metrics
// scraper (private network, or block the path at the reverse proxy).
func Register(router *mux.Router, s *server.APIServer) {
	h := NewHandler(s)
	router.HandleFunc("/metrics", h.Metrics).Methods("GET")
}
