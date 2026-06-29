### Upload Service

Upload Service accepts refund CSV datasets, validates the uploaded file, stores raw files in MinIO,
creates dataset/job records in PostgreSQL, returns CSV previews, and starts the analysis flow by
publishing `dataset.uploaded`.

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
  "jobId": "<uuid>",
  "datasetId": "<uuid>",
  "status": "NORMALIZING",
  "currentStep": "NORMALIZING",
  "message": "Normalizing CSV columns and refund records.",
  "progressPercent": 20,
  "stages": [
    {
      "status": "UPLOADED",
      "message": "Dataset uploaded and ready to start analysis.",
      "state": "completed"
    },
    {
      "status": "NORMALIZING",
      "message": "Normalizing CSV columns and refund records.",
      "state": "current"
    }
  ]
}
```

Update status manually during local demo:

```bash
curl -s -X PATCH http://localhost:8080/api/analysis/<jobId>/status \
  -H "Content-Type: application/json" \
  -d '{"status":"SCORING"}'
```

Supported statuses:

```text
UPLOADED
NORMALIZING
NORMALIZED
BUILDING_RELATIONS
SCORING
COMPLETED
FAILED
```

### Flow Notes

`POST /api/datasets/upload` creates one `analysis_jobs` row in `UPLOADED`.

`POST /api/analysis/{datasetId}/start` reuses that row, publishes one `dataset.uploaded` event with
`datasetId`, `jobId`, `filename`, `filePath`, `fileType`, `uploadedAt`, and `timestamp`, then updates
the job to `NORMALIZING`.
Calling start again for the same dataset returns the existing job id instead of creating another job.

Uploaded CSV validation checks `.csv` extension, empty files, required refund columns, duplicate or
empty headers, inconsistent row length, required empty cells, numeric fields, and supported timestamp
formats. Both the clean canonical dataset and the dirty business-column dataset are accepted.

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

Open exchange `pipeline.exchange`; `dataset.uploaded` messages are published when analysis starts.

MinIO:

```text
http://localhost:9001
login: minioadmin
password: minioadmin
```

Open bucket `datasets`; uploaded objects are stored under `datasets/<datasetId>.csv`.
