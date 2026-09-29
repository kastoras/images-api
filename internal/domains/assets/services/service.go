package assets_services

import (
	"context"
	"io"

	"github.com/kastoras/images-api/internal/server"
)

// Storage is the subset of object storage the service depends on.
// *server.ObjectStorage satisfies it; tests substitute an in-memory fake.
type Storage interface {
	Upload(ctx context.Context, key string, body io.Reader, size int64) error
	UploadWithMetadata(ctx context.Context, key string, body io.Reader, size int64, metadata map[string]string) error
	Download(ctx context.Context, key string) (io.ReadCloser, error)
	Delete(ctx context.Context, key string) error
	DeletePrefix(ctx context.Context, prefix string) error
	HeadObject(ctx context.Context, key string) (metadata map[string]string, size int64, err error)
	ListKeys(ctx context.Context, prefix string) ([]server.ObjectInfo, error)
}

type Service struct {
	server  *server.APIServer
	storage Storage
}

func NewService(s *server.APIServer) *Service {
	svc := &Service{server: s}
	// Guard against a typed-nil pointer becoming a non-nil interface; handlers
	// already refuse requests when storage isn't configured.
	if s.Storage != nil {
		svc.storage = s.Storage
	}
	return svc
}

// resolveMasterKey finds a master's full object key (including its
// extension) from its hash alone. sha256 hex is always 64 characters, so a
// prefix match on "masters/<consumer>/<tenant>/<hash>" can never ambiguously
// match a different master.
func (svc *Service) resolveMasterKey(ctx context.Context, consumer, tenant, hash string) (string, error) {
	objects, err := svc.storage.ListKeys(ctx, masterPrefix(consumer, tenant)+hash)
	if err != nil {
		return "", err
	}
	if len(objects) == 0 {
		return "", server.ErrNotFound
	}
	return objects[0].Key, nil
}
