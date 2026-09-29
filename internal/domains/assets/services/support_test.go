package assets_services

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"io"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/kastoras/images-api/internal/server"
)

type fakeStorage struct {
	mu       sync.Mutex
	objects  map[string][]byte
	metadata map[string]map[string]string

	uploads     int
	downloads   int
	headErr     error
	uploadErr   error
	downloadErr map[string]error
}

func newFakeStorage() *fakeStorage {
	return &fakeStorage{
		objects:     map[string][]byte{},
		metadata:    map[string]map[string]string{},
		downloadErr: map[string]error{},
	}
}

func (f *fakeStorage) put(key string, data []byte) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.objects[key] = data
}

func (f *fakeStorage) has(key string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	_, ok := f.objects[key]
	return ok
}

func (f *fakeStorage) keys() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]string, 0, len(f.objects))
	for k := range f.objects {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func (f *fakeStorage) Upload(_ context.Context, key string, body io.Reader, _ int64) error {
	return f.UploadWithMetadata(context.Background(), key, body, 0, nil)
}

func (f *fakeStorage) UploadWithMetadata(_ context.Context, key string, body io.Reader, _ int64, metadata map[string]string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.uploadErr != nil {
		return f.uploadErr
	}
	data, err := io.ReadAll(body)
	if err != nil {
		return err
	}
	f.objects[key] = data
	f.metadata[key] = metadata
	f.uploads++
	return nil
}

func (f *fakeStorage) Download(_ context.Context, key string) (io.ReadCloser, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.downloads++
	if err := f.downloadErr[key]; err != nil {
		return nil, err
	}
	data, ok := f.objects[key]
	if !ok {
		return nil, server.ErrNotFound
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}

func (f *fakeStorage) Delete(_ context.Context, key string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.objects, key)
	return nil
}

func (f *fakeStorage) DeletePrefix(_ context.Context, prefix string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for k := range f.objects {
		if strings.HasPrefix(k, prefix) {
			delete(f.objects, k)
		}
	}
	return nil
}

func (f *fakeStorage) HeadObject(_ context.Context, key string) (map[string]string, int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.headErr != nil {
		return nil, 0, f.headErr
	}
	data, ok := f.objects[key]
	if !ok {
		return nil, 0, server.ErrNotFound
	}
	return f.metadata[key], int64(len(data)), nil
}

func (f *fakeStorage) ListKeys(_ context.Context, prefix string) ([]server.ObjectInfo, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []server.ObjectInfo
	for k, v := range f.objects {
		if strings.HasPrefix(k, prefix) {
			out = append(out, server.ObjectInfo{Key: k, Size: int64(len(v)), LastModified: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out, nil
}

func newTestService(st Storage) *Service {
	return &Service{
		server:  &server.APIServer{MasterMaxDimension: 200, MasterJPEGQuality: 85},
		storage: st,
	}
}

func testPNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x * 255 / w), G: uint8(y * 255 / h), B: 100, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}

func decodeDims(t *testing.T, data []byte) (int, int) {
	t.Helper()
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("decode config: %v", err)
	}
	return cfg.Width, cfg.Height
}

func createMaster(t *testing.T, svc *Service, consumer, tenant string) *Master {
	t.Helper()
	m, err := svc.Create(context.Background(), consumer, tenant, bytes.NewReader(testPNG(t, 200, 100)))
	if err != nil {
		t.Fatalf("setup Create: %v", err)
	}
	return m
}

func createMasterWith(t *testing.T, svc *Service, consumer, tenant string, w, h int) *Master {
	t.Helper()
	m, err := svc.Create(context.Background(), consumer, tenant, bytes.NewReader(testPNG(t, w, h)))
	if err != nil {
		t.Fatalf("setup Create: %v", err)
	}
	return m
}
