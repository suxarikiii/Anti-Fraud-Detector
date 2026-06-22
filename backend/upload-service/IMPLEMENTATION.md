### Upload Service

Upload Service accepts CSV datasets, stores raw files in MinIO, creates dataset/job records in
PostgreSQL, returns CSV previews, and starts the analysis flow by publishing `dataset.uploaded`.

### Run

From the repository root:

```bash
docker compose up --build
```

The service runs behind the gateway:

```text
http://localhost:8080
```

Direct container port is not exposed in the root compose file. Use the gateway for local checks.

### API Checks

```bash
curl http://localhost:8080/api/datasets/health
```

Expected:

```json
{"service":"upload-service","status":"UP"}
```

Upload the clean dataset:

```bash
curl -s -F "file=@data/clean_refund_dataset.csv" \
  http://localhost:8080/api/datasets/upload
```

Upload the dirty/business-format dataset:

```bash
curl -s -F "file=@data/dirty_business_refund_dataset.csv" \
  http://localhost:8080/api/datasets/upload
```

Expected upload response:

```json
{
  "datasetId": "<uuid>",
  "jobId": "<uuid>",
  "filename": "clean_refund_dataset.csv",
  "status": "UPLOADED"
}
```

Preview:

```bash
curl -s http://localhost:8080/api/datasets/<datasetId>/preview
```

Expected preview response:

```json
{
  "headers": ["order_id", "customer_id"],
  "rows": [["order_1001", "customer_200"]]
}
```

Start analysis:

```bash
curl -s -X POST http://localhost:8080/api/analysis/<datasetId>/start
```

Expected:

```json
{"jobId":"<uuid>"}
```

Status:

```bash
curl -s http://localhost:8080/api/analysis/<jobId>/status
```

Expected after start:

```json
{
  "id": "<uuid>",
  "datasetId": "<uuid>",
  "status": "NORMALIZING",
  "currentStep": "NORMALIZING"
}
```

### Flow Notes

`POST /api/datasets/upload` creates one `analysis_jobs` row in `UPLOADED`.

`POST /api/analysis/{datasetId}/start` reuses that row, publishes one `dataset.uploaded` event with
`datasetId`, `jobId`, `filePath`, `fileType`, and `uploadedAt`, then updates the job to `NORMALIZING`.
Calling start again for the same dataset returns the existing job id instead of creating another job.

### Infrastructure Checks

PostgreSQL:

```bash
docker compose exec postgres psql -U postgres -d upload_db
```

Useful queries:

```sql
select id, name, status, created_at from datasets order by created_at desc limit 5;
select dataset_id, file_path, file_type, uploaded_at from uploaded_files order by uploaded_at desc limit 5;
select id, dataset_id, status, current_step, updated_at from analysis_jobs order by created_at desc limit 5;
```

RabbitMQ:

```text
http://localhost:15672
login: guest
password: guest
```

Open exchange `dataset.events`; `dataset.uploaded` messages are published when analysis starts.

MinIO:

```text
http://localhost:9001
login: minioadmin
password: minioadmin
```

Open bucket `datasets`; uploaded objects are stored under `datasets/<datasetId>.csv`.
