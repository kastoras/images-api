package models

import "time"

const (
	StatusQueued     = "queued"
	StatusProcessing = "processing"
	StatusComplete   = "complete"
	StatusFailed     = "failed"
)

type Job struct {
	ID        string         `json:"id"`
	Operation string         `json:"operation"`
	Status    string         `json:"status"`
	Params    map[string]any `json:"params"`
	ResultURL string         `json:"result_url,omitempty"`
	Error     string         `json:"error,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
}
