# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
# Start full dev environment (app + Redis + MinIO) with Air hot-reload
docker compose down && docker compose up images-api

# Run all tests
go test ./...

# Logs
docker compose logs images-api
```

## Verification

# Run all tests
go test ./...

# Run Linter
golangci-lint run

# Check logs for errors
docker compose logs images-api

No need to build, Air rebuilds automatically

## Architecture

Entry point: `cmd/rest_api/main.go` creates a `mux.Router`, instantiates `server.APIServer`, registers domain routers under `/api/v1`, then starts the server.

**Layer structure:**

- `internal/server/` — `APIServer` struct with shared `Cache`, `Storage`, `Auth`, `Workers`, and `Semaphore` fields that domains consume.
- `internal/domains/<name>/` — one package per domain. Every rich domain follows: `types → validate → parser → handler → router → service`. See `internal/domains/resize/` as the reference. Simple domains (e.g. `health`) use only `handler.go` + `router.go`.
- `internal/utils/errors/` — `ValidationError` type, `NewValidationError`, sentinel errors (`ErrUnsupportedFormat`, etc.)
- `internal/utils/responses/` — HTTP response helpers: `Success`, `Created`, `NoContent`, `Accepted`, `JPEGImage`, `ErrorResponse` (maps typed errors to status codes)
- `internal/server/authentication/` — `Authenticator` interface + `BearerAuthenticator` (static token) / `ZitadelAuthenticator` (OIDC JWT). `NewAuthenticator` selects based on `AUTHENTICATION_TYPE`.
- `internal/middleware/` — `Auth` middleware (populates `Principal` in context) and `RequireRole(role)` (must run after `Auth`).

**Concurrency model:**

The resize service uses a bounded semaphore (`MAX_WORKERS`). When full, requests are queued async: file goes to S3 `pending/<jobID>-original`, metadata to Redis, `202 Accepted` returned. Background workers (`internal/server/worker.go`) drain the queue; they only start when both Cache and Storage are available.

**Adding a new domain:**

Simple: `handler.go` + `router.go` (see `health`). Rich: full layer set. Register with `<name>.Register(apiRouter, api)` in `main.go`. For async support add `api.RegisterProcessor("op", svc.Process)` in `router.go`.

**Testing pattern:**

Tests use stub interfaces rather than mocks. For service tests, construct a minimal `*server.APIServer` by setting only the exported fields the test needs (`Semaphore`, `MaxQueueDepth`, `Cache`, `Storage`). See `internal/domains/resize/service_test.go` (`minimalAPIServer`) and `handler_test.go` (`stubResizer`) as the reference.

## Infrastructure

Docker Compose runs three services: `images-api` (app with Air hot-reload), `redis:8.8.0-alpine`, and `minio` (S3-compatible object storage, console at `:9001`).

Production Dockerfile stages: `deps` → `builder` (CGO disabled, distroless final image) with a Trivy security scan step between builder and final. For local dev, use `AUTHENTICATION_TYPE=bearer` with `API_TOKEN=dev-token-12345`; MinIO defaults to `minioadmin/minioadmin`.

## Dependencies

Uses `github.com/kastoras/go-utilities` (internal utilities library) for `env_parameters.GetString` / `env_parameters.GetDuration` — the pattern for reading all env vars.
