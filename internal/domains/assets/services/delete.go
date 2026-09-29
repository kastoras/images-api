package assets_services

import (
	"context"
	"errors"

	"github.com/kastoras/images-api/internal/server"
)

// Delete removes the master plus every cached derivative under it.
// Idempotent: returns nil even if the master was already gone.
func (svc *Service) Delete(ctx context.Context, consumer, tenant, hash string) error {
	key, err := svc.resolveMasterKey(ctx, consumer, tenant, hash)
	if err != nil && !errors.Is(err, server.ErrNotFound) {
		return err
	}
	if err == nil {
		if delErr := svc.storage.Delete(ctx, key); delErr != nil {
			return delErr
		}
	}
	return svc.storage.DeletePrefix(ctx, derivativePrefix(consumer, tenant, hash))
}
