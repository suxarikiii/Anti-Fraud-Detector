# Anti-Fraud Detector

Anti-Fraud Detector is an event-driven platform for finding suspicious refund approvals. It accepts a CSV dataset, builds relationships between refunds, calculates explainable risk scores, and gives an investigator a workflow for reviewing and exporting results.

![Anti-Fraud Detector](frontend/src/assets/brand/anti-fraud-logo-full.png)

## System at a glance

| Component | Purpose | Local entry point |
| --- | --- | --- |
| [Frontend](frontend/README.md) | Upload, progress, and investigation UI | `http://localhost` |
| [Gateway](backend/gateway/README.md) | Single HTTP entry point and backend routing | `http://localhost:8080` |
| [Upload Service](backend/upload-service/README.md) | CSV validation, storage, dataset metadata, and pipeline state | Internal `:8081` |
| [Relations Service](backend/relations-service/README.md) | Relationship graph and derived features | `http://localhost:8082` |
| [Scoring Service](backend/scoring-service/README.md) | Explainable scoring, investigation decisions, and CSV export | `http://localhost:8083` |

PostgreSQL, RabbitMQ, and MinIO are shared infrastructure dependencies, not application services maintained in this repository.

## End-to-end workflow

```mermaid
flowchart LR
    Analyst[Analyst] --> UI[Frontend]
    UI --> GW[Gateway]
    GW --> Upload[Upload Service]
    GW --> Relations[Relations Service]
    GW --> Scoring[Scoring Service]

    Upload --> MinIO[(MinIO)]
    Upload --> DB[(PostgreSQL)]
    Upload -->|dataset.uploaded| MQ[(RabbitMQ)]
    MQ -->|dataset.normalized| Relations
    Relations -->|refund.relations.built| MQ
    MQ --> Scoring
    Scoring -->|read features| Relations
    Scoring --> DB
    Scoring -->|refund.scoring.completed| MQ
    MQ --> Upload
```

The normalizer that turns `dataset.uploaded` into `dataset.normalized` is an external pipeline stage. Without it, an uploaded job remains in progress unless the lifecycle event is supplied by an integration environment.

## Run and verify

Requirements: Docker Engine with Compose v2 and free ports `80`, `5432`, `5672`, `8080`, `8082`, `8083`, `9000`, `9001`, and `15672`. Make is optional and provides the shorter commands below.

Start and verify the complete application from the repository root using the [`Makefile`](Makefile):

```bash
make up
make ps
make smoke
```

The equivalent direct Docker Compose commands are:

```bash
test -f .env || cp .env.example .env
docker compose config --quiet
docker compose up --build -d
docker compose ps
./scripts/smoke-test.sh
```

Open `http://localhost`. Common whole-stack management commands are:

```bash
make logs
make restart
make down
```

Their Docker Compose equivalents are:

```bash
docker compose logs -f
docker compose restart
docker compose down
```

Commands for running and testing individual services are kept in their own documentation:

| Service | Commands |
| --- | --- |
| Frontend | [Local run](frontend/README.md#run-locally) · [Tests and build](frontend/README.md#verify) |
| API Gateway | [Run and verify](backend/gateway/README.md#run-and-verify) |
| Upload Service | [Run and verify](backend/upload-service/README.md#run-and-verify) |
| Relations Service | [Run and verify](backend/relations-service/README.md#run-and-verify) |
| Scoring Service | [Run and verify](backend/scoring-service/README.md#run-and-verify) |

Use `make clean` or `docker compose down -v` only when you intentionally want to stop the stack and delete local PostgreSQL and MinIO data.

## Main API flow

All UI traffic goes through the gateway:

```text
POST /api/datasets/upload
GET  /api/datasets/{datasetId}/preview
POST /api/analysis/{datasetId}/start
GET  /api/analysis/{jobId}/status
GET  /api/scoring/datasets/{datasetId}/suspicious-approvals
GET  /api/scoring/datasets/{datasetId}/returns/{returnId}/details
PUT  /api/scoring/datasets/{datasetId}/returns/{returnId}/decision
GET  /api/scoring/datasets/{datasetId}/export.csv
```

The complete request and event contracts are documented in [API contracts](docs/api-contracts.md).

## Documentation

Start with the [documentation index](docs/README.md). The key references are:

- [Architecture and boundaries](docs/architecture.md)
- [API and RabbitMQ contracts](docs/api-contracts.md)
- [CSV data format](docs/data-format.md)
- [Scoring rules](docs/scoring-rules.md)
- [Operations and deployment](docs/devops.md)
- [Demo flow](docs/demo-flow.md)
- [Release readiness](docs/release-readiness.md)
- [Frontend external E2E checklist](docs/frontend-e2e-checklist.md)

Service-specific setup, configuration, interactions, and workflows live next to each service in its own README.

## Completed backend capabilities

- Dataset history, terminal archive, analysis retry, and lifecycle audit.
- Idempotent start/retry and reliable RabbitMQ retry/DLQ processing.
- Dataset-scoped relation features and bounded investigation graphs.
- Persisted scoring results, investigation decisions, and filtered CSV export.
- Deterministic scoring validation against shared golden fixtures.

The frontend uses upload, status, scoring details, customer/agent analytics, the relation graph, persisted analyst decisions, filtered CSV export, and dataset history/retry/archive APIs.

## Project status

The repository contains a release-candidate implementation with a connected upload → normalization → relations → scoring flow. The public deployment must still be verified against the exact release commit. Remaining checks are tracked in [release readiness](docs/release-readiness.md).

## License

Released under the [MIT License](LICENSE.md).
