package resize

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// stubResizer is a test double for the Resizer interface.
type stubResizer struct {
	result *ProcessResult
	err    error
}

func (s *stubResizer) Resize(_ context.Context, _ io.Reader, _ ResizeOptions) (*ProcessResult, error) {
	return s.result, s.err
}

// newHandlerWithStub returns a Handler backed by the given stub without any real service.
func newHandlerWithStub(stub Resizer) *Handler {
	return &Handler{service: stub}
}

// makeResizeRequest builds a multipart request containing minimalJPEG + valid width/height.
func makeResizeRequest(t *testing.T) *http.Request {
	t.Helper()
	return buildMultipartRequest(t, minimalJPEG, "image/jpeg", "100", "80", "")
}

func assertStatus(t *testing.T, rec *httptest.ResponseRecorder, want int) {
	t.Helper()
	if rec.Code != want {
		t.Errorf("expected HTTP %d, got %d (body: %s)", want, rec.Code, rec.Body.String())
	}
}

// TestHandler_ValidationError ensures a bad request body returns 400.
func TestHandler_ValidationError(t *testing.T) {
	stub := &stubResizer{result: &ProcessResult{ImageData: []byte("data")}}
	h := newHandlerWithStub(stub)

	// Send a request with no file part — validation should fail.
	req := buildMultipartRequest(t, nil, "", "100", "80", "")
	rec := httptest.NewRecorder()
	h.Resize(rec, req)

	assertStatus(t, rec, http.StatusBadRequest)
}

// TestHandler_UnsupportedFormat_FromValidation ensures Content-Type mismatches return 415.
func TestHandler_UnsupportedFormat_FromValidation(t *testing.T) {
	stub := &stubResizer{}
	h := newHandlerWithStub(stub)

	req := buildMultipartRequest(t, []byte("not an image"), "image/bmp", "100", "80", "")
	rec := httptest.NewRecorder()
	h.Resize(rec, req)

	assertStatus(t, rec, http.StatusUnsupportedMediaType)
}

// TestHandler_ServiceErrUnsupportedFormat ensures ErrUnsupportedFormat from service → 415.
func TestHandler_ServiceErrUnsupportedFormat(t *testing.T) {
	stub := &stubResizer{err: ErrUnsupportedFormat}
	h := newHandlerWithStub(stub)

	rec := httptest.NewRecorder()
	h.Resize(rec, makeResizeRequest(t))

	assertStatus(t, rec, http.StatusUnsupportedMediaType)
}

// TestHandler_ServiceErrTooManyRequests ensures ErrTooManyRequests → 429.
func TestHandler_ServiceErrTooManyRequests(t *testing.T) {
	stub := &stubResizer{err: ErrTooManyRequests}
	h := newHandlerWithStub(stub)

	rec := httptest.NewRecorder()
	h.Resize(rec, makeResizeRequest(t))

	assertStatus(t, rec, http.StatusTooManyRequests)
}

// TestHandler_ServiceErrQueueFull ensures ErrQueueFull → 503.
func TestHandler_ServiceErrQueueFull(t *testing.T) {
	stub := &stubResizer{err: ErrQueueFull}
	h := newHandlerWithStub(stub)

	rec := httptest.NewRecorder()
	h.Resize(rec, makeResizeRequest(t))

	assertStatus(t, rec, http.StatusServiceUnavailable)
}

// TestHandler_ServiceErrUnknown ensures an unrecognised error → 500.
func TestHandler_ServiceErrUnknown(t *testing.T) {
	stub := &stubResizer{err: errors.New("something unexpected")}
	h := newHandlerWithStub(stub)

	rec := httptest.NewRecorder()
	h.Resize(rec, makeResizeRequest(t))

	assertStatus(t, rec, http.StatusInternalServerError)
}

// TestHandler_QueuedResult ensures Queued==true → 202 with job_id in body.
func TestHandler_QueuedResult(t *testing.T) {
	const jobID = "abc-123"
	stub := &stubResizer{result: &ProcessResult{Queued: true, JobID: jobID}}
	h := newHandlerWithStub(stub)

	rec := httptest.NewRecorder()
	h.Resize(rec, makeResizeRequest(t))

	assertStatus(t, rec, http.StatusAccepted)

	// Body shape: {"data":{"job_id":"abc-123","status":"queued"}}
	var outer struct {
		Data struct {
			JobID  string `json:"job_id"`
			Status string `json:"status"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &outer); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if outer.Data.JobID != jobID {
		t.Errorf("expected job_id=%q, got %q", jobID, outer.Data.JobID)
	}
	if outer.Data.Status != "queued" {
		t.Errorf("expected status=queued, got %q", outer.Data.Status)
	}
}

// TestHandler_ImageDataResult ensures Storage-nil path returns 200 with image/jpeg Content-Type.
func TestHandler_ImageDataResult(t *testing.T) {
	imageBytes := []byte{0xff, 0xd8, 0xff, 0xd9} // stub JPEG bytes
	stub := &stubResizer{result: &ProcessResult{ImageData: imageBytes}}
	h := newHandlerWithStub(stub)

	rec := httptest.NewRecorder()
	h.Resize(rec, makeResizeRequest(t))

	assertStatus(t, rec, http.StatusOK)

	ct := rec.Header().Get("Content-Type")
	if ct != "image/jpeg" {
		t.Errorf("expected Content-Type: image/jpeg, got %q", ct)
	}
	if !bytes.Equal(rec.Body.Bytes(), imageBytes) {
		t.Errorf("response body does not match expected image bytes")
	}
}

// TestHandler_URLResult ensures a URL result returns 200 with url field in JSON.
func TestHandler_URLResult(t *testing.T) {
	const presignedURL = "https://s3.example.com/results/some-result.jpg?sig=xxx"
	stub := &stubResizer{result: &ProcessResult{URL: presignedURL}}
	h := newHandlerWithStub(stub)

	rec := httptest.NewRecorder()
	h.Resize(rec, makeResizeRequest(t))

	assertStatus(t, rec, http.StatusOK)

	// Body shape: {"data":{"url":"..."}}
	var outer struct {
		Data struct {
			URL string `json:"url"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &outer); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if outer.Data.URL != presignedURL {
		t.Errorf("expected url=%q, got %q", presignedURL, outer.Data.URL)
	}
}
