<h1 align="center">API Contracts</h1>

All frontend calls should go through the gateway using `/api/...`.

## Upload Service

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/api/datasets/health` | Upload service health. |
| `POST` | `/api/datasets/upload` | Upload CSV refund dataset. |
| `GET` | `/api/datasets/{datasetId}/preview` | Preview uploaded rows. |
| `POST` | `/api/analysis/{datasetId}/start` | Start analysis for uploaded dataset. |
| `GET` | `/api/analysis/{jobId}/status` | Read analysis status. |

Status values:

```text
UPLOADED -> NORMALIZING -> NORMALIZED -> BUILDING_RELATIONS -> SCORING -> COMPLETED
FAILED
```

## Relations Service

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/api/relations/health` | Relations service health. |
| `GET` | `/api/relations/returns/{returnId}` | Return relation context. |
| `GET` | `/api/relations/returns/{returnId}/features` | Demo relation features. |
| `GET` | `/api/relations/datasets/{datasetId}/returns/{returnId}/features` | Dataset-scoped relation features. |
| `GET` | `/api/relations/customers/{customerId}/history` | Customer return history. |
| `GET` | `/api/relations/agents/{agentId}/summary` | Support agent summary. |
| `POST` | `/api/relations/datasets/{datasetId}/rebuild` | Rebuild relation features. |

## Scoring Service

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/api/scoring/health` | Scoring service health. |
| `GET` | `/api/scoring/datasets/{datasetId}/suspicious-approvals` | Ranked approvals with score >= `31`. |
| `GET` | `/api/scoring/datasets/{datasetId}/returns/{returnId}/risk` | Risk score and reasons. |
| `GET` | `/api/scoring/datasets/{datasetId}/returns/{returnId}/details` | Risk plus order/return/feature context. |
| `GET` | `/api/scoring/datasets/{datasetId}/agents/{agentId}/risk-summary` | Agent-level risk summary. |
| `POST` | `/api/scoring/datasets/{datasetId}/recalculate` | Recalculate dataset risk. |

Compatibility routes:

```text
GET /api/scoring/returns/{returnId}/risk
GET /api/scoring/agents/{agentId}/risk-summary
```

These use the `demo` dataset internally.

## Key Response Shapes

Risk response:

```json
{
  "returnId": "return_303075",
  "datasetId": "demo",
  "riskScore": 45,
  "riskLevel": "MEDIUM",
  "topReason": "Refund was approved without attached evidence, so the analyst cannot verify the customer's claim from this record.",
  "reasons": [
    { "type": "NO_EVIDENCE", "scoreImpact": 25 },
    { "type": "HIGH_VALUE_REFUND", "scoreImpact": 20 }
  ],
  "calculatedAt": "2026-06-01T10:15:00Z"
}
```

Details response adds business facts and relation-style features:

```json
{
  "returnId": "return_3041",
  "datasetId": "demo",
  "orderAmount": 1168.27,
  "refundAmount": 1019.25,
  "decision": "APPROVED",
  "manualOverride": true,
  "decisionTimeMinutes": 4,
  "riskScore": 100,
  "riskLevel": "CRITICAL",
  "relationFeatures": {
    "customerReturnCount": 5,
    "agentApprovalRate": 1.0,
    "customerAgentPairCount": 5,
    "clusterSize": 5,
    "strongestRelationType": "REPEATED_AGENT_CUSTOMER_PAIR",
    "featureSource": "CSV_DERIVED_FALLBACK"
  }
}
```

Error response:

```json
{
  "status": 404,
  "error": "Not Found",
  "message": "Return approval was not found: missing_return in dataset: demo",
  "path": "/api/scoring/datasets/demo/returns/missing_return/details",
  "timestamp": "2026-06-01T10:15:00Z"
}
```

## Current Limits

* Literal unknown dataset IDs return `404`.
* Uploaded UUID dataset IDs currently use CSV-derived scoring fallback.
* Relation features are represented in scoring responses, but persisted relation-feature handoff is still follow-up work.
