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
  "orderId": "order_9281",
  "customerId": "customer_512",
  "supportAgentId": "agent_777",
  "datasetId": "demo",
  "riskScore": 85,
  "riskLevel": "CRITICAL",
  "topReason": "Refund was approved without required evidence",
  "reasons": [
    {
      "type": "NO_EVIDENCE",
      "message": "Refund was approved without required evidence",
      "scoreImpact": 25
    },
    {
      "type": "FAST_APPROVAL",
      "message": "Refund was approved very quickly",
      "scoreImpact": 15
    }
  ],
  "calculatedAt": "2026-06-01T10:15:00Z"
}
```

Example return details response:

```json
{
  "returnId": "return_3041",
  "orderId": "order_9281",
  "customerId": "customer_512",
  "supportAgentId": "agent_777",
  "datasetId": "demo",
  "orderAmount": 299.99,
  "refundAmount": 289.99,
  "productCategory": "electronics",
  "returnReason": "Item not as described",
  "evidenceProvided": false,
  "decision": "APPROVED",
  "manualOverride": true,
  "decisionTimeMinutes": 2,
  "timestamp": "2026-06-01T10:08:00Z",
  "riskScore": 85,
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
    }
  ],
  "calculatedAt": "2026-06-01T10:15:00Z"
}
```

Example agent risk summary response:

```json
{
  "agentId": "agent_777",
  "suspiciousApprovalsCount": 14,
  "averageRiskScore": 63.4,
  "highRiskApprovalsCount": 5,
  "criticalRiskApprovalsCount": 2,
  "topReason": "Support agent has unusually high approval rate"
}
```

<h2 align="center">Relations Service</h2>

```text
GET /api/relations/health
GET /api/relations/returns/{returnId}
GET /api/relations/customers/{customerId}
GET /api/relations/agents/{agentId}
POST /api/relations/datasets/{datasetId}/rebuild
```

Example relation summary response:

```json
{
  "returnId": "return_123",
  "customerReturnCount": 8,
  "agentApprovalRate": 0.94,
  "customerAgentPairCount": 5,
  "clusterSize": 8,
  "strongestRelationType": "REPEATED_AGENT_CUSTOMER_PAIR"
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
