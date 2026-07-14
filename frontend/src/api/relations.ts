import { ApiError, isRecord, requestJson } from "./client";

export type CustomerReturn = {
  returnId: string;
  orderId: string;
  reason: string;
  category: string;
  refundAmount: number;
  decisionStatus: string;
  supportAgentId: string;
};

export type CustomerSummary = {
  datasetId: string;
  customerId: string;
  returnCount: number;
  approvedRefundCount: number;
  totalRefundAmount: number;
  averageRefundRatio: number;
  relatedAgents: Array<{ supportAgentId: string; pairCount: number }>;
  recentReturns: CustomerReturn[];
};

export type RelationAgentSummary = {
  supportAgentId: string;
  decisionsCount: number;
  approvalRate: number;
  highValueApprovalCount: number;
  manualOverrideCount: number;
  repeatedCustomerPairCount: number;
  topRiskyCategory: string;
};

export type GraphNode = {
  id: string;
  type: string;
  label: string;
  summary: string;
};

export type GraphEdge = {
  from: string;
  to: string;
  type: string;
  label: string;
  reason: string;
  count?: number;
  weight?: number;
};

export type GraphProjection = {
  datasetId: string;
  returnId: string;
  nodes: GraphNode[];
  edges: GraphEdge[];
  limit: number;
  truncated: boolean;
};

export function getCustomerSummary(datasetId: string, customerId: string) {
  return requestJson<CustomerSummary>(
    `/api/relations/datasets/${datasetId}/customers/${customerId}/summary?limit=10`,
  ).then((value) => {
    if (!isRecord(value) || !Array.isArray(value.recentReturns) || !Array.isArray(value.relatedAgents)) {
      throw new ApiError("Customer analytics response is incomplete.", 502, "INVALID_RELATIONS_RESPONSE");
    }
    return value;
  });
}

export function getRelationAgentSummary(datasetId: string, agentId: string) {
  return requestJson<RelationAgentSummary>(
    `/api/relations/datasets/${datasetId}/agents/${agentId}/summary`,
  ).then((value) => {
    if (!isRecord(value) || typeof value.supportAgentId !== "string") {
      throw new ApiError("Agent analytics response is incomplete.", 502, "INVALID_RELATIONS_RESPONSE");
    }
    return value;
  });
}

export function getReturnGraph(datasetId: string, returnId: string) {
  return requestJson<GraphProjection>(
    `/api/relations/datasets/${datasetId}/returns/${returnId}/graph?limit=24`,
  ).then((value) => {
    if (!isRecord(value) || !Array.isArray(value.nodes) || !Array.isArray(value.edges)) {
      throw new ApiError("Relation graph response is incomplete.", 502, "INVALID_RELATIONS_RESPONSE");
    }
    return value;
  });
}
