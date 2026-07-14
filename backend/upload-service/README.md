<h1 align="center">Upload Service</h1>

The Upload Service is the reliable entry point and lifecycle coordinator for an analysis. It performs bounded CSV ingestion and validation, stores artifacts in MinIO, records datasets/jobs/audit events in PostgreSQL, publishes `dataset.uploaded`, and automatically advances jobs from durable RabbitMQ pipeline events.

<h2 align="center">Running the Upload Service</h2>

Prerequisites:

* Go;
* Docker;
* Docker Compose;
* free local ports `5432`, `5672`, `8081`, `9002`, `9003`, and `15672`.

Start the local infrastructure from the upload-service directory:

```bash
cd backend/upload-service
docker compose up -d postgres rabbitmq minio
```

Run the service:

```bash
cd backend/upload-service
SERVER_PORT=:8081 \
DB_HOST=localhost \
DB_PORT=5432 \
DB_USER=postgres \
DB_PASSWORD=postgres \
DB_NAME=upload_db \
RABBITMQ_URL=amqp://guest:guest@localhost:5672/ \
RABBITMQ_EXCHANGE=pipeline.exchange \
RABBITMQ_UPLOAD_EVENTS_QUEUE=upload.pipeline-events.queue \
RABBITMQ_UPLOAD_DLQ=upload.pipeline-events.dlq \
RABBITMQ_MAX_RETRIES=3 \
MINIO_ENDPOINT=localhost:9002 \
MINIO_ACCESS_KEY=minioadmin \
MINIO_SECRET_KEY=minioadmin \
MINIO_BUCKET=datasets \
MINIO_SECURE=false \
UPLOAD_MAX_FILE_SIZE_BYTES=52428800 \
UPLOAD_MAX_ROWS=250000 \
UPLOAD_MAX_VALIDATION_ERRORS=100 \
HTTP_READ_TIMEOUT=2m \
HTTP_WRITE_TIMEOUT=2m \
go run cmd/main.go
```

The service runs database migrations on startup.

Health check:

```bash
curl http://localhost:8081/api/datasets/health
```

Upload a CSV dataset:

```bash
curl -X POST http://localhost:8081/api/datasets/upload \
  -F "file=@../../data/dirty_business_refund_dataset.csv"
```

Preview the uploaded dataset:

```bash
curl http://localhost:8081/api/datasets/<datasetId>/preview
```

Start analysis for an uploaded dataset:

```bash
curl -X POST http://localhost:8081/api/analysis/<datasetId>/start
```

Check analysis status:

```bash
curl http://localhost:8081/api/analysis/<jobId>/status
```

List datasets, inspect history, retry, and archive:

```bash
curl 'http://localhost:8081/api/datasets?status=FAILED&page=1&pageSize=20'
curl http://localhost:8081/api/datasets/<datasetId>
curl -X POST http://localhost:8081/api/analysis/<jobId>/retry
curl -X POST http://localhost:8081/api/datasets/<datasetId>/archive
```

Status values exposed to the frontend:

```text
UPLOADED -> NORMALIZING -> NORMALIZED -> BUILDING_RELATIONS -> SCORING -> COMPLETED
FAILED
```

Normal operation has no manual status mutation. The legacy PATCH route is not registered unless `ADMIN_STATUS_PATCH_ENABLED=true`; that flag is intended only for isolated admin/development diagnostics and must remain disabled in release environments.

Upload and status errors are returned as structured JSON:

```json
{
  "status": 400,
  "error": "Bad Request",
  "code": "INVALID_CSV",
  "message": "uploaded CSV is empty",
  "path": "/api/datasets/upload",
  "timestamp": "2026-06-01T10:15:00Z"
}
```

Run the gateway smoke flow from the repository root after the Compose stack is up:

```bash
BASE_URL=http://localhost:8080 ./backend/upload-service/scripts/smoke_gateway.sh
```

The smoke flow checks health, bounded upload/preview, start, management endpoints, and structured error responses. When `RABBIT_API_URL` is set, it also publishes correlated lifecycle events and verifies automatic completion.

Useful local consoles:

* RabbitMQ UI: `http://localhost:15672` (`guest` / `guest`);
* MinIO console: `http://localhost:9003` (`minioadmin` / `minioadmin`).

## Lifecycle and reliability policy

* `POST /api/analysis/{datasetId}/start` uses a database-locked claim, so concurrent/repeated calls publish at most once for that job.
* The durable consumer handles `dataset.normalized`, `refund.relations.built`, `refund.scoring.completed`, and `pipeline.failed`. Invalid/correlation errors go directly to the DLQ; transient errors are republished as persistent messages up to `RABBITMQ_MAX_RETRIES` and then dead-lettered.
* Duplicate and out-of-order events are acknowledged without changing state. Every accepted transition is written to `lifecycle_audit_events`.
* Retry is allowed only from `FAILED` or `COMPLETED`. It creates a linked job and preserves prior jobs/results. Repeating retry for the same source job is idempotent.
* Archive is soft-delete and only allowed when every job is terminal. It retains MinIO artifacts, analysis jobs/results, and audit history. Archived datasets are excluded from list results and cannot be started/retried. Physical deletion is deliberately outside the Week 6 API.
* MinIO is written before the database transaction. If the transaction fails, the object is compensating-deleted; a cleanup failure is emitted as a structured error log. Publisher failure marks the claimed job `FAILED` with a public-safe message.

## Validation policy

The service accepts `.csv` files with CSV/plain-text MIME, enforces configured file/row limits, checks required semantic headers and row width, and aggregates up to the configured number of row/column errors. Empty required IDs, duplicate `return_id`, invalid decisions/timestamps/non-finite numbers, non-positive order amounts, and negative refund/time values are errors. Zero refund amounts and missing optional decision time are accepted with warnings. Filenames are sanitized, and previews return at most 20 rows.

Run checks:

```bash
go test ./...
go test -race ./...
go vet ./...
```

Stop local infrastructure:

```bash
cd backend/upload-service
docker compose down
```

Remove local infrastructure volumes:

```bash
cd backend/upload-service
docker compose down -v
```
