package assets

import (
	"context"
	"io"

	assets_services "github.com/kastoras/images-api/internal/domains/assets/services"
	"github.com/kastoras/images-api/internal/imageprocessing"
	"github.com/kastoras/images-api/internal/server"
	internal_errors "github.com/kastoras/images-api/internal/utils/errors"
)

var ErrUnsupportedFormat = internal_errors.ErrUnsupportedFormat

type Handler struct {
	server  *server.APIServer
	service Servicer
}

type assetsList struct {
	Tenant string                        `json:"tenant"`
	Assets []assets_services.ListedAsset `json:"assets"`
}

// Servicer is the interface Handler depends on. *Service satisfies it.
// consumer is always resolved server-side from the authenticated principal —
// never accepted from the request — so one consumer can never address
// another consumer's data no matter what tenant value it sends.
type Servicer interface {
	Create(ctx context.Context, consumer, tenant string, file io.Reader) (*assets_services.Master, error)
	Render(ctx context.Context, consumer, tenant, hash string, opts imageprocessing.ResizeOptions, format string) (data []byte, contentType string, err error)
	Delete(ctx context.Context, consumer, tenant, hash string) error
	List(ctx context.Context, consumer, tenant string) ([]assets_services.ListedAsset, error)
}
