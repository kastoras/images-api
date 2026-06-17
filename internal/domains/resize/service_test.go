package resize

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/kastoras/images-api/internal/imageprocessing"
	"github.com/kastoras/images-api/internal/server"
	internal_errors "github.com/kastoras/images-api/internal/utils/errors"
)

// minimalAPIServer builds the smallest APIServer valid for service tests:
// exported fields Semaphore and MaxQueueDepth are set; Cache and Storage are nil.
func minimalAPIServer(semaphoreSize int, maxQueueDepth int) *server.APIServer {
	s := &server.APIServer{
		Semaphore:     make(chan struct{}, semaphoreSize),
		MaxQueueDepth: maxQueueDepth,
	}
	return s
}

// TestService_ProcessImmediate_StorageNil verifies that when Storage is nil,
// processImmediate returns ImageData bytes and no URL.
func TestService_ProcessImmediate_StorageNil(t *testing.T) {
	s := minimalAPIServer(1, 50)
	svc := NewService(s)

	result, err := svc.processImmediate(context.Background(), bytes.NewReader(minimalJPEG), imageprocessing.ResizeOptions{Width: 2, Height: 2, Mode: imageprocessing.ResizeModeExact})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if result.ImageData == nil {
		t.Error("expected ImageData to be non-nil")
	}
	if result.URL != "" {
		t.Errorf("expected empty URL when Storage is nil, got %q", result.URL)
	}
}

// TestService_Resize_SemaphoreFull_NilCacheAndStorage verifies that when the semaphore
// is exhausted AND both Cache and Storage are nil, Resize returns ErrTooManyRequests.
func TestService_Resize_SemaphoreFull_NilCacheAndStorage(t *testing.T) {
	s := minimalAPIServer(1, 50)
	// Fill the semaphore so no slot is available.
	s.Semaphore <- struct{}{}

	svc := NewService(s)

	_, err := svc.Resize(context.Background(), bytes.NewReader(minimalJPEG), imageprocessing.ResizeOptions{Width: 2, Height: 2, Mode: imageprocessing.ResizeModeExact})
	if err == nil {
		t.Fatal("expected error when semaphore is full and cache/storage are nil")
	}
	if !errors.Is(err, internal_errors.ErrTooManyRequests) {
		t.Errorf("expected ErrTooManyRequests, got %v", err)
	}
}

// TestService_Enqueue_NilCache verifies that enqueue with nil Cache returns ErrTooManyRequests.
func TestService_Enqueue_NilCache(t *testing.T) {
	s := minimalAPIServer(1, 50)
	// Cache and Storage are already nil.
	svc := NewService(s)

	_, err := svc.enqueue(context.Background(), bytes.NewReader(minimalJPEG), imageprocessing.ResizeOptions{Width: 2, Height: 2, Mode: imageprocessing.ResizeModeExact})
	if err == nil {
		t.Fatal("expected error when Cache is nil")
	}
	if !errors.Is(err, internal_errors.ErrTooManyRequests) {
		t.Errorf("expected ErrTooManyRequests, got %v", err)
	}
}

// TestService_ProcessImmediate_InvalidImage verifies that garbage bytes produce ErrUnsupportedFormat.
func TestService_ProcessImmediate_InvalidImage(t *testing.T) {
	s := minimalAPIServer(1, 50)
	svc := NewService(s)

	_, err := svc.processImmediate(context.Background(), bytes.NewReader([]byte("not an image")), imageprocessing.ResizeOptions{Width: 10, Height: 10, Mode: imageprocessing.ResizeModeExact})
	if err == nil {
		t.Fatal("expected error for invalid image bytes")
	}
	if !errors.Is(err, internal_errors.ErrUnsupportedFormat) {
		t.Errorf("expected ErrUnsupportedFormat, got %v", err)
	}
}

// TestService_Resize_SemaphoreAvailable_StorageNil verifies the happy path when a semaphore
// slot is free and Storage is nil: returns ImageData, no error.
func TestService_Resize_SemaphoreAvailable_StorageNil(t *testing.T) {
	s := minimalAPIServer(1, 50)
	svc := NewService(s)

	result, err := svc.Resize(context.Background(), bytes.NewReader(minimalJPEG), imageprocessing.ResizeOptions{Width: 2, Height: 2, Mode: imageprocessing.ResizeModeExact})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if result.ImageData == nil {
		t.Error("expected ImageData to be set when Storage is nil")
	}
	if result.Queued {
		t.Error("expected Queued to be false")
	}
}

// TestService_ProcessImmediate_WidthOnly verifies that a zero height (width-only) request
// succeeds and returns image data — imaging.Resize preserves aspect ratio when height=0.
func TestService_ProcessImmediate_WidthOnly(t *testing.T) {
	s := minimalAPIServer(1, 50)
	svc := NewService(s)

	result, err := svc.processImmediate(context.Background(), bytes.NewReader(minimalJPEG), imageprocessing.ResizeOptions{Width: 2, Height: 0, Mode: imageprocessing.ResizeModeExact})
	if err != nil {
		t.Fatalf("expected no error for width-only resize, got: %v", err)
	}
	if result.ImageData == nil {
		t.Error("expected ImageData to be non-nil")
	}
}

// TestService_ProcessImmediate_HeightOnly verifies that a zero width (height-only) request
// succeeds and returns image data — imaging.Resize preserves aspect ratio when width=0.
func TestService_ProcessImmediate_HeightOnly(t *testing.T) {
	s := minimalAPIServer(1, 50)
	svc := NewService(s)

	result, err := svc.processImmediate(context.Background(), bytes.NewReader(minimalJPEG), imageprocessing.ResizeOptions{Width: 0, Height: 2, Mode: imageprocessing.ResizeModeExact})
	if err != nil {
		t.Fatalf("expected no error for height-only resize, got: %v", err)
	}
	if result.ImageData == nil {
		t.Error("expected ImageData to be non-nil")
	}
}
