# DevOps & Infrastructure

Containerization, orchestration, and CI/CD pipeline for the Anti-Fraud-Detector project.

## Overview

The project consists of 4 application services and 3 infrastructure components, all managed through Docker Compose:

```
┌─────────────────────────────────────────────────────────┐
│  docker compose up                                      │
│                                                         │
│  ┌──────────┐   ┌──────────────────────────────────┐    │
│  │ frontend │   │ gateway (nginx)      :8080       │    │
│  │  :3000   │   │  /api/datasets/  → upload:8081   │    │
│  └──────────┘   │  /api/relations/ → relations:8082│    │
│                 │  /api/scoring    → scoring:8083   │    │
│                 └──────────────────────────────────┘    │
│                                                         │
│  ┌──────────────┐ ┌────────────┐ ┌────────────────┐    │
│  │upload-service│ │ relations  │ │scoring-service │    │
│  │  Go :8081    │ │  Go :8082  │ │ Kotlin :8083   │    │
│  └──────┬───────┘ └─────┬──────┘ └───────┬────────┘    │
│         │               │                │              │
│  ┌──────┴───┐    ┌──────┴─────┐    ┌─────┴──────┐     │
│  │ Postgres │    │  RabbitMQ  │    │   MinIO    │     │
│  │  :5432   │    │ :5672/:15672│   │ :9000/:9001│     │
│  └──────────┘    └────────────┘    └────────────┘     │
└─────────────────────────────────────────────────────────┘
```

## Quick Start

```bash
# 1. Copy environment variables
cp .env.example .env

# 2. Build and start everything
docker compose up --build

# 3. Access the services
#    Frontend:          http://localhost:3000
#    API Gateway:       http://localhost:8080
#    RabbitMQ UI:       http://localhost:15672  (guest/guest)
#    MinIO Console:     http://localhost:9001   (minioadmin/minioadmin)
```

Background mode: `docker compose up --build -d`

Stop: `docker compose down`

Stop and remove data (volumes): `docker compose down -v`

## File Structure

```
.
├── .env.example                        # environment variables template
├── docker-compose.yml                  # full stack orchestration
├── .github/workflows/
│   ├── ci.yml                          # CI: lint + test + build on PRs
│   └── deploy.yml                      # CD: push images + deploy to VM
├── backend/
│   ├── gateway/nginx.conf              # API routing
│   ├── upload-service/
│   │   ├── Dockerfile
│   │   └── .dockerignore
│   ├── relations-service/
│   │   ├── Dockerfile
│   │   └── .dockerignore
│   └── scoring-service/
│       ├── Dockerfile
│       └── .dockerignore
└── frontend/
    ├── Dockerfile
    ├── .dockerignore
    └── nginx.conf                      # SPA fallback
```

## Dockerfiles

All images use multi-stage builds: the first stage compiles the code, the second stage contains only the final binary or artifact. This keeps images small and free of build tools.

### upload-service (Go)

| Stage | Base image | Purpose |
|-------|------------|---------|
| builder | `golang:1.22-alpine` | `go mod download` → `go build` |
| runtime | `alpine:3.20` | copies the binary + `migrations/` directory |

Port: **8081**. SQL migration files are included in the image because the service applies them on startup via `golang-migrate`.

### relations-service (Go)

Same structure as upload-service, but without migrations (the service does not use a SQL database directly).

Port: **8082**.

### scoring-service (Kotlin / Spring Boot)

| Stage | Base image | Purpose |
|-------|------------|---------|
| builder | `gradle:8.10-jdk17-alpine` | `gradle dependencies` (cache layer) → `gradle bootJar` |
| runtime | `eclipse-temurin:17-jre-alpine` | copies `app.jar` |

Port: **8083** (overridden via `SERVER_PORT` env var, since `application.yml` defaults to 8081).

### frontend (React / Vite)

| Stage | Base image | Purpose |
|-------|------------|---------|
| builder | `node:20-alpine` | `npm ci` → `npm run build` |
| runtime | `nginx:1.27-alpine` | serves static files from `/dist` with SPA fallback via `try_files` |

Port: **80** (mapped to 3000 in Compose).

## docker-compose.yml

### Infrastructure Services

**Postgres** — primary database for upload-service. Data is persisted in a Docker volume (`postgres_data`), so it survives `docker compose down` but not `docker compose down -v`.

**RabbitMQ** — message broker connecting the backend services. Management UI: http://localhost:15672. Services exchange events through a pipeline: `dataset.uploaded` → `dataset.normalized` → `refund.relations.built` → `refund.scoring.completed`.

**MinIO** — S3-compatible object storage for uploaded CSV files. Console: http://localhost:9001.

### Healthchecks and depends_on

Each infrastructure service defines a `healthcheck`. Backend services use `depends_on` with `condition: service_healthy`, which means upload-service will not start until Postgres, RabbitMQ, and MinIO are ready to accept connections.

Without healthchecks, Docker starts dependencies "in order" but does not wait for them to become ready — a service might crash because the database has not accepted its first connection yet.

### Networking

Docker Compose creates a shared network automatically. Within this network, services address each other by name (e.g., `postgres`, `rabbitmq`, `minio`). This is why environment variables use `DB_HOST=postgres` instead of `localhost`.

### Environment Variables

Compose reads a `.env` file from the project root. All variables have defaults via the `${VAR:-default}` syntax, so the project will start even without `.env`, but creating one is recommended:

```bash
cp .env.example .env
# edit as needed
```

## API Gateway (nginx)

The file `backend/gateway/nginx.conf` acts as a single entry point for all API requests:

| URL pattern | Backend service |
|-------------|-----------------|
| `/api/datasets/` | upload-service:8081 |
| `/api/analysis` | upload-service:8081 |
| `/api/relations/` | relations-service:8082 |
| `/api/scoring` | scoring-service:8083 |

The frontend sends requests to the gateway at `localhost:8080`, and the gateway proxies them to the appropriate backend service.

## CI/CD

### CI — `.github/workflows/ci.yml`

Triggered on every Pull Request targeting `main`. Runs four parallel jobs:

| Job | Steps |
|-----|-------|
| Upload Service | `go vet` → `go test -race -cover` → `docker build` |
| Relations Service | `go vet` → `go test -race -cover` → `docker build` |
| Scoring Service | `gradle check` (compile + tests) → `docker build` |
| Frontend | `npm ci` → `npm run build` → `docker build` |

If any job fails, the PR cannot be merged (when branch protection is enabled).

### CD — `.github/workflows/deploy.yml`

Triggered on push to `main` (i.e., after a PR is merged).

**Step 1 — Build & Push**: a matrix of 4 services, each built in parallel:
- builds a Docker image via `docker/build-push-action` with GitHub Actions cache
- pushes to Docker Hub with two tags: the commit SHA and `latest`

**Step 2 — Deploy**: connects to the VM via SSH:
- copies `docker-compose.yml`, `nginx.conf`, and `.env.example`
- runs `docker compose pull && docker compose up -d`
- prunes old images

### Required GitHub Secrets

Configure in Settings → Secrets and variables → Actions:

| Secret | Description |
|--------|-------------|
| `DOCKERHUB_USERNAME` | Docker Hub login |
| `DOCKERHUB_TOKEN` | Docker Hub Access Token (not password) |
| `DEPLOY_HOST` | IP or domain of the deployment VM |
| `DEPLOY_USER` | SSH username on the VM |
| `DEPLOY_SSH_KEY` | Private SSH key for VM access |

## Common Commands

```bash
# Rebuild a single service
docker compose up --build upload-service

# Follow logs for a specific service
docker compose logs -f scoring-service

# Open a shell inside a running container
docker compose exec upload-service sh

# Rebuild and restart after code changes
docker compose up --build -d

# Check status of all containers
docker compose ps
```

## Known Limitations

- **scoring-service** listens on port 8081 in `application.yml` instead of 8083. Compose overrides this via the `SERVER_PORT` env var, but the source file should be updated by the scoring-service owner.
- **ML service** (data normalization) is not yet containerized because its code has not been added to the repository.
- **Monitoring** (Prometheus, Grafana) is not included yet — planned as a follow-up.
