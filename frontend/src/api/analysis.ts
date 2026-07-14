import { ApiEnvelope, ApiError, isRecord, requestJson, unwrapData } from "./client";

export type AnalysisJobStatus = {
  id?: string;
  jobId?: string;
  datasetId?: string;
  status?: string;
  currentStep?: string;
  message?: string;
  progressPercent?: number;
  errorMessage?: string;
  updatedAt?: string;
};

export type PollResult =
  | { outcome: "completed"; status: AnalysisJobStatus }
  | { outcome: "failed"; status: AnalysisJobStatus }
  | { outcome: "timeout"; status: AnalysisJobStatus | null };

type PollOptions = {
  intervalMs?: number;
  maxAttempts?: number;
  onUpdate?: (status: AnalysisJobStatus) => void;
  wait?: (milliseconds: number) => Promise<void>;
};

const defaultWait = (milliseconds: number) =>
  new Promise<void>((resolve) => window.setTimeout(resolve, milliseconds));

export function startDatasetAnalysis(datasetId: string) {
  return requestJson<ApiEnvelope<{ datasetId?: string; jobId?: string; status?: string }>>(
    `/api/analysis/${datasetId}/start`,
    { method: "POST" },
  );
}

export function getAnalysisJobStatus(jobId: string) {
  return requestJson<ApiEnvelope<AnalysisJobStatus>>(`/api/analysis/${jobId}/status`);
}

export function retryAnalysisJob(jobId: string) {
  return requestJson<ApiEnvelope<{ jobId?: string; status?: string }>>(`/api/analysis/${jobId}/retry`, {
    method: "POST",
  });
}

export async function pollAnalysisJob(jobId: string, options: PollOptions = {}): Promise<PollResult> {
  const intervalMs = options.intervalMs ?? 1_000;
  const maxAttempts = options.maxAttempts ?? 120;
  const wait = options.wait ?? defaultWait;
  let latest: AnalysisJobStatus | null = null;

  for (let attempt = 0; attempt < maxAttempts; attempt += 1) {
    const value = unwrapData(await getAnalysisJobStatus(jobId));
    if (!isRecord(value)) {
      throw new ApiError("Analysis status response is incomplete.", 502, "INVALID_STATUS_RESPONSE");
    }
    latest = value;
    options.onUpdate?.(latest);

    const state = String(latest.status ?? latest.currentStep ?? "").toUpperCase();
    if (["COMPLETED", "READY", "DONE", "COMPLETE"].includes(state)) {
      return { outcome: "completed", status: latest };
    }
    if (state === "FAILED") {
      return { outcome: "failed", status: latest };
    }
    if (attempt < maxAttempts - 1) await wait(intervalMs);
  }

  return { outcome: "timeout", status: latest };
}
