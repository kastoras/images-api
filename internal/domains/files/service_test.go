package files

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"

	"github.com/kastoras/images-api/internal/server"
)

func createFile(t *testing.T, svc *Service, consumer, visibility string) *File {
	t.Helper()
	f, err := svc.Create(context.Background(), consumer, Upload{
		Tenant: "t1", Filename: "a.pdf", ContentType: "application/pdf",
		Visibility: visibility, Size: int64(len(pdfBytes)), Body: bytes.NewReader(pdfBytes),
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	return f
}

func TestServiceRoundTrip(t *testing.T) {
	svc := &Service{storage: newFakeStorage()}
	f := createFile(t, svc, "acme", visibilityPrivate)

	if f.URL != "/api/v1/files/t1/"+f.ID {
		t.Errorf("private url = %q", f.URL)
	}

	obj, err := svc.Open(context.Background(), "acme", "t1", f.ID, false)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	got, _ := io.ReadAll(obj.Body)
	if !bytes.Equal(got, pdfBytes) || obj.ContentType != "application/pdf" || obj.Filename != "a.pdf" || obj.Size != int64(len(pdfBytes)) {
		t.Fatalf("got %+v body=%q", obj, got)
	}
}

func TestServicePrivateHiddenFromPublicRoute(t *testing.T) {
	svc := &Service{storage: newFakeStorage()}
	f := createFile(t, svc, "acme", visibilityPrivate)

	if _, err := svc.Open(context.Background(), "acme", "t1", f.ID, true); !errors.Is(err, server.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestServicePublicFile(t *testing.T) {
	svc := &Service{storage: newFakeStorage()}
	f := createFile(t, svc, "acme", visibilityPublic)

	if f.URL != "/f/acme/t1/"+f.ID {
		t.Errorf("public url = %q", f.URL)
	}
	if _, err := svc.Open(context.Background(), "acme", "t1", f.ID, true); err != nil {
		t.Fatalf("public Open: %v", err)
	}
}

func TestServiceConsumerIsolation(t *testing.T) {
	svc := &Service{storage: newFakeStorage()}
	f := createFile(t, svc, "acme", visibilityPublic)

	if _, err := svc.Open(context.Background(), "other", "t1", f.ID, false); !errors.Is(err, server.ErrNotFound) {
		t.Fatalf("other consumer must not see file, got %v", err)
	}
}

func TestServiceDelete(t *testing.T) {
	svc := &Service{storage: newFakeStorage()}
	f := createFile(t, svc, "acme", visibilityPrivate)

	if err := svc.Delete(context.Background(), "acme", "t1", f.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := svc.Open(context.Background(), "acme", "t1", f.ID, false); !errors.Is(err, server.ErrNotFound) {
		t.Fatalf("want ErrNotFound after delete, got %v", err)
	}
	if err := svc.Delete(context.Background(), "acme", "t1", f.ID); err != nil {
		t.Fatalf("second Delete must be idempotent: %v", err)
	}
}

// A stored type outside the allow-list must never be echoed back: on the
// public route that would let bad metadata serve HTML.
func TestServiceUnknownStoredTypeServedAsOctetStream(t *testing.T) {
	st := newFakeStorage()
	key := objectKey("acme", "t1", testID)
	st.objects[key] = []byte("<html></html>")
	st.metadata[key] = map[string]string{"mime": "text/html", "visibility": visibilityPublic}

	obj, err := (&Service{storage: st}).Open(context.Background(), "acme", "t1", testID, true)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if obj.ContentType != "application/octet-stream" {
		t.Fatalf("content type = %q", obj.ContentType)
	}
}
