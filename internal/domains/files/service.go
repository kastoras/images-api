package files

import (
	"context"
	"fmt"
	"io"

	"github.com/google/uuid"
	"github.com/kastoras/images-api/internal/server"
)

// Storage is the subset of object storage the service depends on.
// *server.ObjectStorage satisfies it; tests substitute an in-memory fake.
type Storage interface {
	UploadWithMetadata(ctx context.Context, key string, body io.Reader, size int64, metadata map[string]string) error
	Download(ctx context.Context, key string) (io.ReadCloser, error)
	HeadObject(ctx context.Context, key string) (metadata map[string]string, size int64, err error)
	Delete(ctx context.Context, key string) error
}

type Service struct {
	storage Storage
}

func NewService(s *server.APIServer) *Service {
	svc := &Service{}
	// Guard against a typed-nil pointer becoming a non-nil interface; handlers
	// already refuse requests when storage isn't configured.
	if s.Storage != nil {
		svc.storage = s.Storage
	}
	return svc
}

func objectKey(consumer, tenant, id string) string {
	return fmt.Sprintf("files/%s/%s/%s", consumer, tenant, id)
}

// Create stores the file unchanged under a fresh uuid. A uuid rather than a
// content hash: the same bytes may be uploaded again with a different
// visibility and must not collapse into one object.
func (svc *Service) Create(ctx context.Context, consumer string, in Upload) (*File, error) {
	id := uuid.NewString()
	metadata := map[string]string{
		"consumer":   consumer,
		"tenant":     in.Tenant,
		"filename":   in.Filename,
		"mime":       in.ContentType,
		"visibility": in.Visibility,
	}
	if err := svc.storage.UploadWithMetadata(ctx, objectKey(consumer, in.Tenant, id), in.Body, in.Size, metadata); err != nil {
		return nil, fmt.Errorf("upload file: %w", err)
	}

	url := "/api/v1/files/" + in.Tenant + "/" + id
	if in.Visibility == visibilityPublic {
		url = "/f/" + consumer + "/" + in.Tenant + "/" + id
	}

	return &File{
		ID:          id,
		Tenant:      in.Tenant,
		Filename:    in.Filename,
		ContentType: in.ContentType,
		Bytes:       in.Size,
		Visibility:  in.Visibility,
		URL:         url,
	}, nil
}

// Open returns the stored file. With publicOnly, a private file reports
// server.ErrNotFound so the public route never reveals that it exists.
func (svc *Service) Open(ctx context.Context, consumer, tenant, id string, publicOnly bool) (*Object, error) {
	key := objectKey(consumer, tenant, id)

	metadata, size, err := svc.storage.HeadObject(ctx, key)
	if err != nil {
		return nil, err
	}
	if publicOnly && metadata["visibility"] != visibilityPublic {
		return nil, server.ErrNotFound
	}

	body, err := svc.storage.Download(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("download file: %w", err)
	}

	contentType := metadata["mime"]
	if !allowedContentTypes[contentType] {
		contentType = "application/octet-stream"
	}

	return &Object{Body: body, Size: size, ContentType: contentType, Filename: metadata["filename"]}, nil
}

// Delete is idempotent: S3 reports success for a key that does not exist.
func (svc *Service) Delete(ctx context.Context, consumer, tenant, id string) error {
	return svc.storage.Delete(ctx, objectKey(consumer, tenant, id))
}
