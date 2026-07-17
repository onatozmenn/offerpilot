import { act, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";

import App from "./App";
import { ApiClient, ApiProblemError } from "./api/client";
import { idlePollIntervalMs } from "./hooks/useDashboard";
import type {
  Decision,
  Experiment,
  ExperimentDetail,
  ExperimentSummary,
  Offer,
  SimulationRun,
} from "./types/api";

afterEach(() => {
  vi.unstubAllGlobals();
  vi.useRealTimers();
});

describe("ApiClient", () => {
  it("uses the configured origin, encodes opaque queries, and forwards abort signals", async () => {
    const fetchMock = vi.fn<typeof fetch>().mockResolvedValue(
      jsonResponse({ items: [], next_cursor: null }),
    );
    vi.stubGlobal("fetch", fetchMock);
    const controller = new AbortController();

    await expect(
      new ApiClient("https://api.offerpilot.example").listExperiments(25, "opaque cursor", controller.signal),
    ).resolves.toEqual({ items: [], next_cursor: null });

    expect(fetchMock).toHaveBeenCalledOnce();
    expect(fetchMock).toHaveBeenCalledWith(
      "https://api.offerpilot.example/v1/experiments?limit=25&cursor=opaque+cursor",
      expect.objectContaining({ method: "GET", signal: controller.signal }),
    );
  });

  it("retains structured problem diagnostics and never retries a write", async () => {
    const fetchMock = vi.fn<typeof fetch>().mockResolvedValue(
      problemResponse(409, "simulation_already_active", "A simulation is already active."),
    );
    vi.stubGlobal("fetch", fetchMock);

    const request = new ApiClient().createSimulationRun(experimentA.id, {
      seed: 42,
      requests_per_second: 10,
      max_decisions: 100,
    });

    await expect(request).rejects.toBeInstanceOf(ApiProblemError);
    await expect(request).rejects.toEqual(
      expect.objectContaining({
        name: "ApiProblemError",
        status: 409,
        code: "simulation_already_active",
        request_id: "request-test",
      }),
    );
    expect(fetchMock).toHaveBeenCalledOnce();
  });

  it("propagates an AbortSignal cancellation", async () => {
    const fetchMock = vi.fn<typeof fetch>().mockImplementation((_input, init) =>
      new Promise<Response>((_resolve, reject) => {
        init?.signal?.addEventListener("abort", () => reject(abortReason(init.signal)), { once: true });
      }),
    );
    vi.stubGlobal("fetch", fetchMock);
    const controller = new AbortController();
    const request = new ApiClient().getExperiment(experimentA.id, controller.signal);

    controller.abort(new DOMException("superseded", "AbortError"));

    await expect(request).rejects.toMatchObject({ name: "AbortError" });
    expect(fetchMock).toHaveBeenCalledOnce();
  });
});

describe("OfferPilot dashboard", () => {
  it("loads the operational dashboard and exposes server-projected audit data", async () => {
    const server = new MockApiServer([experimentA]);
    installServer(server);
    render(<App />);

    await screen.findByRole("heading", { name: experimentA.name });
    expect(screen.getByText("API ready")).toBeInTheDocument();

    const metrics = requireElement(screen.getByRole("heading", { name: "Headline metrics" }).closest("section"));
    expect(await within(metrics).findByText("1,250")).toBeInTheDocument();
    expect(within(metrics).getByText("0.400")).toBeInTheDocument();
    expect(within(metrics).getByText("60%")).toBeInTheDocument();
    expect(within(metrics).getByText("1.5 ms")).toBeInTheDocument();

    expect(await screen.findByText("Random reference (simulation-only)")).toBeInTheDocument();
    expect(screen.getByText("Oracle reference (simulation-only)")).toBeInTheDocument();
    expect(screen.getByText(/Observed cumulative average reward is/)).toHaveTextContent("0.400");

    const offerTable = screen.getByRole("table", { name: /Server-projected offer selections/ });
    expect(within(offerTable).getByText("Harbor Table")).toBeInTheDocument();
    expect(within(offerTable).getByLabelText("Unavailable: No outcomes")).toBeInTheDocument();

    const feed = screen.getByRole("table", { name: /Recent policy decisions/ });
    expect(within(feed).getByText("Pending outcome")).toBeInTheDocument();
    expect(within(feed).getByText("Converted")).toBeInTheDocument();
    expect(within(feed).getByText("Applied at v85")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Start run" })).toBeEnabled();
    expect(screen.getByRole("button", { name: "Stop run" })).toBeDisabled();
  });

  it("validates and creates the first demo experiment from the keyboard", async () => {
    const user = userEvent.setup();
    const server = new MockApiServer([]);
    installServer(server);
    render(<App />);

    await screen.findByRole("heading", { name: "Create demo experiment" });
    const epsilonInput = screen.getByRole("spinbutton", { name: "Epsilon" });
    const createButton = screen.getByRole("button", { name: "Create experiment" });
    await user.clear(epsilonInput);
    expect(createButton).toBeDisabled();
    await user.type(epsilonInput, "0.2");
    expect(createButton).toBeEnabled();

    const nameInput = screen.getByRole("textbox", { name: "Name" });
    await user.click(nameInput);
    await user.keyboard("{Enter}");

    await screen.findByRole("heading", { name: createdExperiment.name });
    expect(server.countRequests("POST", "/v1/demo/experiments")).toBe(1);
    expect(server.requestBodies.some((body) => body.includes('"epsilon":0.2'))).toBe(true);
  });

  it("validates controls and locks duplicate start and stop commands", async () => {
    const user = userEvent.setup();
    const server = new MockApiServer([experimentA]);
    installServer(server);
    render(<App />);
    await screen.findByRole("heading", { name: experimentA.name });

    const rateInput = screen.getByRole("spinbutton", { name: "Requests per second" });
    await user.clear(rateInput);
    expect(screen.getByRole("button", { name: "Start run" })).toBeDisabled();
    await user.type(rateInput, "10");

    const startDeferred = createDeferred<Response>();
    server.startDeferred = startDeferred;
    const startButton = screen.getByRole("button", { name: "Start run" });
    const controlsForm = requireElement(startButton.closest("form"));
    fireEvent.submit(controlsForm);
    fireEvent.submit(controlsForm);
    expect(server.countRequests("POST", `/v1/experiments/${experimentA.id}/simulation-runs`)).toBe(1);

    await act(async () => {
      startDeferred.resolve(jsonResponse(server.requireRun(), 201));
      await startDeferred.promise;
    });
    await screen.findByText("Running: 0 of 5,000 decisions. Seed 42.");
    expect(screen.getByRole("button", { name: "Stop run" })).toBeEnabled();

    const stopDeferred = createDeferred<Response>();
    server.stopDeferred = stopDeferred;
    const stopButton = screen.getByRole("button", { name: "Stop run" });
    fireEvent.click(stopButton);
    fireEvent.click(stopButton);
    expect(server.countRequests("POST", `/v1/simulation-runs/${runningRun.run_id}/stop`)).toBe(1);

    const stoppingRun: SimulationRun = { ...server.requireRun(), status: "stopping" };
    server.run = { ...server.requireRun(), status: "completed", stopped_at: "2026-07-17T15:02:00Z" };
    await act(async () => {
      stopDeferred.resolve(jsonResponse(stoppingRun, 202));
      await stopDeferred.promise;
    });
    await screen.findByText("Completed: 0 of 5,000 decisions. Seed 42.");
    expect(screen.getByRole("button", { name: "Start run" })).toBeEnabled();
  });

  it("preserves useful panels across independent summary and feed failures", async () => {
    const user = userEvent.setup();
    const server = new MockApiServer([experimentA]);
    installServer(server);
    render(<App />);
    const offerTable = await screen.findByRole("table", { name: /Server-projected offer selections/ });
    expect(await within(offerTable).findByText("Harbor Table")).toBeInTheDocument();

    server.failSummaryFor.add(experimentA.id);
    await user.click(screen.getByRole("button", { name: "Refresh dashboard" }));
    await screen.findAllByText(/Summary is temporarily unavailable\. Request ID: request-test/);
    expect(within(offerTable).getByText("Harbor Table")).toBeInTheDocument();
    expect(within(screen.getByRole("table", { name: /Recent policy decisions/ })).getByText("Pending outcome")).toBeInTheDocument();
    expect(screen.getAllByText(/Stale data|Data is stale/).length).toBeGreaterThan(0);

    server.failSummaryFor.clear();
    server.failDecisionsFor.add(experimentA.id);
    await waitFor(() => {
      expect(screen.getByRole("button", { name: "Refresh dashboard" })).toBeEnabled();
    });
    await user.click(screen.getByRole("button", { name: "Refresh dashboard" }));
    await screen.findByText(/Decision feed is temporarily unavailable\. Request ID: request-test/);
    const metrics = requireElement(screen.getByRole("heading", { name: "Headline metrics" }).closest("section"));
    expect(within(metrics).getByText("0.400")).toBeInTheDocument();
    expect(within(screen.getByRole("table", { name: /Recent policy decisions/ })).getByText("Pending outcome")).toBeInTheDocument();
  });

  it("aborts superseded selections and all in-flight work on unmount", async () => {
    const user = userEvent.setup();
    const server = new MockApiServer([experimentA, experimentB]);
    const oldSummary = createDeferred<Response>();
    server.summaryDeferred.set(experimentA.id, oldSummary);
    server.summaries.set(experimentB.id, makeSummary(experimentB, { average_reward: 0.91 }));
    installServer(server);
    const { unmount } = render(<App />);

    const selector = await screen.findByRole("combobox", { name: "Experiment" });
    await waitFor(() => {
      expect(selector).toHaveValue(experimentA.id);
      expect(server.signalFor(`/v1/experiments/${experimentA.id}/summary?max_learning_points=120`)).not.toBeNull();
    });
    await user.selectOptions(selector, experimentB.id);
    await screen.findByText("0.910");
    expect(server.signalFor(`/v1/experiments/${experimentA.id}/summary?max_learning_points=120`)?.aborted).toBe(true);

    const currentSummary = createDeferred<Response>();
    server.summaryDeferred.set(experimentB.id, currentSummary);
    await user.click(screen.getByRole("button", { name: "Refresh dashboard" }));
    await waitFor(() => {
      expect(server.signalFor(`/v1/experiments/${experimentB.id}/summary?max_learning_points=120`)).not.toBeNull();
    });
    unmount();
    expect(server.signalFor(`/v1/experiments/${experimentB.id}/summary?max_learning_points=120`)?.aborted).toBe(true);
  });

  it("polls through one idle timer and stops polling after unmount", async () => {
    vi.useFakeTimers();
    const server = new MockApiServer([experimentA]);
    installServer(server);
    const { unmount } = render(<App />);

    await act(async () => {
      await vi.advanceTimersByTimeAsync(25);
    });
    expect(server.summaryCalls).toBe(1);

    await act(async () => {
      await vi.advanceTimersByTimeAsync(idlePollIntervalMs - 1);
    });
    expect(server.summaryCalls).toBe(1);

    await act(async () => {
      await vi.advanceTimersByTimeAsync(1);
    });
    expect(server.summaryCalls).toBe(2);

    unmount();
    await vi.advanceTimersByTimeAsync(idlePollIntervalMs * 2);
    expect(server.summaryCalls).toBe(2);
  });
});

interface RecordedRequest {
  method: string;
  path: string;
  signal: AbortSignal | null;
}

class MockApiServer {
  experiments: Experiment[];
  readonly details = new Map<string, ExperimentDetail>();
  readonly summaries = new Map<string, ExperimentSummary>();
  readonly decisions = new Map<string, Decision[]>();
  readonly failSummaryFor = new Set<string>();
  readonly failDecisionsFor = new Set<string>();
  readonly summaryDeferred = new Map<string, Deferred<Response>>();
  readonly requests: RecordedRequest[] = [];
  readonly requestBodies: string[] = [];
  run: SimulationRun | null = null;
  startDeferred: Deferred<Response> | null = null;
  stopDeferred: Deferred<Response> | null = null;
  summaryCalls = 0;

  readonly fetchMock = vi.fn<typeof fetch>(async (input, init) => this.handle(input, init));

  constructor(experiments: Experiment[]) {
    this.experiments = [...experiments];
    for (const experiment of experiments) {
      this.details.set(experiment.id, makeDetail(experiment));
      this.summaries.set(experiment.id, makeSummary(experiment));
      this.decisions.set(experiment.id, makeDecisions(experiment));
    }
  }

  countRequests(method: string, path: string): number {
    return this.requests.filter((request) => request.method === method && request.path === path).length;
  }

  signalFor(path: string): AbortSignal | null {
    for (let index = this.requests.length - 1; index >= 0; index -= 1) {
      const request = this.requests[index];
      if (request?.path === path) {
        return request.signal;
      }
    }
    return null;
  }

  requireRun(): SimulationRun {
    if (this.run === null) {
      throw new Error("Expected a simulation run.");
    }
    return this.run;
  }

  private async handle(input: RequestInfo | URL, init?: RequestInit): Promise<Response> {
    const rawUrl = typeof input === "string" ? input : input instanceof URL ? input.toString() : input.url;
    const parsed = new URL(rawUrl, "https://dashboard.offerpilot.example");
    const path = `${parsed.pathname}${parsed.search}`;
    const method = init?.method ?? "GET";
    const signal = init?.signal ?? null;
    this.requests.push({ method, path, signal });
    if (typeof init?.body === "string") {
      this.requestBodies.push(init.body);
    }

    if (path === "/health/ready") {
      return jsonResponse({ status: "ready" });
    }
    if (path.startsWith("/v1/experiments?")) {
      return jsonResponse({ items: this.experiments, next_cursor: null });
    }
    if (path === "/v1/demo/experiments" && method === "POST") {
      this.experiments = [createdExperiment, ...this.experiments];
      const detail = makeDetail(createdExperiment);
      this.details.set(createdExperiment.id, detail);
      this.summaries.set(createdExperiment.id, makeSummary(createdExperiment));
      this.decisions.set(createdExperiment.id, []);
      return jsonResponse(detail, 201);
    }

    const simulationCreate = path.match(/^\/v1\/experiments\/([^/]+)\/simulation-runs$/);
    if (simulationCreate !== null && method === "POST") {
      this.run = { ...runningRun, experiment_id: simulationCreate[1] ?? experimentA.id };
      return this.startDeferred === null
        ? jsonResponse(this.run, 201)
        : await withAbort(this.startDeferred.promise, signal);
    }
    const simulationStop = path.match(/^\/v1\/simulation-runs\/([^/]+)\/stop$/);
    if (simulationStop !== null && method === "POST") {
      const stopping = { ...this.requireRun(), status: "stopping" as const };
      this.run = stopping;
      return this.stopDeferred === null
        ? jsonResponse(stopping, 202)
        : await withAbort(this.stopDeferred.promise, signal);
    }
    if (path.match(/^\/v1\/simulation-runs\/[^/]+$/) !== null) {
      return jsonResponse(this.requireRun());
    }

    const summaryMatch = path.match(/^\/v1\/experiments\/([^/]+)\/summary\?max_learning_points=120$/);
    if (summaryMatch !== null) {
      const experimentId = summaryMatch[1] ?? "";
      this.summaryCalls += 1;
      if (this.failSummaryFor.has(experimentId)) {
        return problemResponse(503, "summary_unavailable", "Summary is temporarily unavailable.");
      }
      const controlled = this.summaryDeferred.get(experimentId);
      if (controlled !== undefined) {
        return await withAbort(controlled.promise, signal);
      }
      return jsonResponse(requireMapValue(this.summaries, experimentId, "summary"));
    }

    const decisionsMatch = path.match(/^\/v1\/experiments\/([^/]+)\/decisions\?limit=50$/);
    if (decisionsMatch !== null) {
      const experimentId = decisionsMatch[1] ?? "";
      if (this.failDecisionsFor.has(experimentId)) {
        return problemResponse(503, "decision_feed_unavailable", "Decision feed is temporarily unavailable.");
      }
      return jsonResponse({
        items: requireMapValue(this.decisions, experimentId, "decisions"),
        next_cursor: null,
      });
    }

    const experimentMatch = path.match(/^\/v1\/experiments\/([^/?]+)$/);
    if (experimentMatch !== null) {
      return jsonResponse(requireMapValue(this.details, experimentMatch[1] ?? "", "experiment"));
    }
    throw new Error(`Unexpected API request: ${method} ${path}`);
  }
}

interface Deferred<T> {
  promise: Promise<T>;
  resolve: (value: T) => void;
}

function createDeferred<T>(): Deferred<T> {
  let resolvePromise: ((value: T) => void) | undefined;
  const promise = new Promise<T>((resolve) => {
    resolvePromise = resolve;
  });
  return {
    promise,
    resolve: (value) => resolvePromise?.(value),
  };
}

function withAbort(promise: Promise<Response>, signal: AbortSignal | null): Promise<Response> {
  if (signal === null) {
    return promise;
  }
  return new Promise<Response>((resolve, reject) => {
    if (signal.aborted) {
      reject(abortReason(signal));
      return;
    }
    const abort = (): void => reject(abortReason(signal));
    signal.addEventListener("abort", abort, { once: true });
    void promise.then(resolve, reject).finally(() => signal.removeEventListener("abort", abort));
  });
}

function abortReason(signal: AbortSignal | null | undefined): Error {
  return signal?.reason instanceof Error
    ? signal.reason
    : new DOMException("The request was aborted.", "AbortError");
}

function installServer(server: MockApiServer): void {
  vi.stubGlobal("fetch", server.fetchMock);
}

function requireElement<T extends Element>(element: T | null): T {
  if (element === null) {
    throw new Error("Expected an element to exist.");
  }
  return element;
}

function requireMapValue<T>(map: ReadonlyMap<string, T>, key: string, label: string): T {
  const value = map.get(key);
  if (value === undefined) {
    throw new Error(`Missing ${label} fixture for ${key}.`);
  }
  return value;
}

function jsonResponse(value: unknown, status = 200): Response {
  return new Response(JSON.stringify(value), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

function problemResponse(status: number, code: string, detail: string): Response {
  return new Response(
    JSON.stringify({
      type: `https://offerpilot.dev/problems/${code}`,
      title: "Request failed",
      status,
      code,
      detail,
      request_id: "request-test",
    }),
    {
      status,
      headers: {
        "Content-Type": "application/problem+json",
        "X-Request-ID": "request-test",
      },
    },
  );
}

const experimentA: Experiment = {
  id: "742fb9d5-2b29-475f-b593-cbae1c748548",
  slug: "marketplace-learning-a",
  name: "Marketplace learning A",
  status: "running",
  policy_kind: "segmented_epsilon_greedy",
  epsilon: 0.15,
  policy_version: 84,
  created_at: "2026-07-17T15:00:00Z",
  updated_at: "2026-07-17T15:05:00Z",
};

const experimentB: Experiment = {
  ...experimentA,
  id: "26df105a-a62d-4661-bec5-00675018adae",
  slug: "marketplace-learning-b",
  name: "Marketplace learning B",
  created_at: "2026-07-17T14:00:00Z",
};

const createdExperiment: Experiment = {
  ...experimentA,
  id: "9f997dee-282d-4870-9e4b-0bc772117654",
  slug: "marketplace-learning-demo",
  name: "Marketplace learning demo",
  policy_version: 1,
  created_at: "2026-07-17T16:00:00Z",
  updated_at: "2026-07-17T16:00:00Z",
};

const offers: readonly Offer[] = [
  {
    id: "1d842553-6ff8-4ff6-84a8-9d87fe39e802",
    slug: "harbor-table",
    merchant_name: "Harbor Table",
    title: "Chef's evening menu",
    description: "A fictional dining offer.",
    category: "dining",
    active: true,
  },
  {
    id: "939fb40f-0528-4ea8-96cb-aa05c28ea94e",
    slug: "northstar-journeys",
    merchant_name: "Northstar Journeys",
    title: "Weekend city break",
    description: "A fictional travel offer.",
    category: "travel",
    active: true,
  },
];

function makeDetail(experiment: Experiment): ExperimentDetail {
  return { ...experiment, offers: [...offers] };
}

function makeSummary(
  experiment: Experiment,
  overrides: Partial<ExperimentSummary> = {},
): ExperimentSummary {
  return {
    experiment_id: experiment.id,
    policy_kind: experiment.policy_kind,
    policy_version: experiment.policy_version,
    exploration_rate: experiment.epsilon,
    decision_count: 1_250,
    outcome_count: 1_000,
    reward_sum: 400,
    average_reward: 0.4,
    engagement_rate: 0.6,
    ignored_count: 400,
    clicked_count: 450,
    converted_count: 150,
    p50_policy_latency_micros: 650,
    p95_policy_latency_micros: 1_500,
    offer_performance: [
      {
        offer: offers[0] as Offer,
        selection_count: 900,
        outcome_count: 700,
        ignored_count: 280,
        clicked_count: 315,
        converted_count: 105,
        reward_sum: 280,
        empirical_mean: null,
        current_policy_mean: 0.45,
        current_probability: 0.72,
      },
      {
        offer: offers[1] as Offer,
        selection_count: 350,
        outcome_count: 300,
        ignored_count: 120,
        clicked_count: 135,
        converted_count: 45,
        reward_sum: 120,
        empirical_mean: 0.4,
        current_policy_mean: 0.38,
        current_probability: 0.28,
      },
    ],
    learning_series: [
      {
        timestamp: "2026-07-17T15:00:00Z",
        sample_count: 500,
        cumulative_average_reward: 0.32,
      },
      {
        timestamp: "2026-07-17T15:05:00Z",
        sample_count: 1_000,
        cumulative_average_reward: 0.4,
      },
    ],
    random_benchmark: {
      kind: "random",
      expected_average_reward: 0.25,
      sample_count: 1_250,
      reason: null,
      simulation_only: true,
    },
    oracle_benchmark: {
      kind: "oracle",
      expected_average_reward: 0.65,
      sample_count: 1_250,
      reason: null,
      simulation_only: true,
    },
    ope: {
      ips: 0.39,
      snips: 0.4,
      sample_count: 1_000,
      effective_sample_size: 822,
      weight_sum: 998,
      min_weight: 0.4,
      max_weight: 1.8,
      reason: null,
    },
    reasons: { [`offer:${offers[0]?.id}:empirical_mean`]: "no_outcomes" },
    generated_at: "2026-07-17T15:05:01Z",
    ...overrides,
  };
}

function makeDecisions(experiment: Experiment): Decision[] {
  const base: Decision = {
    decision_id: "c7a1d22c-d8f6-4f6b-97a4-a90d8d80e7f0",
    experiment_id: experiment.id,
    context: {
      device_class: "mobile",
      daypart: "evening",
      category_affinity: "dining",
      visitor_type: "returning",
    },
    selected_offer: {
      id: offers[0]?.id ?? "",
      slug: offers[0]?.slug ?? "",
      merchant_name: offers[0]?.merchant_name ?? "",
      title: offers[0]?.title ?? "",
      category: "dining",
    },
    eligible_offer_ids: offers.map((offer) => offer.id),
    propensity: 0.72,
    distribution: [
      { offer_id: offers[0]?.id ?? "", probability: 0.72 },
      { offer_id: offers[1]?.id ?? "", probability: 0.28 },
    ],
    policy_kind: experiment.policy_kind,
    policy_version: 84,
    policy_latency_micros: 55,
    outcome: null,
    created_at: "2026-07-17T15:05:00Z",
  };
  return [
    base,
    {
      ...base,
      decision_id: "5d05e445-56c8-436f-a8bf-5361ef766f12",
      outcome: {
        event_id: "d8cf6db1-8033-479f-a456-df65daf7d3b2",
        outcome: "converted",
        reward: 1,
        occurred_at: "2026-07-17T15:04:00Z",
        received_at: "2026-07-17T15:04:01Z",
        applied_policy_version: 85,
      },
      created_at: "2026-07-17T15:04:00Z",
    },
  ];
}

const runningRun: SimulationRun = {
  run_id: "6328c20e-818c-44e6-a741-7d073e564cee",
  experiment_id: experimentA.id,
  seed: 42,
  requests_per_second: 10,
  max_decisions: 5_000,
  status: "running",
  decision_count: 0,
  outcome_count: 0,
  error_count: 0,
  observed_reward_sum: 0,
  random_expected_reward_sum: 0,
  oracle_expected_reward_sum: 0,
  started_at: "2026-07-17T15:01:00Z",
  stopped_at: null,
  updated_at: "2026-07-17T15:01:00Z",
  error_code: null,
  error_detail: null,
};