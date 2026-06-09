package responses

import (
	"encoding/json"
	"net/http"
)

type errorResponse struct {
	Error string `json:"error"`
}

func BadRequest(w http.ResponseWriter, message string) {
	writeError(w, http.StatusBadRequest, message)
}

func Unauthorized(w http.ResponseWriter) {
	writeError(w, http.StatusUnauthorized, "unauthorized")
}

func NotFound(w http.ResponseWriter) {
	writeError(w, http.StatusNotFound, "not found")
}

func UnsupportedMediaType(w http.ResponseWriter) {
	writeError(w, http.StatusUnsupportedMediaType, "unsupported image format: use JPEG, PNG, GIF or TIFF")
}

func TooManyRequests(w http.ResponseWriter) {
	writeError(w, http.StatusTooManyRequests, "too many concurrent requests")
}

func ServiceUnavailable(w http.ResponseWriter, message string) {
	writeError(w, http.StatusServiceUnavailable, message)
}

func InternalServerError(w http.ResponseWriter) {
	writeError(w, http.StatusInternalServerError, "internal server error")
}

func writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(errorResponse{Error: message})
}
