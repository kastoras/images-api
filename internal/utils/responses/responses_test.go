package responses

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// decodeBody is a test helper that unmarshals the recorder body into a map.
func decodeBody(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&m); err != nil {
		t.Fatalf("decode JSON body: %v (raw: %q)", err, rec.Body.String())
	}
	return m
}

func assertStatus(t *testing.T, rec *httptest.ResponseRecorder, want int) {
	t.Helper()
	if rec.Code != want {
		t.Errorf("expected HTTP %d, got %d", want, rec.Code)
	}
}

func assertContentType(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	ct := rec.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("expected Content-Type: application/json, got %q", ct)
	}
}

// TestSuccess checks 200 and that the payload is wrapped in {"data": ...}.
func TestSuccess(t *testing.T) {
	rec := httptest.NewRecorder()
	Success(rec, map[string]string{"key": "value"})

	assertStatus(t, rec, http.StatusOK)
	assertContentType(t, rec)

	body := decodeBody(t, rec)
	if _, ok := body["data"]; !ok {
		t.Errorf("expected 'data' key in response body, got %v", body)
	}
}

// TestCreated checks 201 and that the payload is wrapped in {"data": ...}.
func TestCreated(t *testing.T) {
	rec := httptest.NewRecorder()
	Created(rec, map[string]string{"id": "123"})

	assertStatus(t, rec, http.StatusCreated)
	assertContentType(t, rec)

	body := decodeBody(t, rec)
	if _, ok := body["data"]; !ok {
		t.Errorf("expected 'data' key in response body, got %v", body)
	}
}

// TestAccepted checks 202 and that the job_id and status fields are present.
func TestAccepted(t *testing.T) {
	const jobID = "job-abc"
	rec := httptest.NewRecorder()
	Accepted(rec, jobID)

	assertStatus(t, rec, http.StatusAccepted)
	assertContentType(t, rec)

	var outer struct {
		Data struct {
			JobID  string `json:"job_id"`
			Status string `json:"status"`
		} `json:"data"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&outer); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if outer.Data.JobID != jobID {
		t.Errorf("expected job_id=%q, got %q", jobID, outer.Data.JobID)
	}
	if outer.Data.Status != "queued" {
		t.Errorf("expected status=queued, got %q", outer.Data.Status)
	}
}

// TestBadRequest checks 400 and that the error field contains the supplied message.
func TestBadRequest(t *testing.T) {
	rec := httptest.NewRecorder()
	BadRequest(rec, "field X is required")

	assertStatus(t, rec, http.StatusBadRequest)
	assertContentType(t, rec)

	body := decodeBody(t, rec)
	if body["error"] != "field X is required" {
		t.Errorf("unexpected error field: %v", body["error"])
	}
}

// TestNotFound checks 404 and that the error field says "not found".
func TestNotFound(t *testing.T) {
	rec := httptest.NewRecorder()
	NotFound(rec)

	assertStatus(t, rec, http.StatusNotFound)
	assertContentType(t, rec)

	body := decodeBody(t, rec)
	if body["error"] != "not found" {
		t.Errorf("unexpected error field: %v", body["error"])
	}
}

// TestTooManyRequests checks 429 and the error message.
func TestTooManyRequests(t *testing.T) {
	rec := httptest.NewRecorder()
	TooManyRequests(rec)

	assertStatus(t, rec, http.StatusTooManyRequests)
	assertContentType(t, rec)

	body := decodeBody(t, rec)
	if body["error"] != "too many concurrent requests" {
		t.Errorf("unexpected error field: %v", body["error"])
	}
}

// TestInternalServerError checks 500 and the error message.
func TestInternalServerError(t *testing.T) {
	rec := httptest.NewRecorder()
	InternalServerError(rec)

	assertStatus(t, rec, http.StatusInternalServerError)
	assertContentType(t, rec)

	body := decodeBody(t, rec)
	if body["error"] != "internal server error" {
		t.Errorf("unexpected error field: %v", body["error"])
	}
}

// TestServiceUnavailable checks 503 with a custom message.
func TestServiceUnavailable(t *testing.T) {
	rec := httptest.NewRecorder()
	ServiceUnavailable(rec, "queue full, try later")

	assertStatus(t, rec, http.StatusServiceUnavailable)
	assertContentType(t, rec)

	body := decodeBody(t, rec)
	if body["error"] != "queue full, try later" {
		t.Errorf("unexpected error field: %v", body["error"])
	}
}

// TestUnauthorized checks 401.
func TestUnauthorized(t *testing.T) {
	rec := httptest.NewRecorder()
	Unauthorized(rec)

	assertStatus(t, rec, http.StatusUnauthorized)
	assertContentType(t, rec)

	body := decodeBody(t, rec)
	if body["error"] != "unauthorized" {
		t.Errorf("unexpected error field: %v", body["error"])
	}
}
