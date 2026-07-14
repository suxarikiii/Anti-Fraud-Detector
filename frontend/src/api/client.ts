export type ApiEnvelope<T> =
  | T
  | {
      data?: T;
      datasetId?: string;
      jobId?: string;
      message?: string;
    };

type ApiErrorBody = {
  code?: string;
  message?: string;
};

async function backendError(response: Response): Promise<ApiError> {
  const contentType = response.headers.get("content-type") ?? "";
  const body = contentType.includes("application/json")
    ? await response.json().catch(() => undefined)
    : undefined;
  const error = isRecord(body) ? (body as ApiErrorBody) : undefined;
  return new ApiError(
    error?.message || response.statusText || `Request failed with status ${response.status}`,
    response.status,
    error?.code,
  );
}

export class ApiError extends Error {
  constructor(
    message: string,
    readonly status: number,
    readonly code?: string,
  ) {
    super(message);
    this.name = "ApiError";
  }
}

export function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

export function unwrapData<T>(value: ApiEnvelope<T>): T {
  if (isRecord(value) && "data" in value) return value.data as T;
  return value as T;
}

export async function requestJson<T>(url: string, init?: RequestInit): Promise<T> {
  let response: Response;
  try {
    response = await fetch(url, init);
  } catch {
    throw new ApiError("Backend is unavailable. Check the connection and try again.", 0, "NETWORK_ERROR");
  }

  const contentType = response.headers.get("content-type") ?? "";
  let body: unknown;
  if (contentType.includes("application/json")) {
    body = await response.json().catch(() => undefined);
  }

  if (!response.ok) {
    const error = isRecord(body) ? (body as ApiErrorBody) : undefined;
    throw new ApiError(error?.message || response.statusText || `Request failed with status ${response.status}`, response.status, error?.code);
  }

  if (response.status === 204) return undefined as T;

  if (!contentType.includes("application/json")) {
    throw new ApiError("Backend returned an unexpected response format.", response.status, "INVALID_RESPONSE");
  }

  return body as T;
}

export async function requestBlob(url: string): Promise<{ blob: Blob; filename: string }> {
  let response: Response;
  try {
    response = await fetch(url);
  } catch {
    throw new ApiError("Backend is unavailable. Check the connection and try again.", 0, "NETWORK_ERROR");
  }
  if (!response.ok) throw await backendError(response);

  const disposition = response.headers.get("content-disposition") ?? "";
  const filename = disposition.match(/filename="?([^";]+)"?/i)?.[1] ?? "suspicious-approvals.csv";
  return { blob: await response.blob(), filename };
}
