package assets_services

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"

	"github.com/kastoras/images-api/internal/imageprocessing"
	"github.com/kastoras/images-api/internal/server"
)

// normalizedMaster is an uploaded image after decoding, validation,
// normalization and re-encoding, ready to be stored.
type normalizedMaster struct {
	data   []byte
	hash   string
	format string
	width  int
	height int
}

// Create normalizes an uploaded image into a master and stores it,
// content-addressed by the hash of its normalized bytes, under this
// consumer's own tenant namespace. Re-uploading identical content for the
// same consumer+tenant is a no-op write (dedup for free).
func (svc *Service) Create(ctx context.Context, consumer, tenant string, file io.Reader) (*Master, error) {
	m, err := svc.normalizeUpload(file)
	if err != nil {
		return nil, err
	}

	if err := svc.storeMaster(ctx, consumer, tenant, m); err != nil {
		return nil, err
	}

	return &Master{ID: m.hash, Consumer: consumer, Tenant: tenant, Width: m.width, Height: m.height, Format: m.format, Bytes: int64(len(m.data))}, nil
}

// normalizeUpload decodes the upload, enforces the source resolution limit,
// and produces the normalized, encoded master bytes plus their hash.
func (svc *Service) normalizeUpload(file io.Reader) (*normalizedMaster, error) {
	src, err := imageprocessing.Decode(file)
	if err != nil {
		return nil, err
	}

	if err := checkSourceMegapixels(src, svc.server.MaxSourceMegapixels); err != nil {
		return nil, err
	}

	normalized := imageprocessing.NormalizeMaster(src, svc.server.MasterMaxDimension)

	var buf bytes.Buffer
	format, err := imageprocessing.EncodeMaster(&buf, normalized, svc.server.MasterJPEGQuality)
	if err != nil {
		return nil, fmt.Errorf("encode master: %w", err)
	}

	bounds := normalized.Bounds()
	return &normalizedMaster{
		data:   buf.Bytes(),
		hash:   sha256Hex(buf.Bytes()),
		format: format,
		width:  bounds.Dx(),
		height: bounds.Dy(),
	}, nil
}

// storeMaster uploads the master unless an object with the same
// content-addressed key already exists.
func (svc *Service) storeMaster(ctx context.Context, consumer, tenant string, m *normalizedMaster) error {
	key := masterKey(consumer, tenant, m.hash, extForFormat(m.format))

	_, _, err := svc.server.Storage.HeadObject(ctx, key)
	if err == nil {
		return nil
	}
	if !errors.Is(err, server.ErrNotFound) {
		return fmt.Errorf("head master: %w", err)
	}

	metadata := map[string]string{
		"consumer": consumer,
		"tenant":   tenant,
		"width":    strconv.Itoa(m.width),
		"height":   strconv.Itoa(m.height),
		"format":   m.format,
	}
	if err := svc.server.Storage.UploadWithMetadata(ctx, key, bytes.NewReader(m.data), int64(len(m.data)), metadata); err != nil {
		return fmt.Errorf("upload master: %w", err)
	}
	return nil
}
