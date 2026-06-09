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
	"github.com/rs/zerolog"
)

type APIServer struct {
	cfg           *config.Config
	Log           zerolog.Logger
	Cache         *Cache
	Storage       *ObjectStorage
	Auth          *AuthServer
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

	if cfg.RedisEnabled {
		cache := initCache(cfg)
		if err := cache.Ping(context.Background()); err != nil {
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
		} else if err := storage.Ping(context.Background()); err != nil {
			s.Log.Warn().Err(err).Msg("s3 unreachable — storage disabled")
		} else {
			s.Storage = storage
			s.Log.Info().Msg("s3 connected")
		}
	}

	s.Auth = initAuthServer(cfg)
	s.Semaphore = make(chan struct{}, cfg.MaxWorkers)

	if s.Cache != nil && s.Storage != nil {
		s.Workers = newWorkerPool(cfg.MaxWorkers)
	}

	return s
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
