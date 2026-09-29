package assets_services

import (
	"context"
	"strings"
	"testing"

	"github.com/kastoras/images-api/internal/imageprocessing"
)

func TestDelete_RemovesMasterAndDerivatives(t *testing.T) {
	st := newFakeStorage()
	svc := newTestService(st)
	m := createMaster(t, svc, "acme", "t1")
	other := createMasterWith(t, svc, "acme", "t1", 90, 90)
	opts := imageprocessing.ResizeOptions{Width: 10, Height: 10, Mode: imageprocessing.ResizeModeExact}
	for _, id := range []string{m.ID, other.ID} {
		if _, _, err := svc.Render(context.Background(), "acme", "t1", id, opts, ""); err != nil {
			t.Fatalf("Render: %v", err)
		}
	}

	if err := svc.Delete(context.Background(), "acme", "t1", m.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	for _, k := range st.keys() {
		if strings.Contains(k, m.ID) {
			t.Errorf("leftover object for deleted master: %s", k)
		}
	}
	if !st.has(masterKey("acme", "t1", other.ID, "jpg")) {
		t.Error("unrelated master was deleted")
	}
	if !st.has(derivativeKey("acme", "t1", other.ID, 10, 10, "exact", "jpg")) {
		t.Error("unrelated derivative was deleted")
	}
}

func TestDelete_IdempotentWhenMissing(t *testing.T) {
	svc := newTestService(newFakeStorage())
	if err := svc.Delete(context.Background(), "acme", "t1", strings.Repeat("c", 64)); err != nil {
		t.Errorf("Delete of missing master should be nil, got %v", err)
	}
}

func TestDelete_OtherConsumerCannotDelete(t *testing.T) {
	st := newFakeStorage()
	svc := newTestService(st)
	m := createMaster(t, svc, "acme", "t1")

	if err := svc.Delete(context.Background(), "other", "t1", m.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if !st.has(masterKey("acme", "t1", m.ID, "jpg")) {
		t.Error("another consumer deleted acme's master")
	}
}
