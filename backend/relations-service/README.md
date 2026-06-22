<h1 align="center">Graph / Refund Relations Service</h1>

<p align="center">
  Relations Service for suspicious refund approval detection in e-commerce support.
</p>

---

<h2 align="center">Week 2 Status</h2>

Week 2 turns the Week 1 skeleton into the first source of relation features for the Scoring Service.

What works now:

* service runs locally and in root `docker-compose.yml`;
* health endpoint is available on `:8082`;
* `/api/relations/returns/{returnId}/features` calculates features from an in-memory normalized test dataset;
* supported demo return IDs: `return_3041`, `return_3006`;
* RabbitMQ consumer stub listens for `dataset.normalized` when `RABBITMQ_ENABLED=true`;
* rebuild flow publishes `refund.relations.built` with `datasetId`, `jobId`, `relationsCount`, and `featuresCount`;
* Graph DB is intentionally left for Week 3 integration.

---

<h2 align="center">Pipeline Position</h2>

```text
dataset.uploaded
  -> normalization service
dataset.normalized
  -> relations-service
refund.relations.built
  -> scoring-service
refund.scoring.completed
  -> dashboard / analysis status
```

---

<h2 align="center">Normalized Input Contract</h2>

Relations Service expects normalized refund records produced by ML / Normalization Service.

Week 2 canonical record shape:

```json
{
  "datasetId": "demo",
  "returnId": "return_3041",
  "customerId": "customer_880",
  "orderId": "order_9101",
  "supportAgentId": "agent_017",
  "productCategory": "electronics",
  "returnReason": "item_not_as_described",
  "decisionId": "decision_7001",
  "decisionStatus": "APPROVED",
  "refundAmount": 420.0,
  "orderAmount": 520.0,
  "manualOverride": true,
  "decisionTimeMs": 3900
}
```

Required fields for Week 2 feature generation:

* `datasetId: string`
* `returnId: string`
* `customerId: string`
* `supportAgentId: string`
* `productCategory: string`
* `returnReason: string`
* `decisionStatus: string`
* `refundAmount: number`
* `orderAmount: number`
* `manualOverride: boolean`

---

<h2 align="center">Feature Schema</h2>

The main Week 2 scoring fields are:

| Field | Type | Meaning |
| --- | --- | --- |
| `customerReturnCount` | integer | Number of return requests for the same `customerId`. |
| `agentApprovalRate` | number | `APPROVED / total decisions` for the same `supportAgentId`. |
| `customerAgentPairCount` | integer | Number of interactions between the same customer and agent. |
| `clusterSize` | integer | Suspicious group size fallback: max of customer, agent, pair, and reason counts. |

Additional prepared fields:

| Field | Type | Meaning |
| --- | --- | --- |
| `customerApprovedRefundCount` | integer | Approved refunds for the same customer. |
| `agentManualOverrideRate` | number | Manual overrides divided by total agent decisions. |
| `agentHighValueApprovalCount` | integer | Approved refunds with amount >= 300 for the same agent. |
| `agentCustomerInteractionCount` | integer | Same value as `customerAgentPairCount` for Scoring compatibility. |
| `categoryRefundRate` | number | Returns in this category divided by all in-memory normalized records. |
| `refundAmountRatio` | number | `refundAmount / orderAmount`. |
| `similarReturnsCount` | integer | Same reason + category + agent, excluding current return. |
| `sameReasonRefundCount` | integer | Records with the same return reason. |
| `strongestRelationType` | string | Dominant relation pattern. |
| `topRelatedReturns` | string array | Related return IDs from the strongest local patterns. |

Example:

```json
{
  "returnId": "return_3041",
  "customerId": "customer_880",
  "supportAgentId": "agent_017",
  "features": {
    "customerReturnCount": 3,
    "customerApprovedRefundCount": 2,
    "agentApprovalRate": 0.75,
    "agentManualOverrideRate": 0.5,
    "agentHighValueApprovalCount": 2,
    "customerAgentPairCount": 2,
    "agentCustomerInteractionCount": 2,
    "categoryRefundRate": 0.8,
    "refundAmountRatio": 0.81,
    "similarReturnsCount": 2,
    "sameReasonRefundCount": 3,
    "clusterSize": 4,
    "strongestRelationType": "AGENT_DECISION_PATTERN",
    "topRelatedReturns": ["return_3006", "return_3110"]
  }
}
```

---

<h2 align="center">REST API</h2>

Run locally:

```bash
cd backend/relations-service
go run cmd/main.go
```

Run through compose:

```bash
docker compose up --build relations-service
```

Endpoints:

```text
GET  /api/relations/health
POST /api/relations/datasets/{datasetId}/rebuild
GET  /api/relations/returns/{returnId}
GET  /api/relations/customers/{customerId}/history
GET  /api/relations/agents/{agentId}/summary
GET  /api/relations/returns/{returnId}/features
```

Week 2 demo:

```bash
curl http://localhost:8082/api/relations/health
curl http://localhost:8082/api/relations/returns/return_3041/features
curl http://localhost:8082/api/relations/returns/return_3006/features
curl -X POST http://localhost:8082/api/relations/datasets/demo/rebuild
```

---

<h2 align="center">RabbitMQ Contract</h2>

RabbitMQ is optional for local REST-only development.

Enable RabbitMQ:

```bash
RABBITMQ_ENABLED=true go run cmd/main.go
```

Environment variables:

```text
SERVER_PORT=:8082
RABBITMQ_ENABLED=false
RABBITMQ_URL=amqp://guest:guest@localhost:5672/
RABBITMQ_EXCHANGE=pipeline.exchange
RABBITMQ_NORMALIZED_QUEUE=relations.dataset-normalized.queue
RABBITMQ_NORMALIZED_ROUTING_KEY=dataset.normalized
RABBITMQ_RELATIONS_BUILT_ROUTING_KEY=refund.relations.built
```

Input event:

```json
{
  "datasetId": "demo",
  "jobId": "job_123",
  "recordsPath": "normalized/demo.json",
  "publishedAt": "2026-06-22T10:00:00Z"
}
```

Output event:

```json
{
  "datasetId": "demo",
  "jobId": "job_123",
  "relationsCount": 35,
  "featuresCount": 5,
  "publishedAt": "2026-06-22T10:00:05Z"
}
```

---

<h2 align="center">Graph Model</h2>

Vertices:

* `Customer`
* `Order`
* `ReturnRequest`
* `SupportAgent`
* `ProductCategory`
* `Decision`
* `DeliveryAddress`, optional
* `PaymentMethod`, optional

Edges:

```text
Customer --PLACED_ORDER--> Order
Customer --REQUESTED_RETURN--> ReturnRequest
Order --HAS_RETURN_REQUEST--> ReturnRequest
ReturnRequest --DECIDED_BY--> SupportAgent
Order --HAS_CATEGORY--> ProductCategory
SupportAgent --MADE_DECISION--> Decision
Decision --APPROVED_RETURN--> ReturnRequest
Decision --DECLINED_RETURN--> ReturnRequest
Customer --USES_ADDRESS--> DeliveryAddress
Customer --USES_PAYMENT_METHOD--> PaymentMethod
Customer --REPEATED_REFUND_PATTERN--> Customer
SupportAgent --REPEATED_APPROVAL_PATTERN--> Customer
```

---

<h2 align="center">Week 3 Integration</h2>

Graph DB is not connected in Week 2.

Week 3 should replace or extend the in-memory builder with:

1. loading normalized records from PostgreSQL / object storage;
2. writing vertices and edges to Graph DB;
3. calculating cluster features from graph traversal;
4. storing relation features for Scoring Service reuse.
