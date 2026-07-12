# Upload Service implementation

Upload Service owns bounded CSV ingestion and the lifecycle of every analysis job.

## Runtime flow

1. `POST /api/datasets/upload` validates the extension, MIME, headers, row shape and business fields while spooling at most `UPLOAD_MAX_FILE_SIZE_BYTES` to a temporary file.
2. The validated file is stored as `datasets/<datasetId>.csv` in MinIO. Dataset, file metadata, initial `UPLOADED` job, and audit record are committed in one PostgreSQL transaction. A database failure triggers compensating MinIO deletion.
3. `POST /api/analysis/{datasetId}/start` locks and claims the latest `UPLOADED` job before publishing persistent `dataset.uploaded`. Concurrent/repeated calls return the same job without another publish.
4. The durable queue consumes `dataset.normalized`, `refund.relations.built`, `refund.scoring.completed`, and `pipeline.failed`. Database row locks enforce correlation and allowed transitions. Duplicate, terminal, and out-of-order events are safe no-ops.
5. A completed job exposes `resultReady=true`; a failed job exposes `failedStage` and a sanitized `errorMessage`.

## Lifecycle and management

```text
UPLOADED -> NORMALIZING -> BUILDING_RELATIONS -> SCORING -> COMPLETED
                         \-------------------------------> FAILED
```

`NORMALIZED` remains in the public status enum for backward compatibility. Receipt of `dataset.normalized` moves directly to `BUILDING_RELATIONS`, because relation processing starts from that event.

Management endpoints:

```text
GET  /api/datasets
GET  /api/datasets/{datasetId}
POST /api/analysis/{jobId}/retry
POST /api/datasets/{datasetId}/archive
```

Retry is idempotent per source job, is allowed from `FAILED`/`COMPLETED`, and preserves earlier runs. Archive is a terminal-only soft delete: objects, jobs/results, and audit records are retained. Manual status PATCH is not registered unless `ADMIN_STATUS_PATCH_ENABLED=true`.

## Operational checks

```bash
go test ./...
go test -race ./...
go vet ./...

BASE_URL=http://localhost:8081 \
RABBIT_API_URL=http://localhost:15672 \
./scripts/smoke_gateway.sh
```

The RabbitMQ consumer uses manual ack, persistent retry republishing, publisher confirms, a configured retry cap, and a durable DLQ. A lost consumer connection terminates the process cleanly so Compose can restart it and resume from the durable queue.

See [OpenAPI](docs/openapi.yaml) and [README](README.md) for the full contract and configuration.
