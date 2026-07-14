import { afterEach, describe, expect, it, vi } from "vitest";
import { pollAnalysisJob } from "./analysis";

afterEach(() => {
  vi.restoreAllMocks();
});

describe("pollAnalysisJob", () => {
  it("returns timeout without inventing a completed result", async () => {
    const fetchMock = vi.spyOn(globalThis, "fetch").mockImplementation(async () =>
      new Response(JSON.stringify({ status: "NORMALIZING", currentStep: "NORMALIZING" }), {
        status: 200,
        headers: { "content-type": "application/json" },
      }),
    );

    const result = await pollAnalysisJob("job-timeout", {
      maxAttempts: 2,
      intervalMs: 0,
      wait: async () => undefined,
    });

    expect(result.outcome).toBe("timeout");
    expect(fetchMock).toHaveBeenCalledTimes(2);
    expect(fetchMock).toHaveBeenCalledWith("/api/analysis/job-timeout/status", undefined);
  });
});
