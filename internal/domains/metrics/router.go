package metrics

import (
	"github.com/gorilla/mux"
	"github.com/kastoras/images-api/internal/server"
)

// Register wires the unauthenticated /metrics route — same trust tier as
// /health: protected by network placement (only reachable on the host's
// Tailscale interface, see /monitoring), not application auth.
func Register(router *mux.Router, s *server.APIServer) {
	h := NewHandler(s)
	router.HandleFunc("/metrics", h.Metrics).Methods("GET")
}
