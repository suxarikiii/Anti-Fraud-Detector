# DevOps

The project runs locally through root `docker-compose.yml`.

## Quick Start

```bash
cp .env.example .env
docker compose up --build
```

| Service | URL |
| --- | --- |
| Frontend | `http://localhost` |
| Gateway | `http://localhost:8080` |
| Scoring Service | `http://localhost:8083` |
| Relations Service | `http://localhost:8082` |
| RabbitMQ UI | `http://localhost:15672` |
| MinIO Console | `http://localhost:9001` |

Stop:

```bash
docker compose down
```

Remove local volumes:

```bash
docker compose down -v
```

## Gateway Routes

| URL prefix | Target |
| --- | --- |
| `/api/datasets/` | upload-service |
| `/api/analysis` | upload-service |
| `/api/relations/` | relations-service |
| `/api/scoring` | scoring-service |

## Local Checks

```bash
cd backend/scoring-service && ./gradlew test
cd backend/scoring-service && ./gradlew clean build
cd frontend && npm ci && npm run test && npm run build
python3 docs/scripts/evaluate_scenarios.py
backend/upload-service/scripts/smoke_gateway.sh
```

## Current CI Status

| Area | Current workflow behavior |
| --- | --- |
| Upload Service | `go vet`, `go test -race -cover`, Docker build. |
| Relations Service | `go vet`, `go test -race -cover`, Docker build. |
| Scoring Service | `./gradlew clean test build`, Docker build. Tests are mandatory. |
| Frontend | `npm ci`, `npm run build`, Docker build. Frontend tests are not yet part of CI. |
| Compose | `docker compose config`. |

## Deployment Status

* Local Docker Compose: implemented.
* VM deployment target: `http://95.181.213.22/`.
* Exact deployed SHA/tag: pending CI/deployment verification; see `docs/release-readiness.md`.
* Health checks must be added after deployment is verified.

## Deployment Notes

Future CD should build Docker images, push commit-SHA/latest tags, and update the VM Compose stack after verification.

Required secrets:

| Secret | Purpose |
| --- | --- |
| `DOCKERHUB_USERNAME` | Docker Hub login. |
| `DOCKERHUB_TOKEN` | Docker Hub token. |
| `DEPLOY_HOST` | VM host. |
| `DEPLOY_USER` | VM SSH user. |
| `DEPLOY_SSH_KEY` | VM private key. |

## Known Limits

* ML/normalization service is still a follow-up integration component.
* Monitoring stack is not included.
* Scoring requires a dataset-scoped Relations snapshot; it has no UUID demo fallback.
