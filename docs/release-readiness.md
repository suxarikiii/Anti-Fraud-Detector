# Week 6 release readiness

Status: **release candidate implementation complete; deployment identity and team evidence pending**.

Public deployment target: <http://95.181.213.22/>

Observed on 2026-07-14: the root URL returned `200`, Relations and Upload health returned `UP`, but `/api/scoring/health` returned `502 Bad Gateway`. The currently deployed environment therefore does **not** satisfy Week 6 release readiness yet.

## Release identity

| Field | Value |
| --- | --- |
| Starting repository SHA | `5c14bcd3673916c501bcba26da99ad16ec8923a6` |
| RC commit | Pending commit/merge of this working tree |
| RC tag | Pending release lead (`week6-rc.N` recommended) |
| Deployed SHA | Pending deployment evidence |
| Identity check | Must satisfy `deployed SHA == documented RC SHA` before sign-off |

The starting SHA is not claimed as the deployed RC: the Week 6 changes are currently uncommitted. Fill the three pending fields only from Git/CI evidence after merge and deployment.

## Reproducible checks

```bash
# Scoring: migration, unit, MVC, persistence, decision and export checks
cd backend/scoring-service
./gradlew clean test build

# Relations contract and dataset isolation
cd ../relations-service
go vet ./...
go test -race -cover ./...

# Other services and frontend
cd ../upload-service && go vet ./... && go test -race -cover ./...
cd ../../frontend && npm ci && npm test && npm run build

# Compose and deterministic fixtures
cd ..
docker compose config
python3 docs/scripts/evaluate_scenarios.py
```

CI makes scoring tests mandatory with `./gradlew clean test build`; `-x test` is removed.

## Smoke / E2E evidence command

After the stack has a normalized dataset loaded in Relations:

```bash
DATASET_ID='<uuid>'
RETURN_ID='<return-id-from-that-dataset>'

curl -fsS "http://localhost:8080/api/relations/datasets/${DATASET_ID}/scoring-inputs"
curl -fsS -X POST "http://localhost:8080/api/scoring/datasets/${DATASET_ID}/recalculate"
curl -fsS "http://localhost:8080/api/scoring/datasets/${DATASET_ID}/returns/${RETURN_ID}/details"
curl -fsS -X PUT -H 'Content-Type: application/json' \
  -d '{"action":"ESCALATE","outcome":"NEEDS_MORE_INFO","note":"Проверить чек","analystId":"rc-smoke"}' \
  "http://localhost:8080/api/scoring/datasets/${DATASET_ID}/returns/${RETURN_ID}/decision"
curl -fsS "http://localhost:8080/api/scoring/datasets/${DATASET_ID}/export.csv" -o /tmp/scoring-export.csv
```

Restart check:

```bash
docker compose restart scoring-service
curl -fsS "http://localhost:8080/api/scoring/datasets/${DATASET_ID}/returns/${RETURN_ID}/decision"
```

## Deployment

1. Merge only after all CI jobs are green.
2. Record the merge SHA above and create the RC tag.
3. Let `.github/workflows/deploy.yml` publish SHA-tagged images and update the VM.
4. Verify `/health`, frontend, scoring health, and one dataset-scoped result.
5. Capture workflow URL, image tags, `git rev-parse HEAD` on the deployment checkout, and an API response artifact.
6. Compare the captured deployed SHA with the RC SHA before calling the release ready.

## Rollback

1. Select the last verified commit SHA/image tag.
2. Set `IMAGE_TAG=<verified-sha>` in the deployment environment.
3. Run `docker compose pull && docker compose up -d --no-build --remove-orphans`.
4. Verify gateway/frontend health and read an existing persisted decision.
5. Flyway V1 is additive. Do not drop scoring tables during rollback; older services ignore them and analyst evidence stays available.

## Known limitations

* Normalization currently runs inside Upload service and shares canonical artifacts with Relations through a Compose volume; independent scaling would require extracting that consumer into its own service.
* Relations dataset snapshots are in-memory and must be rebuilt after Relations restart. Scoring results and investigation decisions are durable in PostgreSQL.
* Dedicated graph storage and production monitoring are not included.
* External evidence (PR/CI links, second-person checkout, public screenshot, deployed SHA) cannot be generated truthfully from this local working tree and remains an explicit release-lead action.

No `production-ready` claim should be made while any item above is pending.
