package assets_services

import (
	"context"
	"testing"
)

func TestList_ScopedToConsumerAndTenant(t *testing.T) {
	st := newFakeStorage()
	svc := newTestService(st)
	a := createMasterWith(t, svc, "acme", "t1", 30, 30)
	b := createMasterWith(t, svc, "acme", "t1", 40, 40)
	createMasterWith(t, svc, "acme", "t2", 50, 50)
	createMasterWith(t, svc, "other", "t1", 60, 60)

	list, err := svc.List(context.Background(), "acme", "t1")
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	got := map[string]ListedAsset{}
	for _, l := range list {
		got[l.ID] = l
	}
	if len(list) != 2 || got[a.ID].ID == "" || got[b.ID].ID == "" {
		t.Fatalf("expected exactly masters %s and %s, got %+v", a.ID, b.ID, list)
	}
	if got[a.ID].LastModified != "2026-01-02T03:04:05Z" || got[a.ID].Bytes != a.Bytes {
		t.Errorf("unexpected entry: %+v", got[a.ID])
	}
}

func TestList_EmptyIsNonNil(t *testing.T) {
	list, err := newTestService(newFakeStorage()).List(context.Background(), "acme", "t1")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if list == nil || len(list) != 0 {
		t.Errorf("expected empty non-nil slice, got %#v", list)
	}
}
