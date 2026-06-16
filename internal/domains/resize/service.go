package resize

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"io"
	"time"

	"github.com/disintegration/imaging"
	"github.com/google/uuid"
	"github.com/kastoras/images-api/internal/models"
	"github.com/kastoras/images-api/internal/server"
	internal_errors "github.com/kastoras/images-api/internal/utils/errors"
)

type Service struct {
	server *server.APIServer
}

func NewService(s *server.APIServer) *Service {
	return &Service{server: s}
}

func (svc *Service) Resize(ctx context.Context, file io.Reader, opts ResizeOptions) (*ProcessResult, error) {
	select {
	case svc.server.Semaphore <- struct{}{}:
		defer func() { <-svc.server.Semaphore }()
		return svc.processImmediate(ctx, file, opts)
	default:
		return svc.enqueue(ctx, file, opts)
	}
}

func (svc *Service) processImmediate(ctx context.Context, file io.Reader, opts ResizeOptions) (*ProcessResult, error) {
	src, err := imaging.Decode(file)
	if err != nil {
		return nil, internal_errors.ErrUnsupportedFormat
	}

	var resized image.Image
	switch opts.Mode {
	case ResizeModeFit:
		resized = imaging.Fit(src, opts.Width, opts.Height, imaging.Lanczos)
	case ResizeModeFill:
		resized = imaging.Fill(src, opts.Width, opts.Height, imaging.Center, imaging.Lanczos)
	default:
		resized = imaging.Resize(src, opts.Width, opts.Height, imaging.Lanczos)
	}

	var buf bytes.Buffer
	if err := imaging.Encode(&buf, resized, imaging.JPEG); err != nil {
		return nil, fmt.Errorf("encode image: %w", err)
	}

	if svc.server.Storage == nil {
		return &ProcessResult{ImageData: buf.Bytes()}, nil
	}

	jobID := uuid.New().String()
	key := fmt.Sprintf("results/%s-result.jpg", jobID)

	if err := svc.server.Storage.Upload(ctx, key, bytes.NewReader(buf.Bytes()), int64(buf.Len())); err != nil {
		return nil, fmt.Errorf("upload result: %w", err)
	}

	url, err := svc.server.Storage.Presign(ctx, key, 24*time.Hour)
	if err != nil {
		return nil, fmt.Errorf("presign: %w", err)
	}

	return &ProcessResult{URL: url}, nil
}

func (svc *Service) enqueue(ctx context.Context, file io.Reader, opts ResizeOptions) (*ProcessResult, error) {
	if svc.server.Cache == nil || svc.server.Storage == nil {
		return nil, internal_errors.ErrTooManyRequests
	}

	depth, err := svc.server.Cache.QueueDepth(ctx)
	if err != nil || depth >= int64(svc.server.MaxQueueDepth) {
		return nil, internal_errors.ErrQueueFull
	}

	jobID := uuid.New().String()
	pendingKey := fmt.Sprintf("pending/%s-original", jobID)

	fileBytes, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}

	if err := svc.server.Storage.Upload(ctx, pendingKey, bytes.NewReader(fileBytes), int64(len(fileBytes))); err != nil {
		return nil, fmt.Errorf("upload pending: %w", err)
	}

	job := &models.Job{
		ID:        jobID,
		Operation: "resize",
		Status:    models.StatusQueued,
		Params:    map[string]any{"width": opts.Width, "height": opts.Height, "mode": string(opts.Mode)},
		CreatedAt: time.Now().UTC(),
	}

	if err := svc.server.Cache.SetJobMeta(ctx, jobID, job, 24*time.Hour); err != nil {
		_ = svc.server.Storage.Delete(ctx, pendingKey)
		return nil, fmt.Errorf("set job meta: %w", err)
	}

	if err := svc.server.Cache.Enqueue(ctx, jobID); err != nil {
		_ = svc.server.Storage.Delete(ctx, pendingKey)
		return nil, fmt.Errorf("enqueue job: %w", err)
	}

	return &ProcessResult{JobID: jobID, Queued: true}, nil
}

// Process handles a queued resize job — registered with the worker pool.
func (svc *Service) Process(ctx context.Context, jobID string, job *models.Job) error {
	body, err := svc.server.Storage.Download(ctx, fmt.Sprintf("pending/%s-original", jobID))
	if err != nil {
		return fmt.Errorf("download original: %w", err)
	}
	defer func() { _ = body.Close() }()

	src, err := imaging.Decode(body)
	if err != nil {
		return fmt.Errorf("decode image: %w", err)
	}

	width := int(job.Params["width"].(float64))
	height := int(job.Params["height"].(float64))
	modeStr, _ := job.Params["mode"].(string)
	mode := ResizeMode(modeStr)
	if mode == "" {
		mode = ResizeModeExact
	}

	var resized image.Image
	switch mode {
	case ResizeModeFit:
		resized = imaging.Fit(src, width, height, imaging.Lanczos)
	case ResizeModeFill:
		resized = imaging.Fill(src, width, height, imaging.Center, imaging.Lanczos)
	default:
		resized = imaging.Resize(src, width, height, imaging.Lanczos)
	}

	var buf bytes.Buffer
	if err := imaging.Encode(&buf, resized, imaging.JPEG); err != nil {
		return fmt.Errorf("encode image: %w", err)
	}

	resultKey := fmt.Sprintf("results/%s-result.jpg", jobID)
	if err := svc.server.Storage.Upload(ctx, resultKey, bytes.NewReader(buf.Bytes()), int64(buf.Len())); err != nil {
		return fmt.Errorf("upload result: %w", err)
	}

	url, err := svc.server.Storage.Presign(ctx, resultKey, 24*time.Hour)
	if err != nil {
		return fmt.Errorf("presign: %w", err)
	}

	job.Status = models.StatusComplete
	job.ResultURL = url
	return svc.server.Cache.SetJobMeta(ctx, jobID, job, 24*time.Hour)
}
