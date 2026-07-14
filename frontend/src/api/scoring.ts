import { requestBlob, requestJson, type ApiEnvelope } from "./client";

export type InvestigationAction = "REVIEW" | "ESCALATE" | "APPROVE_REFUND" | "REJECT_REFUND" | "FREEZE_ACCOUNT";
export type InvestigationOutcome = "OPEN" | "NEEDS_MORE_INFO" | "CONFIRMED_FRAUD" | "FALSE_POSITIVE" | "RESOLVED";

export type InvestigationDecision = {
  datasetId: string;
  returnId: string;
  action: InvestigationAction;
  outcome: InvestigationOutcome;
  note: string;
  analystId: string;
  createdAt: string;
  updatedAt: string;
};

export type InvestigationDecisionRequest = Pick<InvestigationDecision, "action" | "outcome" | "note" | "analystId">;

type ApprovalFilters = {
  risk?: string;
  agent?: string;
  outcome?: InvestigationOutcome;
};

function scoringQuery(filters: ApprovalFilters = {}) {
  const params = new URLSearchParams();
  if (filters.risk) params.set("risk", filters.risk);
  if (filters.agent) params.set("agent", filters.agent);
  if (filters.outcome) params.set("outcome", filters.outcome);
  const query = params.toString();
  return query ? `?${query}` : "";
}

export function getSuspiciousApprovals(datasetId: string, filters: ApprovalFilters = {}) {
  return requestJson<ApiEnvelope<unknown>>(
    `/api/scoring/datasets/${datasetId}/suspicious-approvals${scoringQuery(filters)}`,
  );
}

export function getReturnDetails(datasetId: string, returnId: string) {
  return requestJson<ApiEnvelope<unknown>>(
    `/api/scoring/datasets/${datasetId}/returns/${returnId}/details`,
  );
}

export function getScoringAgentSummary(datasetId: string, agentId: string) {
  return requestJson<ApiEnvelope<unknown>>(
    `/api/scoring/datasets/${datasetId}/agents/${agentId}/risk-summary`,
  );
}

export function getInvestigationDecision(datasetId: string, returnId: string) {
  return requestJson<InvestigationDecision>(
    `/api/scoring/datasets/${datasetId}/returns/${returnId}/decision`,
  );
}

export function saveInvestigationDecision(
  datasetId: string,
  returnId: string,
  decision: InvestigationDecisionRequest,
) {
  return requestJson<InvestigationDecision>(
    `/api/scoring/datasets/${datasetId}/returns/${returnId}/decision`,
    {
      method: "PUT",
      headers: { "content-type": "application/json" },
      body: JSON.stringify(decision),
    },
  );
}

export function exportSuspiciousApprovals(datasetId: string, filters: ApprovalFilters = {}) {
  return requestBlob(`/api/scoring/datasets/${datasetId}/export.csv${scoringQuery(filters)}`);
}
