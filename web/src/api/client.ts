import type {
  ApiProblem,
  CreateDemoExperimentRequest,
  CreateSimulationRunRequest,
  DecisionPage,
  ExperimentDetail,
  ExperimentPage,
  ExperimentSummary,
  Health,
  SimulationRun,
} from "../types/api";

const uuidPattern = /^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i;
const nilUUID = "00000000-0000-0000-0000-000000000000";
const maxResponseBytes = 2 * 1024 * 1024;
const configuredBaseUrl: unknown = import.meta.env.VITE_API_BASE_URL;

export class ApiProblemError extends Error implements ApiProblem {
  readonly type: string;
  readonly title: string;
  readonly status: number;
  readonly code: string;
  readonly detail: string;
  readonly request_id: string;

  constructor(problem: ApiProblem) {
    super(problem.detail);
    this.name = "ApiProblemError";
    this.type = problem.type;
    this.title = problem.title;
    this.status = problem.status;
    this.code = problem.code;
    this.detail = problem.detail;
    this.request_id = problem.request_id;
  }
}

export class ApiResponseError extends Error {
  readonly status: number;
  readonly requestId: string | null;

  constructor(message: string, status: number, requestId: string | null) {
    super(message);
    this.name = "ApiResponseError";
    this.status = status;
    this.requestId = requestId;
  }
}

interface RequestOptions {
  method?: "GET" | "POST";
  body?: unknown;
  signal: AbortSignal | undefined;
  expectedStatuses: readonly number[];
}

export class ApiClient {
  readonly #origin: string;

  constructor(baseUrl: string = typeof configuredBaseUrl === "string" ? configuredBaseUrl : "") {
    this.#origin = normalizeOrigin(baseUrl);
  }

  getHealth(signal?: AbortSignal): Promise<Health> {
    return this.#request<Health>("/health/ready", { expectedStatuses: [200], signal });
  }

  listExperiments(limit = 50, cursor?: string, signal?: AbortSignal): Promise<ExperimentPage> {
    assertIntegerInRange("limit", limit, 1, 100);
    const query = new URLSearchParams({ limit: String(limit) });
    if (cursor !== undefined && cursor !== "") {
      query.set("cursor", cursor);
    }
    return this.#request<ExperimentPage>(`/v1/experiments?${query.toString()}`, {
      expectedStatuses: [200],
      signal,
    });
  }

  getExperiment(experimentId: string, signal?: AbortSignal): Promise<ExperimentDetail> {
    assertUUID("experimentId", experimentId);
    return this.#request<ExperimentDetail>(`/v1/experiments/${experimentId}`, {
      expectedStatuses: [200],
      signal,
    });
  }

  createDemoExperiment(body: CreateDemoExperimentRequest, signal?: AbortSignal): Promise<ExperimentDetail> {
    return this.#request<ExperimentDetail>("/v1/demo/experiments", {
      method: "POST",
      body,
      expectedStatuses: [201],
      signal,
    });
  }

  getExperimentSummary(
    experimentId: string,
    maxLearningPoints = 120,
    signal?: AbortSignal,
  ): Promise<ExperimentSummary> {
    assertUUID("experimentId", experimentId);
    assertIntegerInRange("maxLearningPoints", maxLearningPoints, 1, 120);
    const query = new URLSearchParams({ max_learning_points: String(maxLearningPoints) });
    return this.#request<ExperimentSummary>(`/v1/experiments/${experimentId}/summary?${query.toString()}`, {
      expectedStatuses: [200],
      signal,
    });
  }

  listExperimentDecisions(
    experimentId: string,
    limit = 50,
    cursor?: string,
    signal?: AbortSignal,
  ): Promise<DecisionPage> {
    assertUUID("experimentId", experimentId);
    assertIntegerInRange("limit", limit, 1, 200);
    const query = new URLSearchParams({ limit: String(limit) });
    if (cursor !== undefined && cursor !== "") {
      query.set("cursor", cursor);
    }
    return this.#request<DecisionPage>(`/v1/experiments/${experimentId}/decisions?${query.toString()}`, {
      expectedStatuses: [200],
      signal,
    });
  }

  createSimulationRun(
    experimentId: string,
    body: CreateSimulationRunRequest,
    signal?: AbortSignal,
  ): Promise<SimulationRun> {
    assertUUID("experimentId", experimentId);
    assertSafeInteger("seed", body.seed);
    assertIntegerInRange("requests_per_second", body.requests_per_second, 1, 100);
    assertIntegerInRange("max_decisions", body.max_decisions, 1, 100_000);
    return this.#request<SimulationRun>(`/v1/experiments/${experimentId}/simulation-runs`, {
      method: "POST",
      body,
      expectedStatuses: [201],
      signal,
    });
  }

  getSimulationRun(runId: string, signal?: AbortSignal): Promise<SimulationRun> {
    assertUUID("runId", runId);
    return this.#request<SimulationRun>(`/v1/simulation-runs/${runId}`, {
      expectedStatuses: [200],
      signal,
    });
  }

  stopSimulationRun(runId: string, signal?: AbortSignal): Promise<SimulationRun> {
    assertUUID("runId", runId);
    return this.#request<SimulationRun>(`/v1/simulation-runs/${runId}/stop`, {
      method: "POST",
      expectedStatuses: [202],
      signal,
    });
  }

  async #request<T>(path: string, options: RequestOptions): Promise<T> {
    if (!path.startsWith("/")) {
      throw new TypeError("API paths must be root-relative");
    }
    const headers = new Headers({ Accept: "application/json, application/problem+json" });
    let encodedBody: string | undefined;
    if (options.body !== undefined) {
      headers.set("Content-Type", "application/json");
      encodedBody = JSON.stringify(options.body);
    }

    const requestInit: RequestInit = {
      method: options.method ?? "GET",
      headers,
    };
    if (encodedBody !== undefined) {
      requestInit.body = encodedBody;
    }
    if (options.signal !== undefined) {
      requestInit.signal = options.signal;
    }
    const response = await fetch(`${this.#origin}${path}`, requestInit);
    const requestId = response.headers.get("X-Request-ID");
    const contentType = response.headers.get("Content-Type") ?? "";
    const mediaType = contentType.split(";", 1)[0]?.trim().toLowerCase() ?? "";
    const bodyText = await readBoundedText(response, maxResponseBytes);

    if (!options.expectedStatuses.includes(response.status)) {
      if (mediaType === "application/problem+json") {
        const problem = parseProblem(bodyText, response.status, requestId);
        throw new ApiProblemError(problem);
      }
      throw new ApiResponseError(`Unexpected API status ${response.status}.`, response.status, requestId);
    }
    if (mediaType !== "application/json") {
      throw new ApiResponseError("Successful API responses must use application/json.", response.status, requestId);
    }
    try {
      return JSON.parse(bodyText) as T;
    } catch (error: unknown) {
      throw new ApiResponseError(
        `The API returned malformed JSON: ${error instanceof Error ? error.name : "unknown error"}.`,
        response.status,
        requestId,
      );
    }
  }
}

function normalizeOrigin(value: string): string {
  const trimmed = value.trim();
  if (trimmed === "") {
    return "";
  }
  let parsed: URL;
  try {
    parsed = new URL(trimmed);
  } catch {
    throw new TypeError("VITE_API_BASE_URL must be an absolute HTTP or HTTPS origin.");
  }
  if (
    (parsed.protocol !== "http:" && parsed.protocol !== "https:") ||
    parsed.username !== "" ||
    parsed.password !== "" ||
    parsed.pathname !== "/" ||
    parsed.search !== "" ||
    parsed.hash !== ""
  ) {
    throw new TypeError("VITE_API_BASE_URL must be an absolute origin without credentials or a path.");
  }
  return parsed.origin;
}

function assertUUID(name: string, value: string): void {
  if (!uuidPattern.test(value) || value.toLowerCase() === nilUUID) {
    throw new TypeError(`${name} must be a non-nil UUID.`);
  }
}

function assertIntegerInRange(name: string, value: number, minimum: number, maximum: number): void {
  assertSafeInteger(name, value);
  if (value < minimum || value > maximum) {
    throw new RangeError(`${name} must be between ${minimum} and ${maximum}.`);
  }
}

function assertSafeInteger(name: string, value: number): void {
  if (!Number.isSafeInteger(value)) {
    throw new TypeError(`${name} must be a safe integer.`);
  }
}

async function readBoundedText(response: Response, maximum: number): Promise<string> {
  if (response.body === null) {
    return "";
  }
  const reader = response.body.getReader();
  const decoder = new TextDecoder();
  let size = 0;
  let text = "";
  try {
    while (true) {
      const { done, value } = await reader.read();
      if (done) {
        break;
      }
      size += value.byteLength;
      if (size > maximum) {
        await reader.cancel();
        throw new ApiResponseError("The API response exceeded the size limit.", response.status, response.headers.get("X-Request-ID"));
      }
      text += decoder.decode(value, { stream: true });
    }
    text += decoder.decode();
    return text;
  } finally {
    reader.releaseLock();
  }
}

function parseProblem(text: string, status: number, requestId: string | null): ApiProblem {
  let value: unknown;
  try {
    value = JSON.parse(text) as unknown;
  } catch {
    throw new ApiResponseError("The API returned a malformed problem response.", status, requestId);
  }
  if (!isRecord(value)) {
    throw new ApiResponseError("The API returned an invalid problem response.", status, requestId);
  }
  const type = readString(value, "type");
  const title = readString(value, "title");
  const code = readString(value, "code");
  const detail = readString(value, "detail");
  const responseRequestId = readString(value, "request_id") || requestId;
  const problemStatus = typeof value.status === "number" ? value.status : status;
  if (type === null || title === null || code === null || detail === null || responseRequestId === null) {
    throw new ApiResponseError("The API returned an incomplete problem response.", status, requestId);
  }
  return { type, title, status: problemStatus, code, detail, request_id: responseRequestId };
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function readString(value: Record<string, unknown>, key: string): string | null {
  const field = value[key];
  return typeof field === "string" && field !== "" ? field : null;
}
