<h1 align="center">Graph / Refund Relations Service</h1>

<p align="center">
  Graph / Relations Service for suspicious refund approval detection in e-commerce support.
</p>

---

<h2 align="center">Purpose</h2>

The service builds refund-domain relations between customers, orders, return requests, support agents, product categories, and support decisions.

It prepares relation features for the scoring service so suspicious refund approvals can be explained in the B2B analyst dashboard.

Pipeline position:

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

The service is responsible for:

* Graph DB integration;
* refund relations graph;
* entities: Customer, Order, ReturnRequest, SupportAgent, ProductCategory, Decision;
* relation features for scoring;
* consuming `dataset.normalized`;
* publishing `refund.relations.built`.

---

<h2 align="center">Domain Graph Model</h2>

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
Order --CONTAINS_CATEGORY--> ProductCategory
SupportAgent --MADE_DECISION--> Decision
Decision --APPROVED_RETURN--> ReturnRequest
SupportAgent --APPROVED_RETURN--> ReturnRequest
SupportAgent --DECLINED_RETURN--> ReturnRequest
Customer --USES_ADDRESS--> DeliveryAddress
Customer --USES_PAYMENT_METHOD--> PaymentMethod
Customer --REPEATED_REFUND_PATTERN--> Customer
SupportAgent --REPEATED_APPROVAL_PATTERN--> Customer
```

---

<h2 align="center">Storage Approach</h2>

For the Week 1 skeleton, endpoints return mock data and the storage layer is intentionally not connected yet.

Recommended MVP approach:

* PostgreSQL stores normalized records and calculated relation features.
* Graph DB stores vertices and edges when the pipeline needs deeper cluster traversal.
* REST and RabbitMQ contracts stay stable, so Neo4j or ArangoDB can be added behind the service without changing frontend or scoring-service contracts.

This hybrid approach keeps the first MVP simple while preserving a clear path to real graph queries.

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

Example features response:

```json
{
  "returnId": "return_123",
  "customerId": "customer_789",
  "supportAgentId": "agent_001",
  "features": {
    "customerReturnCount": 8,
    "customerApprovedRefundCount": 6,
    "agentApprovalRate": 0.91,
    "agentHighValueApprovalCount": 14,
    "customerAgentPairCount": 5,
    "categoryRefundRate": 0.27,
    "similarReturnsCount": 4,
    "clusterSize": 9
  }
}
```

---

<h2 align="center">RabbitMQ Behavior</h2>

RabbitMQ is disabled by default so mock REST endpoints can run without infrastructure.

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

Current skeleton behavior:

1. Consume `dataset.normalized`.
2. Decode `datasetId` and `jobId`.
3. Pretend to build refund relations and features.
4. Publish `refund.relations.built`.

Planned real behavior:

1. Load normalized records by `datasetId`.
2. Build graph vertices and edges.
3. Calculate relation features.
4. Save features to PostgreSQL or Graph DB.
5. Publish `refund.relations.built`.

---

<h2 align="center">Scoring Features Contract</h2>

The service prepares:

* `customerReturnCount`
* `customerApprovedRefundCount`
* `agentApprovalRate`
* `agentHighValueApprovalCount`
* `customerAgentPairCount`
* `categoryRefundRate`
* `similarReturnsCount`
* `clusterSize`

Additional planned graph-derived features:

* `agentManualOverrideRate`
* `agentCustomerInteractionCount`
* `refundAmountRatio`
* `sameReasonRefundCount`
* `strongestRelationType`

These features are used by scoring-service to calculate refund approval risk and produce human-readable explanations.
