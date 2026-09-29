package assets_services

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	internal_errors "github.com/kastoras/images-api/internal/utils/errors"
)

func TestCreate_StoresMasterWithMetadata(t *testing.T) {
	st := newFakeStorage()
	svc := newTestService(st)

	m, err := svc.Create(context.Background(), "acme", "t1", bytes.NewReader(testPNG(t, 100, 50)))
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if m.Consumer != "acme" || m.Tenant != "t1" || m.Width != 100 || m.Height != 50 {
		t.Errorf("unexpected master: %+v", m)
	}
	if len(m.ID) != 64 {
		t.Errorf("expected sha256 hex id, got %q", m.ID)
	}
	key := masterKey("acme", "t1", m.ID, extForFormat(m.Format))
	if !st.has(key) {
		t.Fatalf("master not stored at %s; keys=%v", key, st.keys())
	}
	if int64(len(st.objects[key])) != m.Bytes {
		t.Errorf("Bytes = %d, stored %d", m.Bytes, len(st.objects[key]))
	}
	md := st.metadata[key]
	if md["consumer"] != "acme" || md["tenant"] != "t1" || md["width"] != "100" || md["height"] != "50" || md["format"] != m.Format {
		t.Errorf("unexpected metadata: %v", md)
	}
}

func TestCreate_NormalizesToMaxDimension(t *testing.T) {
	svc := newTestService(newFakeStorage())

	m, err := svc.Create(context.Background(), "acme", "t1", bytes.NewReader(testPNG(t, 800, 400)))
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if m.Width > 200 || m.Height > 200 {
		t.Errorf("expected dimensions capped at 200, got %dx%d", m.Width, m.Height)
	}
}

func TestCreate_DedupSkipsSecondUpload(t *testing.T) {
	st := newFakeStorage()
	svc := newTestService(st)
	src := testPNG(t, 60, 60)

	first, err := svc.Create(context.Background(), "acme", "t1", bytes.NewReader(src))
	if err != nil {
		t.Fatalf("first Create: %v", err)
	}
	second, err := svc.Create(context.Background(), "acme", "t1", bytes.NewReader(src))
	if err != nil {
		t.Fatalf("second Create: %v", err)
	}

	if first.ID != second.ID {
		t.Errorf("ids differ: %s vs %s", first.ID, second.ID)
	}
	if st.uploads != 1 {
		t.Errorf("expected 1 upload, got %d", st.uploads)
	}
}

func TestCreate_SameContentDifferentConsumerOrTenantStoredSeparately(t *testing.T) {
	st := newFakeStorage()
	svc := newTestService(st)
	src := testPNG(t, 60, 60)

	for _, c := range [][2]string{{"acme", "t1"}, {"acme", "t2"}, {"other", "t1"}} {
		if _, err := svc.Create(context.Background(), c[0], c[1], bytes.NewReader(src)); err != nil {
			t.Fatalf("Create %v: %v", c, err)
		}
	}
	if st.uploads != 3 {
		t.Errorf("expected 3 uploads, got %d (%v)", st.uploads, st.keys())
	}
}

func TestCreate_InvalidImage(t *testing.T) {
	st := newFakeStorage()
	svc := newTestService(st)

	if _, err := svc.Create(context.Background(), "acme", "t1", strings.NewReader("not an image")); err == nil {
		t.Fatal("expected error for undecodable input")
	}
	if len(st.keys()) != 0 {
		t.Errorf("nothing should be stored, got %v", st.keys())
	}
}

func TestCreate_RejectsOversizedSource(t *testing.T) {
	svc := newTestService(newFakeStorage())
	svc.server.MaxSourceMegapixels = 1

	_, err := svc.Create(context.Background(), "acme", "t1", bytes.NewReader(testPNG(t, 1200, 1000)))
	var verr *internal_errors.ValidationError
	if !errors.As(err, &verr) {
		t.Fatalf("expected ValidationError, got %v", err)
	}
}

func TestCreate_StorageErrors(t *testing.T) {
	boom := errors.New("boom")

	t.Run("head failure", func(t *testing.T) {
		st := newFakeStorage()
		st.headErr = boom
		_, err := newTestService(st).Create(context.Background(), "a", "t", bytes.NewReader(testPNG(t, 20, 20)))
		if !errors.Is(err, boom) {
			t.Errorf("expected wrapped boom, got %v", err)
		}
		if st.uploads != 0 {
			t.Error("must not upload when head fails with non-NotFound error")
		}
	})

	t.Run("upload failure", func(t *testing.T) {
		st := newFakeStorage()
		st.uploadErr = boom
		_, err := newTestService(st).Create(context.Background(), "a", "t", bytes.NewReader(testPNG(t, 20, 20)))
		if !errors.Is(err, boom) {
			t.Errorf("expected wrapped boom, got %v", err)
		}
	})
}
