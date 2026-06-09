package jobs

import (
	"errors"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/kastoras/images-api/internal/models"
	"github.com/kastoras/images-api/internal/server"
	"github.com/kastoras/images-api/internal/utils/responses"
	"github.com/redis/go-redis/v9"
)

type Handler struct {
	server *server.APIServer
}

func NewHandler(s *server.APIServer) *Handler {
	return &Handler{server: s}
}

func (h *Handler) GetJob(w http.ResponseWriter, r *http.Request) {
	if h.server.Cache == nil {
		responses.ServiceUnavailable(w, "job tracking unavailable")
		return
	}

	jobID := mux.Vars(r)["id"]

	var job models.Job
	if err := h.server.Cache.GetJobMeta(r.Context(), jobID, &job); err != nil {
		if errors.Is(err, redis.Nil) {
			responses.NotFound(w)
			return
		}
		responses.InternalServerError(w)
		return
	}

	responses.Success(w, job)
}
