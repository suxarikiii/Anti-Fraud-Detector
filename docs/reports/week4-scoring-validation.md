# Week 4 Scoring Validation Report

## Scope

This report covers the Week 4 Scoring Service stabilization work for refund approval risk scoring.

Implemented scope:

* deterministic rule-based scoring with capped `0-100` scores;
* required risk level boundaries: `LOW`, `MEDIUM`, `HIGH`, `CRITICAL`;
* human-readable risk explanations with numeric context;
* dataset-scoped suspicious approvals, return details, and agent summary contracts;
* JSON error responses for unknown datasets and returns;
* CSV-derived fallback for relation-style features;
* unit and integration tests for scoring rules and HTTP contracts.

## Relation Feature Status

The Scoring Service consumes the `refund.relations.built` event and publishes `refund.scoring.completed`.

Current MVP behavior:

* relation feature names are available in scoring responses;
* feature values are derived from `data/clean_refund_dataset.csv`;
* responses mark this mode with `featureSource: "CSV_DERIVED_FALLBACK"`;
* full persisted relation-feature handoff from Relations Service remains a follow-up integration point.

Feature names prepared for scoring:

```text
customerReturnCount
agentApprovalRate
customerAgentPairCount
clusterSize
refundAmountRatio
strongestRelationType
```

## Demo Cases

| returnId | Expected Level | Main Reasons | Notes |
| --- | --- | --- | --- |
| `return_300347` | LOW | none expected | Normal approval with evidence and no repeated relation pattern. |
| `return_303075` | MEDIUM | `NO_EVIDENCE`, `HIGH_VALUE_REFUND` | High-value refund approved without evidence. |
| `return_3011` | CRITICAL | `NO_EVIDENCE`, `HIGH_VALUE_REFUND`, `FULL_AMOUNT_REFUND` | Full refund of a high-value order without evidence. |
| `return_3041` | CRITICAL | `NO_EVIDENCE`, `HIGH_VALUE_REFUND`, `FAST_APPROVAL`, `MANUAL_OVERRIDE`, relation pattern rules | Repeated customer-agent pattern with manual overrides and fast approvals. |

## Smoke Commands

Direct service:

```bash
curl http://localhost:8083/api/scoring/health
curl http://localhost:8083/api/scoring/datasets/demo/suspicious-approvals
curl http://localhost:8083/api/scoring/datasets/demo/returns/return_3041/details
curl http://localhost:8083/api/scoring/datasets/demo/agents/agent_999/risk-summary
```

Gateway:

```bash
curl http://localhost:8080/api/scoring/health
curl http://localhost:8080/api/scoring/datasets/demo/suspicious-approvals
curl http://localhost:8080/api/scoring/datasets/demo/returns/return_3041/details
curl http://localhost:8080/api/scoring/datasets/demo/agents/agent_999/risk-summary
```

## Local Verification

Executed locally on 2026-06-29:

```bash
cd backend/scoring-service
./gradlew test
./gradlew clean build

cd ../..
docker compose build scoring-service
docker compose up -d scoring-service
curl http://127.0.0.1:8083/api/scoring/health
curl http://127.0.0.1:8083/api/scoring/datasets/demo/suspicious-approvals
curl http://127.0.0.1:8083/api/scoring/datasets/demo/returns/return_3041/details
curl http://127.0.0.1:8083/api/scoring/datasets/demo/agents/agent_999/risk-summary
```

Result: passed.

Smoke highlights:

* health returned `{"status":"UP","service":"scoring-service"}`;
* `return_3041` returned `riskScore: 100`, `riskLevel: "CRITICAL"`, and relation fallback fields;
* `agent_999` returned `totalReturns: 5`, `suspiciousApprovalsCount: 5`, and `criticalRiskCount: 5`;
* suspicious approvals returned a non-empty JSON array sorted by descending score.

Pending/manual validation:

* gateway smoke commands against a running compose stack;
* user feedback session notes or survey results.
