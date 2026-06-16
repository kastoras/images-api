package responses

import (
	"encoding/json"
	"fmt"
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

	resp := Response{Data: data}

	w.WriteHeader(http.StatusOK)
	encodeAndWrite(w, resp)
}

func Created(w http.ResponseWriter, data any) {
	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)

	resp := Response{Data: data}
	encodeAndWrite(w, resp)
}

func NoContent(w http.ResponseWriter) {
	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(http.StatusNoContent)
	json.NewEncoder(w)
}

func Accepted(w http.ResponseWriter, jobID string) {
	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)

	resp := Response{Data: acceptedData{JobID: jobID, Status: "queued"}}
	encodeAndWrite(w, resp)
}

func JPEGImage(w http.ResponseWriter, img []byte) {
	w.Header().Set("Content-Type", "image/jpeg")
	w.WriteHeader(http.StatusOK)

	_, err := w.Write(img)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		fmt.Printf("not able to pring image response, %v \n", err.Error())
		InternalServerError(w)
	}
}

func encodeAndWrite(w http.ResponseWriter, resp Response) {
	err := json.NewEncoder(w).Encode(resp)
	if err != nil {
		fmt.Printf("not able to encode response, %v \n", err.Error())
		InternalServerError(w)
	}
}
