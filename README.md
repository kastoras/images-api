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
| `types.go` | Exported domain types shared across all layers (`ResizeMode`, `ResizeOptions`, `ProcessResult`, `Resizer`, sentinel errors) |
| `parser.go` | Request parsing + validation helpers (`parseResizeRequest`, `parseWidth`, `parseHeight`, `parseMode`, `validateDimensions`) |
| `handler.go` | HTTP handler — routes, error mapping, response writing |
| `router.go` | Wires routes to the handler |
| `service.go` | Business logic — image processing, queue management, S3 upload |

Simple domains (e.g. `health`) use only `handler.go` + `router.go`.

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
| `ZITADEL_ENABLED` | `false` | Use Zitadel OIDC auth; `false` = static bearer token |
| `API_TOKEN` | — | Static bearer token when `ZITADEL_ENABLED=false` |
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

For local dev set `ZITADEL_ENABLED=false` and `API_TOKEN=dev-token-12345`.

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

## License

This project is licensed under the **GNU Affero General Public License v3.0 (AGPL-3.0)**.

You are free to use, modify, and distribute this software — including for commercial purposes — as long as any modified version is also distributed under the same AGPL-3.0 terms. If you run a modified version as a network service, you must make the source code available to users of that service.

See the [LICENSE](LICENSE) file for the full license text.
