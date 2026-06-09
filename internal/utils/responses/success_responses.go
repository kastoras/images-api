package responses

import (
	"encoding/json"
	"net/http"
)

type Response struct {
	Data any `json:"data"`
}

type acceptedData struct {
	JobID  string `json:"job_id"`
	Status string `json:"status"`
}

func Success(w http.ResponseWriter, data any) {
	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	resp := Response{Data: data}

	json.NewEncoder(w).Encode(resp)
}

func Created(w http.ResponseWriter, data any) {
	w.Header().Add("Content-Type", "application/json")

	resp := Response{Data: data}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(resp)
}

func NoContent(w http.ResponseWriter) {
	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(http.StatusNoContent)
	json.NewEncoder(w)
}

func Accepted(w http.ResponseWriter, jobID string) {
	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(Response{Data: acceptedData{JobID: jobID, Status: "queued"}})
}

func JPEGImage(w http.ResponseWriter, img []byte) {
	w.Header().Set("Content-Type", "image/jpeg")
	w.WriteHeader(http.StatusOK)
	w.Write(img)
}
