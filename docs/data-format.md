# Data Format

The MVP uses refund approval rows. Clean datasets use normalized column names; dirty datasets can use business aliases and must be mapped into the same shape.

## Clean CSV

```csv
order_id,customer_id,return_id,support_agent_id,order_amount,refund_amount,product_category,return_reason,evidence_provided,decision,manual_override,decision_time_minutes,timestamp
```

| Column | Type | Meaning |
| --- | --- | --- |
| `order_id` | string | Order identifier. |
| `customer_id` | string | Customer who requested the refund. |
| `return_id` | string | Refund/return request identifier. |
| `support_agent_id` | string | Agent who made the decision. |
| `order_amount` | number | Original order amount. |
| `refund_amount` | number | Requested or approved refund amount. |
| `product_category` | string | Product group. |
| `return_reason` | string | Customer reason. |
| `evidence_provided` | boolean | Proof/photo/document was provided. |
| `decision` | enum | `APPROVED` or `DECLINED`. |
| `manual_override` | boolean | Agent bypassed standard rules. |
| `decision_time_minutes` | integer | Minutes until decision. |
| `timestamp` | ISO string | Event timestamp. |

## Dirty CSV Mapping

| Raw aliases | Normalized column |
| --- | --- |
| `customer_id`, `client_id`, `buyer_id` | `customer_id` |
| `order_id`, `purchase_id` | `order_id` |
| `return_id`, `refund_request_id` | `return_id` |
| `agent_id`, `support_user_id` | `support_agent_id` |
| `refund_amount`, `return_amount` | `refund_amount` |
| `order_amount`, `purchase_amount` | `order_amount` |
| `category`, `product_category` | `product_category` |
| `reason`, `return_reason` | `return_reason` |
| `evidence`, `has_photo`, `proof_provided` | `evidence_provided` |
| `decision`, `status`, `approval_status` | `decision` |
| `manual_override`, `override` | `manual_override` |
| `resolution_minutes`, `decision_time_minutes` | `decision_time_minutes` |
| `created_at`, `decision_time`, `timestamp` | `timestamp` |

Value normalization:

| Raw value | Normalized |
| --- | --- |
| `yes`, `true`, `1` | `true` |
| `no`, `false`, `0` | `false` |
| `approved`, `approve` | `APPROVED` |
| `declined`, `reject` | `DECLINED` |
| `2026-06-01 09:08:00` | `2026-06-01T09:08:00Z` |

## Normalized JSON

```json
{
  "orderId": "order_456",
  "customerId": "customer_789",
  "returnId": "return_123",
  "supportAgentId": "agent_001",
  "orderAmount": 249.99,
  "refundAmount": 249.99,
  "productCategory": "electronics",
  "returnReason": "item_not_as_described",
  "evidenceProvided": false,
  "decision": "APPROVED",
  "manualOverride": true,
  "decisionTimeMinutes": 2,
  "timestamp": "2026-06-01T10:00:00Z"
}
```

## Dataset Files

| File | Purpose |
| --- | --- |
| `data/clean_refund_dataset.csv` | Backend-ready normalized dataset. |
| `data/dataset_labels.csv` | Ground truth for validation only. |
| `data/expected_scores.csv` | Demo scoring baseline. |
| `data/dirty_business_refund_dataset.csv` | Messy business export. |
| `data/dirty_shopflow_refund_dataset.csv` | Messy US-style export. |
| `data/dirty_retailhub_refund_dataset.csv` | Messy EU-style export. |

## Scenario Coverage

The synthetic dataset covers normal handling plus eight suspicious patterns: high-value no-evidence refund, full-amount refund, fast approval, manual override, frequent customer, high-approval agent, repeated customer-agent pair, and suspicious cluster.

| Rule | Threshold |
| --- | --- |
| `HIGH_VALUE_REFUND` | `refund_amount >= 500` |
| `FULL_AMOUNT_REFUND` | `refund_amount / order_amount >= 0.95` |
| `FAST_APPROVAL` | approved in `<= 5` minutes |
