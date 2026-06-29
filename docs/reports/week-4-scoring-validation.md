# Week 4 Scoring Validation Report

## Scope

* Scoring Service stabilization
* Risk scores and explanations
* Unit and integration tests
* Relations-to-scoring integration fallback
* Upload-to-normalization validation status
* Docker and smoke testing evidence

## Related Issues

* #74
* #75
* #77
* #87
* #38

## Implemented

* Added deterministic refund approval scoring through `RiskRuleEngine`.
* Added human-readable explanations with numeric context through `ExplanationBuilder`.
* Split CSV-derived relation-style feature calculation into `CsvDerivedFeatureProvider`.
* Preserved dataset-scoped frontend endpoints and older demo-compatible endpoints.
* Added clean JSON `404` responses for unknown datasets and returns.
* Added `relationFeatures` to return details with `featureSource: "CSV_DERIVED_FALLBACK"`.
* Added tests for scoring rules, risk boundaries, endpoint contracts, sorting, relation fallback fields, and error responses.
* Documented demo return IDs, scoring rules, API response fields, smoke commands, and known limitations.
* Fixed compose parsing by removing the duplicate `RABBITMQ_EXCHANGE` entry.
* Stabilized RabbitMQ startup by declaring the scoring queue/binding and making missing queues non-fatal while the broker initializes.

## API Contract

Stable dataset-scoped endpoints:

```text
GET /api/scoring/health
GET /api/scoring/datasets/{datasetId}/suspicious-approvals
GET /api/scoring/datasets/{datasetId}/returns/{returnId}/risk
GET /api/scoring/datasets/{datasetId}/returns/{returnId}/details
GET /api/scoring/datasets/{datasetId}/agents/{agentId}/risk-summary
POST /api/scoring/datasets/{datasetId}/recalculate
```

Compatibility endpoints:

```text
GET /api/scoring/returns/{returnId}/risk
GET /api/scoring/agents/{agentId}/risk-summary
```

Required response fields verified by tests or smoke checks:

```text
riskScore
riskLevel
topReason
reasons
calculatedAt
```

`GET /api/scoring/datasets/{datasetId}/returns/{returnId}/details` also returns original refund fields and:

```text
relationFeatures.customerReturnCount
relationFeatures.agentApprovalRate
relationFeatures.customerAgentPairCount
relationFeatures.clusterSize
relationFeatures.refundAmountRatio
relationFeatures.strongestRelationType
relationFeatures.featureSource
```

## Demo Return IDs

| returnId | Expected Level | Main Reasons | Why Flagged |
| --- | --- | --- | --- |
| `return_300347` | LOW | none expected | Normal approval with evidence and no repeated relation pattern. |
| `return_303075` | MEDIUM | `NO_EVIDENCE`, `HIGH_VALUE_REFUND` | High-value refund approved without evidence. |
| `return_3011` | CRITICAL | `NO_EVIDENCE`, `HIGH_VALUE_REFUND`, `FULL_AMOUNT_REFUND` | Full refund of a high-value order without evidence. |
| `return_3041` | CRITICAL | `NO_EVIDENCE`, `HIGH_VALUE_REFUND`, `FAST_APPROVAL`, `MANUAL_OVERRIDE`, relation pattern rules | Repeated customer-agent pattern with manual overrides and fast approvals. |

## Test Coverage

Unit tests cover:

* `NO_EVIDENCE`
* `HIGH_VALUE_REFUND`
* `FULL_AMOUNT_REFUND`
* `FAST_APPROVAL`
* `MANUAL_OVERRIDE`
* `AGENT_HIGH_APPROVAL_RATE`
* `CUSTOMER_FREQUENT_RETURNS`
* `REPEATED_AGENT_CUSTOMER_PAIR`
* `SUSPICIOUS_CLUSTER`
* risk boundaries from `LOW` through `CRITICAL`
* score capping at `100`

Controller/integration tests cover:

* suspicious approvals endpoint
* return risk endpoint
* return details endpoint
* agent risk summary endpoint
* clean JSON error response for unknown dataset
* clean JSON error response for unknown return

## Smoke Test Commands

Direct service:

```bash
curl http://localhost:8083/api/scoring/health
curl http://localhost:8083/api/scoring/datasets/demo/suspicious-approvals
curl http://localhost:8083/api/scoring/datasets/demo/returns/return_3041/details
curl http://localhost:8083/api/scoring/datasets/demo/agents/agent_999/risk-summary
```

Gateway, when the root compose stack is running:

```bash
curl http://localhost:8080/api/scoring/health
curl http://localhost:8080/api/scoring/datasets/demo/suspicious-approvals
curl http://localhost:8080/api/scoring/datasets/demo/returns/return_3041/details
curl http://localhost:8080/api/scoring/datasets/demo/agents/agent_999/risk-summary
```

## Relations Integration Status

Relations-to-scoring is MVP fallback-based.

Current behavior:

* Scoring consumes `refund.relations.built`.
* Scoring publishes `refund.scoring.completed`.
* Scoring publishes `pipeline.failed` on processing errors.
* Relation-style feature names are mirrored in scoring responses.
* Feature values are derived from `data/clean_refund_dataset.csv`.
* Responses mark fallback mode with `featureSource: "CSV_DERIVED_FALLBACK"`.

Full persisted relation-feature handoff from Relations Service to Scoring Service remains a follow-up integration item.

## Upload-to-Normalization Status

Scoring currently expects normalized CSV columns equivalent to:

```text
order_id
customer_id
return_id
support_agent_id
order_amount
refund_amount
product_category
return_reason
evidence_provided
decision
manual_override
decision_time_minutes
timestamp
```

API DTOs expose camelCase equivalents such as `returnId`, `supportAgentId`, `orderAmount`, `refundAmount`, `evidenceProvided`, `manualOverride`, and `decisionTimeMinutes`.

When uploaded normalized records are not yet available to Scoring Service, uploaded UUID dataset IDs use the CSV-derived demo fallback. Unknown literal dataset IDs return a clean JSON `404`.

## Local Verification

Executed locally on 2026-06-29:

```bash
cd backend/scoring-service
./gradlew clean build

cd ../..
docker compose build scoring-service
docker compose up -d scoring-service
docker exec anti-frod-detector-rabbitmq-1 rabbitmqctl list_queues name
curl http://127.0.0.1:8083/api/scoring/health
curl http://127.0.0.1:8083/api/scoring/datasets/demo/suspicious-approvals
curl http://127.0.0.1:8083/api/scoring/datasets/demo/returns/return_3041/details
curl http://127.0.0.1:8083/api/scoring/datasets/demo/agents/agent_999/risk-summary

docker compose up -d gateway
curl http://127.0.0.1:8080/api/scoring/health
curl http://127.0.0.1:8080/api/scoring/datasets/demo/suspicious-approvals
curl http://127.0.0.1:8080/api/scoring/datasets/demo/returns/return_3041/details
curl http://127.0.0.1:8080/api/scoring/datasets/demo/agents/agent_999/risk-summary
```

Result:

```text
BUILD SUCCESSFUL
scoring.refund-relations-built.queue
health: {"status":"UP","service":"scoring-service"}
suspicious approvals: count 4798, sorted True, required_fields True
return_3041 details: riskScore 100, riskLevel CRITICAL, featureSource CSV_DERIVED_FALLBACK
agent_999 summary: totalReturns 5, suspiciousApprovalsCount 5, criticalRiskCount 5
gateway scoring health: {"status":"UP","service":"scoring-service"}
gateway suspicious approvals: count 4798, sorted True, required_fields True
gateway return_3041 details: riskScore 100, riskLevel CRITICAL, featureSource CSV_DERIVED_FALLBACK
gateway agent_999 summary: totalReturns 5, suspiciousApprovalsCount 5, criticalRiskCount 5
```

Local RabbitMQ note: the development broker had an old `pipeline.exchange` with type `direct`. The current project services declare `pipeline.exchange` as `topic`, so the stale local exchange was removed through the RabbitMQ management API before gateway smoke checks.

## Known Limitations

* Scoring does not yet persist calculated scores in a database.
* Full relation-feature storage and handoff from Relations Service is not implemented.
* Uploaded UUID dataset IDs use CSV-derived fallback data until normalized dataset storage is connected.
* User feedback session notes are pending manual validation.

## Next Steps

* Connect persisted normalized dataset records to Scoring Service.
* Replace CSV-derived relation fallback with relation-feature reads when storage is available.
* Add CI evidence from the repository workflow after the branch is pushed.
* Attach user feedback notes or survey results after validation sessions.
