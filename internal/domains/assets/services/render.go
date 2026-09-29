package assets_services

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/kastoras/images-api/internal/imageprocessing"
)

// Render returns the requested derivative, generating and caching it under a
// deterministic key on first request and serving straight from cache after.
func (svc *Service) Render(ctx context.Context, consumer, tenant, hash string, opts imageprocessing.ResizeOptions, format string) ([]byte, string, error) {
	masterObjectKey, err := svc.resolveMasterKey(ctx, consumer, tenant, hash)
	if err != nil {
		return nil, "", err
	}

	if format == "" {
		format = formatFromExt(strings.TrimPrefix(filepath.Ext(masterObjectKey), "."))
	}
	contentType := contentTypeForFormat(format)

	derivKey := derivativeKey(consumer, tenant, hash, opts.Width, opts.Height, string(opts.Mode), extForFormat(format))

	data, cached, err := svc.readCachedDerivative(ctx, derivKey)
	if err != nil {
		return nil, "", err
	}
	if cached {
		return data, contentType, nil
	}

	data, err = svc.generateDerivative(ctx, masterObjectKey, derivKey, opts, format)
	if err != nil {
		return nil, "", err
	}
	return data, contentType, nil
}

// readCachedDerivative returns the cached derivative bytes if present. A
// failed download is treated as a cache miss; only a failure while reading an
// existing object is an error.
func (svc *Service) readCachedDerivative(ctx context.Context, derivKey string) ([]byte, bool, error) {
	body, err := svc.storage.Download(ctx, derivKey)
	if err != nil {
		return nil, false, nil
	}
	defer func() { _ = body.Close() }()

	data, err := io.ReadAll(body)
	if err != nil {
		return nil, false, fmt.Errorf("read cached derivative: %w", err)
	}
	return data, true, nil
}

// generateDerivative resizes the master into the requested format and stores
// the result under derivKey so later requests are served from cache.
func (svc *Service) generateDerivative(ctx context.Context, masterObjectKey, derivKey string, opts imageprocessing.ResizeOptions, format string) ([]byte, error) {
	body, err := svc.storage.Download(ctx, masterObjectKey)
	if err != nil {
		return nil, fmt.Errorf("download master: %w", err)
	}
	defer func() { _ = body.Close() }()

	src, err := imageprocessing.Decode(body)
	if err != nil {
		return nil, fmt.Errorf("decode master: %w", err)
	}

	resized := imageprocessing.Resize(src, opts)

	var buf bytes.Buffer
	if _, err := encodeAs(&buf, resized, format, svc.server.MasterJPEGQuality); err != nil {
		return nil, fmt.Errorf("encode derivative: %w", err)
	}

	if err := svc.storage.Upload(ctx, derivKey, bytes.NewReader(buf.Bytes()), int64(buf.Len())); err != nil {
		return nil, fmt.Errorf("upload derivative: %w", err)
	}

	return buf.Bytes(), nil
}
