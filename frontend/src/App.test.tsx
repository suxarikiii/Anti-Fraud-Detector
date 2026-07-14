import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import App, { AnalysisStatusList, ApprovalsPage, DetailsPage, RiskBadge } from "./App";
import { RelationGraph } from "./components/RelationGraph";

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
  window.localStorage.clear();
  window.history.replaceState(null, "", "/");
});

describe("Week 4 scoring dashboard", () => {
  it("renders risk badge labels", () => {
    render(<RiskBadge level="CRITICAL" />);

    expect(screen.getByText("Critical")).toBeInTheDocument();
  });

  it("shows dataset source and CSV schema onboarding before upload", () => {
    render(<App />);

    expect(screen.getByText("No dataset")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /download sample csv/i })).toHaveAttribute(
      "download",
      "anti_fraud_sample_refunds.csv",
    );
    expect(screen.getByText("Accepted CSV schema")).toBeInTheDocument();
    expect(screen.getByText("order_id")).toBeInTheDocument();
    expect(screen.getAllByText("Required").length).toBeGreaterThan(0);
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
    expect(screen.getByText("Missing evidence")).toBeInTheDocument();
    expect(screen.getByText("Return was approved without supporting evidence.")).toBeInTheDocument();
    expect(screen.getAllByText("agent_api_1").length).toBeGreaterThan(0);
    expect(screen.getByText("Scoring thresholds")).toBeInTheDocument();
    expect(screen.getByText("80-100")).toBeInTheDocument();
  });

  it("enables analyst decision controls and saves through the provided handler", async () => {
    const onReviewChange = vi.fn();
    const onSaveDecision = vi.fn();

    render(
      <DetailsPage
        agentSummary={null}
        approval={details}
        onOpenApproval={vi.fn()}
        onReviewChange={onReviewChange}
        onSaveDecision={onSaveDecision}
        decisionStatus="empty"
        review={{
          action: "REVIEW",
          outcome: "OPEN",
          note: "",
          analystId: "analyst-test",
        }}
        status="ready"
      />,
    );

    expect(screen.getByText(/no saved decision yet/i)).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "Escalate" }));
    await userEvent.click(screen.getByRole("button", { name: "Confirmed fraud" }));
    await userEvent.click(screen.getByRole("button", { name: "Save decision" }));

    expect(onReviewChange).toHaveBeenCalledWith("return_api_1", { action: "ESCALATE" });
    expect(onReviewChange).toHaveBeenCalledWith("return_api_1", { outcome: "CONFIRMED_FRAUD" });
    expect(onSaveDecision).toHaveBeenCalled();
  });

  it("renders analysis status progress", () => {
    render(<AnalysisStatusList analysisStepIndex={4} isAnalyzing />);

    expect(screen.getByText("Scoring")).toBeInTheDocument();
    expect(screen.getByText("In progress")).toBeInTheDocument();
    expect(screen.getByText("Relations built")).toBeInTheDocument();
  });

  it("renders an interactive graph with edge details and no duplicated text list", () => {
    render(
      <RelationGraph
        graph={{
          datasetId: "dataset-graph",
          returnId: "return_api_1",
          nodes: [
            { id: "return:return_api_1", type: "return", label: "return_api_1", summary: "Selected return" },
            { id: "customer:customer_api_1", type: "customer", label: "customer_api_1", summary: "Customer" },
          ],
          edges: [
            {
              from: "customer:customer_api_1",
              to: "return:return_api_1",
              type: "REQUESTED",
              label: "requested",
              reason: "Customer requested this return.",
            },
          ],
          limit: 2,
          truncated: true,
        }}
      />,
    );

    expect(screen.getByRole("application", { name: /2 nodes and 1 edges/i })).toBeInTheDocument();
    expect(screen.getByLabelText("Graph legend")).toBeInTheDocument();
    expect(screen.getByLabelText("Relation type legend")).toBeInTheDocument();
    expect(screen.getByLabelText("Relation details")).toHaveTextContent("Customer requested this return.");
    expect(screen.getByRole("button", { name: /customer: customer_api_1/i })).toHaveAttribute("tabindex", "0");
    expect(screen.queryByRole("list", { name: "Relation details" })).not.toBeInTheDocument();
    expect(screen.getByRole("status")).toHaveTextContent("Graph is truncated to 2 nodes.");
  });

  it("renders a truthful empty relation graph", () => {
    render(<RelationGraph graph={{ datasetId: "dataset-empty", returnId: "return-empty", nodes: [], edges: [], limit: 24, truncated: false }} />);

    expect(screen.getByText("No relation graph")).toBeInTheDocument();
    expect(screen.getByText(/no connected entities/i)).toBeInTheDocument();
  });

  it("lays out the maximum graph without overlapping node centers", () => {
    const nodes = Array.from({ length: 24 }, (_, index) => ({
      id: index === 0 ? "return:return_center" : `return:return_related_${index}`,
      type: "return",
      label: index === 0 ? "return_center" : `return_related_${index}`,
      summary: index === 0 ? "Selected return" : "Related return",
    }));
    const { container } = render(
      <RelationGraph graph={{ datasetId: "dataset-dense", returnId: "return_center", nodes, edges: [], limit: 24, truncated: false }} />,
    );

    const points = [...container.querySelectorAll<SVGGElement>(".graph-node")].map((node) => {
      const match = node.getAttribute("transform")?.match(/translate\(([-\d.]+) ([-\d.]+)\)/);
      if (!match) throw new Error("Graph node has no position");
      return { x: Number(match[1]), y: Number(match[2]) };
    });
    const distances = points.flatMap((point, index) => points.slice(index + 1).map((other) => Math.hypot(point.x - other.x, point.y - other.y)));
    expect(Math.min(...distances)).toBeGreaterThan(95);
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

    expect(screen.queryByText("Live backend")).not.toBeInTheDocument();
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
    expect(screen.getByText("Fast approval")).toBeInTheDocument();
    expect(screen.getByText("Approval was unusually fast for this refund amount.")).toBeInTheDocument();
    expect(screen.getByText("+12")).toBeInTheDocument();
    expect(screen.getByText("No")).toBeInTheDocument();
    expect(screen.getByText("4")).toBeInTheDocument();
    expect(screen.getAllByText("agent_snake_7").length).toBeGreaterThan(0);
  });

  it("shows details unavailable state without fabricated investigation data", async () => {
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

    expect(await screen.findByText(/backend is unavailable/i)).toBeInTheDocument();
    expect(screen.queryByText("Why was this refund flagged?")).not.toBeInTheDocument();
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

  it("loads dataset history but does not preload approvals before a dataset is selected", async () => {
    const fetchMock = vi.spyOn(globalThis, "fetch").mockRejectedValue(new Error("network down"));

    render(<App />);
    await userEvent.click(screen.getByRole("button", { name: /approvals/i }));

    expect(screen.getByText("No approvals loaded")).toBeInTheDocument();
    expect(screen.queryByText("return_123")).not.toBeInTheDocument();
    expect(fetchMock).toHaveBeenCalledWith("/api/datasets?page=1&pageSize=50", undefined);
    expect(fetchMock.mock.calls.some(([url]) => String(url).includes("suspicious-approvals"))).toBe(false);
  });

  it("stops on FAILED and does not request scoring results", async () => {
    const requestedUrls: string[] = [];
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
      const url = String(input);
      requestedUrls.push(url);
      if (url.endsWith("/api/datasets/upload")) return jsonResponse({ datasetId: "failed-dataset", jobId: "failed-job" });
      if (url.endsWith("/api/datasets/failed-dataset/preview")) return jsonResponse({ headers: [], rows: [], rowCount: 2, truncated: false });
      if (url.endsWith("/api/analysis/failed-dataset/start")) return jsonResponse({ jobId: "failed-job" });
      if (url.endsWith("/api/analysis/failed-job/status")) {
        return jsonResponse({ status: "FAILED", currentStep: "NORMALIZING", errorMessage: "Normalizer rejected the artifact" });
      }
      return jsonResponse({});
    });

    render(<App />);
    await uploadAndRunAnalysis();

    expect(await screen.findByText("Normalizer rejected the artifact")).toBeInTheDocument();
    expect(requestedUrls.some((url) => url.includes("suspicious-approvals"))).toBe(false);
  });

  it("distinguishes an empty successful scoring result", async () => {
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
      const url = String(input);
      if (url.endsWith("/api/datasets/upload")) return jsonResponse({ datasetId: "empty-dataset", jobId: "empty-job" });
      if (url.endsWith("/api/datasets/empty-dataset/preview")) return jsonResponse({ headers: [], rows: [], rowCount: 1, truncated: false });
      if (url.endsWith("/api/analysis/empty-dataset/start")) return jsonResponse({ jobId: "empty-job" });
      if (url.endsWith("/api/analysis/empty-job/status")) return jsonResponse({ status: "COMPLETED" });
      if (url.endsWith("/api/scoring/datasets/empty-dataset/suspicious-approvals")) return jsonResponse([]);
      return jsonResponse({});
    });

    render(<App />);
    await uploadAndRunAnalysis();

    expect((await screen.findAllByText("No suspicious cases")).length).toBeGreaterThan(0);
    expect(screen.queryByText(/scoring results are unavailable/i)).not.toBeInTheDocument();
  });

  it("restores dataset and job context after refresh", async () => {
    window.localStorage.setItem(
      "anti-fraud.analysis-session.v1",
      JSON.stringify({ datasetId: "saved-dataset", jobId: "saved-job" }),
    );
    window.history.replaceState(null, "", "/?page=approvals");
    const requestedUrls: string[] = [];
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
      const url = String(input);
      requestedUrls.push(url);
      if (url.endsWith("/api/datasets/saved-dataset/preview")) {
        return jsonResponse({ headers: ["return_id"], rows: [{ return_id: "return_saved" }], rowCount: 1, truncated: false });
      }
      if (url.endsWith("/api/analysis/saved-job/status")) return jsonResponse({ status: "COMPLETED" });
      if (url.endsWith("/api/scoring/datasets/saved-dataset/suspicious-approvals")) return jsonResponse([]);
      return jsonResponse({});
    });

    render(<App />);

    expect((await screen.findAllByText("No suspicious cases")).length).toBeGreaterThan(0);
    expect(requestedUrls).toContain("/api/datasets/saved-dataset/preview");
    expect(requestedUrls).toContain("/api/scoring/datasets/saved-dataset/suspicious-approvals");
    expect(window.location.search).toContain("datasetId=saved-dataset");
    expect(window.location.search).toContain("jobId=saved-job");
  });

  it("loads and persists an analyst decision through the scoring API", async () => {
    let savedBody: Record<string, unknown> | null = null;
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input, init) => {
      const url = String(input);
      if (url.startsWith("/api/datasets?page=")) return jsonResponse({ items: [], page: 1, pageSize: 50, total: 0, totalPages: 0 });
      if (url.endsWith("/api/datasets/upload")) return jsonResponse({ datasetId: "uploaded", jobId: "job_uploaded" });
      if (url.endsWith("/api/datasets/uploaded/preview")) return jsonResponse([]);
      if (url.endsWith("/api/analysis/uploaded/start")) return jsonResponse({ jobId: "job_uploaded" });
      if (url.endsWith("/api/analysis/job_uploaded/status")) return jsonResponse({ status: "COMPLETED" });
      if (url.includes("/api/scoring/datasets/uploaded/suspicious-approvals")) return jsonResponse([approval]);
      if (url.endsWith("/api/scoring/datasets/uploaded/returns/return_api_1/details")) return jsonResponse(details);
      if (url.endsWith("/api/scoring/datasets/uploaded/returns/return_api_1/decision")) {
        if (init?.method === "PUT") {
          savedBody = JSON.parse(String(init.body));
          return jsonResponse({
            datasetId: "uploaded",
            returnId: "return_api_1",
            ...savedBody,
            createdAt: "2026-07-14T10:00:00Z",
            updatedAt: "2026-07-14T11:00:00Z",
          });
        }
        return jsonResponse({
          datasetId: "uploaded",
          returnId: "return_api_1",
          action: "REVIEW",
          outcome: "OPEN",
          note: "Initial review",
          analystId: "analyst-1",
          createdAt: "2026-07-14T10:00:00Z",
          updatedAt: "2026-07-14T10:00:00Z",
        });
      }
      return jsonResponse({});
    });

    render(<App />);
    await uploadAndRunAnalysis();
    await userEvent.click(await screen.findByRole("button", { name: "Review" }));

    expect(await screen.findByDisplayValue("Initial review")).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "Escalate" }));
    await userEvent.click(screen.getByRole("button", { name: "Needs more info" }));
    await userEvent.clear(screen.getByLabelText("Reviewer notes"));
    await userEvent.type(screen.getByLabelText("Reviewer notes"), "Request receipt");
    await userEvent.click(screen.getByRole("button", { name: "Save decision" }));

    await waitFor(() => expect(savedBody).toMatchObject({
      action: "ESCALATE",
      outcome: "NEEDS_MORE_INFO",
      note: "Request receipt",
      analystId: "analyst-1",
    }));
    expect(await screen.findByText(/last saved/i)).toBeInTheDocument();
  });

  it("downloads filtered CSV export with backend filename", async () => {
    const requestedUrls: string[] = [];
    const createObjectURL = vi.fn(() => "blob:export");
    const revokeObjectURL = vi.fn();
    Object.defineProperty(URL, "createObjectURL", { configurable: true, value: createObjectURL });
    Object.defineProperty(URL, "revokeObjectURL", { configurable: true, value: revokeObjectURL });
    vi.spyOn(HTMLAnchorElement.prototype, "click").mockImplementation(() => undefined);
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
      const url = String(input);
      requestedUrls.push(url);
      if (url.startsWith("/api/datasets?page=")) return jsonResponse({ items: [], page: 1, pageSize: 50, total: 0, totalPages: 0 });
      if (url.endsWith("/api/datasets/upload")) return jsonResponse({ datasetId: "uploaded", jobId: "job_uploaded" });
      if (url.endsWith("/api/datasets/uploaded/preview")) return jsonResponse([]);
      if (url.endsWith("/api/analysis/uploaded/start")) return jsonResponse({ jobId: "job_uploaded" });
      if (url.endsWith("/api/analysis/job_uploaded/status")) return jsonResponse({ status: "COMPLETED" });
      if (url.includes("/suspicious-approvals")) return jsonResponse([approval]);
      if (url.includes("/export.csv")) {
        return new Response("returnId\nreturn_api_1", {
          status: 200,
          headers: {
            "content-type": "text/csv;charset=UTF-8",
            "content-disposition": 'attachment; filename="scoring-uploaded.csv"',
          },
        });
      }
      return jsonResponse({});
    });

    render(<App />);
    await uploadAndRunAnalysis();
    await userEvent.selectOptions(screen.getByDisplayValue("All risk levels"), "HIGH");
    await userEvent.selectOptions(screen.getByLabelText("Investigation outcome"), "OPEN");
    await userEvent.click(screen.getByRole("button", { name: "Download CSV" }));

    await waitFor(() => expect(requestedUrls).toContain("/api/scoring/datasets/uploaded/export.csv?risk=HIGH&outcome=OPEN"));
    expect(createObjectURL).toHaveBeenCalled();
    expect(revokeObjectURL).toHaveBeenCalledWith("blob:export");
  });

  it("retries a failed analysis and switches to the new job", async () => {
    const requestedUrls: string[] = [];
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
      const url = String(input);
      requestedUrls.push(url);
      if (url.startsWith("/api/datasets?page=")) return jsonResponse({ items: [], page: 1, pageSize: 50, total: 0, totalPages: 0 });
      if (url.endsWith("/api/datasets/upload")) return jsonResponse({ datasetId: "retry-dataset", jobId: "failed-job" });
      if (url.endsWith("/api/datasets/retry-dataset/preview")) return jsonResponse([]);
      if (url.endsWith("/api/analysis/retry-dataset/start")) return jsonResponse({ jobId: "failed-job" });
      if (url.endsWith("/api/analysis/failed-job/status")) return jsonResponse({ status: "FAILED", errorMessage: "Scoring failed" });
      if (url.endsWith("/api/analysis/failed-job/retry")) return jsonResponse({ jobId: "retry-job", status: "NORMALIZING" });
      if (url.endsWith("/api/analysis/retry-job/status")) return jsonResponse({ status: "COMPLETED" });
      if (url.includes("/suspicious-approvals")) return jsonResponse([]);
      return jsonResponse({});
    });

    render(<App />);
    await uploadAndRunAnalysis();
    expect(await screen.findByText("Scoring failed")).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "Retry analysis" }));

    expect((await screen.findAllByText("No suspicious cases")).length).toBeGreaterThan(0);
    expect(requestedUrls).toContain("/api/analysis/failed-job/retry");
    expect(window.location.search).toContain("jobId=retry-job");
  });

  it("loads and archives a terminal dataset from history", async () => {
    const confirm = vi.spyOn(window, "confirm").mockReturnValue(true);
    const requestedUrls: string[] = [];
    let archived = false;
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input, init) => {
      const url = String(input);
      requestedUrls.push(url);
      if (url.startsWith("/api/datasets?page=")) {
        return jsonResponse({
          items: archived ? [] : [{ id: "history-dataset", name: "history.csv", filename: "history.csv", status: "COMPLETED", resultReady: true, latestJobId: "history-job", rowCount: 12, sizeBytes: 100, warningCount: 0, uploadedAt: "2026-07-14T10:00:00Z" }],
          page: 1,
          pageSize: 50,
          total: archived ? 0 : 1,
          totalPages: archived ? 0 : 1,
        });
      }
      if (url.endsWith("/api/datasets/history-dataset/archive") && init?.method === "POST") {
        archived = true;
        return Promise.resolve(new Response(null, { status: 204 }));
      }
      return jsonResponse({});
    });

    render(<App />);
    expect(await screen.findByText("history.csv")).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "Archive" }));

    await waitFor(() => expect(requestedUrls).toContain("/api/datasets/history-dataset/archive"));
    expect(confirm).toHaveBeenCalled();
    expect(await screen.findByText("No previous datasets.")).toBeInTheDocument();
  });
});

async function uploadAndRunAnalysis() {
  const input = document.querySelector('input[type="file"]') as HTMLInputElement;
  await userEvent.upload(input, new File(["return_id,order_id"], "returns.csv", { type: "text/csv" }));
  const startButton = screen.getByRole("button", { name: /start analysis/i });
  await waitFor(() => expect(startButton).toBeEnabled());
  await userEvent.click(startButton);
}

function jsonResponse(body: unknown) {
  return Promise.resolve(
    new Response(JSON.stringify(body), {
      status: 200,
      headers: { "content-type": "application/json" },
    }),
  );
}
