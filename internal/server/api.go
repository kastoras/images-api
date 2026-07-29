package server

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"github.com/kastoras/images-api/internal/config"
	"github.com/kastoras/images-api/internal/server/authentication"
	"github.com/rs/zerolog"
)

type APIServer struct {
	cfg           *config.Config
	Log           zerolog.Logger
	Cache         *Cache
	Storage       *ObjectStorage
	Auth          authentication.Authenticator
	Workers       *WorkerPool
	Semaphore     chan struct{}
	MaxQueueDepth int
	processors    map[string]ProcessorFunc
}

func NewAPIServer(cfg *config.Config) *APIServer {
	s := &APIServer{
		cfg:           cfg,
		MaxQueueDepth: cfg.MaxQueueDepth,
		processors:    make(map[string]ProcessorFunc),
	}

	s.Log = initLogger(cfg)

	// Redis and S3 are pinged once here and never re-checked, so a dependency
	// that is not ready yet would stay disabled for the life of the process.
	// Swarm gives no startup ordering (depends_on is ignored), so that is the
	// normal case on a cold boot, not an edge case. Both waits share one
	// deadline to bound total startup time.
	var depDeadline time.Time
	if cfg.DependencyWaitTimeout > 0 {
		depDeadline = time.Now().Add(cfg.DependencyWaitTimeout)
	}

	if cfg.RedisEnabled {
		cache := initCache(cfg)
		if err := s.waitForDependency("redis", depDeadline, cache.Ping); err != nil {
			s.Log.Warn().Err(err).Msg("redis unavailable — cache disabled")
		} else {
			s.Cache = cache
			s.Log.Info().Msg("redis connected")
		}
	}

	if cfg.S3Enabled {
		storage, err := initObjectStorage(context.Background(), cfg)
		if err != nil {
			s.Log.Warn().Err(err).Msg("s3 init failed — storage disabled")
		} else if err := s.waitForDependency("s3", depDeadline, storage.Ping); err != nil {
			s.Log.Warn().Err(err).Msg("s3 unreachable — storage disabled")
		} else {
			s.Storage = storage
			s.Log.Info().Msg("s3 connected")
		}
	}

	auth, err := authentication.NewAuthenticator(context.Background(), cfg, s.Log)
	if err != nil {
		s.Log.Fatal().Err(err).Msg("authentication init failed")
	}
	s.Auth = auth
	s.Semaphore = make(chan struct{}, cfg.MaxWorkers)

	if s.Cache != nil && s.Storage != nil {
		s.Workers = newWorkerPool(cfg.MaxWorkers)
	}

	return s
}

const (
	initialDependencyBackoff = 500 * time.Millisecond
	maxDependencyBackoff     = 5 * time.Second
)

// waitForDependency retries ping with exponential backoff until it succeeds or
// deadline passes, returning the last error on give-up. A zero deadline means
// a single attempt, preserving the original fail-fast behaviour.
func (s *APIServer) waitForDependency(name string, deadline time.Time, ping func(context.Context) error) error {
	ctx := context.Background()
	if deadline.IsZero() {
		return ping(ctx)
	}

	ctx, cancel := context.WithDeadline(ctx, deadline)
	defer cancel()

	backoff := initialDependencyBackoff

	for attempt := 1; ; attempt++ {
		err := ping(ctx)
		if err == nil {
			if attempt > 1 {
				s.Log.Info().Str("dependency", name).Int("attempts", attempt).Msg("dependency ready after retry")
			}
			return nil
		}

		// Don't sleep past the deadline just to fail anyway.
		if time.Now().Add(backoff).After(deadline) {
			return err
		}

		s.Log.Debug().
			Err(err).
			Str("dependency", name).
			Int("attempt", attempt).
			Dur("retry_in", backoff).
			Msg("dependency not ready — retrying")

		timer := time.NewTimer(backoff)
		select {
		case <-ctx.Done():
			timer.Stop()
			return err
		case <-timer.C:
		}

		if backoff < maxDependencyBackoff {
			backoff *= 2
			if backoff > maxDependencyBackoff {
				backoff = maxDependencyBackoff
			}
		}
	}
}

func (s *APIServer) Start(router *mux.Router) error {
	srv := &http.Server{
		Addr:         ":" + s.cfg.Port,
		Handler:      router,
		ReadTimeout:  s.cfg.ReadTimeout,
		WriteTimeout: s.cfg.WriteTimeout,
		IdleTimeout:  s.cfg.IdleTimeout,
	}

	workerCtx, cancelWorkers := context.WithCancel(context.Background())
	defer cancelWorkers()

	if s.Workers != nil {
		s.StartWorkers(workerCtx)
	}

	errCh := make(chan error, 1)
	go func() {
		s.Log.Info().Str("port", s.cfg.Port).Msg("server listening")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-errCh:
		return err
	case sig := <-quit:
		s.Log.Info().Str("signal", sig.String()).Msg("shutting down")
	}

	cancelWorkers()
	if s.Workers != nil {
		s.StopWorkers()
	}

	if s.Cache != nil {
		if err := s.Cache.Close(); err != nil {
			s.Log.Warn().Err(err).Msg("cache close error")
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	return srv.Shutdown(shutdownCtx)
}
