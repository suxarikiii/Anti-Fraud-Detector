<h1 align="center">Graph / Refund Relations Service</h1>

<p align="center">
  Relations Service for suspicious refund approval detection in e-commerce support.
</p>

---

<h2 align="center">Week 4 Status</h2>

Week 4 stabilizes the relation feature contract for Scoring Service and frontend integration.

What works now:

* service runs locally and in root `docker-compose.yml`;
* health endpoint is available on `:8082`;
* `/api/relations/returns/{returnId}/features` calculates features from normalized refund records;
* `/api/relations/datasets/{datasetId}/returns/{returnId}/features` provides a dataset-aware feature endpoint;
* all demo return IDs from `data/clean_refund_dataset.csv` are supported: `return_3001` through `return_3045`;
* RabbitMQ consumer stub listens for `dataset.normalized` when `RABBITMQ_ENABLED=true`;
* rebuild flow publishes `refund.relations.built` with `datasetId`, `jobId`, `relationsCount`, and `featuresCount`;
* Graph DB is intentionally left for future integration.

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

Week 4 canonical record shape:

```json
{
  "datasetId": "demo",
  "returnId": "return_3041",
  "customerId": "customer_999",
  "orderId": "order_9101",
  "supportAgentId": "agent_999",
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

Required fields for feature generation:

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

The main scoring fields are:

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
    "topRelatedReturns": ["return_3042", "return_3043", "return_3044", "return_3045"]
  }
}
```

---

<h2 align="center">Running the Relations Service</h2>

Prerequisites:

* Go;
* RabbitMQ if `RABBITMQ_ENABLED=true`;
* free local port `8082`.

If RabbitMQ is not already running, start the shared local broker from the upload-service compose file:

```bash
cd backend/upload-service
docker compose up -d rabbitmq
```

Run locally without RabbitMQ:

```bash
cd backend/relations-service
go run cmd/main.go
```

Run locally with RabbitMQ enabled:

```bash
cd backend/relations-service
SERVER_PORT=:8082 \
RABBITMQ_ENABLED=true \
RABBITMQ_URL=amqp://guest:guest@localhost:5672/ \
RABBITMQ_EXCHANGE=pipeline.exchange \
RABBITMQ_NORMALIZED_QUEUE=relations.dataset-normalized.queue \
RABBITMQ_NORMALIZED_ROUTING_KEY=dataset.normalized \
RABBITMQ_RELATIONS_BUILT_ROUTING_KEY=refund.relations.built \
go run cmd/main.go
```

Default port: `:8082`.

Health check:

```bash
curl http://localhost:8082/api/relations/health
```

Rebuild relations for a dataset:

```bash
curl -X POST http://localhost:8082/api/relations/datasets/demo/rebuild
```

Example relation feature request:

```bash
curl http://localhost:8082/api/relations/returns/return_3041/features
```

<h2 align="center">REST API</h2>

Endpoints:

```text
GET  /api/relations/health
POST /api/relations/datasets/{datasetId}/rebuild
GET  /api/relations/returns/{returnId}
GET  /api/relations/customers/{customerId}/history
GET  /api/relations/agents/{agentId}/summary
GET  /api/relations/returns/{returnId}/features
```

Week 4 smoke demo:

```bash
curl http://localhost:8082/api/relations/health
curl http://localhost:8082/api/relations/returns/return_3041/features
curl http://localhost:8082/api/relations/datasets/demo/returns/return_3041/features
curl http://localhost:8082/api/relations/returns/return_3006/features
curl -X POST http://localhost:8082/api/relations/datasets/demo/rebuild
```

Unknown return IDs return `404 Not Found`.

Sample response artifact:

```text
backend/relations-service/testdata/return_3041_features.json
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
  "relationsCount": 315,
  "featuresCount": 45,
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

<h2 align="center">Relations-to-Scoring Integration</h2>

Scoring Service can map the response from:

```text
GET /api/relations/datasets/{datasetId}/returns/{returnId}/features
```

The stable fields for scoring are:

```text
customerReturnCount
agentApprovalRate
customerAgentPairCount
clusterSize
refundAmountRatio
strongestRelationType
topRelatedReturns
similarReturnsCount
sameReasonRefundCount
```

Demo high-risk case:

```text
return_3041
```

Why it is useful for investigation:

* `customerReturnCount` shows frequent returns by the same customer;
* `customerAgentPairCount` shows repeated customer-agent interaction;
* `agentApprovalRate` shows unusual support agent approval behavior;
* `strongestRelationType` and `topRelatedReturns` explain the relation pattern to an analyst.

RabbitMQ integration is available as a local/demo stub through `POST /api/relations/datasets/{datasetId}/rebuild` and the `refund.relations.built` publisher. Full end-to-end scoring consumption remains a team integration point with Scoring Service.

---

<h2 align="center">Graph DB Integration</h2>

Graph DB is not connected in Week 4.

Future work should replace or extend the CSV/in-memory builder with:

1. loading normalized records from PostgreSQL / object storage;
2. writing vertices and edges to Graph DB;
3. calculating cluster features from graph traversal;
4. storing relation features for Scoring Service reuse.
