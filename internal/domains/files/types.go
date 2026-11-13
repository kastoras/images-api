package files

import (
	"context"
	"io"

	"github.com/kastoras/images-api/internal/server"
)

const (
	visibilityPrivate = "private"
	visibilityPublic  = "public"
)

type Handler struct {
	server  *server.APIServer
	service Servicer
}

// Servicer is the interface Handler depends on. *Service satisfies it.
// consumer is always resolved server-side from the authenticated principal
// (or, on the public route, only reaches files marked public).
type Servicer interface {
	Create(ctx context.Context, consumer string, in Upload) (*File, error)
	Open(ctx context.Context, consumer, tenant, id string, publicOnly bool) (*Object, error)
	Delete(ctx context.Context, consumer, tenant, id string) error
}

// Upload is a validated file ready to be stored.
type Upload struct {
	Tenant      string
	Filename    string
	ContentType string
	Visibility  string
	Size        int64
	Body        io.Reader
}

// File describes a stored file.
type File struct {
	ID          string `json:"id"`
	Tenant      string `json:"tenant"`
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	Bytes       int64  `json:"bytes"`
	Visibility  string `json:"visibility"`
	URL         string `json:"url"`
}

// Object is an opened stored file. The caller must close Body.
type Object struct {
	Body        io.ReadCloser
	Size        int64
	ContentType string
	Filename    string
}
