package assets_services

import (
	"context"

	"github.com/kastoras/images-api/internal/server"
)

type Service struct {
	server *server.APIServer
}

func NewService(s *server.APIServer) *Service {
	return &Service{server: s}
}

// resolveMasterKey finds a master's full object key (including its
// extension) from its hash alone. sha256 hex is always 64 characters, so a
// prefix match on "masters/<consumer>/<tenant>/<hash>" can never ambiguously
// match a different master.
func (svc *Service) resolveMasterKey(ctx context.Context, consumer, tenant, hash string) (string, error) {
	objects, err := svc.server.Storage.ListKeys(ctx, masterPrefix(consumer, tenant)+hash)
	if err != nil {
		return "", err
	}
	if len(objects) == 0 {
		return "", server.ErrNotFound
	}
	return objects[0].Key, nil
}
