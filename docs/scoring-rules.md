# Scoring rules — Week 6 RC

Scoring is deterministic and additive. Reasons are evaluated in the order below and the final score is capped at `100`.

| Rule | Condition | Impact |
| --- | --- | ---: |
| `NO_EVIDENCE` | Approved refund without evidence | +25 |
| `HIGH_VALUE_REFUND` | Refund amount >= `500.00` | +20 |
| `FULL_AMOUNT_REFUND` | Refund/order ratio >= `0.95` | +15 |
| `FAST_APPROVAL` | Approved in <= `5` minutes | +15 |
| `MANUAL_OVERRIDE` | Manual override used | +20 |
| `AGENT_HIGH_APPROVAL_RATE` | At least `5` agent decisions and approval rate > `0.85` | +30 |
| `CUSTOMER_FREQUENT_RETURNS` | At least `5` customer returns in this dataset | +20 |
| `REPEATED_AGENT_CUSTOMER_PAIR` | Pair count >= `3` in this dataset | +25 |
| `SUSPICIOUS_CLUSTER` | Relation cluster size >= `5` | +25 |

Risk boundaries are exact:

```text
0..30   LOW
31..60  MEDIUM
61..80  HIGH
81..100 CRITICAL
```

## Feature provenance

Production requests use the dataset-scoped Relations snapshot and return `RELATIONS_SERVICE`. The stored result keeps the source, `featureVersion`, calculation version and calculation timestamp.

`DEMO_CSV` is an explicit test/demo provider guarded by `SCORING_DEMO_ENABLED=true`. There is no UUID-to-demo fallback and no `CSV_DERIVED_FALLBACK` production value.

## Stable fixtures

| Level | Fixture | Expected |
| --- | --- | --- |
| LOW | `return_3001` | `15 LOW` |
| MEDIUM | `return_303075` | `45 MEDIUM` |
| HIGH | `return_3006` | `75 HIGH` |
| CRITICAL | `return_3041` | `100 CRITICAL` |

The fixture expectations are asserted by the mandatory Gradle test job.
