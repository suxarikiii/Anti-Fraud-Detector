<h1 align="center">API Contracts</h1>

This document lists MVP API endpoints for Fraud & Abuse Detection System.

<h2 align="center">Upload Service</h2>

```text
GET /api/datasets/health
POST /api/datasets/upload
GET /api/datasets/{datasetId}/preview
GET /api/analysis/{jobId}/status
```

Example analysis status response:

```json
{
  "jobId": "job_456",
  "datasetId": "dataset_123",
  "status": "SCORING",
  "updatedAt": "2026-06-01T10:12:00Z"
}
```

<h2 align="center">Scoring Service</h2>

```text
GET /api/scoring/health
GET /api/scoring/datasets/{datasetId}/suspicious-approvals
GET /api/scoring/datasets/{datasetId}/returns/{returnId}/risk
GET /api/scoring/datasets/{datasetId}/returns/{returnId}/details
GET /api/scoring/datasets/{datasetId}/agents/{agentId}/risk-summary
GET /api/scoring/returns/{returnId}/risk
GET /api/scoring/agents/{agentId}/risk-summary
POST /api/scoring/datasets/{datasetId}/recalculate
```

`GET /api/scoring/returns/{returnId}/risk` and `GET /api/scoring/agents/{agentId}/risk-summary` are temporary demo-compatible endpoints for the current CSV-backed mode. They internally use the `demo` dataset until normalized dataset storage is introduced.

Example suspicious approvals response:

```json
[
  {
    "returnId": "return_123",
    "orderId": "order_456",
    "customerId": "customer_789",
    "supportAgentId": "agent_001",
    "refundAmount": 249.99,
    "decision": "APPROVED",
    "riskScore": 84,
    "riskLevel": "HIGH",
    "topReason": "Refund approved without evidence for a high-value order"
  }
]
```

Example return risk response:

```json
{
  "returnId": "return_3041",
  "orderId": "order_1041",
  "customerId": "customer_999",
  "supportAgentId": "agent_999",
  "datasetId": "demo",
  "riskScore": 100,
  "riskLevel": "CRITICAL",
  "topReason": "Refund was approved without required evidence",
  "reasons": [
    {
      "type": "NO_EVIDENCE",
      "message": "Refund was approved without required evidence",
      "scoreImpact": 25
    },
    {
      "type": "MANUAL_OVERRIDE",
      "message": "Manual override was used for this refund approval",
      "scoreImpact": 20
    }
  ],
  "calculatedAt": "2026-06-01T10:15:00Z"
}
```

Example return details response:

```json
{
  "returnId": "return_3041",
  "orderId": "order_1041",
  "customerId": "customer_999",
  "supportAgentId": "agent_999",
  "datasetId": "demo",
  "orderAmount": 1168.27,
  "refundAmount": 1019.25,
  "productCategory": "electronics",
  "returnReason": "item_not_as_described",
  "evidenceProvided": false,
  "decision": "APPROVED",
  "manualOverride": true,
  "decisionTimeMinutes": 4,
  "timestamp": "2026-06-17T09:01:00Z",
  "riskScore": 100,
  "riskLevel": "CRITICAL",
  "topReason": "Refund was approved without required evidence",
  "reasons": [
    {
      "type": "NO_EVIDENCE",
      "message": "Refund was approved without required evidence",
      "scoreImpact": 25
    },
    {
      "type": "HIGH_VALUE_REFUND",
      "message": "Refund amount is unusually high compared to order amount",
      "scoreImpact": 20
    },
    {
      "type": "MANUAL_OVERRIDE",
      "message": "Manual override was used for this refund approval",
      "scoreImpact": 20
    },
    {
      "type": "AGENT_HIGH_APPROVAL_RATE",
      "message": "Support agent has unusually high approval rate",
      "scoreImpact": 30
    },
    {
      "type": "CUSTOMER_FREQUENT_RETURNS",
      "message": "Customer has frequent refund requests",
      "scoreImpact": 20
    },
    {
      "type": "REPEATED_AGENT_CUSTOMER_PAIR",
      "message": "Same support agent repeatedly approved refunds for this customer",
      "scoreImpact": 25
    },
    {
      "type": "SUSPICIOUS_CLUSTER",
      "message": "Refund approval belongs to a suspicious relation cluster",
      "scoreImpact": 25
    }
  ],
  "calculatedAt": "2026-06-01T10:15:00Z"
}
```

Example agent risk summary response:

```json
{
  "agentId": "agent_777",
  "suspiciousApprovalsCount": 4,
  "averageRiskScore": 59.0,
  "highRiskApprovalsCount": 1,
  "criticalRiskApprovalsCount": 1,
  "topReason": "Support agent has unusually high approval rate"
}
```

<h2 align="center">Relations Service</h2>

```text
GET /api/relations/health
GET /api/relations/returns/{returnId}
GET /api/relations/returns/{returnId}/features
GET /api/relations/datasets/{datasetId}/returns/{returnId}/features
GET /api/relations/customers/{customerId}/history
GET /api/relations/agents/{agentId}/summary
POST /api/relations/datasets/{datasetId}/rebuild
```

Stable relation feature response for Scoring Service:

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
    "categoryRefundRate": 0.27,
    "refundAmountRatio": 0.87,
    "similarReturnsCount": 0,
    "sameReasonRefundCount": 13,
    "clusterSize": 13,
    "strongestRelationType": "SAME_REASON_PATTERN",
    "topRelatedReturns": [
      "return_3042",
      "return_3043",
      "return_3044",
      "return_3045"
    ]
  }
}
```

Example rebuild response:

```json
{
  "datasetId": "demo",
  "jobId": "relations-job-demo",
  "status": "RELATIONS_REBUILD_STARTED",
  "relationsCount": 315,
  "featuresCount": 45
}
```

<h2 align="center">ML Service</h2>

```text
GET /api/ml/health
GET /api/ml/datasets/{datasetId}/mapping
```

Example mapping response:

```json
{
  "datasetId": "dataset_123",
  "mapping": {
    "buyer_id": "customer_id",
    "purchase_id": "order_id",
    "refund_request_id": "return_id",
    "support_user_id": "support_agent_id",
    "approval_status": "decision"
  }
}
```
