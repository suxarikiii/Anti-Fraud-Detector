export type AnalysisSession = {
  datasetId: string;
  jobId: string;
};

const storageKey = "anti-fraud.analysis-session.v1";

function hasStorage() {
  return typeof window !== "undefined" && Boolean(window.localStorage);
}

export function loadAnalysisSession(): AnalysisSession {
  if (!hasStorage()) return { datasetId: "", jobId: "" };
  try {
    const value = JSON.parse(window.localStorage.getItem(storageKey) ?? "{}") as Partial<AnalysisSession>;
    return {
      datasetId: typeof value.datasetId === "string" ? value.datasetId : "",
      jobId: typeof value.jobId === "string" ? value.jobId : "",
    };
  } catch {
    return { datasetId: "", jobId: "" };
  }
}

export function saveAnalysisSession(session: AnalysisSession) {
  if (!hasStorage()) return;
  window.localStorage.setItem(storageKey, JSON.stringify(session));
}

export function clearAnalysisSession() {
  if (!hasStorage()) return;
  window.localStorage.removeItem(storageKey);
}
