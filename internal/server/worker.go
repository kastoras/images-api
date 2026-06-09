package server

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/kastoras/images-api/internal/models"
)

type ProcessorFunc func(ctx context.Context, jobID string, job *models.Job) error

type WorkerPool struct {
	size int
	wg   sync.WaitGroup
}

func newWorkerPool(size int) *WorkerPool {
	return &WorkerPool{size: size}
}

func (s *APIServer) RegisterProcessor(operation string, fn ProcessorFunc) {
	s.processors[operation] = fn
}

func (s *APIServer) StartWorkers(ctx context.Context) {
	for i := 0; i < s.Workers.size; i++ {
		s.Workers.wg.Add(1)
		go s.workerLoop(ctx)
	}
	s.Log.Info().Int("workers", s.Workers.size).Msg("worker pool started")
}

func (s *APIServer) workerLoop(ctx context.Context) {
	defer s.Workers.wg.Done()
	for {
		jobID, err := s.Cache.Dequeue(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			s.Log.Warn().Err(err).Msg("dequeue error")
			continue
		}
		if err := s.processJob(ctx, jobID); err != nil {
			s.Log.Error().Err(err).Str("job_id", jobID).Msg("job failed")
		}
	}
}

func (s *APIServer) processJob(ctx context.Context, jobID string) error {
	var job models.Job
	if err := s.Cache.GetJobMeta(ctx, jobID, &job); err != nil {
		return fmt.Errorf("get job meta: %w", err)
	}

	job.Status = models.StatusProcessing
	_ = s.Cache.SetJobMeta(ctx, jobID, &job, 24*time.Hour)

	processor, ok := s.processors[job.Operation]
	if !ok {
		return fmt.Errorf("no processor registered for operation: %s", job.Operation)
	}

	if err := processor(ctx, jobID, &job); err != nil {
		job.Status = models.StatusFailed
		job.Error = err.Error()
		_ = s.Cache.SetJobMeta(ctx, jobID, &job, 24*time.Hour)
		return err
	}

	_ = s.Storage.Delete(ctx, fmt.Sprintf("pending/%s-original", jobID))
	return nil
}

func (s *APIServer) StopWorkers() {
	s.Workers.wg.Wait()
}
