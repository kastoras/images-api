# Images API

A Go REST API for image processing. Accepts image uploads and performs transformations (resize, optimize) either synchronously or via a background job queue depending on load and available infrastructure.

## How it works

Requests hit a semaphore-controlled slot pool (default: 10 concurrent). When a slot is free the image is processed inline and the result is returned immediately. When all slots are busy the job is pushed to a Redis queue and processed by a background worker pool — the caller gets a `job_id` to poll.

If Redis or S3 is unavailable the service degrades gracefully:
- No S3: processed image bytes are returned directly in the response body
- No Redis or S3: only 10 concurrent requests are served; overflow returns `429`

## Architecture

### Resize domain layer structure

Each domain lives in `internal/domains/<name>/`. The `resize` domain is the full reference pattern for rich domains:

| File | Responsibility |
|---|---|
| `types.go` | Exported types, sentinel errors, interfaces |
| `validate.go` | Pre-parse request validation — fails fast before any I/O |
| `parser.go` | Request parsing helpers |
| `handler.go` | HTTP handler methods |
| `router.go` | Wires routes to the handler |
| `service.go` | Business logic — image processing, queue management, S3 upload |

Simple domains (e.g. `health`) use only `handler.go` + `router.go`. All new rich domains must follow the full pattern above.

## Local development

```bash
cp .env.example .env
# Edit .env — see Environment section below
docker compose up
```

Air hot-reload runs inside the container. File changes are picked up automatically; verify rebuilds in the `docker compose up` logs.

**Services started:**

| Service | URL |
|---|---|
| API | `http://localhost:8080` |
| MinIO console | `http://localhost:9001` (user: `minioadmin` / `minioadmin`) |
| Redis | `redis:6379` (internal) |

## Environment

Copy `.env.example` to `.env` and fill in the values.

| Variable | Default | Description |
|---|---|---|
| `API_PORT` | `8080` | Listening port |
| `REDIS_ENABLED` | `false` | Enable Redis queue and job metadata |
| `S3_ENABLED` | `false` | Enable MinIO/S3 object storage |
| `AUTHENTICATION_TYPE` | `bearer` | Auth implementation: `bearer` (static token) or `zitadel` (OIDC JWT) |
| `API_TOKEN` | — | Static bearer token when `AUTHENTICATION_TYPE=bearer`. Fallback: maps to a single consumer named `default` when `API_TOKENS` is unset. |
| `API_TOKENS` | — | Multiple named bearer tokens, one per consumer: `consumer:token,consumer:token`. Takes precedence over `API_TOKEN` when set. See "Assets — consumers and tenants" below. |
| `MAX_UPLOAD_SIZE_BYTES` | `26214400` (25MB) | Max upload size for `POST /api/v1/assets` |
| `MAX_SOURCE_MEGAPIXELS` | `40` | Reject uploads/render requests above this pixel count |
| `MASTER_MAX_DIMENSION` | `2560` | Long-edge cap a stored master is normalized to (never upscaled) |
| `MASTER_JPEG_QUALITY` | `85` | JPEG quality used when encoding masters/derivatives |
| `ZITADEL_ISSUER` | — | Zitadel issuer URL |
| `ZITADEL_CLIENT_ID` | — | Zitadel client ID |
| `ZITADEL_AUDIENCE` | `image-api` | Expected JWT audience |
| `REDIS_URL` | `redis:6379` | Redis connection address |
| `REDIS_PASSWORD` | — | Redis password |
| `S3_ENDPOINT` | — | S3/MinIO endpoint URL |
| `S3_REGION` | `us-east-1` | S3 region |
| `S3_BUCKET` | `image-processor` | S3 bucket name |
| `S3_ACCESS_KEY` | — | S3 access key |
| `S3_SECRET_KEY` | — | S3 secret key |
| `S3_USE_PATH_STYLE` | `false` | Use path-style URLs (required for MinIO) |
| `MAX_WORKERS` | `10` | Concurrent processing slots |
| `MAX_QUEUE_DEPTH` | `50` | Max pending jobs before rejecting with 503 |
| `PROCESSING_TIMEOUT` | `10` | Per-job timeout in seconds |
| `READ_TIMEOUT` | `15` | HTTP read timeout in seconds |
| `WRITE_TIMEOUT` | `15` | HTTP write timeout in seconds |
| `IDLE_TIMEOUT` | `60` | HTTP idle timeout in seconds |
| `LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error` |
| `LOKI_URL` | — | Loki push target for promtail (observability profile only) |
| `ENVIRONMENT` | `dev` | `environment` label attached to shipped logs (observability profile only) |
| `METRICS_BIND` | `127.0.0.1` | Interface the exporter ports bind to (observability profile only) |

For local dev set `AUTHENTICATION_TYPE=bearer` and `API_TOKEN=dev-token-12345`.

## Observability

### Metrics

`GET /metrics` serves Prometheus metrics (no authentication, like `/health`). **Do not expose it publicly** — restrict it to your scraper via a private network or block the path at your reverse proxy.

| Metric | Description |
|---|---|
| `http_requests_total{method,route,status}` | Requests by route template (not raw path) |
| `http_request_duration_seconds{method,route}` | Request duration histogram |
| `images_api_inflight_requests` | Semaphore slots currently in use |
| `images_api_semaphore_capacity` | Configured `MAX_WORKERS` |
| `images_api_queue_depth` | Jobs pending in the Redis queue (only when Redis is enabled; `NaN` if Redis can't be queried) |
| `go_*`, `process_*` | Standard Go runtime / process collectors |

### Optional exporters and log shipping

The compose file includes node-exporter, cAdvisor, redis-exporter and promtail behind the `observability` profile. They are not started by default:

```bash
docker compose --profile observability up
```

Set `LOKI_URL` (and optionally `ENVIRONMENT`) in `.env` first. Exporter ports (`9100`, `8081`, `9121`) bind to `127.0.0.1` by default and are unauthenticated — set `METRICS_BIND` only if the host is on a private network. MinIO's metrics endpoint is enabled without a token for local dev; restrict access to it in production.

## Zitadel setup

Set `AUTHENTICATION_TYPE=zitadel` and configure these variables in `.env`:

| Variable | Where to find it |
|---|---|
| `ZITADEL_ISSUER` | Zitadel instance URL, e.g. `https://<instance>.zitadel.cloud` |
| `ZITADEL_AUDIENCE` | Resource ID of your API application (shown in the API app settings) |
| `ZITADEL_CLIENT_ID` | Client ID (for future introspection use) |

**Steps:**
1. In Zitadel, create a **Project** and inside it an **API application** (type: API, auth method: JWT).
2. Copy the **Issuer URL** from your instance's domain settings into `ZITADEL_ISSUER`.
3. Copy the **Resource ID** of the API application into `ZITADEL_AUDIENCE`.
4. Clients must send a valid JWT access token: `Authorization: Bearer <jwt>`.

Token signatures are verified **offline** using Zitadel's JWKS endpoint (`<issuer>/oauth/v2/keys`) — no per-request call to Zitadel is made. The JWKS is refreshed automatically in the background.

To add a new identity provider (e.g. Keycloak) in the future: implement the `Authenticator` interface in `internal/server/authentication/` and register it in `factory.go`.

## Tests

```bash
go test ./...
go test ./internal/domains/health/... -run TestHealth
```

## API

All `/api/v1/*` routes require `Authorization: Bearer <token>`.

### Health

```
GET /health
```

No authentication required.

**Response `200`:**
```json
{"data": {"status": "ok", "timestamp": "2026-06-08T10:00:00Z"}}
```

---

### Resize image

```
POST /api/v1/resize
Content-Type: multipart/form-data
```

**Form fields:**

| Field | Type | Required | Description |
|---|---|---|---|
| `file` | file | yes | Image to resize (`image/jpeg`, `image/png`, `image/gif`, `image/tiff`) |
| `width` | integer | one of | Target width in pixels (positive). Omit to calculate from aspect ratio. |
| `height` | integer | one of | Target height in pixels (positive). Omit to calculate from aspect ratio. |
| `mode` | string | no | Resize mode: `exact` (default), `fit`, `fill` — see below |

At least one of `width` or `height` must be provided. When only one is given, the other is calculated automatically to preserve the original aspect ratio (`mode` must be `exact` or omitted).

**Resize modes:**

| Mode | Behaviour |
|---|---|
| `exact` | Resize to exactly `width`×`height`. Aspect ratio is not preserved — the image may be distorted. |
| `fit` | Scale proportionally so the image fits within the `width`×`height` bounding box. Neither dimension exceeds the target; aspect ratio is preserved; no cropping. Output may be smaller than requested if the source is smaller. Both dimensions required. |
| `fill` | Scale proportionally, then centre-crop to exactly `width`×`height`. Output is always the exact requested size. Both dimensions required. |

**Example:**

```bash
curl -X POST http://localhost:8080/api/v1/resize \
  -H "Authorization: Bearer dev-token-12345" \
  -F "file=@/path/to/image.jpg" \
  -F "width=800" \
  -F "height=600" \
  -F "mode=fit"
```

**Response — processed immediately, S3 available `200`:**
```json
{"data": {"url": "https://..."}}
```

The `url` is a presigned S3 link valid for 24 hours.

**Response — processed immediately, S3 unavailable `200`:**

Returns raw JPEG bytes with `Content-Type: image/jpeg`.

**Response — queued (all slots busy) `202`:**
```json
{"data": {"job_id": "e3b0c442-...", "status": "queued"}}
```

Poll `GET /api/v1/jobs/{job_id}` until `status` is `complete` or `failed`.

**Error responses:**

| Status | Reason |
|---|---|
| `400` | Missing or invalid field (`width`, `height`, `mode`, or form structure) |
| `415` | Unsupported image format |
| `429` | All slots busy and no queue available (Redis/S3 disabled) |
| `503` | Queue is full (`MAX_QUEUE_DEPTH` reached) |

---

### Get job status

```
GET /api/v1/jobs/{id}
```

**Example:**

```bash
curl http://localhost:8080/api/v1/jobs/e3b0c442-... \
  -H "Authorization: Bearer dev-token-12345"
```

**Response `200`:**
```json
{
  "data": {
    "id": "e3b0c442-...",
    "operation": "resize",
    "status": "complete",
    "params": {"width": 800, "height": 600, "mode": "fit"},
    "result_url": "https://...",
    "created_at": "2026-06-08T10:00:00Z"
  }
}
```

**Job status values:**

| Status | Meaning |
|---|---|
| `queued` | Waiting in the Redis queue |
| `processing` | Worker has picked it up |
| `complete` | Done — `result_url` contains the presigned download link |
| `failed` | Processing failed — `error` field contains the reason |

**Error responses:**

| Status | Reason |
|---|---|
| `404` | Job ID not found or expired |
| `503` | Redis unavailable |

---

### Assets — persistent, tenant-scoped image storage

Unlike `/resize` (stateless, throw-away), the `assets` domain stores a normalized **master** per uploaded image, content-addressed by its own hash, and generates+caches **derivatives** (specific sizes) on demand. Re-uploading identical content, or requesting a size that's already been rendered, does no extra work.

#### Consumers and tenants

images-api is a shared, multi-consumer service: several independent applications can use the same deployment and credentials pool. To keep them from ever touching each other's data, storage is namespaced two ways:

- **Consumer** — *which calling application* (e.g. one product vs. another). This is **never** a request parameter: it's resolved from your authenticated identity (the Zitadel JWT's `sub`, or which named bearer token you presented — see `API_TOKENS` above). A caller can only ever read/write inside its own consumer namespace, no matter what it sends in a request.
- **Tenant** — a sub-identifier *within* your own consumer namespace (e.g. one customer site of yours vs. another). This *is* a plain request parameter (`tenant`) — it's yours to define, and it only ever reaches data inside your own consumer's namespace.

Both are validated against `^[A-Za-z0-9_-]{1,128}$`.

#### Upload a master

```
POST /api/v1/assets
Content-Type: multipart/form-data
Authorization: Bearer <token>
```

| Field | Type | Required | Description |
|---|---|---|---|
| `file` | file | yes | `image/jpeg`, `image/png`, or `image/webp` |
| `tenant` | string | yes | Your own sub-identifier for this image |

The image is normalized (long edge capped to `MASTER_MAX_DIMENSION`, never upscaled) and re-encoded (JPEG, or PNG if it has real transparency) before storage — the original bytes you uploaded are never persisted.

**Response `201`:**
```json
{"data": {"id": "<sha256>", "consumer": "your-consumer", "tenant": "42", "width": 2560, "height": 1706, "format": "jpeg", "bytes": 69075}}
```

#### Render a derivative

```
GET /a/{consumer}/{tenant}/{hash}/render?width=&height=&mode=&format=
```

**No authentication** — this is a public, CDN-cacheable URL (`Cache-Control: public, max-age=31536000, immutable`), the same trust model as a plain static file URL: knowledge of the exact hash is what gates access.

| Param | Required | Description |
|---|---|---|
| `width` / `height` | one of | Target size in pixels |
| `mode` | no | `exact`, `fit` (default here), or `fill` — same semantics as `/resize` |
| `format` | no | `jpeg` or `png`; omit to keep the master's own format |

First request for a given size generates and caches it; every request after is served straight from cache.

#### Delete an asset

```
DELETE /api/v1/assets/{tenant}/{hash}
Authorization: Bearer <token>
```

Deletes the master and every cached derivative under it. `consumer` is resolved from your credential, not the path. Idempotent — `204` even if it's already gone.

#### List a tenant's assets

```
GET /api/v1/assets?tenant=<id>
Authorization: Bearer <token>
```

Returns every master id, size, and last-modified time for your consumer+tenant. Intended for reconciliation/cleanup jobs and "show me what's stored" tooling.

#### Future: bring your own storage

Today all consumers share one images-api-operated bucket, isolated by the consumer-derived key prefix above. A consumer needing physically separate storage (data residency, full custody, etc.) is a designed-for future extension: both AWS S3 and MinIO natively support `AssumeRoleWithWebIdentity` — a consumer's own Zitadel-issued token can be exchanged for temporary, policy-scoped credentials against their own storage account, with no long-lived secret ever held by images-api. Not implemented yet.

---

## License

This project is licensed under the **GNU Affero General Public License v3.0 (AGPL-3.0)**.

You are free to use, modify, and distribute this software — including for commercial purposes — as long as any modified version is also distributed under the same AGPL-3.0 terms. If you run a modified version as a network service, you must make the source code available to users of that service.

See the [LICENSE](LICENSE) file for the full license text.
