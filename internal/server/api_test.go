package server

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rs/zerolog"
)

func testServer() *APIServer {
	return &APIServer{Log: zerolog.Nop()}
}

// stubPing fails the first failures calls, then succeeds.
func stubPing(failures int, err error) (func(context.Context) error, *int) {
	calls := 0
	return func(context.Context) error {
		calls++
		if calls <= failures {
			return err
		}
		return nil
	}, &calls
}

func TestWaitForDependencySucceedsImmediately(t *testing.T) {
	s := testServer()
	ping, calls := stubPing(0, nil)

	if err := s.waitForDependency("redis", time.Now().Add(time.Second), ping); err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if *calls != 1 {
		t.Fatalf("expected 1 call, got %d", *calls)
	}
}

func TestWaitForDependencyRetriesUntilReady(t *testing.T) {
	s := testServer()
	ping, calls := stubPing(2, errors.New("not ready"))

	start := time.Now()
	if err := s.waitForDependency("s3", time.Now().Add(5*time.Second), ping); err != nil {
		t.Fatalf("expected eventual success, got %v", err)
	}

	if *calls != 3 {
		t.Fatalf("expected 3 calls, got %d", *calls)
	}
	// 500ms + 1s of backoff between the three attempts.
	if elapsed := time.Since(start); elapsed < 1400*time.Millisecond {
		t.Fatalf("expected backoff between attempts, only waited %v", elapsed)
	}
}

func TestWaitForDependencyGivesUpAtDeadlineAndReturnsLastError(t *testing.T) {
	s := testServer()
	wantErr := errors.New("connection refused")
	ping, calls := stubPing(1000, wantErr)

	start := time.Now()
	err := s.waitForDependency("redis", time.Now().Add(1200*time.Millisecond), ping)

	if !errors.Is(err, wantErr) {
		t.Fatalf("expected the dependency error to be returned, got %v", err)
	}
	if *calls < 2 {
		t.Fatalf("expected multiple attempts before giving up, got %d", *calls)
	}
	// Must not overshoot the deadline waiting for a retry it will never make.
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("gave up too late: %v", elapsed)
	}
}

// A zero deadline preserves the original single-attempt behaviour, so setting
// DEPENDENCY_WAIT_TIMEOUT=0 opts back out of retrying.
func TestWaitForDependencyZeroDeadlineTriesOnce(t *testing.T) {
	s := testServer()
	wantErr := errors.New("nope")
	ping, calls := stubPing(1000, wantErr)

	err := s.waitForDependency("s3", time.Time{}, ping)

	if !errors.Is(err, wantErr) {
		t.Fatalf("expected %v, got %v", wantErr, err)
	}
	if *calls != 1 {
		t.Fatalf("expected exactly 1 call, got %d", *calls)
	}
}
