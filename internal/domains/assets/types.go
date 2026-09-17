package assets

import (
	"context"
	"io"

	"github.com/kastoras/images-api/internal/imageprocessing"
	"github.com/kastoras/images-api/internal/server"
	internal_errors "github.com/kastoras/images-api/internal/utils/errors"
)

var ErrUnsupportedFormat = internal_errors.ErrUnsupportedFormat

type Handler struct {
	server  *server.APIServer
	service Servicer
}

// Master describes a stored, normalized image belonging to one consumer's
// (calling service's) tenant namespace.
type Master struct {
	ID       string `json:"id"`
	Consumer string `json:"consumer"`
	Tenant   string `json:"tenant"`
	Width    int    `json:"width"`
	Height   int    `json:"height"`
	Format   string `json:"format"`
	Bytes    int64  `json:"bytes"`
}

// ListedAsset is one entry in a tenant's master listing.
type ListedAsset struct {
	ID           string `json:"id"`
	Bytes        int64  `json:"bytes"`
	LastModified string `json:"last_modified"`
}

type assetsList struct {
	Tenant string        `json:"tenant"`
	Assets []ListedAsset `json:"assets"`
}

// Servicer is the interface Handler depends on. *Service satisfies it.
// consumer is always resolved server-side from the authenticated principal —
// never accepted from the request — so one consumer can never address
// another consumer's data no matter what tenant value it sends.
type Servicer interface {
	Create(ctx context.Context, consumer, tenant string, file io.Reader) (*Master, error)
	Render(ctx context.Context, consumer, tenant, hash string, opts imageprocessing.ResizeOptions, format string) (data []byte, contentType string, err error)
	Delete(ctx context.Context, consumer, tenant, hash string) error
	List(ctx context.Context, consumer, tenant string) ([]ListedAsset, error)
}
