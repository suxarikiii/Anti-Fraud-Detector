import {
  CBadge,
  CButton,
  CCard,
  CCardBody,
  CCardHeader,
  CCol,
  CContainer,
  CFormInput,
  CFormLabel,
  CFormSelect,
  CFormTextarea,
  CFormText,
  CHeader,
  CHeaderBrand,
  CInputGroup,
  CInputGroupText,
  CListGroup,
  CListGroupItem,
  CNav,
  CNavItem,
  CNavLink,
  CProgress,
  CProgressBar,
  CRow,
  CSidebar,
  CSidebarBrand,
  CSidebarFooter,
  CSidebarHeader,
  CTable,
  CTableBody,
  CTableDataCell,
  CTableHead,
  CTableHeaderCell,
  CTableRow,
} from "@coreui/react";
import {
  AlertTriangle,
  ArrowRight,
  BarChart3,
  CheckCircle2,
  ChevronLeft,
  ChevronRight,
  CircleDot,
  Clock3,
  Database,
  Download,
  FileSpreadsheet,
  Filter,
  Gauge,
  Info,
  LayoutDashboard,
  Search,
  ShieldAlert,
  Upload,
  UserRound,
  UsersRound,
  WifiOff,
} from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import {
  getAnalysisJobStatus,
  pollAnalysisJob,
  retryAnalysisJob,
  startDatasetAnalysis,
  type AnalysisJobStatus,
} from "./api/analysis";
import { ApiError, type ApiEnvelope, isRecord, unwrapData } from "./api/client";
import {
  archiveDataset,
  getDatasetDetails,
  getDatasetPreview,
  listDatasets,
  uploadDataset,
  type DatasetSummary,
  type DatasetDetailsResponse,
} from "./api/datasets";
import {
  getCustomerSummary,
  getRelationAgentSummary,
  getReturnGraph,
  type CustomerSummary,
  type GraphProjection,
  type RelationAgentSummary,
} from "./api/relations";
import {
  exportSuspiciousApprovals,
  getInvestigationDecision,
  getReturnDetails,
  getScoringAgentSummary,
  getSuspiciousApprovals,
  saveInvestigationDecision,
  type InvestigationAction,
  type InvestigationOutcome,
} from "./api/scoring";
import antiFraudLogoFull from "./assets/brand/anti-fraud-logo-full.png";
import antiFraudLogoMark from "./assets/brand/anti-fraud-logo-mark.png";
import { CustomerAnalytics } from "./components/CustomerAnalytics";
import { DatasetHistory } from "./components/DatasetHistory";
import { RelationGraph } from "./components/RelationGraph";
import {
  clearAnalysisSession,
  loadAnalysisSession,
  saveAnalysisSession,
} from "./state/analysisSession";

type Page = "dataset" | "approvals" | "details";
type RiskLevel = "LOW" | "MEDIUM" | "HIGH" | "CRITICAL";
type Decision = "APPROVED" | "REJECTED";
type ColumnMapping = Record<string, string>;
type PreviewRow = Record<string, string>;

type DatasetPreview = {
  headers: string[];
  rows: PreviewRow[];
  rowCount: number;
  truncated: boolean;
};

type Approval = {
  returnId: string;
  orderId: string;
  customerId: string;
  supportAgentId: string;
  datasetId?: string;
  refundAmount: number;
  decision: Decision;
  riskScore: number;
  riskLevel: RiskLevel;
  topReason: string;
};

type Explanation = {
  type: string;
  message: string;
  scoreImpact: number;
};

type ReturnRisk = Approval & {
  datasetId: string;
  orderAmount: number;
  productCategory: string;
  returnReason: string;
  evidenceProvided: boolean;
  manualOverride: boolean;
  decisionTimeMinutes: number;
  timestamp?: string;
  calculatedAt?: string;
  paymentMethod?: string;
  shippingRegion?: string;
  reasons: Explanation[];
  relatedApprovals: Approval[];
};

type ReturnDetailsResponse = Partial<Omit<ReturnRisk, "reasons">> & {
  reasons?: Explanation[];
  explanations?: Explanation[];
};

type AgentSummary = {
  agentId: string;
  approvalRate: number;
  manualOverrideRate: number;
  highRiskApprovals: number;
  repeatedCustomerPairs: number;
};

type ApiStatus = "idle" | "loading" | "ready" | "empty" | "failed" | "unavailable";
type DataSourceMode = "idle" | "checking" | "live" | "empty" | "failed" | "unavailable";
type CaseReview = {
  action: InvestigationAction;
  outcome: InvestigationOutcome;
  note: string;
  analystId: string;
  updatedAt?: string;
};

type UploadResponse = {
  datasetId?: string;
  jobId?: string;
};

type AnalysisStatus =
  | "UPLOADED"
  | "NORMALIZING"
  | "NORMALIZED"
  | "BUILDING_RELATIONS"
  | "SCORING"
  | "COMPLETED";

const defaultCaseReview: CaseReview = {
  action: "REVIEW",
  outcome: "OPEN",
  note: "",
  analystId: "frontend-analyst",
};

const sampleCsvTemplate = `order_id,customer_id,return_id,support_agent_id,order_amount,refund_amount,product_category,return_reason,evidence_provided,decision,manual_override,decision_time_minutes,timestamp
order_456,customer_789,return_123,agent_001,11999,11500,electronics,item_not_as_described,false,APPROVED,true,2,2026-06-01 09:06:00
order_785,customer_184,return_204,agent_002,3490,1290,clothing,size_issue,true,APPROVED,false,18,2026-06-01 09:22:00
`;

const sampleCsvHref = `data:text/csv;charset=utf-8,${encodeURIComponent(sampleCsvTemplate)}`;

const csvSchema = [
  {
    semantic: "order_id",
    required: true,
    accepted: ["order_id", "purchase_id"],
    description: "Original purchase identifier",
  },
  {
    semantic: "customer_id",
    required: true,
    accepted: ["customer_id", "buyer_id", "client_id"],
    description: "Buyer or customer identifier",
  },
  {
    semantic: "return_id",
    required: true,
    accepted: ["return_id", "refund_request_id"],
    description: "Refund or return request identifier",
  },
  {
    semantic: "support_agent_id",
    required: true,
    accepted: ["support_agent_id", "agent_id", "support_user_id"],
    description: "Support employee who made the decision",
  },
  {
    semantic: "order_amount",
    required: true,
    accepted: ["order_amount", "purchase_amount"],
    description: "Original order amount",
  },
  {
    semantic: "refund_amount",
    required: true,
    accepted: ["refund_amount", "return_amount"],
    description: "Refunded amount",
  },
  {
    semantic: "decision",
    required: true,
    accepted: ["decision", "status", "approval_status"],
    description: "Approval result",
  },
  {
    semantic: "timestamp",
    required: true,
    accepted: ["timestamp", "created_at", "decision_time"],
    description: "Decision timestamp",
  },
  {
    semantic: "decision_time_minutes",
    required: false,
    accepted: ["decision_time_minutes", "resolution_minutes"],
    description: "Minutes before the support decision",
  },
  {
    semantic: "evidence_provided",
    required: false,
    accepted: ["evidence_provided", "has_photo", "proof_provided"],
    description: "Photo, chat, or delivery evidence flag",
  },
  {
    semantic: "manual_override",
    required: false,
    accepted: ["manual_override", "override"],
    description: "Manual exception used by an agent",
  },
];

function detectColumnMapping(headers: string[]): ColumnMapping {
  return Object.fromEntries(
    headers.map((header) => {
      const normalized = header.trim().toLowerCase();
      const field = csvSchema.find((candidate) => candidate.accepted.includes(normalized));
      return [header, field?.semantic ?? "Unmapped"];
    }),
  );
}

function getMappingWarnings(headers: string[]) {
  const normalized = new Set(headers.map((header) => header.trim().toLowerCase()));
  return csvSchema
    .filter((field) => field.required && !field.accepted.some((alias) => normalized.has(alias)))
    .map((field) => `Required field ${field.semantic} was not detected.`);
}

const riskThresholds = [
  { level: "LOW" as RiskLevel, range: "0-29", meaning: "Routine approval" },
  { level: "MEDIUM" as RiskLevel, range: "30-59", meaning: "Needs quick review" },
  { level: "HIGH" as RiskLevel, range: "60-79", meaning: "Investigate before closing" },
  { level: "CRITICAL" as RiskLevel, range: "80-100", meaning: "Escalate immediately" },
];

const scoringRuleDefinitions = [
  {
    type: "NO_EVIDENCE",
    label: "Missing evidence",
    threshold: "No proof attached to approved refund",
    source: "CSV evidence flag",
  },
  {
    type: "HIGH_VALUE_REFUND",
    label: "High-value refund",
    threshold: "Refund amount is above the high-value threshold",
    source: "Order and refund amount",
  },
  {
    type: "FAST_APPROVAL",
    label: "Fast approval",
    threshold: "Decision time is unusually short",
    source: "Decision time minutes",
  },
  {
    type: "MANUAL_OVERRIDE",
    label: "Manual override",
    threshold: "Approved refund used a manual exception",
    source: "Manual override flag",
  },
  {
    type: "REPEATED_AGENT_CUSTOMER_PAIR",
    label: "Repeated pair",
    threshold: "Same agent-customer pair appears in related approvals",
    source: "Graph relations",
  },
];

const statusSteps: AnalysisStatus[] = [
  "UPLOADED",
  "NORMALIZING",
  "NORMALIZED",
  "BUILDING_RELATIONS",
  "SCORING",
  "COMPLETED",
];

const statusDescriptions: Record<AnalysisStatus, string> = {
  UPLOADED: "File received",
  NORMALIZING: "Columns detected",
  NORMALIZED: "Rows prepared",
  BUILDING_RELATIONS: "Relations built",
  SCORING: "Risk scores calculated",
  COMPLETED: "Ready for review",
};

const navItems: Array<{ page: Page; label: string; icon: typeof Upload }> = [
  { page: "dataset", label: "Dataset", icon: Upload },
  { page: "approvals", label: "Approvals", icon: LayoutDashboard },
  { page: "details", label: "Investigation", icon: ShieldAlert },
];

const approvalsPageSize = 10;

const columnLabels: Record<string, string> = {
  returnId: "Return ID",
  orderId: "Order ID",
  customerId: "Customer ID",
  supportAgentId: "Agent ID",
  refundAmount: "Refund amount",
  decision: "Decision",
  riskScore: "Risk score",
  riskLevel: "Risk level",
  topReason: "Main reason",
  order_id: "Order ID",
  customer_id: "Customer ID",
  return_id: "Return ID",
  support_agent_id: "Agent ID",
  order_amount: "Order amount",
  refund_amount: "Refund amount",
  product_category: "Category",
  return_reason: "Return reason",
  evidence_provided: "Evidence",
  manual_override: "Manual override",
  decision_time_minutes: "Decision time",
};

const actionLabels: Record<InvestigationAction, string> = {
  REVIEW: "Review",
  ESCALATE: "Escalate",
  APPROVE_REFUND: "Approve refund",
  REJECT_REFUND: "Reject refund",
  FREEZE_ACCOUNT: "Freeze account",
};

const outcomeLabels: Record<InvestigationOutcome, string> = {
  OPEN: "Open",
  NEEDS_MORE_INFO: "Needs more info",
  CONFIRMED_FRAUD: "Confirmed fraud",
  FALSE_POSITIVE: "False positive",
  RESOLVED: "Resolved",
};

function getScoringRule(type: string) {
  return scoringRuleDefinitions.find((rule) => rule.type === type);
}

function getInitialPage(): Page {
  const pageParam = new URLSearchParams(window.location.search).get("page");
  if (pageParam === "approvals" || pageParam === "details") return pageParam;
  return "dataset";
}

function formatMoney(value: number) {
  return new Intl.NumberFormat("en-US", {
    style: "currency",
    currency: "USD",
    maximumFractionDigits: 0,
  }).format(value);
}

function formatEnum(value: string) {
  return value
    .toLowerCase()
    .split("_")
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
    .join(" ");
}

function normalizeUploadResponse(raw: ApiEnvelope<UploadResponse>): UploadResponse {
  const data = unwrapData<UploadResponse>(raw);
  return {
    datasetId: data?.datasetId ?? (isRecord(raw) ? String(raw.datasetId ?? "") : ""),
    jobId: data?.jobId ?? (isRecord(raw) ? String(raw.jobId ?? "") : ""),
  };
}

function normalizePreview(raw: unknown): DatasetPreview {
  const value = unwrapData(raw as ApiEnvelope<unknown>);

  if (Array.isArray(value)) {
    const rows = value.filter(isRecord).map((row) =>
      Object.fromEntries(Object.entries(row).map(([key, cell]) => [key, String(cell ?? "")])),
    );
    return {
      headers: rows[0] ? Object.keys(rows[0]) : [],
      rows,
      rowCount: rows.length,
      truncated: false,
    };
  }

  if (!isRecord(value)) {
    return { headers: [], rows: [], rowCount: 0, truncated: false };
  }

  const headers = Array.isArray(value.headers) ? value.headers.map(String) : [];
  const rowsValue = Array.isArray(value.rows) ? value.rows : [];
  const rows = rowsValue.map((row) => {
    if (Array.isArray(row)) {
      return Object.fromEntries(headers.map((header, index) => [header, String(row[index] ?? "")]));
    }
    if (isRecord(row)) {
      return Object.fromEntries(Object.entries(row).map(([key, cell]) => [key, String(cell ?? "")]));
    }
    return {};
  });

  return {
    headers: headers.length > 0 ? headers : rows[0] ? Object.keys(rows[0]) : [],
    rows,
    rowCount: toNumber(value.rowCount ?? value.row_count, rows.length),
    truncated: toBoolean(value.truncated, false),
  };
}

function normalizeRiskLevel(value: unknown, score = 0): RiskLevel {
  const normalized = String(value ?? "").toUpperCase();
  if (normalized === "LOW" || normalized === "MEDIUM" || normalized === "HIGH" || normalized === "CRITICAL") {
    return normalized;
  }
  if (score >= 80) return "CRITICAL";
  if (score >= 60) return "HIGH";
  if (score >= 30) return "MEDIUM";
  return "LOW";
}

function normalizeDecision(value: unknown): Decision {
  return String(value ?? "APPROVED").toUpperCase() === "REJECTED" ? "REJECTED" : "APPROVED";
}

function toNumber(value: unknown, fallback = 0) {
  const number = Number(value);
  return Number.isFinite(number) ? number : fallback;
}

function toBoolean(value: unknown, fallback = false) {
  if (typeof value === "boolean") return value;
  if (typeof value === "number") return value !== 0;
  if (typeof value === "string") {
    const normalized = value.trim().toLowerCase();
    if (["true", "1", "yes"].includes(normalized)) return true;
    if (["false", "0", "no"].includes(normalized)) return false;
  }
  return fallback;
}

function normalizeApproval(raw: unknown, datasetId: string): Approval | null {
  if (!isRecord(raw)) return null;
  const refundAmount = toNumber(raw.refundAmount ?? raw.refund_amount, 0);
  const riskScore = toNumber(raw.riskScore ?? raw.risk_score, 0);
  const returnId = String(raw.returnId ?? raw.return_id ?? raw.id ?? "");
  if (!returnId) return null;

  return {
    returnId,
    orderId: String(raw.orderId ?? raw.order_id ?? ""),
    customerId: String(raw.customerId ?? raw.customer_id ?? ""),
    supportAgentId: String(raw.supportAgentId ?? raw.support_agent_id ?? ""),
    datasetId: String(datasetId || raw.datasetId || raw.dataset_id || ""),
    refundAmount,
    decision: normalizeDecision(raw.decision),
    riskScore,
    riskLevel: normalizeRiskLevel(raw.riskLevel ?? raw.risk_level, riskScore),
    topReason: String(raw.topReason ?? raw.top_reason ?? "Risk reasons are available in the details view"),
  };
}

function normalizeExplanation(raw: unknown): Explanation | null {
  if (!isRecord(raw)) return null;
  const type = String(raw.type ?? raw.reasonType ?? raw.reason_type ?? "");
  const message = String(raw.message ?? raw.description ?? "");
  if (!type || !message) return null;
  return {
    type,
    message,
    scoreImpact: toNumber(raw.scoreImpact ?? raw.score_impact ?? raw.impact, 0),
  };
}

function normalizeApprovals(raw: unknown, datasetId: string): Approval[] {
  const value = unwrapData(raw as ApiEnvelope<unknown>);
  if (!Array.isArray(value)) return [];
  return value
    .map((item) => normalizeApproval(item, datasetId))
    .filter((item): item is Approval => item !== null);
}

function normalizeAgentSummary(raw: unknown, agentId: string): AgentSummary {
  const value = unwrapData(raw as ApiEnvelope<unknown>);
  if (!isRecord(value)) {
    return { agentId, approvalRate: 0, manualOverrideRate: 0, highRiskApprovals: 0, repeatedCustomerPairs: 0 };
  }

  const percent = (input: unknown) => {
    const number = toNumber(input, 0);
    return number >= 0 && number <= 1 ? Math.round(number * 1000) / 10 : number;
  };

  return {
    agentId: String(value.agentId ?? value.agent_id ?? agentId),
    approvalRate: percent(value.approvalRate ?? value.approval_rate),
    manualOverrideRate: percent(value.manualOverrideRate ?? value.manual_override_rate),
    highRiskApprovals: toNumber(
      value.highRiskApprovals ?? value.high_risk_approvals ?? value.highRiskApprovalsCount,
      0,
    ),
    repeatedCustomerPairs: toNumber(
      value.repeatedCustomerPairs ?? value.repeated_customer_pairs,
      0,
    ),
  };
}

function normalizeStatusStep(status: AnalysisJobStatus): AnalysisStatus {
  const rawStep = String(status.currentStep ?? status.status ?? "UPLOADED").toUpperCase();
  if (rawStep === "READY" || rawStep === "DONE" || rawStep === "COMPLETE") return "COMPLETED";
  if (statusSteps.includes(rawStep as AnalysisStatus)) return rawStep as AnalysisStatus;
  return "UPLOADED";
}

function buildReturnDetailsBase(approval: Approval): ReturnRisk {
  return {
    ...approval,
    datasetId: approval.datasetId ?? "",
    orderAmount: 0,
    productCategory: "",
    returnReason: "",
    evidenceProvided: false,
    manualOverride: false,
    decisionTimeMinutes: 0,
    reasons: [],
    relatedApprovals: [],
  };
}

function toReturnRisk(data: ReturnDetailsResponse, approval: Approval): ReturnRisk {
  const base = buildReturnDetailsBase(approval);
  const record: Record<string, unknown> = isRecord(data) ? data : {};
  const rawReasons = data.reasons ?? data.explanations;
  const normalizedReasons = Array.isArray(rawReasons)
    ? rawReasons.map(normalizeExplanation).filter((item): item is Explanation => item !== null)
    : [];
  const rawRelatedApprovals = data.relatedApprovals ?? record.related_approvals;
  const normalizedRelatedApprovals = Array.isArray(rawRelatedApprovals)
    ? rawRelatedApprovals
        .map((item) => normalizeApproval(item, approval.datasetId ?? ""))
        .filter((item): item is Approval => item !== null)
    : [];
  const riskScore = toNumber(data.riskScore ?? record.risk_score, approval.riskScore);
  const riskLevel = normalizeRiskLevel(data.riskLevel ?? record.risk_level, riskScore);

  return {
    ...base,
    ...data,
    returnId: String(data.returnId ?? record.return_id ?? approval.returnId),
    orderId: String(data.orderId ?? record.order_id ?? approval.orderId),
    customerId: String(data.customerId ?? record.customer_id ?? approval.customerId),
    supportAgentId: String(data.supportAgentId ?? record.support_agent_id ?? approval.supportAgentId),
    datasetId: String(data.datasetId ?? record.dataset_id ?? approval.datasetId ?? ""),
    refundAmount: toNumber(data.refundAmount ?? record.refund_amount, approval.refundAmount),
    decision: normalizeDecision(data.decision ?? approval.decision),
    riskScore,
    riskLevel,
    topReason: String(data.topReason ?? record.top_reason ?? approval.topReason),
    orderAmount: toNumber(data.orderAmount ?? record.order_amount, base.orderAmount),
    productCategory: String(data.productCategory ?? record.product_category ?? base.productCategory),
    returnReason: String(data.returnReason ?? record.return_reason ?? base.returnReason),
    evidenceProvided: toBoolean(data.evidenceProvided ?? record.evidence_provided, base.evidenceProvided),
    manualOverride: toBoolean(data.manualOverride ?? record.manual_override, base.manualOverride),
    decisionTimeMinutes: toNumber(
      data.decisionTimeMinutes ?? record.decision_time_minutes,
      base.decisionTimeMinutes,
    ),
    paymentMethod: String(data.paymentMethod ?? record.payment_method ?? base.paymentMethod ?? ""),
    shippingRegion: String(data.shippingRegion ?? record.shipping_region ?? base.shippingRegion ?? ""),
    reasons: normalizedReasons,
    relatedApprovals: normalizedRelatedApprovals,
  };
}

function getErrorMessage(error: unknown, fallback: string) {
  return error instanceof ApiError && error.message ? error.message : fallback;
}

function getInitialAnalysisSession() {
  const stored = loadAnalysisSession();
  const params = new URLSearchParams(window.location.search);
  return {
    datasetId: params.get("datasetId") || stored.datasetId,
    jobId: params.get("jobId") || stored.jobId,
  };
}

function getInitialRiskFilter(): "ALL" | RiskLevel {
  const value = new URLSearchParams(window.location.search).get("risk");
  return value === "LOW" || value === "MEDIUM" || value === "HIGH" || value === "CRITICAL" ? value : "ALL";
}

function getInitialOutcomeFilter(): "ALL" | InvestigationOutcome {
  const value = new URLSearchParams(window.location.search).get("outcome");
  return value && value in outcomeLabels ? (value as InvestigationOutcome) : "ALL";
}

function App() {
  const [initialSession] = useState(getInitialAnalysisSession);
  const [page, setPage] = useState<Page>(getInitialPage);
  const [uploadedFile, setUploadedFile] = useState("");
  const [uploadProgress, setUploadProgress] = useState(0);
  const [isUploading, setIsUploading] = useState(false);
  const [currentDatasetId, setCurrentDatasetId] = useState(initialSession.datasetId);
  const [currentJobId, setCurrentJobId] = useState(initialSession.jobId);
  const [previewData, setPreviewData] = useState<DatasetPreview>(() => normalizePreview([]));
  const [columnMapping, setColumnMapping] = useState<ColumnMapping>({});
  const [approvalRows, setApprovalRows] = useState<Approval[]>([]);
  const [selectedApproval, setSelectedApproval] = useState<ReturnRisk | null>(null);
  const [agentSummary, setAgentSummary] = useState<AgentSummary | null>(null);
  const [customerSummary, setCustomerSummary] = useState<CustomerSummary | null>(null);
  const [relationAgentSummary, setRelationAgentSummary] = useState<RelationAgentSummary | null>(null);
  const [relationGraph, setRelationGraph] = useState<GraphProjection | null>(null);
  const [contextStatus, setContextStatus] = useState<ApiStatus>("idle");
  const [contextError, setContextError] = useState("");
  const [uploadStatus, setUploadStatus] = useState<ApiStatus>("idle");
  const [analysisStatus, setAnalysisStatus] = useState<ApiStatus>("idle");
  const [approvalsStatus, setApprovalsStatus] = useState<ApiStatus>("idle");
  const [detailsStatus, setDetailsStatus] = useState<ApiStatus>("idle");
  const [riskFilter, setRiskFilter] = useState<"ALL" | RiskLevel>(getInitialRiskFilter);
  const [outcomeFilter, setOutcomeFilter] = useState<"ALL" | InvestigationOutcome>(getInitialOutcomeFilter);
  const [query, setQuery] = useState(() => new URLSearchParams(window.location.search).get("query") ?? "");
  const [sidebarCollapsed, setSidebarCollapsed] = useState(false);
  const [isAnalyzing, setIsAnalyzing] = useState(false);
  const [analysisStepIndex, setAnalysisStepIndex] = useState(-1);
  const [uploadError, setUploadError] = useState("");
  const [analysisError, setAnalysisError] = useState("");
  const [approvalsError, setApprovalsError] = useState("");
  const [detailsError, setDetailsError] = useState("");
  const [caseReview, setCaseReview] = useState<CaseReview>(defaultCaseReview);
  const [decisionStatus, setDecisionStatus] = useState<ApiStatus>("idle");
  const [decisionError, setDecisionError] = useState("");
  const [exportStatus, setExportStatus] = useState<ApiStatus>("idle");
  const [exportError, setExportError] = useState("");
  const [datasets, setDatasets] = useState<DatasetSummary[]>([]);
  const [datasetsStatus, setDatasetsStatus] = useState<ApiStatus>("idle");
  const [datasetsError, setDatasetsError] = useState("");
  const [datasetBusyId, setDatasetBusyId] = useState("");
  const [datasetDetails, setDatasetDetails] = useState<DatasetDetailsResponse | null>(null);

  const filteredApprovals = useMemo(() => {
    const normalized = query.trim().toLowerCase();
    return approvalRows.filter((approval) => {
      const matchesRisk = riskFilter === "ALL" || approval.riskLevel === riskFilter;
      const matchesSearch =
        !normalized ||
        [approval.returnId, approval.customerId, approval.supportAgentId, approval.orderId]
          .join(" ")
          .toLowerCase()
          .includes(normalized);
      return matchesRisk && matchesSearch;
    });
  }, [approvalRows, query, riskFilter]);

  const dataSourceMode = useMemo<DataSourceMode>(() => {
    const statuses = [uploadStatus, analysisStatus, approvalsStatus, detailsStatus, contextStatus];
    if (isUploading || isAnalyzing || statuses.includes("loading")) return "checking";
    if (statuses.includes("failed")) return "failed";
    if (statuses.includes("unavailable")) return "unavailable";
    if (statuses.includes("empty")) return "empty";
    if (statuses.includes("ready")) return "live";
    return "idle";
  }, [analysisStatus, approvalsStatus, contextStatus, detailsStatus, isAnalyzing, isUploading, uploadStatus]);

  useEffect(() => {
    if (currentDatasetId) {
      saveAnalysisSession({ datasetId: currentDatasetId, jobId: currentJobId });
    } else {
      clearAnalysisSession();
    }

    const params = new URLSearchParams(window.location.search);
    params.set("page", page);
    if (currentDatasetId) params.set("datasetId", currentDatasetId);
    else params.delete("datasetId");
    if (currentJobId) params.set("jobId", currentJobId);
    else params.delete("jobId");
    if (riskFilter !== "ALL") params.set("risk", riskFilter);
    else params.delete("risk");
    if (outcomeFilter !== "ALL") params.set("outcome", outcomeFilter);
    else params.delete("outcome");
    if (query.trim()) params.set("query", query.trim());
    else params.delete("query");
    window.history.replaceState(null, "", `${window.location.pathname}?${params.toString()}`);
  }, [currentDatasetId, currentJobId, outcomeFilter, page, query, riskFilter]);

  function navigate(nextPage: Page) {
    if (nextPage !== "details") {
      const params = new URLSearchParams(window.location.search);
      params.delete("returnId");
      window.history.replaceState(null, "", `${window.location.pathname}?${params.toString()}`);
    }
    setPage(nextPage);
  }

  useEffect(() => {
    void loadDatasetHistory();
  }, []);

  async function loadDatasetHistory() {
    setDatasetsStatus("loading");
    setDatasetsError("");
    try {
      const response = unwrapData(await listDatasets());
      const items = Array.isArray(response?.items) ? response.items : [];
      setDatasets(items);
      setDatasetsStatus(items.length > 0 ? "ready" : "empty");
    } catch (error) {
      setDatasets([]);
      setDatasetsStatus("unavailable");
      setDatasetsError(getErrorMessage(error, "Dataset history is unavailable."));
    }
  }

  useEffect(() => {
    if (!initialSession.datasetId) return;
    let cancelled = false;

    async function restore() {
      setUploadStatus("ready");
      try {
        const preview = normalizePreview(await getDatasetPreview(initialSession.datasetId));
        if (!cancelled) {
          setPreviewData(preview);
          setColumnMapping(detectColumnMapping(preview.headers));
        }
      } catch (error) {
        if (!cancelled) {
          setUploadStatus(error instanceof ApiError && error.status === 404 ? "failed" : "unavailable");
          setUploadError(getErrorMessage(error, "The saved dataset is no longer available."));
        }
      }

      let resultsReady = false;
      if (initialSession.jobId && !cancelled) {
        try {
          const savedStatus = unwrapData(await getAnalysisJobStatus(initialSession.jobId));
          const savedState = String(savedStatus.status ?? savedStatus.currentStep ?? "UPLOADED").toUpperCase();
          const stepIndex = statusSteps.indexOf(normalizeStatusStep(savedStatus));
          if (stepIndex >= 0) setAnalysisStepIndex(stepIndex);

          if (["COMPLETED", "READY", "DONE", "COMPLETE"].includes(savedState)) {
            setAnalysisStatus("ready");
            setAnalysisStepIndex(statusSteps.length - 1);
            resultsReady = true;
          } else if (savedState === "FAILED") {
            setAnalysisStatus("failed");
            setAnalysisError(savedStatus.errorMessage || savedStatus.message || "The saved analysis failed.");
          } else if (savedState !== "UPLOADED") {
            setIsAnalyzing(true);
            setAnalysisStatus("loading");
            const result = await pollAnalysisJob(initialSession.jobId, {
              onUpdate: (status) => {
                const currentIndex = statusSteps.indexOf(normalizeStatusStep(status));
                if (currentIndex >= 0) setAnalysisStepIndex(currentIndex);
              },
            });
            setIsAnalyzing(false);
            if (result.outcome === "completed") {
              setAnalysisStatus("ready");
              setAnalysisStepIndex(statusSteps.length - 1);
              resultsReady = true;
            } else if (result.outcome === "failed") {
              setAnalysisStatus("failed");
              setAnalysisError(result.status.errorMessage || result.status.message || "The saved analysis failed.");
            } else {
              setAnalysisStatus("unavailable");
              setAnalysisError("The saved analysis is still running. Status polling timed out.");
            }
          }
        } catch (error) {
          setAnalysisStatus(error instanceof ApiError && error.status === 404 ? "failed" : "unavailable");
          setAnalysisError(getErrorMessage(error, "The saved analysis status is unavailable."));
        }
      }

      if (getInitialPage() !== "dataset" && resultsReady && !cancelled) {
        const approvals = await loadSuspiciousApprovals(initialSession.datasetId);
        const returnId = new URLSearchParams(window.location.search).get("returnId");
        if (getInitialPage() === "details" && returnId && approvals.length > 0 && !cancelled) {
          const selected = approvals.find((approval) => approval.returnId === returnId);
          if (selected) await openApproval(selected);
        }
      }
    }

    void restore();
    return () => {
      cancelled = true;
    };
  }, []);

  async function handleUpload(event: React.ChangeEvent<HTMLInputElement>) {
    const file = event.target.files?.[0];
    if (!file) return;
    let uploadSucceeded = false;

    setUploadedFile(file.name);
    setIsUploading(true);
    setUploadStatus("loading");
    setAnalysisStatus("idle");
    setUploadProgress(35);
    setCurrentJobId("");
    setCurrentDatasetId("");
    setApprovalRows([]);
    setSelectedApproval(null);
    setAgentSummary(null);
    setCustomerSummary(null);
    setRelationAgentSummary(null);
    setRelationGraph(null);
    setContextStatus("idle");
    setApprovalsStatus("idle");
    setDetailsStatus("idle");
    setDecisionStatus("idle");
    setCaseReview(defaultCaseReview);
    setAnalysisStepIndex(-1);
    setUploadError("");
    setAnalysisError("");
    setApprovalsError("");
    setDetailsError("");

    try {
      const upload = normalizeUploadResponse(await uploadDataset(file));
      const uploadedDatasetId = upload.datasetId ?? "";
      if (!uploadedDatasetId) throw new Error("Missing uploaded dataset id");

      setCurrentDatasetId(uploadedDatasetId);
      setCurrentJobId(upload.jobId ?? "");
      setUploadProgress(70);

      const preview = normalizePreview(await getDatasetPreview(uploadedDatasetId));

      setPreviewData(preview);
      setColumnMapping(detectColumnMapping(preview.headers));
      setAnalysisStepIndex(0);
      setUploadStatus("ready");
      uploadSucceeded = true;
      await loadDatasetHistory();
    } catch (error) {
      setCurrentDatasetId("");
      setCurrentJobId("");
      setPreviewData(normalizePreview([]));
      setColumnMapping({});
      setAnalysisStepIndex(-1);
      setUploadStatus("failed");
      setUploadError(getErrorMessage(error, "Upload failed. Check the CSV and try again."));
    } finally {
      setUploadProgress(uploadSucceeded ? 100 : 0);
      setIsUploading(false);
    }
  }

  async function startAnalysis() {
    const datasetId = currentDatasetId;
    if (!datasetId) return;

    setIsAnalyzing(true);
    setAnalysisStatus("loading");
    setAnalysisStepIndex(0);
    setAnalysisError("");

    try {
      const startResponse = normalizeUploadResponse(await startDatasetAnalysis(datasetId));
      const jobId = startResponse.jobId || currentJobId;
      if (!jobId) throw new Error("Missing analysis job id");

      setCurrentJobId(jobId);
      await monitorAnalysisJob(jobId, datasetId);
    } catch (error) {
      setAnalysisStatus(error instanceof ApiError && error.status === 404 ? "failed" : "unavailable");
      setAnalysisError(getErrorMessage(error, "Analysis status is unavailable."));
    } finally {
      setIsAnalyzing(false);
    }
  }

  async function monitorAnalysisJob(jobId: string, datasetId: string) {
    const result = await pollAnalysisJob(jobId, {
      onUpdate: (status) => {
        const stepIndex = statusSteps.indexOf(normalizeStatusStep(status));
        if (stepIndex >= 0) setAnalysisStepIndex(stepIndex);
      },
    });

    if (result.outcome === "failed") {
      setAnalysisStatus("failed");
      setAnalysisError(result.status.errorMessage || result.status.message || "Analysis failed.");
      await loadDatasetHistory();
      return false;
    }
    if (result.outcome === "timeout") {
      setAnalysisStatus("unavailable");
      setAnalysisError("Analysis is still running, but status polling timed out. Refresh to resume this job.");
      return false;
    }

    setAnalysisStepIndex(statusSteps.length - 1);
    setAnalysisStatus("ready");
    await loadSuspiciousApprovals(datasetId, outcomeFilter);
    await loadDatasetHistory();
    navigate("approvals");
    return true;
  }

  async function loadSuspiciousApprovals(
    datasetId: string,
    outcome: "ALL" | InvestigationOutcome = outcomeFilter,
  ): Promise<Approval[]> {
    setApprovalsStatus("loading");
    setApprovalsError("");
    try {
      const raw = await getSuspiciousApprovals(datasetId, {
        outcome: outcome === "ALL" ? undefined : outcome,
      });
      const normalized = normalizeApprovals(raw, datasetId);
      setApprovalRows(normalized);
      setApprovalsStatus(normalized.length > 0 ? "ready" : "empty");
      return normalized;
    } catch (error) {
      setApprovalRows([]);
      setApprovalsStatus(error instanceof ApiError && error.status === 404 ? "failed" : "unavailable");
      setApprovalsError(getErrorMessage(error, "Scoring results are unavailable."));
      return [];
    }
  }

  async function changeOutcomeFilter(value: "ALL" | InvestigationOutcome) {
    setOutcomeFilter(value);
    if (currentDatasetId && analysisStatus === "ready") {
      await loadSuspiciousApprovals(currentDatasetId, value);
    }
  }

  async function downloadApprovals() {
    if (!currentDatasetId) return;
    setExportStatus("loading");
    setExportError("");
    try {
      const { blob, filename } = await exportSuspiciousApprovals(currentDatasetId, {
        risk: riskFilter === "ALL" ? undefined : riskFilter,
        outcome: outcomeFilter === "ALL" ? undefined : outcomeFilter,
      });
      const href = URL.createObjectURL(blob);
      const anchor = document.createElement("a");
      anchor.href = href;
      anchor.download = filename;
      anchor.click();
      URL.revokeObjectURL(href);
      setExportStatus("ready");
    } catch (error) {
      setExportStatus("failed");
      setExportError(getErrorMessage(error, "CSV export failed."));
    }
  }

  async function selectDataset(dataset: DatasetSummary) {
    setDatasetBusyId(dataset.id);
    setCurrentDatasetId(dataset.id);
    setCurrentJobId(dataset.latestJobId ?? "");
    setUploadedFile(dataset.filename || dataset.name);
    setUploadStatus("loading");
    setAnalysisError("");
    try {
      const [previewResponse, detailsResponse] = await Promise.all([
        getDatasetPreview(dataset.id),
        getDatasetDetails(dataset.id),
      ]);
      const preview = normalizePreview(previewResponse);
      const details = unwrapData(detailsResponse);
      setDatasetDetails(
        details && Array.isArray(details.analysisHistory) && Array.isArray(details.auditEvents)
          ? details
          : null,
      );
      setPreviewData(preview);
      setColumnMapping(detectColumnMapping(preview.headers));
      setUploadStatus("ready");
      if (dataset.status === "COMPLETED" && dataset.resultReady) {
        setAnalysisStatus("ready");
        setAnalysisStepIndex(statusSteps.length - 1);
        await loadSuspiciousApprovals(dataset.id, outcomeFilter);
        navigate("approvals");
      } else {
        setAnalysisStatus(dataset.status === "FAILED" ? "failed" : "idle");
        navigate("dataset");
      }
    } catch (error) {
      setUploadStatus("unavailable");
      setUploadError(getErrorMessage(error, "Dataset preview is unavailable."));
    } finally {
      setDatasetBusyId("");
    }
  }

  async function retryDataset(dataset?: DatasetSummary) {
    const jobId = dataset?.latestJobId ?? currentJobId;
    const datasetId = dataset?.id ?? currentDatasetId;
    if (!jobId || !datasetId) return;

    setDatasetBusyId(datasetId);
    setIsAnalyzing(true);
    setAnalysisStatus("loading");
    setAnalysisError("");
    setAnalysisStepIndex(0);
    try {
      const response = unwrapData(await retryAnalysisJob(jobId));
      const retryJobId = response?.jobId ?? "";
      if (!retryJobId) throw new Error("Missing retry job id");
      setCurrentDatasetId(datasetId);
      setCurrentJobId(retryJobId);
      await monitorAnalysisJob(retryJobId, datasetId);
    } catch (error) {
      setAnalysisStatus("failed");
      setAnalysisError(getErrorMessage(error, "Analysis retry failed."));
    } finally {
      setDatasetBusyId("");
      setIsAnalyzing(false);
    }
  }

  async function archiveHistoryDataset(dataset: DatasetSummary) {
    if (!window.confirm(`Archive ${dataset.filename || dataset.id}?`)) return;
    setDatasetBusyId(dataset.id);
    setDatasetsError("");
    try {
      await archiveDataset(dataset.id);
      if (dataset.id === currentDatasetId) {
        setCurrentDatasetId("");
        setCurrentJobId("");
        setUploadedFile("");
        setPreviewData(normalizePreview([]));
        setColumnMapping({});
        setApprovalRows([]);
        setSelectedApproval(null);
        setDatasetDetails(null);
        setUploadStatus("idle");
        setAnalysisStatus("idle");
      }
      await loadDatasetHistory();
    } catch (error) {
      setDatasetsStatus("failed");
      setDatasetsError(getErrorMessage(error, "Dataset archive failed."));
    } finally {
      setDatasetBusyId("");
    }
  }

  async function loadDecision(datasetId: string, returnId: string) {
    setDecisionStatus("loading");
    setDecisionError("");
    try {
      const decision = await getInvestigationDecision(datasetId, returnId);
      if (!decision.action || !decision.outcome) {
        setCaseReview(defaultCaseReview);
        setDecisionStatus("empty");
        return;
      }
      setCaseReview({
        action: decision.action,
        outcome: decision.outcome,
        note: decision.note,
        analystId: decision.analystId,
        updatedAt: decision.updatedAt,
      });
      setDecisionStatus("ready");
    } catch (error) {
      if (error instanceof ApiError && error.status === 404) {
        setCaseReview(defaultCaseReview);
        setDecisionStatus("empty");
      } else {
        setDecisionStatus("unavailable");
        setDecisionError(getErrorMessage(error, "Analyst decision is unavailable."));
      }
    }
  }

  async function saveDecision() {
    if (!selectedApproval || !currentDatasetId) return;
    if (
      ["CONFIRMED_FRAUD", "FALSE_POSITIVE", "RESOLVED"].includes(caseReview.outcome) &&
      !window.confirm(`Save final outcome “${outcomeLabels[caseReview.outcome]}”?`)
    ) {
      return;
    }
    setDecisionStatus("loading");
    setDecisionError("");
    try {
      const saved = await saveInvestigationDecision(currentDatasetId, selectedApproval.returnId, caseReview);
      setCaseReview({
        action: saved.action,
        outcome: saved.outcome,
        note: saved.note,
        analystId: saved.analystId,
        updatedAt: saved.updatedAt,
      });
      setDecisionStatus("ready");
    } catch (error) {
      setDecisionStatus("failed");
      setDecisionError(getErrorMessage(error, "Analyst decision could not be saved."));
    }
  }

  async function loadAgentSummary(datasetId: string, agentId: string) {
    try {
      const raw = await getScoringAgentSummary(datasetId, agentId);
      setAgentSummary(normalizeAgentSummary(raw, agentId));
    } catch {
      setAgentSummary(null);
    }
  }

  async function loadInvestigationContext(datasetId: string, approval: Approval) {
    setContextStatus("loading");
    setContextError("");
    setCustomerSummary(null);
    setRelationAgentSummary(null);
    setRelationGraph(null);

    const [customerResult, agentResult, graphResult] = await Promise.allSettled([
      getCustomerSummary(datasetId, approval.customerId),
      getRelationAgentSummary(datasetId, approval.supportAgentId),
      getReturnGraph(datasetId, approval.returnId),
    ]);

    if (customerResult.status === "fulfilled") setCustomerSummary(customerResult.value);
    if (agentResult.status === "fulfilled") setRelationAgentSummary(agentResult.value);
    if (graphResult.status === "fulfilled") setRelationGraph(graphResult.value);

    const successes = [customerResult, agentResult, graphResult].filter((result) => result.status === "fulfilled").length;
    if (successes === 0) {
      setContextStatus("unavailable");
      const firstError = [customerResult, agentResult, graphResult].find((result) => result.status === "rejected");
      setContextError(
        firstError?.status === "rejected"
          ? getErrorMessage(firstError.reason, "Relations analytics are unavailable.")
          : "Relations analytics are unavailable.",
      );
    } else {
      setContextStatus("ready");
      if (successes < 3) setContextError("Some relation analytics could not be loaded.");
    }
  }

  async function openApproval(approval: Approval) {
    const detailDatasetId = approval.datasetId ?? currentDatasetId;
    if (!detailDatasetId) return;

    setSelectedApproval(buildReturnDetailsBase(approval));
    setAgentSummary(null);
    setCustomerSummary(null);
    setRelationAgentSummary(null);
    setRelationGraph(null);
    setContextStatus("loading");
    setContextError("");
    setCaseReview(defaultCaseReview);
    setDecisionStatus("loading");
    setDecisionError("");
    setDetailsStatus("loading");
    setDetailsError("");
    const params = new URLSearchParams(window.location.search);
    params.set("returnId", approval.returnId);
    window.history.replaceState(null, "", `${window.location.pathname}?${params.toString()}`);
    navigate("details");

    try {
      const data = await getReturnDetails(detailDatasetId, approval.returnId);
      const details = toReturnRisk(unwrapData(data) as ReturnDetailsResponse, approval);
      setSelectedApproval(details);
      setDetailsStatus("ready");
      await Promise.all([
        loadAgentSummary(detailDatasetId, details.supportAgentId),
        loadInvestigationContext(detailDatasetId, details),
        loadDecision(detailDatasetId, details.returnId),
      ]);
    } catch (error) {
      setSelectedApproval(null);
      setAgentSummary(null);
      setDecisionStatus("idle");
      setDetailsStatus(error instanceof ApiError && error.status === 404 ? "failed" : "unavailable");
      setDetailsError(getErrorMessage(error, "Investigation details are unavailable."));
    }
  }

  return (
    <div className={sidebarCollapsed ? "app-shell sidebar-collapsed" : "app-shell"}>
      <CSidebar className="app-sidebar" narrow={sidebarCollapsed} unfoldable={false} visible>
        <CSidebarHeader>
          <CSidebarBrand className="sidebar-brand">
            {sidebarCollapsed ? (
              <img alt="Anti-Fraud" className="brand-mark" src={antiFraudLogoMark} />
            ) : (
              <img alt="Anti-Fraud" className="brand-logo" src={antiFraudLogoFull} />
            )}
          </CSidebarBrand>
        </CSidebarHeader>

        <CNav className="sidebar-nav" variant="pills">
          {navItems.map((item) => {
            const Icon = item.icon;
            return (
              <CNavItem key={item.page}>
                <CNavLink active={page === item.page} as="button" onClick={() => navigate(item.page)}>
                  <Icon size={18} />
                  {!sidebarCollapsed && <span>{item.label}</span>}
                </CNavLink>
              </CNavItem>
            );
          })}
        </CNav>

        <CSidebarFooter>
          <div className="sidebar-collapse-row">
            <CButton
              color="secondary"
              size="sm"
              variant="ghost"
              className="sidebar-toggle"
              onClick={() => setSidebarCollapsed((value) => !value)}
              title={sidebarCollapsed ? "Expand sidebar" : "Collapse sidebar"}
            >
              {sidebarCollapsed ? <ChevronRight size={18} /> : <ChevronLeft size={18} />}
              {!sidebarCollapsed && <span>Collapse sidebar</span>}
            </CButton>
          </div>
        </CSidebarFooter>
      </CSidebar>

      <main className="main">
        <CHeader className="app-header">
          <CHeaderBrand as="h1">Refund approval risk analytics</CHeaderBrand>
          <DataSourceBanner mode={dataSourceMode} />
        </CHeader>

        {page === "dataset" && (
          <DatasetPage
            uploadedFile={uploadedFile}
            uploadProgress={uploadProgress}
            isUploading={isUploading}
            uploadStatus={uploadStatus}
            datasetId={currentDatasetId}
            jobId={currentJobId}
            previewData={previewData}
            columnMapping={columnMapping}
            isAnalyzing={isAnalyzing}
            analysisStatus={analysisStatus}
            uploadError={uploadError}
            analysisError={analysisError}
            analysisStepIndex={analysisStepIndex}
            canStartAnalysis={Boolean(currentDatasetId) && uploadStatus === "ready" && analysisStatus !== "failed"}
            datasets={datasets}
            datasetsStatus={datasetsStatus}
            datasetsError={datasetsError}
            datasetBusyId={datasetBusyId}
            datasetDetails={datasetDetails}
            onUpload={handleUpload}
            onStart={startAnalysis}
            onRetry={() => void retryDataset()}
            onRefreshDatasets={() => void loadDatasetHistory()}
            onSelectDataset={(dataset) => void selectDataset(dataset)}
            onRetryDataset={(dataset) => void retryDataset(dataset)}
            onArchiveDataset={(dataset) => void archiveHistoryDataset(dataset)}
          />
        )}
        {page === "approvals" && (
          <ApprovalsPage
            approvals={filteredApprovals}
            query={query}
            riskFilter={riskFilter}
            outcomeFilter={outcomeFilter}
            onQueryChange={setQuery}
            onRiskFilterChange={setRiskFilter}
            onOutcomeFilterChange={(value) => void changeOutcomeFilter(value)}
            onExport={() => void downloadApprovals()}
            exportStatus={exportStatus}
            exportError={exportError}
            onOpenApproval={openApproval}
            status={approvalsStatus}
            error={approvalsError}
          />
        )}
        {page === "details" && (
          <DetailsPage
            agentSummary={agentSummary}
            approval={selectedApproval}
            onOpenApproval={openApproval}
            status={detailsStatus}
            error={detailsError}
            customerSummary={customerSummary}
            relationAgentSummary={relationAgentSummary}
            relationGraph={relationGraph}
            contextStatus={contextStatus}
            contextError={contextError}
            review={caseReview}
            decisionStatus={decisionStatus}
            decisionError={decisionError}
            onReviewChange={(_returnId, patch) => setCaseReview((current) => ({ ...current, ...patch }))}
            onSaveDecision={() => void saveDecision()}
          />
        )}
      </main>
    </div>
  );
}

function DatasetPage({
  uploadedFile,
  uploadProgress,
  isUploading,
  uploadStatus,
  datasetId,
  jobId,
  previewData,
  columnMapping,
  isAnalyzing,
  analysisStatus,
  uploadError,
  analysisError,
  analysisStepIndex,
  canStartAnalysis,
  datasets,
  datasetsStatus,
  datasetsError,
  datasetBusyId,
  datasetDetails,
  onUpload,
  onStart,
  onRetry,
  onRefreshDatasets,
  onSelectDataset,
  onRetryDataset,
  onArchiveDataset,
}: {
  uploadedFile: string;
  uploadProgress: number;
  isUploading: boolean;
  uploadStatus: ApiStatus;
  datasetId: string;
  jobId: string;
  previewData: DatasetPreview;
  columnMapping: ColumnMapping;
  isAnalyzing: boolean;
  analysisStatus: ApiStatus;
  uploadError: string;
  analysisError: string;
  analysisStepIndex: number;
  canStartAnalysis: boolean;
  datasets: DatasetSummary[];
  datasetsStatus: ApiStatus;
  datasetsError: string;
  datasetBusyId: string;
  datasetDetails: DatasetDetailsResponse | null;
  onUpload: (event: React.ChangeEvent<HTMLInputElement>) => void;
  onStart: () => void;
  onRetry: () => void;
  onRefreshDatasets: () => void;
  onSelectDataset: (dataset: DatasetSummary) => void;
  onRetryDataset: (dataset: DatasetSummary) => void;
  onArchiveDataset: (dataset: DatasetSummary) => void;
}) {
  const hasPreview = previewData.headers.length > 0 || previewData.rows.length > 0;
  const hasMapping = Object.keys(columnMapping).length > 0;
  const mappingWarnings = getMappingWarnings(previewData.headers);

  return (
    <CContainer fluid className="px-0">
      <CRow className="g-4 mb-4">
        <CCol lg={5}>
          <CCard className="h-100">
            <CCardHeader>
              <SectionTitle icon={Upload} title="Upload dataset" text="Upload a CSV file and review the detected structure." />
            </CCardHeader>
            <CCardBody>
              {uploadStatus === "failed" && (
                <ApiNotice tone="warning" text={uploadError || "Upload failed. Check the CSV and try again."} />
              )}
              {uploadStatus === "unavailable" && (
                <ApiNotice tone="warning" text={uploadError || "The saved dataset is unavailable."} />
              )}
              <CFormLabel className="dropzone">
                <FileSpreadsheet size={34} />
                <strong>{uploadedFile || "No file selected"}</strong>
                <span>Choose CSV file</span>
                <CFormInput accept=".csv" className="visually-hidden" onChange={onUpload} type="file" />
              </CFormLabel>

              <div className="sample-actions">
                <CButton color="primary" href={sampleCsvHref} download="anti_fraud_sample_refunds.csv" variant="outline">
                  <Download size={17} />
                  Download sample CSV
                </CButton>
                <span>Accepted aliases are listed in the schema checklist.</span>
              </div>

              <div className="d-flex justify-content-between mt-4 mb-2">
                <span>Upload progress</span>
                <strong>{uploadProgress}%</strong>
              </div>
              <CProgress className="mb-4" height={10}>
                <CProgressBar color="primary" value={uploadProgress} />
              </CProgress>

              <CListGroup flush className="mb-4">
                {datasetId ? (
                  <CListGroupItem className="info-row">
                    <span className="text-body-secondary">Dataset ID</span>
                    <strong>{datasetId}</strong>
                  </CListGroupItem>
                ) : (
                  <CListGroupItem className="info-row">
                    <span className="text-body-secondary">Dataset ID</span>
                    <strong>Upload required</strong>
                  </CListGroupItem>
                )}
                {jobId && (
                  <CListGroupItem className="info-row">
                    <span className="text-body-secondary">Job ID</span>
                    <strong>{jobId}</strong>
                  </CListGroupItem>
                )}
              </CListGroup>

              {(isUploading || isAnalyzing) && (
                <div className="document-loader mb-4" role="status" aria-live="polite">
                  <div className="document-loader-icon">
                    <FileSpreadsheet size={24} />
                  </div>
                  <div className="flex-grow-1">
                    <strong>{isUploading ? "Uploading document" : "Preparing document"}</strong>
                    <CProgress className="mt-2" height={8}>
                      <CProgressBar color="primary" animated value={100} />
                    </CProgress>
                  </div>
                </div>
              )}

              <CButton color="primary" disabled={!canStartAnalysis || isUploading || isAnalyzing} onClick={onStart}>
                {isAnalyzing ? (
                  <>
                    Analyzing <span className="button-spinner" aria-hidden="true" />
                  </>
                ) : isUploading ? (
                  <>
                    Uploading <span className="button-spinner" aria-hidden="true" />
                  </>
                ) : (
                  <>
                    Start analysis <ArrowRight size={17} />
                  </>
                )}
              </CButton>
            </CCardBody>
          </CCard>
        </CCol>

        <CCol lg={7}>
          <CCard className="h-100">
            <CCardHeader>
              <SectionTitle icon={Clock3} title="Analysis progress" text="The pipeline is complete when scored approvals are ready." />
            </CCardHeader>
            <CCardBody>
              {analysisStatus === "failed" && (
                <>
                  <ApiNotice tone="warning" text={analysisError || "Analysis failed. Retry this job or upload a corrected dataset."} />
                  {jobId && (
                    <CButton className="mb-3" color="primary" disabled={isAnalyzing} onClick={onRetry} variant="outline">
                      Retry analysis
                    </CButton>
                  )}
                </>
              )}
              {analysisStatus === "unavailable" && (
                <ApiNotice tone="warning" text={analysisError || "Analysis status is unavailable."} />
              )}
              <AnalysisStatusList analysisStepIndex={analysisStepIndex} isAnalyzing={isAnalyzing} />
            </CCardBody>
          </CCard>
        </CCol>
      </CRow>

      <CRow className="g-4">
        <CCol xl={8}>
          <CCard className="h-100">
            <CCardHeader>
              <SectionTitle icon={FileSpreadsheet} title="Dataset preview" text="Rows returned by the upload service preview endpoint." />
            </CCardHeader>
            <CCardBody>
              {hasPreview ? (
                <>
                  <div className="preview-meta">
                    <strong>{previewData.rowCount} rows detected</strong>
                    {previewData.truncated && <span>Preview limited to {previewData.rows.length} rows</span>}
                  </div>
                  {mappingWarnings.map((warning) => (
                    <ApiNotice key={warning} tone="warning" text={warning} />
                  ))}
                  <DataTable
                    columns={previewData.headers}
                    rows={previewData.rows}
                    renderCell={(row, column) => formatPreviewCell(String(row[column] ?? ""))}
                  />
                </>
              ) : (
                <EmptyState title="No dataset uploaded" text="Choose a CSV file to preview its rows." />
              )}
            </CCardBody>
          </CCard>
        </CCol>
        <CCol xl={4}>
          <CCard className="mb-4">
            <CCardHeader>
              <SectionTitle icon={Info} title="Accepted CSV schema" text="Required fields must map to the normalized refund model." />
            </CCardHeader>
            <CListGroup flush>
              {csvSchema.map((field) => (
                <CListGroupItem className="schema-item" key={field.semantic}>
                  <div>
                    <strong>{field.semantic}</strong>
                    <div className="text-body-secondary small">{field.description}</div>
                    <div className="schema-aliases">{field.accepted.join(", ")}</div>
                  </div>
                  <CBadge color={field.required ? "danger" : "secondary"}>
                    {field.required ? "Required" : "Optional"}
                  </CBadge>
                </CListGroupItem>
              ))}
            </CListGroup>
          </CCard>

          <CCard>
            <CCardHeader>
              <SectionTitle icon={Filter} title="Column mapping" text="Detected normalization contract for uploaded rows." />
            </CCardHeader>
            <CListGroup flush>
              {hasMapping ? (
                Object.entries(columnMapping).map(([source, target]) => (
                  <CListGroupItem className="d-flex mapping-item" key={source}>
                    <span className="text-body-secondary">{source}</span>
                    <strong>{target}</strong>
                  </CListGroupItem>
                ))
              ) : (
                <CListGroupItem>
                  <EmptyState title="No mapping yet" text="Column mapping appears after a dataset upload." />
                </CListGroupItem>
              )}
            </CListGroup>
          </CCard>
        </CCol>
      </CRow>

      <DatasetHistory
        busyId={datasetBusyId}
        currentDatasetId={datasetId}
        details={datasetDetails}
        error={datasetsError}
        items={datasets}
        onArchive={onArchiveDataset}
        onRefresh={onRefreshDatasets}
        onRetry={onRetryDataset}
        onSelect={onSelectDataset}
        status={datasetsStatus}
      />

    </CContainer>
  );
}

export function ApprovalsPage({
  approvals,
  query,
  riskFilter,
  outcomeFilter,
  onQueryChange,
  onRiskFilterChange,
  onOutcomeFilterChange,
  onExport,
  exportStatus = "idle",
  exportError = "",
  onOpenApproval,
  status,
  error = "",
}: {
  approvals: Approval[];
  query: string;
  riskFilter: "ALL" | RiskLevel;
  outcomeFilter?: "ALL" | InvestigationOutcome;
  onQueryChange: (value: string) => void;
  onRiskFilterChange: (value: "ALL" | RiskLevel) => void;
  onOutcomeFilterChange?: (value: "ALL" | InvestigationOutcome) => void;
  onExport?: () => void;
  exportStatus?: ApiStatus;
  exportError?: string;
  onOpenApproval: (approval: Approval) => void;
  status: ApiStatus;
  error?: string;
}) {
  const [currentPage, setCurrentPage] = useState(0);
  const criticalCount = approvals.filter((approval) => approval.riskLevel === "CRITICAL").length;
  const highCount = approvals.filter((approval) => approval.riskLevel === "HIGH").length;
  const averageScore =
    approvals.length > 0
      ? (approvals.reduce((total, approval) => total + approval.riskScore, 0) / approvals.length).toFixed(1)
      : "0.0";
  const pageCount = Math.max(1, Math.ceil(approvals.length / approvalsPageSize));
  const safePage = Math.min(currentPage, pageCount - 1);
  const pageStart = safePage * approvalsPageSize;
  const pageEnd = Math.min(pageStart + approvalsPageSize, approvals.length);
  const paginatedApprovals = approvals.slice(pageStart, pageEnd);

  useEffect(() => {
    setCurrentPage(0);
  }, [query, riskFilter, outcomeFilter, approvals.length]);

  function handleQueryChange(value: string) {
    setCurrentPage(0);
    onQueryChange(value);
  }

  function handleRiskFilterChange(value: "ALL" | RiskLevel) {
    setCurrentPage(0);
    onRiskFilterChange(value);
  }

  return (
    <CContainer fluid className="px-0">
      <CRow className="g-3 mb-4">
        <MetricCard icon={ShieldAlert} label="Critical cases" value={String(criticalCount)} color="danger" />
        <MetricCard icon={AlertTriangle} label="High-risk cases" value={String(highCount)} color="warning" />
        <MetricCard icon={BarChart3} label="Average risk score" value={averageScore} />
      </CRow>

      <CCard>
        <CCardHeader className="d-flex justify-content-between align-items-start gap-3 flex-wrap">
          <div>
            <strong>Suspicious approvals</strong>
            <CFormText>Search by return, order, customer, or agent.</CFormText>
          </div>
          <div className="filters">
            <CButton
              color="secondary"
              disabled={!onExport || exportStatus === "loading"}
              onClick={onExport}
              variant="outline"
            >
              <Download size={16} />
              {exportStatus === "loading" ? "Downloading…" : "Download CSV"}
            </CButton>
            <CInputGroup>
              <CInputGroupText>
                <Search size={16} />
              </CInputGroupText>
              <CFormInput
                onChange={(event) => handleQueryChange(event.target.value)}
                placeholder="Search ID"
                value={query}
              />
            </CInputGroup>
            <CInputGroup>
              <CInputGroupText>
                <Filter size={16} />
              </CInputGroupText>
              <CFormSelect
                onChange={(event) => handleRiskFilterChange(event.target.value as "ALL" | RiskLevel)}
                value={riskFilter}
              >
                <option value="ALL">All risk levels</option>
                <option value="CRITICAL">Critical</option>
                <option value="HIGH">High</option>
                <option value="MEDIUM">Medium</option>
                <option value="LOW">Low</option>
              </CFormSelect>
            </CInputGroup>
            <CInputGroup>
              <CInputGroupText>
                <CircleDot size={16} />
              </CInputGroupText>
              <CFormSelect
                aria-label="Investigation outcome"
                onChange={(event) => onOutcomeFilterChange?.(event.target.value as "ALL" | InvestigationOutcome)}
                value={outcomeFilter ?? "ALL"}
              >
                <option value="ALL">All outcomes</option>
                {Object.entries(outcomeLabels).map(([value, label]) => (
                  <option key={value} value={value}>{label}</option>
                ))}
              </CFormSelect>
            </CInputGroup>
          </div>
        </CCardHeader>
        <CCardBody>
          {status === "loading" && <ApiNotice tone="loading" text="Loading suspicious approvals from scoring API." />}
          {(status === "failed" || status === "unavailable") && (
            <ApiNotice tone="warning" text={error || "Scoring results are unavailable."} />
          )}
          {exportStatus === "failed" && <ApiNotice tone="warning" text={exportError || "CSV export failed."} />}
          {approvals.length > 0 ? (
            <>
              <DataTable
                columns={[
                  "returnId",
                  "customerId",
                  "supportAgentId",
                  "refundAmount",
                  "decision",
                  "riskScore",
                  "riskLevel",
                  "topReason",
                  "",
                ]}
                rows={paginatedApprovals}
                renderCell={(row, column) => {
                  if (column === "refundAmount") return formatMoney(row.refundAmount);
                  if (column === "decision") return formatEnum(row.decision);
                  if (column === "riskLevel") return <RiskBadge level={row.riskLevel} />;
                  if (column === "riskScore") return <ScoreBar score={row.riskScore} />;
                  if (column === "") {
                    return (
                      <CButton color="primary" size="sm" variant="outline" onClick={() => onOpenApproval(row)}>
                        Review
                      </CButton>
                    );
                  }
                  return row[column as keyof Approval] as string | number;
                }}
              />
              <div className="pagination-row">
                <span>
                  Showing {pageStart + 1}-{pageEnd} of {approvals.length}
                </span>
                <div className="pagination-actions">
                  <CButton
                    aria-label="Previous page"
                    color="secondary"
                    disabled={safePage === 0}
                    size="sm"
                    variant="outline"
                    onClick={() => setCurrentPage((page) => Math.max(page - 1, 0))}
                  >
                    <ChevronLeft size={16} />
                  </CButton>
                  <strong>
                    {safePage + 1} / {pageCount}
                  </strong>
                  <CButton
                    aria-label="Next page"
                    color="secondary"
                    disabled={safePage >= pageCount - 1}
                    size="sm"
                    variant="outline"
                    onClick={() => setCurrentPage((page) => Math.min(page + 1, pageCount - 1))}
                  >
                    <ChevronRight size={16} />
                  </CButton>
                </div>
              </div>
            </>
          ) : status === "empty" ? (
            <EmptyState title="No suspicious cases" text="Analysis completed successfully and found no approvals above the risk threshold." />
          ) : status === "idle" ? (
            <EmptyState title="No approvals loaded" text="Upload a dataset and run analysis to review suspicious approvals." />
          ) : null}
        </CCardBody>
      </CCard>
    </CContainer>
  );
}

export function DetailsPage({
  agentSummary,
  approval,
  onOpenApproval,
  review = defaultCaseReview,
  onReviewChange = () => undefined,
  status,
  error = "",
  customerSummary = null,
  relationAgentSummary = null,
  relationGraph = null,
  contextStatus = "idle",
  contextError = "",
  decisionStatus = "idle",
  decisionError = "",
  onSaveDecision = () => undefined,
}: {
  agentSummary: AgentSummary | null;
  approval: ReturnRisk | null;
  onOpenApproval: (approval: Approval) => void;
  review?: CaseReview;
  onReviewChange?: (returnId: string, patch: Partial<CaseReview>) => void;
  status: ApiStatus;
  error?: string;
  customerSummary?: CustomerSummary | null;
  relationAgentSummary?: RelationAgentSummary | null;
  relationGraph?: GraphProjection | null;
  contextStatus?: ApiStatus;
  contextError?: string;
  decisionStatus?: ApiStatus;
  decisionError?: string;
  onSaveDecision?: () => void;
}) {
  if (!approval) {
    return (
      <CContainer fluid className="px-0">
        {(status === "failed" || status === "unavailable") && (
          <ApiNotice tone="warning" text={error || "Investigation details are unavailable."} />
        )}
        {status === "loading" ? (
          <ApiNotice tone="loading" text="Loading return details and explanations." />
        ) : (
          <EmptyState title="No investigation selected" text="Open an approval after analysis to inspect its risk details." />
        )}
      </CContainer>
    );
  }

  if (status === "loading") {
    return (
      <CContainer fluid className="px-0">
        <ApiNotice tone="loading" text="Loading return details and explanations." />
      </CContainer>
    );
  }

  const summary = agentSummary;

  return (
    <CContainer fluid className="px-0">
      <CCard className="mb-4">
        <CCardBody className="details-hero">
          <div>
            <div className="eyebrow">Refund approval details</div>
            <h2>{approval.returnId}</h2>
            <p className="text-body-secondary mb-0">{approval.topReason}</p>
          </div>
          <div className="risk-summary">
            <div className="text-body-secondary">Risk score</div>
            <strong>{approval.riskScore}</strong>
            <RiskBadge level={approval.riskLevel} />
          </div>
        </CCardBody>
      </CCard>

      <CRow className="g-4 mb-4">
        <InfoPanel
          title="Return request"
          rows={[
            ["Return ID", approval.returnId],
            ["Customer", <CButton as="a" color="link" href="#customer-analytics-title" key="customer-link" size="sm">{approval.customerId}</CButton>],
            ["Reason", formatEnum(approval.returnReason)],
            ["Refund amount", formatMoney(approval.refundAmount)],
            ["Evidence", approval.evidenceProvided ? "Provided" : "Missing"],
          ]}
        />
        <InfoPanel
          title="Order"
          rows={[
            ["Order ID", approval.orderId],
            ["Order amount", formatMoney(approval.orderAmount)],
            ["Category", formatEnum(approval.productCategory)],
            ["Payment", approval.paymentMethod ? formatEnum(approval.paymentMethod) : "Unknown"],
          ]}
        />
        <InfoPanel
          title="Decision"
          rows={[
            ["Agent", <CButton as="a" color="link" href="#agent-analytics-title" key="agent-link" size="sm">{approval.supportAgentId}</CButton>],
            ["Result", formatEnum(approval.decision)],
            ["Manual override", approval.manualOverride ? "Yes" : "No"],
            ["Decision time", `${approval.decisionTimeMinutes} min`],
          ]}
        />
      </CRow>

      <CRow className="g-4 mb-4">
        <CCol lg={7}>
          <CCard className="h-100">
            <CCardHeader>
              <strong>Why was this refund flagged?</strong>
            </CCardHeader>
            <CListGroup flush>
              {approval.reasons.map((item) => {
                const rule = getScoringRule(item.type);
                return (
                  <CListGroupItem className="explanation-item" key={item.type}>
                    <div>
                      <strong>{rule?.label ?? formatEnum(item.type)}</strong>
                      <div className="text-body-secondary">{item.message}</div>
                      {rule && (
                        <div className="rule-meta">
                          <span>{rule.threshold}</span>
                          <span>{rule.source}</span>
                        </div>
                      )}
                    </div>
                    <CBadge color="danger">+{item.scoreImpact}</CBadge>
                  </CListGroupItem>
                );
              })}
            </CListGroup>
          </CCard>
        </CCol>
        <CCol lg={5}>
          <CCard className="mb-4">
            <CCardHeader>
              <strong>Investigation context</strong>
            </CCardHeader>
            <CCardBody>
              <CRow className="g-3">
                <Signal
                  icon={UserRound}
                  title="Repeated customer pairs"
                  value={relationAgentSummary ? String(relationAgentSummary.repeatedCustomerPairCount) : "Unavailable"}
                  color="warning"
                />
                <Signal
                  icon={UsersRound}
                  title="Agent approval rate"
                  value={relationAgentSummary ? `${(relationAgentSummary.approvalRate <= 1 ? relationAgentSummary.approvalRate * 100 : relationAgentSummary.approvalRate).toFixed(1)}%` : "Unavailable"}
                  color="danger"
                />
                <Signal
                  icon={Gauge}
                  title="Manual overrides"
                  value={relationAgentSummary ? String(relationAgentSummary.manualOverrideCount) : "Unavailable"}
                  color="danger"
                />
              </CRow>
              <CListGroup flush className="mt-3">
                <CListGroupItem className="info-row">
                  <span className="text-body-secondary">Agent summary</span>
                  <strong>{summary?.agentId ?? approval.supportAgentId}</strong>
                </CListGroupItem>
                <CListGroupItem className="info-row">
                  <span className="text-body-secondary">High-risk approvals</span>
                  <strong>{summary?.highRiskApprovals ?? "Unavailable"}</strong>
                </CListGroupItem>
              </CListGroup>
            </CCardBody>
          </CCard>

          <CCard>
            <CCardHeader>
              <strong>Scoring thresholds</strong>
            </CCardHeader>
            <CListGroup flush>
              {riskThresholds.map((threshold) => (
                <CListGroupItem className="threshold-item" key={threshold.level}>
                  <RiskBadge level={threshold.level} />
                  <strong>{threshold.range}</strong>
                  <span className="text-body-secondary">{threshold.meaning}</span>
                </CListGroupItem>
              ))}
            </CListGroup>
          </CCard>
        </CCol>
      </CRow>

      <CCard className="mb-4">
        <CCardHeader>
          <strong>Customer and agent analytics</strong>
        </CCardHeader>
        <CCardBody>
          {contextStatus === "loading" && <ApiNotice tone="loading" text="Loading dataset-scoped relation analytics." />}
          {contextError && <ApiNotice tone="warning" text={contextError} />}
          <CustomerAnalytics customer={customerSummary} agent={relationAgentSummary} />
        </CCardBody>
      </CCard>

      <CCard className="mb-4">
        <CCardHeader>
          <strong>Relation graph</strong>
        </CCardHeader>
        <CCardBody>
          {contextStatus === "loading" ? (
            <ApiNotice tone="loading" text="Loading relation graph." />
          ) : relationGraph ? (
            <RelationGraph graph={relationGraph} />
          ) : (
            <EmptyState title="Relation graph unavailable" text="No graph data was returned for this dataset and return." />
          )}
        </CCardBody>
      </CCard>

      <CCard className="mb-4">
        <CCardHeader>
          <strong>Analyst decision</strong>
        </CCardHeader>
        <CCardBody className="review-panel">
          {decisionStatus === "loading" && <ApiNotice tone="loading" text="Loading or saving analyst decision." />}
          {(decisionStatus === "failed" || decisionStatus === "unavailable") && (
            <ApiNotice tone="warning" text={decisionError || "Analyst decision is unavailable."} />
          )}
          {decisionStatus === "empty" && (
            <div className="review-status" role="status">No saved decision yet. Complete the fields and save this case.</div>
          )}
          <div className="review-controls">
            <div>
              <div className="eyebrow">Next action</div>
              <div className="button-strip">
                {(["REVIEW", "ESCALATE", "APPROVE_REFUND", "REJECT_REFUND", "FREEZE_ACCOUNT"] as InvestigationAction[]).map((action) => (
                  <CButton
                    color={review.action === action ? "primary" : "secondary"}
                    key={action}
                    size="sm"
                    variant={review.action === action ? undefined : "outline"}
                    disabled={decisionStatus === "loading"}
                    onClick={() => onReviewChange(approval.returnId, { action })}
                  >
                    {actionLabels[action]}
                  </CButton>
                ))}
              </div>
            </div>
            <div>
              <div className="eyebrow">Decision label</div>
              <div className="button-strip">
                {(["OPEN", "NEEDS_MORE_INFO", "CONFIRMED_FRAUD", "FALSE_POSITIVE", "RESOLVED"] as InvestigationOutcome[]).map((outcome) => (
                  <CButton
                    color={review.outcome === outcome ? "primary" : "secondary"}
                    key={outcome}
                    size="sm"
                    variant={review.outcome === outcome ? undefined : "outline"}
                    disabled={decisionStatus === "loading"}
                    onClick={() => onReviewChange(approval.returnId, { outcome })}
                  >
                    {outcomeLabels[outcome]}
                  </CButton>
                ))}
              </div>
            </div>
          </div>
          <CFormTextarea
            aria-label="Reviewer notes"
            disabled={decisionStatus === "loading"}
            onChange={(event) => onReviewChange(approval.returnId, { note: event.target.value })}
            placeholder="Reviewer notes"
            rows={3}
            value={review.note}
          />
          <div className="review-save-row">
            <CFormInput
              aria-label="Analyst ID"
              disabled={decisionStatus === "loading"}
              onChange={(event) => onReviewChange(approval.returnId, { analystId: event.target.value })}
              placeholder="Analyst ID"
              value={review.analystId}
            />
            <CButton color="primary" disabled={decisionStatus === "loading" || !review.analystId.trim()} onClick={onSaveDecision}>
              Save decision
            </CButton>
          </div>
          {review.updatedAt && <div className="review-status">Last saved {new Date(review.updatedAt).toLocaleString()}</div>}
        </CCardBody>
      </CCard>

      <CCard>
        <CCardHeader>
          <strong>Related approvals</strong>
        </CCardHeader>
        <CCardBody>
          <DataTable
            columns={["returnId", "customerId", "supportAgentId", "refundAmount", "riskScore", "riskLevel", ""]}
            rows={approval.relatedApprovals}
            renderCell={(row, column) => {
              if (column === "refundAmount") return formatMoney(row.refundAmount);
              if (column === "riskLevel") return <RiskBadge level={row.riskLevel} />;
              if (column === "riskScore") return <ScoreBar score={row.riskScore} />;
              if (column === "") {
                return (
                  <CButton color="primary" size="sm" variant="outline" onClick={() => onOpenApproval(row)}>
                    Review
                  </CButton>
                );
              }
              return row[column as keyof Approval] as string | number;
            }}
          />
        </CCardBody>
      </CCard>
    </CContainer>
  );
}

function ApiNotice({ tone, text }: { tone: "loading" | "warning"; text: string }) {
  return (
    <div className={`api-notice ${tone}`} role={tone === "loading" ? "status" : "alert"} aria-live="polite">
      {tone === "loading" ? <span className="button-spinner dark" aria-hidden="true" /> : <AlertTriangle size={17} />}
      <span>{text}</span>
    </div>
  );
}

function DataSourceBanner({ mode }: { mode: DataSourceMode }) {
  if (mode === "live") return null;

  const config: Record<
    Exclude<DataSourceMode, "live">,
    {
      icon: typeof Upload;
      label: string;
      detail: string;
    }
  > = {
    idle: {
      icon: Database,
      label: "No dataset",
      detail: "Upload a CSV to connect live services.",
    },
    checking: {
      icon: CircleDot,
      label: "Checking APIs",
      detail: "Upload, analysis, or scoring request is running.",
    },
    empty: {
      icon: CheckCircle2,
      label: "No suspicious cases",
      detail: "Analysis completed successfully with an empty result set.",
    },
    unavailable: {
      icon: AlertTriangle,
      label: "Service unavailable",
      detail: "No substitute data is shown. Retry when the backend is available.",
    },
    failed: {
      icon: WifiOff,
      label: "API failed",
      detail: "Backend request failed; retry or verify deployment.",
    },
  };
  const current = config[mode];
  const Icon = current.icon;

  return (
    <div className={`data-source-banner ${mode}`} role="status" aria-live="polite">
      <Icon size={17} />
      <div>
        <strong>{current.label}</strong>
        <span>{current.detail}</span>
      </div>
    </div>
  );
}

function EmptyState({ title, text }: { title: string; text: string }) {
  return (
    <div className="empty-state">
      <FileSpreadsheet size={26} />
      <strong>{title}</strong>
      <span>{text}</span>
    </div>
  );
}

export function AnalysisStatusList({
  analysisStepIndex,
  isAnalyzing,
}: {
  analysisStepIndex: number;
  isAnalyzing: boolean;
}) {
  return (
    <CListGroup flush>
      {statusSteps.map((step, index) => {
        const isComplete = index < analysisStepIndex || (!isAnalyzing && analysisStepIndex >= index);
        const isActive = isAnalyzing && index === analysisStepIndex;
        const stateLabel = isActive ? "In progress" : isComplete ? statusDescriptions[step] : "Waiting";

        return (
          <CListGroupItem
            className={`status-step ${isComplete ? "complete" : ""} ${isActive ? "active" : ""}`}
            key={step}
          >
            <span className="status-marker">
              <CheckCircle2 size={18} />
            </span>
            <div>
              <strong>{formatEnum(step)}</strong>
              <div className="text-body-secondary small">{stateLabel}</div>
            </div>
          </CListGroupItem>
        );
      })}
    </CListGroup>
  );
}

function SectionTitle({
  icon: Icon,
  title,
  text,
}: {
  icon: typeof Upload;
  title: string;
  text: string;
}) {
  return (
    <div className="section-title">
      <span className="icon-box">
        <Icon size={18} />
      </span>
      <div>
        <strong>{title}</strong>
        <CFormText>{text}</CFormText>
      </div>
    </div>
  );
}

function InfoPanel({ title, rows }: { title: string; rows: Array<[string, React.ReactNode]> }) {
  return (
    <CCol lg={4}>
      <CCard className="h-100">
        <CCardHeader>
          <strong>{title}</strong>
        </CCardHeader>
        <CListGroup flush>
          {rows.map(([label, value]) => (
            <CListGroupItem className="info-row" key={label}>
              <span className="text-body-secondary">{label}</span>
              <strong>{value}</strong>
            </CListGroupItem>
          ))}
        </CListGroup>
      </CCard>
    </CCol>
  );
}

function MetricCard({
  icon: Icon,
  label,
  value,
  color = "secondary",
}: {
  icon: typeof Upload;
  label: string;
  value: string;
  color?: "secondary" | "warning" | "danger";
}) {
  return (
    <CCol sm={6} xl={4}>
      <CCard className="metric-card h-100">
        <CCardBody>
          <CBadge color={color} className="metric-icon">
            <Icon size={18} />
          </CBadge>
          <div className="text-body-secondary mt-3">{label}</div>
          <strong>{value}</strong>
        </CCardBody>
      </CCard>
    </CCol>
  );
}

function Signal({
  icon: Icon,
  title,
  value,
  color,
}: {
  icon: typeof Upload;
  title: string;
  value: string;
  color: "warning" | "danger";
}) {
  return (
    <CCol sm={6} xl={4}>
      <CCard className="h-100">
        <CCardBody>
          <CBadge color={color} className="metric-icon">
            <Icon size={18} />
          </CBadge>
          <div className="text-body-secondary mt-3">{title}</div>
          <strong>{value}</strong>
        </CCardBody>
      </CCard>
    </CCol>
  );
}

function DataTable<T extends Record<string, unknown>>({
  columns,
  rows,
  renderCell,
}: {
  columns: string[];
  rows: T[];
  renderCell: (row: T, column: string) => React.ReactNode;
}) {
  return (
    <CTable align="middle" hover responsive>
      <CTableHead>
        <CTableRow>
          {columns.map((column) => (
            <CTableHeaderCell key={column}>{columnLabels[column] ?? column}</CTableHeaderCell>
          ))}
        </CTableRow>
      </CTableHead>
      <CTableBody>
        {rows.map((row, index) => (
          <CTableRow key={String(row.returnId ?? row.order_id ?? index)}>
            {columns.map((column) => (
              <CTableDataCell key={column}>{renderCell(row, column)}</CTableDataCell>
            ))}
          </CTableRow>
        ))}
      </CTableBody>
    </CTable>
  );
}

export function RiskBadge({ level }: { level: RiskLevel }) {
  const colorByLevel: Record<RiskLevel, "success" | "info" | "warning" | "danger"> = {
    LOW: "success",
    MEDIUM: "info",
    HIGH: "warning",
    CRITICAL: "danger",
  };
  return <CBadge color={colorByLevel[level]}>{formatEnum(level)}</CBadge>;
}

function ScoreBar({ score }: { score: number }) {
  const color = score > 80 ? "danger" : score > 60 ? "warning" : score > 30 ? "info" : "success";
  return (
    <div className="score-cell">
      <span>{score}</span>
      <CProgress height={8}>
        <CProgressBar color={color} value={score} />
      </CProgress>
    </div>
  );
}

function formatPreviewCell(value: string) {
  if (value === "true") return "Yes";
  if (value === "false") return "No";
  if (/^[A-Z_]+$/.test(value) || value.includes("_")) return formatEnum(value);
  return value;
}

export default App;
