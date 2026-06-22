<h1 align="center">Demo Flow</h1>

This document describes the MVP demo scenario for Fraud & Abuse Detection System.

The demo focuses on suspicious refund approvals in e-commerce support workflows.

<h2 align="center">Main Flow</h2>

1. Open the frontend dashboard.
2. Upload a CSV dataset with orders, return requests, and support decisions.
3. Show dataset preview.
4. Show column mapping.
5. Start analysis.
6. Show analysis status:

```text
UPLOADED
NORMALIZING
NORMALIZED
BUILDING_RELATIONS
SCORING
COMPLETED
```

7. Open suspicious refund approvals dashboard.
8. Show a table with:

```text
returnId
orderId
customerId
supportAgentId
refundAmount
decision
riskScore
riskLevel
topReason
```

9. Open one suspicious approval.
10. Show:

* risk score;
* risk level;
* explanations;
* customer return history;
* support agent approval behavior;
* related orders or refund requests.

11. Explain why the approval is suspicious.

---

<h2 align="center">User Flow Diagram</h2>

```mermaid
flowchart TD
    A[Analyst opens Frontend Dashboard] --> B[Upload Dataset Page]

    B --> C[Upload e-commerce refund dataset]
    C --> D[Dataset Preview Page]

    D --> E[Review uploaded rows]
    E --> F[Review or confirm column mapping]

    F --> G[Start Analysis]
    G --> H[Analysis Status Page]

    H --> S1[UPLOADED]
    S1 --> S2[NORMALIZING]
    S2 --> S3[NORMALIZED]
    S3 --> S4[BUILDING_RELATIONS]
    S4 --> S5[SCORING]
    S5 --> S6[COMPLETED]

    S6 --> I[Suspicious Refund Approvals Dashboard]

    I --> J[View suspicious approvals table]
    J --> K[Select one suspicious refund approval]

    K --> L[Refund Approval Details Page]

    L --> M[Risk Score]
    L --> N[Risk Level]
    L --> O[Explanation Reasons]
    L --> P[Order Details]
    L --> Q[Return Request Details]
    L --> R[Customer Return History]
    L --> T[Support Agent Approval Behavior]
    L --> U[Related Refund Approvals]

    M --> V[Analyst understands why approval is suspicious]
    N --> V
    O --> V
    P --> V
    Q --> V
    R --> V
    T --> V
    U --> V
````

---

<h2 align="center">Example Investigation Narrative</h2>

An analyst opens a high-risk refund approval and sees that the refund was approved in two minutes, had no evidence, used a manual override, and refunded almost the full order amount. The same support agent also has an unusually high approval rate and has repeatedly approved refunds for the same customer.

<h2 align="center">Demo Scoring Checks</h2>

```bash
curl http://localhost:8083/api/scoring/health
curl http://localhost:8083/api/scoring/datasets/demo/suspicious-approvals
curl http://localhost:8083/api/scoring/returns/return_3041/risk
curl http://localhost:8083/api/scoring/datasets/demo/returns/return_3041/details
curl http://localhost:8083/api/scoring/agents/agent_777/risk-summary
```
