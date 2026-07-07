<h1 align="center">Upload Service</h1>

The Upload Service handles CSV dataset upload, MinIO file storage, PostgreSQL dataset records, analysis job creation, preview API, and RabbitMQ pipeline exchange publishing (`pipeline.exchange`).

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
MINIO_ENDPOINT=localhost:9002 \
MINIO_ACCESS_KEY=minioadmin \
MINIO_SECRET_KEY=minioadmin \
MINIO_BUCKET=datasets \
MINIO_SECURE=false \
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

Status values exposed to the frontend:

```text
UPLOADED -> NORMALIZING -> NORMALIZED -> BUILDING_RELATIONS -> SCORING -> COMPLETED
FAILED
```

For local demo evidence, status can be advanced manually:

```bash
curl -X PATCH http://localhost:8081/api/analysis/<jobId>/status \
  -H "Content-Type: application/json" \
  -d '{"status":"SCORING"}'
```

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

The smoke flow checks health, upload, preview, start analysis, status transitions, and expected JSON errors for empty CSV, missing dataset, and missing job.

Useful local consoles:

* RabbitMQ UI: `http://localhost:15672` (`guest` / `guest`);
* MinIO console: `http://localhost:9003` (`minioadmin` / `minioadmin`).

Current MVP integration note:

* `POST /api/analysis/{datasetId}/start` publishes `dataset.uploaded` to `pipeline.exchange` and moves the job to `NORMALIZING`.
* Downstream normalization, relation building, and scoring may still be represented by prepared demo datasets or manual status updates when the full async pipeline is not running.
* If publishing `dataset.uploaded` fails, the upload job is marked `FAILED` with a readable `errorMessage`.

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
