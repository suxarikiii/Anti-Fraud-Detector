<h1 align="center">Upload Service</h1>

The Upload Service handles CSV dataset upload, MinIO file storage, PostgreSQL dataset records, analysis job creation, preview API, and RabbitMQ event publishing.

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

Start analysis for an uploaded dataset:

```bash
curl -X POST http://localhost:8081/api/analysis/<datasetId>/start
```

Check analysis status:

```bash
curl http://localhost:8081/api/analysis/<jobId>/status
```

Useful local consoles:

* RabbitMQ UI: `http://localhost:15672` (`guest` / `guest`);
* MinIO console: `http://localhost:9003` (`minioadmin` / `minioadmin`).

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
