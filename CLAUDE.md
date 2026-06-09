# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
# Start full dev environment (app + Redis + MinIO) with Air hot-reload
docker compose down && docker compose up images-api

# Tests
go test ./...
go test ./internal/domains/health/... -run TestHealth

# Logs
docker compose logs images-api
```

To verify a code change, check the `docker compose up` logs — Air detects file changes and rebuilds automatically inside the container.

## Architecture

Entry point: `cmd/rest_api/main.go` creates a `mux.Router`, instantiates `server.APIServer`, registers domain routers under `/api/v1`, then starts the server.

**Layer structure:**

- `internal/server/` — `APIServer` struct with port, HTTP client, graceful shutdown, and timeout config. Stub files (`cache.go`, `logger.go`, `object_storage.go`, `auth_server.go`) are placeholders for Redis cache, structured logger, MinIO client, and Zitadel auth — to be wired into `APIServer`.
- `internal/domains/<name>/` — one package per domain. Simple domains use two files; the `resize` domain is the full reference pattern:
  - `types.go` — exported domain types (`ResizeMode`, `ResizeOptions`, `ProcessResult`, `Resizer` interface, sentinel errors)
  - `parser.go` — request parsing + validation helpers (`parseResizeRequest`, `parseWidth`, `parseHeight`, `parseMode`, `validateDimensions`, `resizeRequest` struct)
  - `handler.go` — `Handler` struct, one method per route, error mapping to HTTP status
  - `router.go` — `Register(router *mux.Router, s *server.APIServer)` wires routes
  - `service.go` — business logic (imaging, queue, S3 upload)
- `internal/utils/` — JSON response helpers: `Success`, `Created`, `NoContent`, `Accepted`

**Adding a new domain:**
1. Simple domain: create `handler.go` + `router.go` following the `health` pattern
2. Rich domain: follow the `resize` pattern — `types.go`, `parser.go`, `handler.go`, `router.go`, `service.go`
3. Call `<name>.Register(apiRouter, api)` in `main.go`

## Infrastructure

Docker Compose runs three services: `images-api` (app with Air hot-reload), `redis:8.8.0-alpine`, and `minio` (S3-compatible object storage, console at `:9001`).

Production Dockerfile stages: `deps` → `builder` (CGO disabled, distroless final image) with a Trivy security scan step between builder and final.

## Environment / Feature Flags

Copy `.env.example` to `.env`. Key flags:

| Variable | Purpose |
|---|---|
| `REDIS_ENABLED` | Toggle Redis caching |
| `S3_ENABLED` | Toggle MinIO/S3 object storage |
| `ZITADEL_ENABLED` | Use Zitadel OIDC auth (false = `API_TOKEN` bearer auth) |

For local dev: `ZITADEL_ENABLED=false` and set `API_TOKEN=dev-token-12345`. MinIO defaults use `minioadmin/minioadmin`.

## Dependencies

Uses `github.com/kastoras/go-utilities` (internal utilities library) for `env_parameters.GetString` / `env_parameters.GetDuration` — the pattern for reading all env vars.
