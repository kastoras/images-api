package assets

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	"io"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/disintegration/imaging"
	"github.com/kastoras/images-api/internal/imageprocessing"
	"github.com/kastoras/images-api/internal/server"
	internal_errors "github.com/kastoras/images-api/internal/utils/errors"
)

type Service struct {
	server *server.APIServer
}

func NewService(s *server.APIServer) *Service {
	return &Service{server: s}
}

// Create normalizes an uploaded image into a master and stores it,
// content-addressed by the hash of its normalized bytes, under this
// consumer's own tenant namespace. Re-uploading identical content for the
// same consumer+tenant is a no-op write (dedup for free).
func (svc *Service) Create(ctx context.Context, consumer, tenant string, file io.Reader) (*Master, error) {
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

	hash := sha256Hex(buf.Bytes())
	key := masterKey(consumer, tenant, hash, extForFormat(format))
	bounds := normalized.Bounds()
	width, height := bounds.Dx(), bounds.Dy()

	if _, _, err := svc.server.Storage.HeadObject(ctx, key); err != nil {
		if !errors.Is(err, server.ErrNotFound) {
			return nil, fmt.Errorf("head master: %w", err)
		}

		metadata := map[string]string{
			"consumer": consumer,
			"tenant":   tenant,
			"width":    strconv.Itoa(width),
			"height":   strconv.Itoa(height),
			"format":   format,
		}
		if err := svc.server.Storage.UploadWithMetadata(ctx, key, bytes.NewReader(buf.Bytes()), int64(buf.Len()), metadata); err != nil {
			return nil, fmt.Errorf("upload master: %w", err)
		}
	}

	return &Master{ID: hash, Consumer: consumer, Tenant: tenant, Width: width, Height: height, Format: format, Bytes: int64(buf.Len())}, nil
}

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
	ext := extForFormat(format)

	derivKey := derivativeKey(consumer, tenant, hash, opts.Width, opts.Height, string(opts.Mode), ext)

	if body, err := svc.server.Storage.Download(ctx, derivKey); err == nil {
		defer func() { _ = body.Close() }()
		data, readErr := io.ReadAll(body)
		if readErr != nil {
			return nil, "", fmt.Errorf("read cached derivative: %w", readErr)
		}
		return data, contentTypeForFormat(format), nil
	}

	body, err := svc.server.Storage.Download(ctx, masterObjectKey)
	if err != nil {
		return nil, "", fmt.Errorf("download master: %w", err)
	}
	defer func() { _ = body.Close() }()

	src, err := imageprocessing.Decode(body)
	if err != nil {
		return nil, "", fmt.Errorf("decode master: %w", err)
	}

	resized := imageprocessing.Resize(src, opts)

	var buf bytes.Buffer
	if _, err := encodeAs(&buf, resized, format, svc.server.MasterJPEGQuality); err != nil {
		return nil, "", fmt.Errorf("encode derivative: %w", err)
	}

	if err := svc.server.Storage.Upload(ctx, derivKey, bytes.NewReader(buf.Bytes()), int64(buf.Len())); err != nil {
		return nil, "", fmt.Errorf("upload derivative: %w", err)
	}

	return buf.Bytes(), contentTypeForFormat(format), nil
}

// Delete removes the master plus every cached derivative under it.
// Idempotent: returns nil even if the master was already gone.
func (svc *Service) Delete(ctx context.Context, consumer, tenant, hash string) error {
	key, err := svc.resolveMasterKey(ctx, consumer, tenant, hash)
	if err != nil && !errors.Is(err, server.ErrNotFound) {
		return err
	}
	if err == nil {
		if delErr := svc.server.Storage.Delete(ctx, key); delErr != nil {
			return delErr
		}
	}
	return svc.server.Storage.DeletePrefix(ctx, derivativePrefix(consumer, tenant, hash))
}

// List returns every master stored for a consumer's tenant. Used for the
// reconciliation sweep and future "show a tenant's assets" use cases.
func (svc *Service) List(ctx context.Context, consumer, tenant string) ([]ListedAsset, error) {
	objects, err := svc.server.Storage.ListKeys(ctx, masterPrefix(consumer, tenant))
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

func masterKey(consumer, tenant, hash, ext string) string {
	return fmt.Sprintf("masters/%s/%s/%s.%s", consumer, tenant, hash, ext)
}

func masterPrefix(consumer, tenant string) string {
	return fmt.Sprintf("masters/%s/%s/", consumer, tenant)
}

func derivativeKey(consumer, tenant, hash string, width, height int, mode, ext string) string {
	return fmt.Sprintf("derivatives/%s/%s/%s/%dx%d-%s.%s", consumer, tenant, hash, width, height, mode, ext)
}

func derivativePrefix(consumer, tenant, hash string) string {
	return fmt.Sprintf("derivatives/%s/%s/%s/", consumer, tenant, hash)
}

func hashFromMasterKey(key string) string {
	base := key[strings.LastIndex(key, "/")+1:]
	return strings.TrimSuffix(base, filepath.Ext(base))
}

func extForFormat(format string) string {
	if format == "png" {
		return "png"
	}
	return "jpg"
}

func formatFromExt(ext string) string {
	if ext == "png" {
		return "png"
	}
	return "jpeg"
}

func contentTypeForFormat(format string) string {
	if format == "png" {
		return "image/png"
	}
	return "image/jpeg"
}

func encodeAs(w io.Writer, img image.Image, format string, quality int) (string, error) {
	if format == "png" {
		return "png", imaging.Encode(w, img, imaging.PNG)
	}
	return "jpeg", imaging.Encode(w, img, imaging.JPEG, imaging.JPEGQuality(quality))
}

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func checkSourceMegapixels(img image.Image, maxMegapixels int) error {
	if maxMegapixels <= 0 {
		return nil
	}
	b := img.Bounds()
	pixels := int64(b.Dx()) * int64(b.Dy())
	if pixels > int64(maxMegapixels)*1_000_000 {
		return internal_errors.NewValidationError("image exceeds maximum allowed resolution")
	}
	return nil
}
