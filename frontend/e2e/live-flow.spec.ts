import { expect, test } from "@playwright/test";

test("upload to investigation keeps one dataset and renders relation context", async ({ page }) => {
  const requestedUrls: string[] = [];
  let savedDecision: Record<string, unknown> | null = null;
  await page.route("**/api/**", async (route) => {
    const url = new URL(route.request().url());
    if (!url.pathname.startsWith("/api/")) return route.continue();
    requestedUrls.push(url.pathname);
    const json = (body: unknown, status = 200) => route.fulfill({ status, contentType: "application/json", body: JSON.stringify(body) });

    if (url.pathname === "/api/datasets" && route.request().method() === "GET") {
      return json({ items: [], page: 1, pageSize: 50, total: 0, totalPages: 0 });
    }
    if (url.pathname === "/api/datasets/upload") return json({ datasetId: "dataset-e2e", jobId: "job-e2e" }, 201);
    if (url.pathname === "/api/datasets/dataset-e2e/preview") {
      return json({ headers: ["return_id", "customer_id"], rows: [{ return_id: "return-e2e", customer_id: "customer-e2e" }], rowCount: 1, truncated: false });
    }
    if (url.pathname === "/api/analysis/dataset-e2e/start") return json({ jobId: "job-e2e", status: "NORMALIZING" }, 201);
    if (url.pathname === "/api/analysis/job-e2e/status") return json({ status: "COMPLETED", currentStep: "COMPLETED" });
    if (url.pathname === "/api/scoring/datasets/dataset-e2e/suspicious-approvals") {
      return json([{ datasetId: "dataset-e2e", returnId: "return-e2e", orderId: "order-e2e", customerId: "customer-e2e", supportAgentId: "agent-e2e", refundAmount: 900, decision: "APPROVED", riskScore: 82, riskLevel: "CRITICAL", topReason: "Repeated suspicious relation" }]);
    }
    if (url.pathname === "/api/scoring/datasets/dataset-e2e/returns/return-e2e/details") {
      return json({ datasetId: "dataset-e2e", returnId: "return-e2e", orderId: "order-e2e", customerId: "customer-e2e", supportAgentId: "agent-e2e", refundAmount: 900, orderAmount: 1000, decision: "APPROVED", riskScore: 82, riskLevel: "CRITICAL", topReason: "Repeated suspicious relation", productCategory: "electronics", returnReason: "defective", evidenceProvided: false, manualOverride: true, decisionTimeMinutes: 2, reasons: [{ type: "NO_EVIDENCE", message: "No evidence was attached.", scoreImpact: 25 }] });
    }
    if (url.pathname.endsWith("/agents/agent-e2e/risk-summary")) return json({ agentId: "agent-e2e", approvalRate: 0.9, highRiskApprovals: 2 });
    if (url.pathname.endsWith("/customers/customer-e2e/summary")) return json({ datasetId: "dataset-e2e", customerId: "customer-e2e", returnCount: 3, approvedRefundCount: 2, totalRefundAmount: 1500, averageRefundRatio: 0.75, relatedAgents: [], recentReturns: [] });
    if (url.pathname.endsWith("/agents/agent-e2e/summary")) return json({ supportAgentId: "agent-e2e", decisionsCount: 4, approvalRate: 0.75, highValueApprovalCount: 2, manualOverrideCount: 1, repeatedCustomerPairCount: 2, topRiskyCategory: "electronics" });
    if (url.pathname.endsWith("/returns/return-e2e/graph")) return json({ datasetId: "dataset-e2e", returnId: "return-e2e", nodes: [{ id: "return:return-e2e", type: "return", label: "return-e2e", summary: "Selected return" }, { id: "customer:customer-e2e", type: "customer", label: "customer-e2e", summary: "Customer" }], edges: [{ from: "customer:customer-e2e", to: "return:return-e2e", type: "REQUESTED", label: "requested", reason: "Customer requested this return." }], limit: 24, truncated: false });
    if (url.pathname.endsWith("/returns/return-e2e/decision")) {
      if (route.request().method() === "PUT") {
        savedDecision = route.request().postDataJSON() as Record<string, unknown>;
        return json({ datasetId: "dataset-e2e", returnId: "return-e2e", ...savedDecision, createdAt: "2026-07-14T10:00:00Z", updatedAt: "2026-07-14T11:00:00Z" });
      }
      return json({ message: "No decision" }, 404);
    }
    if (url.pathname.endsWith("/datasets/dataset-e2e/export.csv")) {
      return route.fulfill({
        status: 200,
        contentType: "text/csv;charset=UTF-8",
        headers: { "content-disposition": 'attachment; filename="scoring-dataset-e2e.csv"' },
        body: "returnId,outcome\nreturn-e2e,NEEDS_MORE_INFO\n",
      });
    }
    return json({ message: "Unexpected route" }, 404);
  });

  await page.goto("/");
  await page.locator('input[type="file"]').setInputFiles({
    name: "refunds.csv",
    mimeType: "text/csv",
    buffer: Buffer.from("return_id,customer_id\nreturn-e2e,customer-e2e\n"),
  });
  await expect(page.getByText("dataset-e2e")).toBeVisible();
  await page.getByRole("button", { name: /start analysis/i }).click();
  await expect(page.getByText("return-e2e").first()).toBeVisible();
  await page.getByRole("button", { name: "Review" }).click();

  await expect(page.getByText("No evidence was attached.")).toBeVisible();
  await expect(page.getByRole("heading", { name: "Customer history" })).toBeVisible();
  const relationGraph = page.getByRole("application", { name: /interactive relation graph with 2 nodes and 1 edges/i });
  await expect(relationGraph).toBeVisible();
  const returnNode = relationGraph.getByRole("button", { name: /return: return-e2e/i });
  const customerNode = relationGraph.getByRole("button", { name: /customer: customer-e2e/i });
  const returnBefore = await returnNode.boundingBox();
  const customerBox = await customerNode.boundingBox();
  if (!returnBefore || !customerBox) throw new Error("Relation graph nodes have no browser geometry");
  await page.mouse.move(returnBefore.x + returnBefore.width / 2, returnBefore.y + returnBefore.height / 2);
  await page.mouse.down();
  await page.mouse.move(customerBox.x + customerBox.width / 2, customerBox.y + customerBox.height / 2, { steps: 8 });
  await page.mouse.up();
  const returnAfter = await returnNode.boundingBox();
  const customerAfter = await customerNode.boundingBox();
  if (!returnAfter || !customerAfter) throw new Error("Relation graph nodes disappeared after dragging");
  expect(Math.hypot(
    returnAfter.x + returnAfter.width / 2 - (customerAfter.x + customerAfter.width / 2),
    returnAfter.y + returnAfter.height / 2 - (customerAfter.y + customerAfter.height / 2),
  )).toBeGreaterThan(40);
  await expect(page.getByText(/no saved decision yet/i)).toBeVisible();
  await page.getByRole("button", { name: "Escalate" }).click();
  await page.getByRole("button", { name: "Needs more info" }).click();
  await page.getByLabel("Reviewer notes").fill("Request receipt");
  await page.getByRole("button", { name: "Save decision" }).click();
  await expect(page.getByText(/last saved/i)).toBeVisible();
  expect(savedDecision).toMatchObject({ action: "ESCALATE", outcome: "NEEDS_MORE_INFO", note: "Request receipt" });

  await page.getByRole("button", { name: "Approvals" }).click();
  const downloadPromise = page.waitForEvent("download");
  await page.getByRole("button", { name: "Download CSV" }).click();
  const download = await downloadPromise;
  expect(download.suggestedFilename()).toBe("scoring-dataset-e2e.csv");
  expect(requestedUrls.some((url) => url.includes("/datasets/demo/"))).toBe(false);
  expect(
    requestedUrls
      .filter((url) => url.includes("/datasets/") && url !== "/api/datasets/upload")
      .every((url) => url.includes("dataset-e2e")),
  ).toBe(true);
});
