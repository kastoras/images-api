package metrics

import (
	"net/http"

	"github.com/kastoras/images-api/internal/server"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Handler struct {
	server *server.APIServer
}

func NewHandler(s *server.APIServer) *Handler {
	return &Handler{server: s}
}

func (h *Handler) Metrics(w http.ResponseWriter, r *http.Request) {
	promhttp.HandlerFor(h.server.Metrics, promhttp.HandlerOpts{}).ServeHTTP(w, r)
}
