<h1 align="center">Graph / Refund Relations Service</h1>

<p align="center">
  Relations Service for suspicious refund approval detection in e-commerce support.
</p>

---

<h2 align="center">Current Status</h2>

The current MVP stabilizes the relation feature contract for Scoring Service and frontend integration, and now keeps relation features scoped by uploaded dataset.

What works now:

* service runs locally and in root `docker-compose.yml`;
* health endpoint is available on `:8082`;
* `/api/relations/returns/{returnId}/features` calculates demo-compatible features from normalized refund records;
* `/api/relations/datasets/{datasetId}/returns/{returnId}/features` returns cached dataset-scoped features by `datasetId + returnId`;
* dataset-aware customer history, customer summary, support-agent summary, ranked-agent, related-return, and graph projection endpoints are available;
* return IDs from `data/clean_refund_dataset.csv` are supported for the local demo dataset;
* RabbitMQ consumer listens for `dataset.normalized` when `RABBITMQ_ENABLED=true`;
* normalized dataset artifacts are loaded from `recordsPath` (`file://...` or local path), validated, and atomically stored per dataset;
* rebuild and ingestion publish `refund.relations.built` with dataset, record, relation, feature, schema, and version metadata;
* failed ingestion publishes `pipeline.failed` with stage and error context;
* dedicated Graph DB storage is not connected and remains future/optional MVP work.

---

<h2 align="center">Pipeline Position</h2>

```text
dataset.uploaded
  -> planned normalization stage
dataset.normalized
  -> relations-service
refund.relations.built
  -> scoring-service
refund.scoring.completed
  -> dashboard / analysis status
```

---

<h2 align="center">Normalized Input Contract</h2>

Relations Service expects normalized refund records. In Compose it loads the prepared demo CSV from `/data/clean_refund_dataset.csv`. In the pipeline it consumes `dataset.normalized` and loads the normalized artifact referenced by `recordsPath`.

Canonical record shape:

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
  "decisionTimeMinutes": 4
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
* `decisionTimeMinutes: integer`

The CSV artifact must contain these columns:

```text
order_id,customer_id,return_id,support_agent_id,order_amount,refund_amount,product_category,return_reason,decision,manual_override,decision_time_minutes
```

Validation rejects empty datasets, missing required columns, duplicate `return_id` values, mismatched dataset IDs, and record-count mismatches when `recordCount` is provided by the event. A failed dataset is not stored.

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
| `sameReasonRefundCount` | integer | Records with the same return reason handled by the same support agent. |
| `strongestRelationType` | string | Dominant relation pattern. |
| `topRelatedReturns` | string array | Related return IDs from the strongest local patterns. |
| `explanationSummary` | string | Short business-readable explanation of the strongest signal. |
| `explanationSignals` | string array | Human-readable investigation signals for scoring details and frontend pages. |

Example:

```json
{
  "returnId": "return_3041",
  "datasetId": "demo",
  "customerId": "customer_999",
  "supportAgentId": "agent_999",
  "featureVersion": 1780000000000000000,
  "calculatedAt": "2026-06-28T16:27:46Z",
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
RELATIONS_DATASET_ID=demo \
RELATIONS_DATASET_PATH=../../data/clean_refund_dataset.csv \
RELATIONS_DEMO_FALLBACK_ENABLED=false \
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
RELATIONS_DATASET_ID=demo \
RELATIONS_DATASET_PATH=../../data/clean_refund_dataset.csv \
RELATIONS_DEMO_FALLBACK_ENABLED=false \
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
GET  /api/relations/datasets/{datasetId}/returns/{returnId}
GET  /api/relations/datasets/{datasetId}/returns/{returnId}/features
GET  /api/relations/datasets/{datasetId}/returns/{returnId}/related?limit=8
GET  /api/relations/datasets/{datasetId}/returns/{returnId}/graph?limit=24
GET  /api/relations/datasets/{datasetId}/customers/{customerId}/history
GET  /api/relations/datasets/{datasetId}/customers/{customerId}/summary?limit=10
GET  /api/relations/datasets/{datasetId}/agents/{agentId}/summary
GET  /api/relations/datasets/{datasetId}/agents/ranked?limit=10&sort=averageClusterSize
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
curl http://localhost:8082/api/relations/datasets/demo/returns/return_3041/related
curl http://localhost:8082/api/relations/datasets/demo/returns/return_3041/graph
curl http://localhost:8082/api/relations/datasets/demo/customers/customer_999/summary
curl http://localhost:8082/api/relations/datasets/demo/agents/ranked?limit=5
curl http://localhost:8082/api/relations/returns/return_3006/features
curl -X POST http://localhost:8082/api/relations/datasets/demo/rebuild
```

Week 5 product demo cases:

```bash
# suspicious cluster: frequent customer returns + repeated customer-agent pair
curl http://localhost:8082/api/relations/datasets/demo/returns/return_3041/features

# repeated customer-agent approval pattern
curl http://localhost:8082/api/relations/datasets/demo/returns/return_3036/features

# high-value approval without evidence, explained mostly through agent behavior
curl http://localhost:8082/api/relations/datasets/demo/returns/return_3006/features
```

Unknown return IDs return `404 Not Found`.

Sample response artifact:

```text
backend/relations-service/testdata/return_3041_features.json
```

---

<h2 align="center">RabbitMQ Contract</h2>

RabbitMQ pipeline exchange: `pipeline.exchange`. RabbitMQ is optional for local REST-only development.

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
RELATIONS_DATASET_ID=demo
RELATIONS_DATASET_PATH=../../data/clean_refund_dataset.csv
RELATIONS_DEMO_FALLBACK_ENABLED=false
```

Input event:

```json
{
  "datasetId": "demo",
  "jobId": "job_123",
  "recordsPath": "file:///data/clean_refund_dataset.csv",
  "recordCount": 45,
  "schemaVersion": "refund-normalized.v1",
  "publishedAt": "2026-06-22T10:00:00Z"
}
```

Output event:

```json
{
  "datasetId": "demo",
  "jobId": "job_123",
  "recordsPath": "file:///data/clean_refund_dataset.csv",
  "recordsCount": 45,
  "relationsCount": 315,
  "featuresCount": 45,
  "schemaVersion": "refund-normalized.v1",
  "featureVersion": 1780000000000000000,
  "publishedAt": "2026-06-22T10:00:05Z"
}
```

Failure event:

```json
{
  "datasetId": "demo",
  "jobId": "job_123",
  "stage": "RELATIONS",
  "message": "invalid normalized dataset: duplicate returnId return_3041 in dataset demo",
  "failedAt": "2026-06-22T10:00:05Z"
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
* `strongestRelationType`, `topRelatedReturns`, `explanationSummary`, and `explanationSignals` explain the relation pattern to an analyst.

---

<h2 align="center">Design Decision</h2>

Relations Service is separate from Scoring Service because it answers a different product question.

Scoring decides how risky a refund approval is. Relations explains why the case is connected to suspicious behavior: repeated customer returns, repeated customer-agent interactions, agent-level approval patterns, similar returns, and related return IDs for investigation.

Keeping this layer separate makes the MVP easier to evolve:

* Scoring can change risk weights without rebuilding graph/relation extraction.
* Frontend can show relation context on the refund details page without duplicating scoring rules.
* Future Graph DB traversal can replace the current CSV/in-memory implementation behind the same API contract.

---

<h2 align="center">Feedback-Driven Refinement</h2>

Week 5 focuses on making features understandable for business users, not only technically available.

Prioritized changes:

* scoped `sameReasonRefundCount` to the same support agent so common global reasons do not dominate explanations;
* kept `clusterSize` as a stable fallback over local relation counts;
* added `explanationSummary` and `explanationSignals` for scoring explanations and frontend details;
* documented demo cases and smoke commands for report evidence.

RabbitMQ integration is available as a local/demo stub through `POST /api/relations/datasets/{datasetId}/rebuild` and the `refund.relations.built` publisher. Full end-to-end scoring consumption remains a team integration point with Scoring Service.

---

<h2 align="center">Graph DB Status</h2>

Relations Service currently computes graph-style relation features through service logic and API contracts. Dedicated Graph DB storage is not connected yet and remains optional/future work for the MVP.

Future work should replace or extend the CSV/in-memory builder with:

1. loading normalized records from PostgreSQL / object storage;
2. writing vertices and edges to Graph DB;
3. calculating cluster features from graph traversal;
4. storing relation features for Scoring Service reuse.
