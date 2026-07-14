# Demo flow

The demo shows the analyst path from refund dataset upload to investigation of a suspicious approval.

## Main Scenario

1. Open the frontend dashboard.
2. Upload `data/clean_refund_dataset.csv` or `data/dirty_business_refund_dataset.csv`.
3. Review preview/mapping.
4. Start analysis and show statuses:

```text
UPLOADED -> NORMALIZING -> NORMALIZED -> BUILDING_RELATIONS -> SCORING -> COMPLETED
```

5. Open suspicious approvals.
6. Filter/search by return ID, customer, agent, or risk level.
7. Open a high-risk return and explain the risk score, reasons, customer behavior, agent behavior, and related approvals.

## Screenshots

| Screen | File |
| --- | --- |
| Dataset upload / status | [01-dataset-upload.png](./assets/screenshots/01-dataset-upload.png) |
| Suspicious approvals | [02-suspicious-approvals.png](./assets/screenshots/02-suspicious-approvals.png) |
| Refund investigation | [03-refund-investigation.png](./assets/screenshots/03-refund-investigation.png) |

## Demo Return IDs

| Case | returnId | Expected Score / Level | Main Reasons |
| --- | --- | --- | --- |
| Normal / low | `return_3001` | `15 LOW` | `FULL_AMOUNT_REFUND` |
| Medium | `return_303075` | `45 MEDIUM` | `NO_EVIDENCE`, `HIGH_VALUE_REFUND` |
| High | `return_3006` | `75 HIGH` | `NO_EVIDENCE`, `HIGH_VALUE_REFUND`, `AGENT_HIGH_APPROVAL_RATE` |
| Critical | `return_3041` | `100 CRITICAL` | `NO_EVIDENCE`, `HIGH_VALUE_REFUND`, `FAST_APPROVAL`, `MANUAL_OVERRIDE`, relation pattern rules |

## Smoke Commands

Run the full stack:

```bash
docker compose up --build
```

Upload and status:

```bash
curl -s http://localhost:8080/api/datasets/health
curl -s -F "file=@data/clean_refund_dataset.csv" http://localhost:8080/api/datasets/upload
curl -s http://localhost:8080/api/datasets/<datasetId>/preview
curl -s -X POST http://localhost:8080/api/analysis/<datasetId>/start
curl -s http://localhost:8080/api/analysis/<jobId>/status
```

Scoring checks:

```bash
curl http://localhost:8080/api/scoring/health
curl http://localhost:8080/api/scoring/datasets/demo/suspicious-approvals
curl http://localhost:8080/api/scoring/datasets/demo/returns/return_3001/risk
curl http://localhost:8080/api/scoring/datasets/demo/returns/return_303075/risk
curl http://localhost:8080/api/scoring/datasets/demo/returns/return_3006/risk
curl http://localhost:8080/api/scoring/datasets/demo/returns/return_3041/details
```

## Narrative

For `return_3041`, the analyst sees a critical case: the refund was approved without evidence, was high value, was approved quickly, used manual override, and belongs to repeated customer-agent relation patterns. This is enough context to send the case to manual review.
