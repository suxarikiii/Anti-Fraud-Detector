# Refund Data Normalization

## Overview

This document defines the first version of the data normalization format for the **Fraud & Abuse Detection System** MVP.

The MVP domain is focused on detecting suspicious refund approvals in e-commerce customer support workflows. Different e-commerce companies may upload CSV datasets with different column names and value formats. The normalization layer converts these raw business datasets into a common internal refund approval format that can be used by backend services, graph relations service, and scoring service.

---

## Clean Refund Dataset

The clean dataset uses the internal normalized column names expected by our system.

```csv
order_id,customer_id,return_id,support_agent_id,order_amount,refund_amount,product_category,return_reason,evidence_provided,decision,manual_override,decision_time_minutes,timestamp
```

### Clean Dataset Columns

| Column | Type | Description |
|---|---:|---|
| `order_id` | string | Unique identifier of the order |
| `customer_id` | string | Unique identifier of the customer who requested the refund |
| `return_id` | string | Unique identifier of the refund / return request |
| `support_agent_id` | string | Unique identifier of the support agent who made the decision |
| `order_amount` | number | Original order amount |
| `refund_amount` | number | Amount refunded to the customer |
| `product_category` | string | Product category, for example `electronics`, `clothing`, `home` |
| `return_reason` | string | Reason for the refund request |
| `evidence_provided` | boolean | Whether the customer provided evidence, such as photo or proof |
| `decision` | string | Support decision, for example `APPROVED` or `DECLINED` |
| `manual_override` | boolean | Whether the support agent manually bypassed normal approval rules |
| `decision_time_minutes` | integer | Number of minutes between request creation and support decision |
| `timestamp` | string | Refund request / decision timestamp in ISO format |

---

## Dirty Business Refund Dataset

The dirty business dataset simulates a CSV export from an external e-commerce company. It contains the same refund approval information as the clean dataset, but uses business-specific column names and less standardized value formats.

```csv
purchase_id,buyer_id,refund_request_id,agent_id,purchase_amount,return_amount,category,reason,has_photo,status,override,resolution_minutes,created_at
```

Example dirty row:

```csv
purchase_1001,buyer_200,refund_req_3001,agent_001,197.97,85.85,clothing,wrong_size,yes,approved,no,43,2026-06-01 09:08:00
```

---

## Normalized Refund Approval Format

After normalization, each refund approval record should be converted into the following internal JSON format.

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

### Normalized Field Descriptions

| JSON Field | Source Column | Type | Description |
|---|---|---:|---|
| `returnId` | `return_id` | string | Internal refund / return request identifier |
| `orderId` | `order_id` | string | Internal order identifier |
| `customerId` | `customer_id` | string | Internal customer identifier |
| `supportAgentId` | `support_agent_id` | string | Internal support agent identifier |
| `orderAmount` | `order_amount` | number | Original order amount |
| `refundAmount` | `refund_amount` | number | Refund amount approved or requested |
| `productCategory` | `product_category` | string | Product category |
| `returnReason` | `return_reason` | string | Reason for the refund request |
| `evidenceProvided` | `evidence_provided` | boolean | Whether proof/evidence was provided |
| `decision` | `decision` | string | Normalized decision value, for example `APPROVED` or `DECLINED` |
| `manualOverride` | `manual_override` | boolean | Whether normal rules were manually overridden |
| `decisionTimeMinutes` | `decision_time_minutes` | integer | Decision time in minutes |
| `timestamp` | `timestamp` | string | Normalized timestamp in ISO format |

---

## Column Mapping Rules

The normalization service should map different possible raw column names into one internal clean format.

| Raw Column Aliases | Normalized Column |
|---|---|
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

---

## Mapping Example

### Dirty Business Input

```json
{
  "purchase_id": "purchase_1001",
  "buyer_id": "buyer_200",
  "refund_request_id": "refund_req_3001",
  "agent_id": "agent_001",
  "purchase_amount": 197.97,
  "return_amount": 85.85,
  "category": "clothing",
  "reason": "wrong_size",
  "has_photo": "yes",
  "status": "approved",
  "override": "no",
  "resolution_minutes": 43,
  "created_at": "2026-06-01 09:08:00"
}
```

### Normalized Output

```json
{
  "returnId": "return_3001",
  "orderId": "order_1001",
  "customerId": "customer_200",
  "supportAgentId": "agent_001",
  "orderAmount": 197.97,
  "refundAmount": 85.85,
  "productCategory": "clothing",
  "returnReason": "wrong_size",
  "evidenceProvided": true,
  "decision": "APPROVED",
  "manualOverride": false,
  "decisionTimeMinutes": 43,
  "timestamp": "2026-06-01T09:08:00Z"
}
```

---

## Value Normalization Rules

Column names are not the only part that may differ between companies. Some values also need to be normalized.

| Raw Value | Normalized Value |
|---|---|
| `yes`, `true`, `1` | `true` |
| `no`, `false`, `0` | `false` |
| `approved`, `approve`, `APPROVED` | `APPROVED` |
| `declined`, `reject`, `DECLINED` | `DECLINED` |
| `purchase_1001` | `order_1001` |
| `buyer_200` | `customer_200` |
| `refund_req_3001` | `return_3001` |
| `2026-06-01 09:08:00` | `2026-06-01T09:08:00Z` |

---

## Synthetic Dataset Scenario Coverage

The synthetic dataset should include both normal and suspicious refund approval patterns.

Each scenario is represented by 5 generated cases.

1. Normal refund approvals with evidence and reasonable timing
2. High-value refund approved without evidence
3. Full-amount refund approved for expensive order
4. Very fast approval by support agent
5. Manual override on high-value refund
6. Customer with frequent refund requests
7. Support agent with unusually high approval rate
8. Repeated agent-customer approval pattern
9. Suspicious cluster: same agent + frequent customer returns + manual overrides

The main clean dataset should not contain an extra `scenario` column because backend services expect only business fields. Scenario coverage is documented separately in `scenario_coverage.csv`.

---

## RabbitMQ Events

The normalization service participates in the asynchronous backend pipeline.

```text
consume: dataset.uploaded
publish: dataset.normalized
```

### Event Meaning

| Event | Direction | Meaning |
|---|---|---|
| `dataset.uploaded` | consumed | A new refund dataset was uploaded and is ready for normalization |
| `dataset.normalized` | published | The uploaded dataset was mapped and converted into the internal refund approval format |
