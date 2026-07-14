import { requestJson, type ApiEnvelope } from "./client";

export type DatasetUploadResponse = {
  datasetId?: string;
  jobId?: string;
  filename?: string;
  status?: string;
};

export type DatasetSummary = {
  id: string;
  name: string;
  filename: string;
  status: string;
  resultReady: boolean;
  latestJobId?: string;
  rowCount: number;
  sizeBytes: number;
  warningCount: number;
  uploadedAt: string;
};

export type DatasetListResponse = {
  items: DatasetSummary[];
  page: number;
  pageSize: number;
  total: number;
  totalPages: number;
};

export type DatasetJob = {
  jobId: string;
  datasetId: string;
  status: string;
  currentStep?: string;
  resultReady: boolean;
  errorMessage?: string;
  retryOfJobId?: string;
  createdAt: string;
  updatedAt: string;
};

export type DatasetAuditEvent = {
  id: number;
  datasetId: string;
  jobId?: string;
  eventType: string;
  fromStatus?: string;
  toStatus?: string;
  message?: string;
  createdAt: string;
};

export type DatasetDetailsResponse = {
  dataset: { id: string; originalFilename: string; status: string; rowCount: number; warnings: string[] };
  analysisHistory: DatasetJob[];
  auditEvents: DatasetAuditEvent[];
};

export function uploadDataset(file: File) {
  const body = new FormData();
  body.append("file", file);
  return requestJson<ApiEnvelope<DatasetUploadResponse>>("/api/datasets/upload", { method: "POST", body });
}

export function getDatasetPreview(datasetId: string) {
  return requestJson<ApiEnvelope<unknown>>(`/api/datasets/${datasetId}/preview`);
}

export function listDatasets() {
  return requestJson<ApiEnvelope<DatasetListResponse>>("/api/datasets?page=1&pageSize=50");
}

export function getDatasetDetails(datasetId: string) {
  return requestJson<ApiEnvelope<DatasetDetailsResponse>>(`/api/datasets/${datasetId}`);
}

export function archiveDataset(datasetId: string) {
  return requestJson<void>(`/api/datasets/${datasetId}/archive`, { method: "POST" });
}
