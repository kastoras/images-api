package assets_services

import (
	"context"
	"time"
)

// List returns every master stored for a consumer's tenant. Used for the
// reconciliation sweep and future "show a tenant's assets" use cases.
func (svc *Service) List(ctx context.Context, consumer, tenant string) ([]ListedAsset, error) {
	objects, err := svc.storage.ListKeys(ctx, masterPrefix(consumer, tenant))
	if err != nil {
		return nil, err
	}

	list := make([]ListedAsset, 0, len(objects))
	for _, obj := range objects {
		list = append(list, ListedAsset{
			ID:           hashFromMasterKey(obj.Key),
			Bytes:        obj.Size,
			LastModified: obj.LastModified.UTC().Format(time.RFC3339),
		})
	}
	return list, nil
}
