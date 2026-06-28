import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import App, { AnalysisStatusList, ApprovalsPage, DetailsPage, RiskBadge } from "./App";

const approval = {
  returnId: "return_api_1",
  orderId: "order_api_1",
  customerId: "customer_api_1",
  supportAgentId: "agent_api_1",
  datasetId: "demo",
  refundAmount: 1500,
  decision: "APPROVED" as const,
  riskScore: 91,
  riskLevel: "CRITICAL" as const,
  topReason: "Missing evidence on high-value refund",
};

const mediumApproval = {
  returnId: "return_medium_1",
  orderId: "order_medium_1",
  customerId: "customer_medium_1",
  supportAgentId: "agent_medium_1",
  datasetId: "demo",
  refundAmount: 220,
  decision: "APPROVED" as const,
  riskScore: 44,
  riskLevel: "MEDIUM" as const,
  topReason: "Elevated return frequency",
};

const details = {
  ...approval,
  orderAmount: 1800,
  productCategory: "electronics",
  returnReason: "defective",
  evidenceProvided: false,
  manualOverride: true,
  decisionTimeMinutes: 3,
  paymentMethod: "card",
  shippingRegion: "Moscow",
  reasons: [
    {
      type: "NO_EVIDENCE",
      message: "Return was approved without supporting evidence.",
      scoreImpact: 25,
    },
  ],
  relatedApprovals: [],
};

afterEach(() => {
  vi.restoreAllMocks();
});

describe("Week 4 scoring dashboard", () => {
  it("renders risk badge labels", () => {
    render(<RiskBadge level="CRITICAL" />);

    expect(screen.getByText("Critical")).toBeInTheDocument();
  });

  it("renders approvals table with risk score and review action", () => {
    render(
      <ApprovalsPage
        approvals={[approval]}
        onOpenApproval={vi.fn()}
        onQueryChange={vi.fn()}
        onRiskFilterChange={vi.fn()}
        query=""
        riskFilter="ALL"
        status="ready"
      />,
    );

    expect(screen.getByText("Suspicious approvals")).toBeInTheDocument();
    expect(screen.getAllByText("return_api_1").length).toBeGreaterThan(0);
    expect(screen.getByText("91")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Review" })).toBeInTheDocument();
  });

  it("paginates approvals by 10 rows", async () => {
    const approvals = Array.from({ length: 12 }, (_, index) => ({
      ...approval,
      returnId: `return_page_${String(index + 1).padStart(2, "0")}`,
      orderId: `order_page_${index + 1}`,
      customerId: `customer_page_${index + 1}`,
      supportAgentId: `agent_page_${index + 1}`,
    }));

    render(
      <ApprovalsPage
        approvals={approvals}
        onOpenApproval={vi.fn()}
        onQueryChange={vi.fn()}
        onRiskFilterChange={vi.fn()}
        query=""
        riskFilter="ALL"
        status="ready"
      />,
    );

    expect(screen.getByText("Showing 1-10 of 12")).toBeInTheDocument();
    expect(screen.getByText("return_page_01")).toBeInTheDocument();
    expect(screen.queryByText("return_page_11")).not.toBeInTheDocument();

    await userEvent.click(screen.getByRole("button", { name: /next page/i }));

    expect(screen.getByText("Showing 11-12 of 12")).toBeInTheDocument();
    expect(screen.getByText("return_page_11")).toBeInTheDocument();
    expect(screen.queryByText("return_page_01")).not.toBeInTheDocument();
  });

  it("renders details explanations and agent summary", () => {
    render(
      <DetailsPage
        agentSummary={{
          agentId: "agent_api_1",
          approvalRate: 88,
          manualOverrideRate: 41,
          highRiskApprovals: 5,
          repeatedCustomerPairs: 2,
        }}
        approval={details}
        onOpenApproval={vi.fn()}
        status="ready"
      />,
    );

    expect(screen.getByText("Why was this refund flagged?")).toBeInTheDocument();
    expect(screen.getByText("No Evidence")).toBeInTheDocument();
    expect(screen.getByText("Return was approved without supporting evidence.")).toBeInTheDocument();
    expect(screen.getAllByText("agent_api_1").length).toBeGreaterThan(0);
  });

  it("renders analysis status progress", () => {
    render(<AnalysisStatusList analysisStepIndex={4} isAnalyzing />);

    expect(screen.getByText("Scoring")).toBeInTheDocument();
    expect(screen.getByText("In progress")).toBeInTheDocument();
    expect(screen.getByText("Relations built")).toBeInTheDocument();
  });

  it("filters approvals by search text and risk level", async () => {
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
      const url = String(input);
      if (url.endsWith("/api/datasets/upload")) {
        return jsonResponse({ datasetId: "uploaded", jobId: "job_uploaded" });
      }
      if (url.endsWith("/api/datasets/uploaded/preview")) {
        return jsonResponse([]);
      }
      if (url.endsWith("/api/analysis/uploaded/start")) {
        return jsonResponse({ jobId: "job_uploaded" });
      }
      if (url.endsWith("/api/analysis/job_uploaded/status")) {
        return jsonResponse({ status: "COMPLETED" });
      }
      if (url.endsWith("/api/scoring/datasets/uploaded/suspicious-approvals")) {
        return jsonResponse([approval, mediumApproval]);
      }
      return jsonResponse({});
    });

    render(<App />);
    await uploadAndRunAnalysis();

    expect(await screen.findByText("return_api_1")).toBeInTheDocument();
    expect(screen.getByText("return_medium_1")).toBeInTheDocument();

    await userEvent.type(screen.getByPlaceholderText("Search ID"), "customer_api_1");
    expect(screen.getAllByText("return_api_1").length).toBeGreaterThan(0);
    expect(screen.queryByText("return_medium_1")).not.toBeInTheDocument();

    await userEvent.clear(screen.getByPlaceholderText("Search ID"));
    await userEvent.selectOptions(screen.getByDisplayValue("All risk levels"), "MEDIUM");
    expect(screen.queryByText("return_api_1")).not.toBeInTheDocument();
    expect(screen.getByText("return_medium_1")).toBeInTheDocument();
  });

  it("loads scoring data and details from gateway API", async () => {
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
      const url = String(input);
      if (url.endsWith("/api/datasets/upload")) {
        return jsonResponse({ datasetId: "uploaded", jobId: "job_uploaded" });
      }
      if (url.endsWith("/api/datasets/uploaded/preview")) {
        return jsonResponse([]);
      }
      if (url.endsWith("/api/analysis/uploaded/start")) {
        return jsonResponse({ jobId: "job_uploaded" });
      }
      if (url.endsWith("/api/analysis/job_uploaded/status")) {
        return jsonResponse({ status: "COMPLETED" });
      }
      if (url.endsWith("/api/scoring/datasets/uploaded/suspicious-approvals")) {
        return jsonResponse([approval]);
      }
      if (url.endsWith("/api/scoring/datasets/uploaded/returns/return_api_1/details")) {
        return jsonResponse(details);
      }
      if (url.endsWith("/api/scoring/datasets/uploaded/agents/agent_api_1/risk-summary")) {
        return jsonResponse({
          agentId: "agent_api_1",
          approvalRate: 88,
          manualOverrideRate: 41,
          highRiskApprovals: 5,
          repeatedCustomerPairs: 2,
        });
      }
      return jsonResponse({});
    });

    render(<App />);
    await uploadAndRunAnalysis();

    expect(await screen.findByText("return_api_1")).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "Review" }));

    expect(await screen.findByText("Return was approved without supporting evidence.")).toBeInTheDocument();
    expect(screen.getAllByText("agent_api_1").length).toBeGreaterThan(0);
  });

  it("normalizes snake_case details and agent summary responses", async () => {
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
      const url = String(input);
      if (url.endsWith("/api/datasets/upload")) {
        return jsonResponse({ datasetId: "uploaded", jobId: "job_uploaded" });
      }
      if (url.endsWith("/api/datasets/uploaded/preview")) {
        return jsonResponse([]);
      }
      if (url.endsWith("/api/analysis/uploaded/start")) {
        return jsonResponse({ jobId: "job_uploaded" });
      }
      if (url.endsWith("/api/analysis/job_uploaded/status")) {
        return jsonResponse({ status: "COMPLETED" });
      }
      if (url.endsWith("/api/scoring/datasets/uploaded/suspicious-approvals")) {
        return jsonResponse([approval]);
      }
      if (url.endsWith("/api/scoring/datasets/uploaded/returns/return_api_1/details")) {
        return jsonResponse({
          return_id: "return_api_1",
          order_id: "order_api_1",
          customer_id: "customer_api_1",
          support_agent_id: "agent_snake_7",
          refund_amount: 1500,
          order_amount: 1800,
          product_category: "mobile_accessories",
          return_reason: "item_not_as_described",
          evidence_provided: "false",
          manual_override: "false",
          decision_time_minutes: 6,
          risk_score: 72,
          risk_level: "HIGH",
          top_reason: "Snake case payload was normalized",
          explanations: [
            {
              reason_type: "FAST_APPROVAL",
              description: "Approval was unusually fast for this refund amount.",
              score_impact: 12,
            },
          ],
        });
      }
      if (url.endsWith("/api/scoring/datasets/uploaded/agents/agent_snake_7/risk-summary")) {
        return jsonResponse({
          agent_id: "agent_snake_7",
          approval_rate: 67,
          manual_override_rate: 11,
          high_risk_approvals: 4,
          repeated_customer_pairs: 2,
        });
      }
      return jsonResponse({});
    });

    render(<App />);
    await uploadAndRunAnalysis();
    await userEvent.click(await screen.findByRole("button", { name: "Review" }));

    expect(await screen.findByText("Snake case payload was normalized")).toBeInTheDocument();
    expect(screen.getByText("Fast Approval")).toBeInTheDocument();
    expect(screen.getByText("Approval was unusually fast for this refund amount.")).toBeInTheDocument();
    expect(screen.getByText("+12")).toBeInTheDocument();
    expect(screen.getByText("No")).toBeInTheDocument();
    expect(screen.getByText("67%")).toBeInTheDocument();
    expect(screen.getAllByText("agent_snake_7").length).toBeGreaterThan(0);
  });

  it("shows details fallback notice when return details API fails", async () => {
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
      const url = String(input);
      if (url.endsWith("/api/datasets/upload")) {
        return jsonResponse({ datasetId: "uploaded", jobId: "job_uploaded" });
      }
      if (url.endsWith("/api/datasets/uploaded/preview")) {
        return jsonResponse([]);
      }
      if (url.endsWith("/api/analysis/uploaded/start")) {
        return jsonResponse({ jobId: "job_uploaded" });
      }
      if (url.endsWith("/api/analysis/job_uploaded/status")) {
        return jsonResponse({ status: "COMPLETED" });
      }
      if (url.endsWith("/api/scoring/datasets/uploaded/suspicious-approvals")) {
        return jsonResponse([approval]);
      }
      if (url.endsWith("/api/scoring/datasets/uploaded/returns/return_api_1/details")) {
        throw new Error("details service unavailable");
      }
      return jsonResponse({});
    });

    render(<App />);
    await uploadAndRunAnalysis();
    await userEvent.click(await screen.findByRole("button", { name: "Review" }));

    expect(await screen.findByText("Details API is unavailable. Showing offline investigation fallback.")).toBeInTheDocument();
    expect(screen.getAllByText("return_api_1").length).toBeGreaterThan(0);
    expect(screen.getByText("Why was this refund flagged?")).toBeInTheDocument();
  });

  it("uses /api gateway routes for scoring requests", async () => {
    const requestedUrls: string[] = [];
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
      const url = String(input);
      requestedUrls.push(url);
      if (url.endsWith("/api/datasets/upload")) {
        return jsonResponse({ datasetId: "uploaded", jobId: "job_uploaded" });
      }
      if (url.endsWith("/api/datasets/uploaded/preview")) {
        return jsonResponse([]);
      }
      if (url.endsWith("/api/analysis/uploaded/start")) {
        return jsonResponse({ jobId: "job_uploaded" });
      }
      if (url.endsWith("/api/analysis/job_uploaded/status")) {
        return jsonResponse({ status: "COMPLETED" });
      }
      if (url.endsWith("/api/scoring/datasets/uploaded/suspicious-approvals")) {
        return jsonResponse([approval]);
      }
      if (url.endsWith("/api/scoring/datasets/uploaded/returns/return_api_1/details")) {
        return jsonResponse(details);
      }
      if (url.endsWith("/api/scoring/datasets/uploaded/agents/agent_api_1/risk-summary")) {
        return jsonResponse({
          agentId: "agent_api_1",
          approvalRate: 88,
          manualOverrideRate: 41,
          highRiskApprovals: 5,
          repeatedCustomerPairs: 2,
        });
      }
      return jsonResponse({});
    });

    render(<App />);
    await uploadAndRunAnalysis();
    await userEvent.click(await screen.findByRole("button", { name: "Review" }));
    await screen.findByText("Return was approved without supporting evidence.");

    expect(requestedUrls).toContain("/api/scoring/datasets/uploaded/suspicious-approvals");
    expect(requestedUrls).toContain("/api/scoring/datasets/uploaded/returns/return_api_1/details");
    expect(requestedUrls).toContain("/api/scoring/datasets/uploaded/agents/agent_api_1/risk-summary");
    expect(requestedUrls.every((url) => url.startsWith("/api/"))).toBe(true);
  });

  it("does not preload approvals before the user uploads a dataset", async () => {
    const fetchMock = vi.spyOn(globalThis, "fetch").mockRejectedValue(new Error("network down"));

    render(<App />);
    await userEvent.click(screen.getByRole("button", { name: /approvals/i }));

    expect(screen.getByText("No approvals loaded")).toBeInTheDocument();
    expect(screen.queryByText("return_123")).not.toBeInTheDocument();
    expect(fetchMock).not.toHaveBeenCalled();
  });
});

async function uploadAndRunAnalysis() {
  const input = document.querySelector('input[type="file"]') as HTMLInputElement;
  await userEvent.upload(input, new File(["return_id,order_id"], "returns.csv", { type: "text/csv" }));
  await screen.findByText("uploaded");
  await userEvent.click(screen.getByRole("button", { name: /start analysis/i }));
}

function jsonResponse(body: unknown) {
  return Promise.resolve(
    new Response(JSON.stringify(body), {
      status: 200,
      headers: { "content-type": "application/json" },
    }),
  );
}
