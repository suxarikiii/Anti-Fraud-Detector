import type { CustomerSummary, RelationAgentSummary } from "../api/relations";

function money(value: number) {
  return new Intl.NumberFormat("en-US", { style: "currency", currency: "USD", maximumFractionDigits: 0 }).format(value);
}

function percentage(value: number) {
  const normalized = value >= 0 && value <= 1 ? value * 100 : value;
  return `${normalized.toFixed(1)}%`;
}

export function CustomerAnalytics({
  customer,
  agent,
}: {
  customer: CustomerSummary | null;
  agent: RelationAgentSummary | null;
}) {
  return (
    <div className="analytics-grid">
      <section aria-labelledby="customer-analytics-title">
        <h3 id="customer-analytics-title">Customer history</h3>
        {customer ? (
          <>
            <dl className="analytics-metrics">
              <div><dt>Returns</dt><dd>{customer.returnCount}</dd></div>
              <div><dt>Approved refunds</dt><dd>{customer.approvedRefundCount}</dd></div>
              <div><dt>Total refund</dt><dd>{money(customer.totalRefundAmount)}</dd></div>
              <div><dt>Average refund ratio</dt><dd>{percentage(customer.averageRefundRatio)}</dd></div>
            </dl>
            <ul className="analytics-list" aria-label="Recent customer returns">
              {customer.recentReturns.map((item) => (
                <li key={item.returnId}>
                  <strong>{item.returnId}</strong>
                  <span>{item.category || item.reason} · {money(item.refundAmount)} · {item.decisionStatus}</span>
                </li>
              ))}
            </ul>
          </>
        ) : <p className="text-body-secondary">Customer analytics are unavailable.</p>}
      </section>

      <section aria-labelledby="agent-analytics-title">
        <h3 id="agent-analytics-title">Support agent analytics</h3>
        {agent ? (
          <dl className="analytics-metrics">
            <div><dt>Decisions</dt><dd>{agent.decisionsCount}</dd></div>
            <div><dt>Approval rate</dt><dd>{percentage(agent.approvalRate)}</dd></div>
            <div><dt>Manual overrides</dt><dd>{agent.manualOverrideCount}</dd></div>
            <div><dt>High-value approvals</dt><dd>{agent.highValueApprovalCount}</dd></div>
            <div><dt>Repeated pairs</dt><dd>{agent.repeatedCustomerPairCount}</dd></div>
            <div><dt>Top risky category</dt><dd>{agent.topRiskyCategory || "None"}</dd></div>
          </dl>
        ) : <p className="text-body-secondary">Agent analytics are unavailable.</p>}
      </section>
    </div>
  );
}
