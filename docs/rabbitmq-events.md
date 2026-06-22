<h1 align="center">RabbitMQ Events</h1>

Backend services communicate asynchronously through a RabbitMQ direct exchange.

<h2 align="center">dataset.uploaded</h2>

Producer: Upload Service

Consumer: ML / Normalization Service

Payload:

```json
{
  "datasetId": "dataset_123",
  "jobId": "job_456",
  "filename": "refund_approvals_dataset.csv",
  "timestamp": "2026-06-01T10:00:00Z"
}
```

<h2 align="center">dataset.normalized</h2>

Producer: ML / Normalization Service

Consumer: Graph / Relations Service

Payload:

```json
{
  "datasetId": "dataset_123",
  "jobId": "job_456",
  "normalizedRows": 1200,
  "timestamp": "2026-06-01T10:05:00Z"
}
```

<h2 align="center">refund.relations.built</h2>

Producer: Graph / Relations Service

Consumer: Scoring Service

Payload:

```json
{
  "datasetId": "dataset_123",
  "jobId": "job_456",
  "relationsCount": 3500,
  "featuresReady": true,
  "timestamp": "2026-06-01T10:10:00Z"
}
```

<h2 align="center">refund.scoring.completed</h2>

Producer: Scoring Service

Consumer: Upload Service / Status API

Payload:

```json
{
  "datasetId": "dataset_123",
  "jobId": "job_456",
  "scoredApprovalsCount": 1200,
  "suspiciousApprovalsCount": 37,
  "status": "COMPLETED",
  "eventType": "refund.scoring.completed",
  "timestamp": "2026-06-01T10:15:00Z"
}
```

<h2 align="center">pipeline.failed</h2>

Producer: Any service

Consumer: Upload Service / Status API

Payload:

```json
{
  "datasetId": "dataset_123",
  "jobId": "job_456",
  "failedStep": "SCORING",
  "errorMessage": "Failed to calculate refund risk scores",
  "timestamp": "2026-06-01T10:12:00Z"
}
```
