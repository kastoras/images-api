package health

import (
	"net/http"
	"time"

	"github.com/kastoras/images-api/internal/server"
	"github.com/kastoras/images-api/internal/utils/responses"
)

type Handler struct {
	server *server.APIServer
}

func NewHandler(s *server.APIServer) *Handler {
	return &Handler{server: s}
}

type healthResponse struct {
	Status    string `json:"status"`
	Timestamp string `json:"timestamp"`
}

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	responses.Success(w, healthResponse{
		Status:    "ok",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	})
}
