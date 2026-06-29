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
  Clock3,
  FileSpreadsheet,
  Filter,
  Gauge,
  LayoutDashboard,
  Search,
  ShieldAlert,
  Upload,
  UserRound,
  UsersRound,
} from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import antiFraudLogoFull from "./assets/brand/anti-fraud-logo-full.png";
import antiFraudLogoMark from "./assets/brand/anti-fraud-logo-mark.png";

type Page = "dataset" | "approvals" | "details";
type RiskLevel = "LOW" | "MEDIUM" | "HIGH" | "CRITICAL";
type Decision = "APPROVED" | "REJECTED";
type ColumnMapping = Record<string, string>;
type PreviewRow = Record<string, string>;

type DatasetPreview = {
  headers: string[];
  rows: PreviewRow[];
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

type ApiStatus = "idle" | "loading" | "ready" | "fallback";

type UploadResponse = {
  datasetId?: string;
  jobId?: string;
};

type AnalysisJobStatus = {
  id?: string;
  jobId?: string;
  datasetId?: string;
  status?: string;
  currentStep?: string;
  updatedAt?: string;
};

type ApiEnvelope<T> = T | {
  data?: T;
  datasetId?: string;
  jobId?: string;
  message?: string;
};

type AnalysisStatus =
  | "UPLOADED"
  | "NORMALIZING"
  | "NORMALIZED"
  | "BUILDING_RELATIONS"
  | "SCORING"
  | "COMPLETED";

const scoringDatasetId = "demo";

const fallbackPreviewRows: PreviewRow[] = [
  {
    order_id: "order_456",
    customer_id: "customer_789",
    return_id: "return_123",
    support_agent_id: "agent_001",
    order_amount: "11999.00",
    refund_amount: "11500.00",
    product_category: "electronics",
    return_reason: "item_not_as_described",
    evidence_provided: "false",
    decision: "APPROVED",
    manual_override: "true",
    decision_time_minutes: "2",
  },
  {
    order_id: "order_785",
    customer_id: "customer_184",
    return_id: "return_204",
    support_agent_id: "agent_002",
    order_amount: "3490.00",
    refund_amount: "1290.00",
    product_category: "clothing",
    return_reason: "size_issue",
    evidence_provided: "true",
    decision: "APPROVED",
    manual_override: "false",
    decision_time_minutes: "18",
  },
  {
    order_id: "order_901",
    customer_id: "customer_789",
    return_id: "return_891",
    support_agent_id: "agent_001",
    order_amount: "8990.00",
    refund_amount: "8990.00",
    product_category: "mobile_accessories",
    return_reason: "defective",
    evidence_provided: "false",
    decision: "APPROVED",
    manual_override: "true",
    decision_time_minutes: "3",
  },
];

const fallbackMapping: ColumnMapping = {
  order_id: "order_id",
  customer_id: "customer_id",
  return_id: "return_id",
  support_agent_id: "support_agent_id",
  order_amount: "order_amount",
  refund_amount: "refund_amount",
  product_category: "product_category",
  return_reason: "return_reason",
  evidence_provided: "evidence_provided",
  decision: "decision",
  manual_override: "manual_override",
  decision_time_minutes: "decision_time_minutes",
};

const fallbackApprovals: Approval[] = [
  {
    returnId: "return_123",
    orderId: "order_456",
    customerId: "customer_789",
    supportAgentId: "agent_001",
    refundAmount: 11500,
    decision: "APPROVED",
    riskScore: 84,
    riskLevel: "CRITICAL",
    topReason: "Refund approved without evidence for high-value order",
  },
  {
    returnId: "return_891",
    orderId: "order_901",
    customerId: "customer_789",
    supportAgentId: "agent_001",
    refundAmount: 8990,
    decision: "APPROVED",
    riskScore: 81,
    riskLevel: "CRITICAL",
    topReason: "Repeated customer-agent pair with manual override",
  },
  {
    returnId: "return_377",
    orderId: "order_812",
    customerId: "customer_412",
    supportAgentId: "agent_004",
    refundAmount: 6400,
    decision: "APPROVED",
    riskScore: 76,
    riskLevel: "HIGH",
    topReason: "Fast approval for full amount refund",
  },
  {
    returnId: "return_204",
    orderId: "order_785",
    customerId: "customer_184",
    supportAgentId: "agent_002",
    refundAmount: 1290,
    decision: "APPROVED",
    riskScore: 42,
    riskLevel: "MEDIUM",
    topReason: "Customer has elevated return frequency",
  },
  {
    returnId: "return_618",
    orderId: "order_270",
    customerId: "customer_029",
    supportAgentId: "agent_003",
    refundAmount: 399,
    decision: "REJECTED",
    riskScore: 18,
    riskLevel: "LOW",
    topReason: "Low amount, decision rejected",
  },
];

const riskDetails: ReturnRisk = {
  ...fallbackApprovals[0],
  datasetId: scoringDatasetId,
  orderAmount: 11999,
  productCategory: "electronics",
  returnReason: "item_not_as_described",
  evidenceProvided: false,
  manualOverride: true,
  decisionTimeMinutes: 2,
  paymentMethod: "card",
  shippingRegion: "Moscow",
  reasons: [
    {
      type: "NO_EVIDENCE",
      message: "Return was approved without photo, chat, or delivery evidence.",
      scoreImpact: 25,
    },
    {
      type: "HIGH_VALUE_REFUND",
      message: "Refund amount is above the high-value threshold.",
      scoreImpact: 20,
    },
    {
      type: "FAST_APPROVAL",
      message: "Support decision was made in 2 minutes.",
      scoreImpact: 15,
    },
    {
      type: "MANUAL_OVERRIDE",
      message: "Agent used manual override on an approved refund.",
      scoreImpact: 20,
    },
    {
      type: "REPEATED_AGENT_CUSTOMER_PAIR",
      message: "Same agent approved multiple refunds for this customer.",
      scoreImpact: 25,
    },
  ],
  relatedApprovals: fallbackApprovals.slice(1, 4),
};

const fallbackAgentSummary: AgentSummary = {
  agentId: "agent_001",
  approvalRate: 94,
  manualOverrideRate: 38,
  highRiskApprovals: 7,
  repeatedCustomerPairs: 3,
};

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

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function unwrapData<T>(value: ApiEnvelope<T>): T {
  if (isRecord(value) && "data" in value) return value.data as T;
  return value as T;
}

async function requestJson<T>(url: string, init?: RequestInit): Promise<T> {
  const response = await fetch(url, init);
  if (!response.ok) throw new Error(response.statusText);
  const contentType = response.headers.get("content-type") ?? "";
  if (!contentType.includes("application/json")) throw new Error("Expected JSON response");
  return (await response.json()) as T;
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
    };
  }

  if (!isRecord(value)) {
    return { headers: [], rows: [] };
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
  if (!isRecord(value)) return { ...fallbackAgentSummary, agentId };

  return {
    agentId: String(value.agentId ?? value.agent_id ?? agentId),
    approvalRate: toNumber(value.approvalRate ?? value.approval_rate, fallbackAgentSummary.approvalRate),
    manualOverrideRate: toNumber(
      value.manualOverrideRate ?? value.manual_override_rate,
      fallbackAgentSummary.manualOverrideRate,
    ),
    highRiskApprovals: toNumber(
      value.highRiskApprovals ?? value.high_risk_approvals,
      fallbackAgentSummary.highRiskApprovals,
    ),
    repeatedCustomerPairs: toNumber(
      value.repeatedCustomerPairs ?? value.repeated_customer_pairs,
      fallbackAgentSummary.repeatedCustomerPairs,
    ),
  };
}

function normalizeStatusStep(status: AnalysisJobStatus): AnalysisStatus {
  const rawStep = String(status.currentStep ?? status.status ?? "UPLOADED").toUpperCase();
  if (rawStep === "READY" || rawStep === "DONE" || rawStep === "COMPLETE") return "COMPLETED";
  if (statusSteps.includes(rawStep as AnalysisStatus)) return rawStep as AnalysisStatus;
  return "UPLOADED";
}

function isCompletedStatus(status: AnalysisJobStatus) {
  const rawStatus = String(status.status ?? status.currentStep ?? "").toUpperCase();
  return ["COMPLETED", "READY", "DONE", "COMPLETE"].includes(rawStatus);
}

function sleep(ms: number) {
  return new Promise((resolve) => window.setTimeout(resolve, ms));
}

function buildReturnDetailsFallback(approval: Approval): ReturnRisk {
  return {
    ...riskDetails,
    ...approval,
    datasetId: approval.datasetId ?? riskDetails.datasetId,
    relatedApprovals: riskDetails.relatedApprovals.filter((item) => item.returnId !== approval.returnId),
  };
}

function toReturnRisk(data: ReturnDetailsResponse, approval: Approval): ReturnRisk {
  const fallback = buildReturnDetailsFallback(approval);
  const record: Record<string, unknown> = isRecord(data) ? data : {};
  const rawReasons = data.reasons ?? data.explanations;
  const normalizedReasons = Array.isArray(rawReasons)
    ? rawReasons.map(normalizeExplanation).filter((item): item is Explanation => item !== null)
    : [];
  const rawRelatedApprovals = data.relatedApprovals ?? record.related_approvals;
  const normalizedRelatedApprovals = Array.isArray(rawRelatedApprovals)
    ? rawRelatedApprovals
        .map((item) => normalizeApproval(item, approval.datasetId ?? scoringDatasetId))
        .filter((item): item is Approval => item !== null)
    : [];
  const riskScore = toNumber(data.riskScore ?? record.risk_score, approval.riskScore);
  const riskLevel = normalizeRiskLevel(data.riskLevel ?? record.risk_level, riskScore);

  return {
    ...fallback,
    ...data,
    returnId: String(data.returnId ?? record.return_id ?? approval.returnId),
    orderId: String(data.orderId ?? record.order_id ?? approval.orderId),
    customerId: String(data.customerId ?? record.customer_id ?? approval.customerId),
    supportAgentId: String(data.supportAgentId ?? record.support_agent_id ?? approval.supportAgentId),
    datasetId: String(data.datasetId ?? record.dataset_id ?? approval.datasetId ?? scoringDatasetId),
    refundAmount: toNumber(data.refundAmount ?? record.refund_amount, approval.refundAmount),
    decision: normalizeDecision(data.decision ?? approval.decision),
    riskScore,
    riskLevel,
    topReason: String(data.topReason ?? record.top_reason ?? approval.topReason),
    orderAmount: toNumber(data.orderAmount ?? record.order_amount, fallback.orderAmount),
    productCategory: String(data.productCategory ?? record.product_category ?? fallback.productCategory),
    returnReason: String(data.returnReason ?? record.return_reason ?? fallback.returnReason),
    evidenceProvided: toBoolean(data.evidenceProvided ?? record.evidence_provided, fallback.evidenceProvided),
    manualOverride: toBoolean(data.manualOverride ?? record.manual_override, fallback.manualOverride),
    decisionTimeMinutes: toNumber(
      data.decisionTimeMinutes ?? record.decision_time_minutes,
      fallback.decisionTimeMinutes,
    ),
    paymentMethod: String(data.paymentMethod ?? record.payment_method ?? fallback.paymentMethod ?? ""),
    shippingRegion: String(data.shippingRegion ?? record.shipping_region ?? fallback.shippingRegion ?? ""),
    reasons: normalizedReasons.length > 0 ? normalizedReasons : fallback.reasons,
    relatedApprovals: normalizedRelatedApprovals.length > 0 ? normalizedRelatedApprovals : fallback.relatedApprovals,
  };
}

function App() {
  const [page, setPage] = useState<Page>(getInitialPage);
  const [uploadedFile, setUploadedFile] = useState("");
  const [uploadProgress, setUploadProgress] = useState(0);
  const [isUploading, setIsUploading] = useState(false);
  const [currentDatasetId, setCurrentDatasetId] = useState("");
  const [currentJobId, setCurrentJobId] = useState("");
  const [previewData, setPreviewData] = useState<DatasetPreview>(() => normalizePreview([]));
  const [columnMapping, setColumnMapping] = useState<ColumnMapping>({});
  const [approvalRows, setApprovalRows] = useState<Approval[]>([]);
  const [selectedApproval, setSelectedApproval] = useState<ReturnRisk | null>(null);
  const [agentSummary, setAgentSummary] = useState<AgentSummary | null>(null);
  const [approvalsStatus, setApprovalsStatus] = useState<ApiStatus>("idle");
  const [detailsStatus, setDetailsStatus] = useState<ApiStatus>("idle");
  const [riskFilter, setRiskFilter] = useState<"ALL" | RiskLevel>("ALL");
  const [query, setQuery] = useState("");
  const [sidebarCollapsed, setSidebarCollapsed] = useState(false);
  const [isAnalyzing, setIsAnalyzing] = useState(false);
  const [analysisStepIndex, setAnalysisStepIndex] = useState(-1);

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

  async function handleUpload(event: React.ChangeEvent<HTMLInputElement>) {
    const file = event.target.files?.[0];
    if (!file) return;
    let uploadSucceeded = false;

    setUploadedFile(file.name);
    setIsUploading(true);
    setUploadProgress(35);
    setCurrentJobId("");
    setCurrentDatasetId("");
    setApprovalRows([]);
    setSelectedApproval(null);
    setAgentSummary(null);
    setApprovalsStatus("idle");
    setDetailsStatus("idle");
    setAnalysisStepIndex(-1);

    try {
      const formData = new FormData();
      formData.append("file", file);

      const upload = normalizeUploadResponse(
        await requestJson<ApiEnvelope<UploadResponse>>("/api/datasets/upload", {
          method: "POST",
          body: formData,
        }),
      );
      const uploadedDatasetId = upload.datasetId ?? "";
      if (!uploadedDatasetId) throw new Error("Missing uploaded dataset id");

      setCurrentDatasetId(uploadedDatasetId);
      setCurrentJobId(upload.jobId ?? "");
      setUploadProgress(70);

      const preview = await requestJson<ApiEnvelope<unknown>>(`/api/datasets/${uploadedDatasetId}/preview`)
        .then(normalizePreview)
        .catch(() => normalizePreview([]));

      setPreviewData(preview);
      setColumnMapping(fallbackMapping);
      setAnalysisStepIndex(0);
      uploadSucceeded = true;
    } catch {
      setCurrentDatasetId("");
      setCurrentJobId("");
      setPreviewData(normalizePreview([]));
      setColumnMapping({});
      setAnalysisStepIndex(-1);
    } finally {
      setUploadProgress(uploadSucceeded ? 100 : 0);
      setIsUploading(false);
    }
  }

  async function startAnalysis() {
    const datasetId = currentDatasetId;
    if (!datasetId) return;

    let lastStepIndex = 0;

    setIsAnalyzing(true);
    setAnalysisStepIndex(0);

    try {
      const startResponse = normalizeUploadResponse(
        await requestJson<ApiEnvelope<UploadResponse>>(`/api/analysis/${datasetId}/start`, {
          method: "POST",
        }),
      );
      const jobId = startResponse.jobId || currentJobId;
      if (!jobId) throw new Error("Missing analysis job id");

      setCurrentJobId(jobId);

      for (let attempt = 0; attempt < 6; attempt += 1) {
        const status = unwrapData(
          await requestJson<ApiEnvelope<AnalysisJobStatus>>(`/api/analysis/${jobId}/status`),
        );
        const step = normalizeStatusStep(status);
        const stepIndex = statusSteps.indexOf(step);
        if (stepIndex >= 0) {
          lastStepIndex = stepIndex;
          setAnalysisStepIndex(stepIndex);
        }
        if (step === "COMPLETED" || isCompletedStatus(status)) break;
        await sleep(1000);
      }
    } catch {
      lastStepIndex = 0;
    }

    if (lastStepIndex < statusSteps.length - 1) {
      for (let index = Math.max(lastStepIndex + 1, 1); index < statusSteps.length; index += 1) {
        setAnalysisStepIndex(index);
        await sleep(index === statusSteps.length - 1 ? 500 : 700);
      }
    }

    setAnalysisStepIndex(statusSteps.length - 1);
    setIsAnalyzing(false);
    await loadSuspiciousApprovals(datasetId);
    setPage("approvals");
  }

  async function loadSuspiciousApprovals(datasetId: string) {
    setApprovalsStatus("loading");
    const datasetCandidates = [datasetId].filter(
      (value, index, values) => value && values.indexOf(value) === index,
    );

    for (const candidate of datasetCandidates) {
      try {
        const raw = await requestJson<ApiEnvelope<unknown>>(
          `/api/scoring/datasets/${candidate}/suspicious-approvals`,
        );
        const normalized = normalizeApprovals(raw, candidate);
        if (normalized.length > 0) {
          setApprovalRows(normalized);
          setApprovalsStatus("ready");
          return;
        }
      } catch {
        continue;
      }
    }

    setApprovalRows([]);
    setApprovalsStatus("fallback");
  }

  async function loadAgentSummary(datasetId: string, agentId: string) {
    try {
      const raw = await requestJson<ApiEnvelope<unknown>>(
        `/api/scoring/datasets/${datasetId}/agents/${agentId}/risk-summary`,
      );
      setAgentSummary(normalizeAgentSummary(raw, agentId));
    } catch {
      setAgentSummary({ ...fallbackAgentSummary, agentId });
    }
  }

  async function openApproval(approval: Approval) {
    const detailDatasetId = approval.datasetId ?? currentDatasetId;
    if (!detailDatasetId) return;

    setSelectedApproval(buildReturnDetailsFallback(approval));
    setAgentSummary({ ...fallbackAgentSummary, agentId: approval.supportAgentId });
    setDetailsStatus("loading");
    setPage("details");

    try {
      const data = await requestJson<ApiEnvelope<ReturnDetailsResponse>>(
        `/api/scoring/datasets/${detailDatasetId}/returns/${approval.returnId}/details`,
      );
      const details = toReturnRisk(unwrapData(data), approval);
      setSelectedApproval(details);
      setDetailsStatus("ready");
      await loadAgentSummary(detailDatasetId, details.supportAgentId);
    } catch {
      const fallback = buildReturnDetailsFallback(approval);
      setSelectedApproval(fallback);
      setAgentSummary({ ...fallbackAgentSummary, agentId: fallback.supportAgentId });
      setDetailsStatus("fallback");
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
                <CNavLink active={page === item.page} as="button" onClick={() => setPage(item.page)}>
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
        </CHeader>

        {page === "dataset" && (
          <DatasetPage
            uploadedFile={uploadedFile}
            uploadProgress={uploadProgress}
            isUploading={isUploading}
            datasetId={currentDatasetId}
            jobId={currentJobId}
            previewData={previewData}
            columnMapping={columnMapping}
            isAnalyzing={isAnalyzing}
            analysisStepIndex={analysisStepIndex}
            canStartAnalysis={Boolean(currentDatasetId)}
            onUpload={handleUpload}
            onStart={startAnalysis}
          />
        )}
        {page === "approvals" && (
          <ApprovalsPage
            approvals={filteredApprovals}
            query={query}
            riskFilter={riskFilter}
            onQueryChange={setQuery}
            onRiskFilterChange={setRiskFilter}
            onOpenApproval={openApproval}
            status={approvalsStatus}
          />
        )}
        {page === "details" && (
          <DetailsPage
            agentSummary={agentSummary}
            approval={selectedApproval}
            onOpenApproval={openApproval}
            status={detailsStatus}
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
  datasetId,
  jobId,
  previewData,
  columnMapping,
  isAnalyzing,
  analysisStepIndex,
  canStartAnalysis,
  onUpload,
  onStart,
}: {
  uploadedFile: string;
  uploadProgress: number;
  isUploading: boolean;
  datasetId: string;
  jobId: string;
  previewData: DatasetPreview;
  columnMapping: ColumnMapping;
  isAnalyzing: boolean;
  analysisStepIndex: number;
  canStartAnalysis: boolean;
  onUpload: (event: React.ChangeEvent<HTMLInputElement>) => void;
  onStart: () => void;
}) {
  const hasPreview = previewData.headers.length > 0 || previewData.rows.length > 0;
  const hasMapping = Object.keys(columnMapping).length > 0;

  return (
    <CContainer fluid className="px-0">
      <CRow className="g-4 mb-4">
        <CCol lg={5}>
          <CCard className="h-100">
            <CCardHeader>
              <SectionTitle icon={Upload} title="Upload dataset" text="Upload a CSV file and review the detected structure." />
            </CCardHeader>
            <CCardBody>
              <CFormLabel className="dropzone">
                <FileSpreadsheet size={34} />
                <strong>{uploadedFile || "No file selected"}</strong>
                <span>Choose CSV file</span>
                <CFormInput accept=".csv" className="visually-hidden" onChange={onUpload} type="file" />
              </CFormLabel>

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
                <DataTable
                  columns={previewData.headers}
                  rows={previewData.rows}
                  renderCell={(row, column) => formatPreviewCell(String(row[column] ?? ""))}
                />
              ) : (
                <EmptyState title="No dataset uploaded" text="Choose a CSV file to preview its rows." />
              )}
            </CCardBody>
          </CCard>
        </CCol>
        <CCol xl={4}>
          <CCard className="h-100">
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

    </CContainer>
  );
}

export function ApprovalsPage({
  approvals,
  query,
  riskFilter,
  onQueryChange,
  onRiskFilterChange,
  onOpenApproval,
  status,
}: {
  approvals: Approval[];
  query: string;
  riskFilter: "ALL" | RiskLevel;
  onQueryChange: (value: string) => void;
  onRiskFilterChange: (value: "ALL" | RiskLevel) => void;
  onOpenApproval: (approval: Approval) => void;
  status: ApiStatus;
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
  }, [query, riskFilter, approvals.length]);

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
          </div>
        </CCardHeader>
        <CCardBody>
          {status === "loading" && <ApiNotice tone="loading" text="Loading suspicious approvals from scoring API." />}
          {status === "fallback" && (
            <ApiNotice tone="warning" text="Scoring API is unavailable. No approvals were loaded." />
          )}
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
          ) : (
            <EmptyState title="No approvals loaded" text="Upload a dataset and run analysis to review suspicious approvals." />
          )}
        </CCardBody>
      </CCard>
    </CContainer>
  );
}

export function DetailsPage({
  agentSummary,
  approval,
  onOpenApproval,
  status,
}: {
  agentSummary: AgentSummary | null;
  approval: ReturnRisk | null;
  onOpenApproval: (approval: Approval) => void;
  status: ApiStatus;
}) {
  if (!approval) {
    return (
      <CContainer fluid className="px-0">
        <EmptyState title="No investigation selected" text="Open an approval after analysis to inspect its risk details." />
      </CContainer>
    );
  }

  const summary = agentSummary ?? { ...fallbackAgentSummary, agentId: approval.supportAgentId };

  return (
    <CContainer fluid className="px-0">
      {status === "loading" && <ApiNotice tone="loading" text="Loading return details and explanations." />}
      {status === "fallback" && (
        <ApiNotice tone="warning" text="Details API is unavailable. Showing offline investigation fallback." />
      )}
      <CCard className="mb-4">
        <CCardBody className="details-hero">
          <div>
            <div className="eyebrow">Refund approval details</div>
            <h2>{approval.returnId}</h2>
            <p className="text-body-secondary mb-0">{approval.topReason}</p>
          </div>
          <CCard className="risk-card">
            <CCardBody>
              <div className="text-body-secondary">Risk score</div>
              <strong>{approval.riskScore}</strong>
              <RiskBadge level={approval.riskLevel} />
            </CCardBody>
          </CCard>
        </CCardBody>
      </CCard>

      <CRow className="g-4 mb-4">
        <InfoPanel
          title="Return request"
          rows={[
            ["Return ID", approval.returnId],
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
            ["Agent", approval.supportAgentId],
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
              {approval.reasons.map((item) => (
                <CListGroupItem className="explanation-item" key={item.type}>
                  <div>
                    <strong>{formatEnum(item.type)}</strong>
                    <div className="text-body-secondary">{item.message}</div>
                  </div>
                  <CBadge color="danger">+{item.scoreImpact}</CBadge>
                </CListGroupItem>
              ))}
            </CListGroup>
          </CCard>
        </CCol>
        <CCol lg={5}>
          <CCard className="h-100">
            <CCardHeader>
              <strong>Investigation context</strong>
            </CCardHeader>
            <CCardBody>
              <CRow className="g-3">
                <Signal
                  icon={UserRound}
                  title="Repeated customer pairs"
                  value={String(summary.repeatedCustomerPairs)}
                  color="warning"
                />
                <Signal
                  icon={UsersRound}
                  title="Agent approval rate"
                  value={`${summary.approvalRate}%`}
                  color="danger"
                />
                <Signal
                  icon={Gauge}
                  title="Manual override rate"
                  value={`${summary.manualOverrideRate}%`}
                  color="danger"
                />
              </CRow>
              <CListGroup flush className="mt-3">
                <CListGroupItem className="info-row">
                  <span className="text-body-secondary">Agent summary</span>
                  <strong>{summary.agentId}</strong>
                </CListGroupItem>
                <CListGroupItem className="info-row">
                  <span className="text-body-secondary">High-risk approvals</span>
                  <strong>{summary.highRiskApprovals}</strong>
                </CListGroupItem>
              </CListGroup>
            </CCardBody>
          </CCard>
        </CCol>
      </CRow>

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

function InfoPanel({ title, rows }: { title: string; rows: Array<[string, string]> }) {
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
