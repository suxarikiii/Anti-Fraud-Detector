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

Preview response:

```json
{
  "headers": ["order_id", "customer_id", "return_id"],
  "rows": [
    {
      "order_id": "order_1001",
      "customer_id": "customer_200",
      "return_id": "return_3001"
    }
  ],
  "rawRows": [["order_1001", "customer_200", "return_3001"]],
  "rowCount": 1,
  "truncated": false
}
```

Analysis status response includes the current status, current step, dashboard message, progress percentage, and stage list:

```json
{
  "jobId": "c8f7844b-f2fb-42a1-b45f-397d56f3ad2f",
  "datasetId": "8f79f612-770b-48fd-9d25-79d88d1e4211",
  "status": "NORMALIZING",
  "currentStep": "NORMALIZING",
  "message": "Normalizing CSV columns and refund records.",
  "progressPercent": 20,
  "stages": [
    { "status": "UPLOADED", "message": "Dataset uploaded and ready to start analysis.", "state": "completed" },
    { "status": "NORMALIZING", "message": "Normalizing CSV columns and refund records.", "state": "current" }
  ]
}
```

Upload/status errors use one JSON shape:

```json
{
  "status": 400,
  "error": "Bad Request",
  "code": "INVALID_CSV",
  "message": "uploaded CSV is empty",
  "path": "/api/datasets/upload",
  "timestamp": "2026-06-01T10:15:00Z"
}
```

Common upload/status error codes: `INVALID_CSV`, `INVALID_DATASET_ID`, `INVALID_JOB_ID`, `DATASET_NOT_FOUND`, `JOB_NOT_FOUND`, `INVALID_ANALYSIS_STATUS`, `ANALYSIS_START_FAILED`.

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

Relation features response:

```json
{
  "returnId": "return_3041",
  "customerId": "customer_999",
  "supportAgentId": "agent_999",
  "features": {
    "customerReturnCount": 5,
    "customerApprovedRefundCount": 5,
    "agentApprovalRate": 1,
    "agentManualOverrideRate": 1,
    "agentHighValueApprovalCount": 5,
    "customerAgentPairCount": 5,
    "agentCustomerInteractionCount": 5,
    "categoryRefundRate": 0.16,
    "refundAmountRatio": 0.87,
    "similarReturnsCount": 0,
    "sameReasonRefundCount": 2,
    "clusterSize": 5,
    "strongestRelationType": "CUSTOMER_RETURN_PATTERN",
    "topRelatedReturns": ["return_3042", "return_3043", "return_3044", "return_3045"],
    "explanationSummary": "Customer refund history is the strongest relation signal.",
    "explanationSignals": [
      "Customer has 5 refund requests in the demo dataset.",
      "Support agent approval rate is 100%.",
      "Customer and support agent interacted on 5 return requests.",
      "Refund amount is 87% of the original order amount.",
      "Relation cluster fallback size is 5.",
      "Related returns for investigation: return_3042, return_3043, return_3044, return_3045."
    ]
  }
}
```

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
