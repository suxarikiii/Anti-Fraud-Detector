<h1 align="center">Scoring Rules</h1>

The MVP uses rule-based scoring to calculate a refund approval risk score.

<h2 align="center">Design Decisions</h2>

| Decision | Rationale |
| --- | --- |
| Rule-based scoring | The MVP needs deterministic and explainable results before real labelled production fraud data is available. |
| Risk levels | Analysts can triage faster with `LOW`, `MEDIUM`, `HIGH`, and `CRITICAL` instead of only a raw score. |
| Explanations | Every score includes analyst-readable reasons so a reviewer can verify the evidence, amount, timing, override, agent behavior, customer behavior, and relation context. |
| CSV-derived fallback | Relation-style features are derived from the demo CSV until persisted relation features are connected to scoring. |

| Rule | Condition | Score Impact |
| --- | --- | --- |
| NO_EVIDENCE | Refund was approved and no evidence was provided | +25 |
| HIGH_VALUE_REFUND | Refund amount is at least `500.00` | +20 |
| FULL_AMOUNT_REFUND | Refund amount is at least `95%` of order amount | +15 |
| FAST_APPROVAL | Refund was approved in `5` minutes or less | +15 |
| MANUAL_OVERRIDE | Manual override was used | +20 |
| AGENT_HIGH_APPROVAL_RATE | Agent has at least `5` decisions and approval rate is above `85%` | +30 |
| CUSTOMER_FREQUENT_RETURNS | Customer has at least `5` return requests in the dataset | +20 |
| REPEATED_AGENT_CUSTOMER_PAIR | Same agent handled at least `3` return requests for the same customer | +25 |
| SUSPICIOUS_CLUSTER | CSV-derived or relation-derived cluster size is at least `5` | +25 |

Rules are evaluated in the table order. That keeps `topReason` and the `reasons`
array stable for the same input. Additive scores are capped at `100`.

<h2 align="center">Risk Levels</h2>

```text
0-30 LOW
31-60 MEDIUM
61-80 HIGH
81-100 CRITICAL
```

<h2 align="center">Feature Source</h2>

Scoring is ready to consume relation-style features:

```text
customerReturnCount
agentApprovalRate
customerAgentPairCount
clusterSize
refundAmountRatio
strongestRelationType
```

In the current MVP, these values are derived from `data/clean_refund_dataset.csv`
inside Scoring Service and are returned with `featureSource: "CSV_DERIVED_FALLBACK"`.
The RabbitMQ event flow remains `refund.relations.built` into scoring and
`refund.scoring.completed` out of scoring.

<h2 align="center">Demo Return IDs</h2>

| Case | returnId | Expected Score / Level | Main Reasons |
| --- | --- | --- | --- |
| Normal / low | `return_3001` | `15 LOW` | `FULL_AMOUNT_REFUND` |
| Medium | `return_303075` | `45 MEDIUM` | `NO_EVIDENCE`, `HIGH_VALUE_REFUND` |
| High | `return_3006` | `75 HIGH` | `NO_EVIDENCE`, `HIGH_VALUE_REFUND`, `AGENT_HIGH_APPROVAL_RATE` |
| Critical | `return_3041` | `100 CRITICAL` | `NO_EVIDENCE`, `HIGH_VALUE_REFUND`, `FAST_APPROVAL`, `MANUAL_OVERRIDE`, relation pattern rules |
