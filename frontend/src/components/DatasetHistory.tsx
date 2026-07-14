import {
  CBadge,
  CButton,
  CCard,
  CCardBody,
  CCardHeader,
  CTable,
  CTableBody,
  CTableDataCell,
  CTableHead,
  CTableHeaderCell,
  CTableRow,
} from "@coreui/react";
import { Archive, History, RefreshCw } from "lucide-react";
import type { DatasetDetailsResponse, DatasetSummary } from "../api/datasets";

type Props = {
  items: DatasetSummary[];
  currentDatasetId: string;
  status: "idle" | "loading" | "ready" | "empty" | "failed" | "unavailable";
  error: string;
  busyId: string;
  details: DatasetDetailsResponse | null;
  onRefresh: () => void;
  onSelect: (dataset: DatasetSummary) => void;
  onRetry: (dataset: DatasetSummary) => void;
  onArchive: (dataset: DatasetSummary) => void;
};

const terminalStatuses = new Set(["COMPLETED", "FAILED"]);

export function DatasetHistory({
  items,
  currentDatasetId,
  status,
  error,
  busyId,
  details,
  onRefresh,
  onSelect,
  onRetry,
  onArchive,
}: Props) {
  return (
    <CCard className="mt-4">
      <CCardHeader className="d-flex justify-content-between align-items-center gap-3 flex-wrap">
        <div>
          <strong>Dataset history</strong>
          <div className="text-body-secondary small">Open, retry, or archive previous analysis runs.</div>
        </div>
        <CButton color="secondary" onClick={onRefresh} size="sm" variant="outline">
          <RefreshCw size={15} /> Refresh
        </CButton>
      </CCardHeader>
      <CCardBody>
        {status === "loading" && <div role="status">Loading dataset history…</div>}
        {(status === "failed" || status === "unavailable") && <div role="alert">{error || "Dataset history is unavailable."}</div>}
        {status === "empty" && <div>No previous datasets.</div>}
        {items.length > 0 && (
          <div className="table-responsive">
            <CTable align="middle" hover>
              <CTableHead>
                <CTableRow>
                  <CTableHeaderCell>Dataset</CTableHeaderCell>
                  <CTableHeaderCell>Status</CTableHeaderCell>
                  <CTableHeaderCell>Rows</CTableHeaderCell>
                  <CTableHeaderCell>Uploaded</CTableHeaderCell>
                  <CTableHeaderCell>Actions</CTableHeaderCell>
                </CTableRow>
              </CTableHead>
              <CTableBody>
                {items.map((dataset) => {
                  const busy = busyId === dataset.id;
                  const terminal = terminalStatuses.has(dataset.status);
                  return (
                    <CTableRow key={dataset.id} active={dataset.id === currentDatasetId}>
                      <CTableDataCell>
                        <strong>{dataset.filename || dataset.name}</strong>
                        <div className="text-body-secondary small dataset-id">{dataset.id}</div>
                      </CTableDataCell>
                      <CTableDataCell>
                        <CBadge color={dataset.status === "FAILED" ? "danger" : dataset.resultReady ? "success" : "secondary"}>
                          {dataset.status.replace(/_/g, " ")}
                        </CBadge>
                      </CTableDataCell>
                      <CTableDataCell>{dataset.rowCount}</CTableDataCell>
                      <CTableDataCell>{formatDate(dataset.uploadedAt)}</CTableDataCell>
                      <CTableDataCell>
                        <div className="dataset-actions">
                          <CButton color="primary" disabled={busy} onClick={() => onSelect(dataset)} size="sm" variant="outline">
                            <History size={14} /> Open
                          </CButton>
                          {terminal && dataset.latestJobId && (
                            <CButton color="secondary" disabled={busy} onClick={() => onRetry(dataset)} size="sm" variant="outline">
                              <RefreshCw size={14} /> Retry
                            </CButton>
                          )}
                          {terminal && (
                            <CButton color="danger" disabled={busy} onClick={() => onArchive(dataset)} size="sm" variant="outline">
                              <Archive size={14} /> Archive
                            </CButton>
                          )}
                        </div>
                      </CTableDataCell>
                    </CTableRow>
                  );
                })}
              </CTableBody>
            </CTable>
          </div>
        )}
        {details && (
          <div className="dataset-run-history">
            <h3>Selected dataset runs</h3>
            {details.analysisHistory.length > 0 ? (
              <ul>
                {details.analysisHistory.map((job) => (
                  <li key={job.jobId}>
                    <strong>{job.status.replace(/_/g, " ")}</strong>
                    <span>{job.jobId}</span>
                    <span>{formatDate(job.createdAt)}</span>
                    {job.errorMessage && <span className="text-danger">{job.errorMessage}</span>}
                  </li>
                ))}
              </ul>
            ) : (
              <div>No analysis runs recorded.</div>
            )}
            {details.auditEvents.length > 0 && (
              <details>
                <summary>Lifecycle audit ({details.auditEvents.length})</summary>
                <ul>
                  {details.auditEvents.slice(0, 10).map((event) => (
                    <li key={event.id}>
                      <strong>{event.eventType.replace(/_/g, " ")}</strong>
                      <span>{event.message || `${event.fromStatus || "—"} → ${event.toStatus || "—"}`}</span>
                      <span>{formatDate(event.createdAt)}</span>
                    </li>
                  ))}
                </ul>
              </details>
            )}
          </div>
        )}
      </CCardBody>
    </CCard>
  );
}

function formatDate(value: string) {
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? "Unknown" : date.toLocaleString();
}
