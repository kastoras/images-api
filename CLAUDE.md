# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
# Start full dev environment (app + Redis + MinIO) with Air hot-reload
docker compose down && docker compose up images-api

# Run all tests
go test ./...

# Run a single test by name
go test ./internal/domains/resize/... -run TestService_ProcessImmediate_StorageNil

# Logs
docker compose logs images-api
```

To verify a code change, check the `docker compose up` logs — Air detects file changes and rebuilds automatically inside the container.

## Architecture

Entry point: `cmd/rest_api/main.go` creates a `mux.Router`, instantiates `server.APIServer`, registers domain routers under `/api/v1`, then starts the server.

**Layer structure:**

- `internal/server/` — `APIServer` struct holding port, HTTP client, graceful shutdown config, and the shared `Cache`, `Storage`, `Auth`, `Workers`, and `Semaphore` fields that domains consume.
- `internal/domains/<name>/` — one package per domain. Every domain follows this layer pattern (the `resize` domain is the reference implementation):
  - `types.go` — exported types, sentinel errors, interfaces
  - `validate.go` — pre-parse request validation (fail fast before any I/O)
  - `parser.go` — request parsing helpers
  - `handler.go` — HTTP handler methods
  - `router.go` — `Register(router *mux.Router, s *server.APIServer)` wires routes
  - `service.go` — business logic

  Simple domains (e.g. `health`) may use only `handler.go` + `router.go`. All new rich domains must follow the full pattern above.

- `internal/utils/errors/` — `ValidationError` type, `NewValidationError`, sentinel errors (`ErrUnsupportedFormat`, etc.)
- `internal/utils/responses/` — HTTP response helpers: `Success`, `Created`, `NoContent`, `Accepted`, `JPEGImage`, `ErrorResponse` (maps typed errors to status codes)
- `internal/server/authentication/` — `Authenticator` interface + implementations: `BearerAuthenticator` (static token, local dev), `ZitadelAuthenticator` (OIDC JWT via JWKS). `NewAuthenticator(ctx, cfg, log)` factory selects based on `AUTHENTICATION_TYPE` (`"zitadel"` or `"bearer"`).
- `internal/middleware/` — `Auth` middleware (populates `Principal` in context) and `RequireRole(role)` middleware for per-route authorization (must run after `Auth`).

**Concurrency model:**

The resize service uses a bounded semaphore (`api.Semaphore`, size = `MAX_WORKERS`) for immediate processing. When the semaphore is full, requests fall through to an async queue path: the original file is uploaded to S3 under `pending/<jobID>-original`, job metadata is written to Redis, and `202 Accepted` is returned with a job ID. Background workers (`internal/server/worker.go`) dequeue jobs and call the registered `ProcessorFunc` for each operation. Workers are only started when both Cache and Storage are available.

**Adding a new domain:**
1. Simple domain: create `handler.go` + `router.go` following the `health` pattern.
2. Rich domain: follow the full layer pattern above.
3. Call `<name>.Register(apiRouter, api)` in `main.go`.
4. If the domain supports async processing, register a processor in `router.go`:
   ```go
   api.RegisterProcessor("operation-name", svc.Process)
   ```

**Testing pattern:**

Tests use stub interfaces rather than mocks. For service tests, construct a minimal `*server.APIServer` by setting only the exported fields the test needs (`Semaphore`, `MaxQueueDepth`, `Cache`, `Storage`). See `internal/domains/resize/service_test.go` (`minimalAPIServer`) and `handler_test.go` (`stubResizer`) as the reference.

## Infrastructure

Docker Compose runs three services: `images-api` (app with Air hot-reload), `redis:8.8.0-alpine`, and `minio` (S3-compatible object storage, console at `:9001`).

Production Dockerfile stages: `deps` → `builder` (CGO disabled, distroless final image) with a Trivy security scan step between builder and final.

## Environment / Feature Flags

Copy `.env.example` to `.env`. Key flags:

| Variable | Purpose |
|---|---|
| `REDIS_ENABLED` | Toggle Redis caching |
| `S3_ENABLED` | Toggle MinIO/S3 object storage |
| `AUTHENTICATION_TYPE` | `zitadel` or `bearer` — selects the auth implementation |

For local dev: `AUTHENTICATION_TYPE=bearer` and set `API_TOKEN=dev-token-12345`. MinIO defaults use `minioadmin/minioadmin`.

## Dependencies

Uses `github.com/kastoras/go-utilities` (internal utilities library) for `env_parameters.GetString` / `env_parameters.GetDuration` — the pattern for reading all env vars.
