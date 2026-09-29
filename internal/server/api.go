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
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
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
	Metrics       *prometheus.Registry
	processors    map[string]ProcessorFunc

	// images domain settings, copied from cfg so domains don't need cfg itself.
	MaxUploadSizeBytes  int
	MaxSourceMegapixels int
	MasterMaxDimension  int
	MasterJPEGQuality   int
}

func NewAPIServer(cfg *config.Config) *APIServer {
	s := &APIServer{
		cfg:                 cfg,
		MaxQueueDepth:       cfg.MaxQueueDepth,
		processors:          make(map[string]ProcessorFunc),
		MaxUploadSizeBytes:  cfg.MaxUploadSizeBytes,
		MaxSourceMegapixels: cfg.MaxSourceMegapixels,
		MasterMaxDimension:  cfg.MasterMaxDimension,
		MasterJPEGQuality:   cfg.MasterJPEGQuality,
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

	// The IdP gets the same bounded wait as Redis and S3 — Swarm gives no
	// startup ordering, so an issuer that is not up yet on a cold boot is
	// normal. It does NOT get their fallback, though: Redis and S3 degrade to
	// a warning and the service runs with those features off, but an
	// authenticator with no keys rejects 100% of traffic while still answering
	// /health with 200. That is indistinguishable from healthy to any external
	// monitor, so it is fatal instead — the task exits, Swarm restarts it, and
	// update_config.failure_action rolls the deploy back.
	if cfg.AuthenticationType == "zitadel" {
		ping := func(ctx context.Context) error {
			return authentication.PingJWKS(ctx, cfg.ZitadelIssuer)
		}
		if err := s.waitForDependency("zitadel", depDeadline, ping); err != nil {
			s.Log.Fatal().Err(err).
				Str("jwks_url", authentication.JWKSURL(cfg.ZitadelIssuer)).
				Msg("zitadel JWKS unreachable — refusing to start with an empty key set")
		}
		s.Log.Info().Str("issuer", cfg.ZitadelIssuer).Msg("zitadel JWKS reachable")
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

	s.initMetrics()

	return s
}

// initMetrics builds a dedicated registry (not prometheus.DefaultRegisterer)
// so /metrics doesn't depend on global state, consistent with the rest of
// this struct threading its own Cache/Storage/Auth rather than using
// package-level singletons. Called after Semaphore/Cache are set above —
// the GaugeFuncs close over s, so they read whatever those fields hold at
// scrape time, not at registration time.
func (s *APIServer) initMetrics() {
	s.Metrics = prometheus.NewRegistry()
	s.Metrics.MustRegister(collectors.NewGoCollector())
	s.Metrics.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))

	s.Metrics.MustRegister(prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Name: "images_api_inflight_requests",
			Help: "Requests currently holding a semaphore slot (synchronous resize work in flight).",
		},
		func() float64 { return float64(len(s.Semaphore)) },
	))
	s.Metrics.MustRegister(prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Name: "images_api_semaphore_capacity",
			Help: "Configured MAX_WORKERS — the semaphore's total capacity.",
		},
		func() float64 { return float64(cap(s.Semaphore)) },
	))

	if s.Cache != nil {
		s.Metrics.MustRegister(prometheus.NewGaugeFunc(
			prometheus.GaugeOpts{
				Name: "images_api_queue_depth",
				Help: "Jobs waiting in the async processing queue (Redis-backed).",
			},
			func() float64 {
				depth, err := s.Cache.QueueDepth(context.Background())
				if err != nil {
					return -1
				}
				return float64(depth)
			},
		))
	}
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
