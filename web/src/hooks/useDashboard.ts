import { useCallback, useEffect, useRef, useState, useTransition } from "react";

import { ApiClient, ApiProblemError } from "../api/client";
import type {
  CreateDemoExperimentRequest,
  Decision,
  Experiment,
  ExperimentDetail,
  ExperimentSummary,
  Health,
  PolicyKind,
  SimulationRun,
  SimulationRunStatus,
} from "../types/api";

export const activePollIntervalMs = 750;
export const idlePollIntervalMs = 5_000;

export type PanelStatus = "idle" | "loading" | "ready" | "stale" | "error";
export type SimulationFormField = "seed" | "requestsPerSecond" | "maxDecisions";

export interface SimulationFormValues {
  seed: number | "";
  requestsPerSecond: number | "";
  maxDecisions: number | "";
}

export interface DashboardStatus {
  health: PanelStatus;
  experiments: PanelStatus;
  detail: PanelStatus;
  summary: PanelStatus;
  decisions: PanelStatus;
  run: PanelStatus;
}

export interface DashboardErrors {
  health: string | null;
  experiments: string | null;
  detail: string | null;
  summary: string | null;
  decisions: string | null;
  run: string | null;
  command: string | null;
}

export interface CommandPending {
  create: boolean;
  start: boolean;
  stop: boolean;
}

export interface DashboardState {
  health: Health | null;
  experiments: Experiment[];
  selectedExperiment: Experiment | null;
  experimentDetail: ExperimentDetail | null;
  summary: ExperimentSummary | null;
  decisions: Decision[];
  selectedRun: SimulationRun | null;
  formValues: SimulationFormValues;
  status: DashboardStatus;
  errors: DashboardErrors;
  commandPending: CommandPending;
  isExperimentSwitchPending: boolean;
  canStart: boolean;
  canStop: boolean;
  selectExperiment: (experiment: Experiment) => void;
  createDemo: (name: string, policyKind: PolicyKind, epsilon?: number) => Promise<void>;
  startRun: () => Promise<void>;
  stopRun: () => Promise<void>;
  updateFormValue: (field: SimulationFormField, value: string | number) => void;
  refresh: () => Promise<void>;
}

const defaultClient = new ApiClient();

const initialStatus: DashboardStatus = {
  health: "loading",
  experiments: "loading",
  detail: "idle",
  summary: "idle",
  decisions: "idle",
  run: "idle",
};

const initialErrors: DashboardErrors = {
  health: null,
  experiments: null,
  detail: null,
  summary: null,
  decisions: null,
  run: null,
  command: null,
};

const initialFormValues: SimulationFormValues = {
  seed: 42,
  requestsPerSecond: 25,
  maxDecisions: 5_000,
};

export function useDashboard(client: ApiClient = defaultClient): DashboardState {
  const [health, setHealth] = useState<Health | null>(null);
  const [experiments, setExperiments] = useState<Experiment[]>([]);
  const [selectedExperiment, setSelectedExperiment] = useState<Experiment | null>(null);
  const [experimentDetail, setExperimentDetail] = useState<ExperimentDetail | null>(null);
  const [summary, setSummary] = useState<ExperimentSummary | null>(null);
  const [decisions, setDecisions] = useState<Decision[]>([]);
  const [selectedRun, setSelectedRun] = useState<SimulationRun | null>(null);
  const [formValues, setFormValues] = useState<SimulationFormValues>(initialFormValues);
  const [status, setStatus] = useState<DashboardStatus>(initialStatus);
  const [errors, setErrors] = useState<DashboardErrors>(initialErrors);
  const [commandPending, setCommandPending] = useState<CommandPending>({
    create: false,
    start: false,
    stop: false,
  });
  const [isExperimentSwitchPending, startTransition] = useTransition();

  const mountedRef = useRef(false);
  const healthRef = useRef<Health | null>(null);
  const experimentsRef = useRef<Experiment[]>([]);
  const detailRef = useRef<ExperimentDetail | null>(null);
  const summaryRef = useRef<ExperimentSummary | null>(null);
  const decisionsRef = useRef<Decision[]>([]);
  const decisionsLoadedRef = useRef(false);
  const selectedExperimentRef = useRef<Experiment | null>(null);
  const selectedRunRef = useRef<SimulationRun | null>(null);
  const healthControllerRef = useRef<AbortController | null>(null);
  const experimentsControllerRef = useRef<AbortController | null>(null);
  const refreshControllerRef = useRef<AbortController | null>(null);
  const commandControllersRef = useRef(new Set<AbortController>());
  const refreshSequenceRef = useRef(0);
  const commandLocksRef = useRef({ create: false, start: false, stop: false });

  const applySelectedExperiment = useCallback(
    (experiment: Experiment): void => {
      selectedExperimentRef.current = experiment;
      startTransition(() => {
        setSelectedExperiment(experiment);
      });
    },
    [startTransition],
  );

  const refreshHealth = useCallback(
    async (showLoading: boolean): Promise<void> => {
      healthControllerRef.current?.abort();
      const controller = new AbortController();
      healthControllerRef.current = controller;
      if (showLoading && healthRef.current === null) {
        setStatus((current) => ({ ...current, health: "loading" }));
      }
      try {
        const nextHealth = await client.getHealth(controller.signal);
        if (!mountedRef.current || controller.signal.aborted) {
          return;
        }
        healthRef.current = nextHealth;
        setHealth(nextHealth);
        setStatus((current) => ({ ...current, health: "ready" }));
        setErrors((current) => ({ ...current, health: null }));
      } catch (error: unknown) {
        if (!mountedRef.current || controller.signal.aborted || isAbortError(error)) {
          return;
        }
        setStatus((current) => ({
          ...current,
          health: healthRef.current === null ? "error" : "stale",
        }));
        setErrors((current) => ({ ...current, health: errorMessage(error) }));
      } finally {
        if (healthControllerRef.current === controller) {
          healthControllerRef.current = null;
        }
      }
    },
    [client],
  );

  const loadExperiments = useCallback(
    async (showLoading: boolean): Promise<void> => {
      experimentsControllerRef.current?.abort();
      const controller = new AbortController();
      experimentsControllerRef.current = controller;
      if (showLoading && experimentsRef.current.length === 0) {
        setStatus((current) => ({ ...current, experiments: "loading" }));
      }
      try {
        const page = await client.listExperiments(50, undefined, controller.signal);
        if (!mountedRef.current || controller.signal.aborted) {
          return;
        }
        const nextExperiments = [...page.items].sort(compareExperimentsNewestFirst);
        experimentsRef.current = nextExperiments;
        setExperiments(nextExperiments);
        setStatus((current) => ({ ...current, experiments: "ready" }));
        setErrors((current) => ({ ...current, experiments: null }));

        const currentSelection = selectedExperimentRef.current;
        const nextSelection = currentSelection
          ? (nextExperiments.find((experiment) => experiment.id === currentSelection.id) ?? nextExperiments[0] ?? null)
          : (nextExperiments[0] ?? null);
        if (nextSelection !== null) {
          if (currentSelection?.id === nextSelection.id) {
            selectedExperimentRef.current = nextSelection;
            setSelectedExperiment(nextSelection);
          } else {
            applySelectedExperiment(nextSelection);
          }
        } else {
          selectedExperimentRef.current = null;
          setSelectedExperiment(null);
        }
      } catch (error: unknown) {
        if (!mountedRef.current || controller.signal.aborted || isAbortError(error)) {
          return;
        }
        setStatus((current) => ({
          ...current,
          experiments: experimentsRef.current.length === 0 ? "error" : "stale",
        }));
        setErrors((current) => ({ ...current, experiments: errorMessage(error) }));
      } finally {
        if (experimentsControllerRef.current === controller) {
          experimentsControllerRef.current = null;
        }
      }
    },
    [applySelectedExperiment, client],
  );

  const refreshSelection = useCallback(
    async (experimentId: string, runId: string | null, includeDetail: boolean): Promise<void> => {
      const sequence = ++refreshSequenceRef.current;
      refreshControllerRef.current?.abort();
      const controller = new AbortController();
      refreshControllerRef.current = controller;

      const hadDetail = detailRef.current !== null;
      const hadSummary = summaryRef.current !== null;
      const hadDecisions = decisionsLoadedRef.current;
      const hadRun = selectedRunRef.current !== null;

      const [detailResult, summaryResult, decisionsResult, runResult] = await Promise.allSettled([
        includeDetail
          ? client.getExperiment(experimentId, controller.signal)
          : Promise.resolve<ExperimentDetail | null>(null),
        client.getExperimentSummary(experimentId, 120, controller.signal),
        client.listExperimentDecisions(experimentId, 50, undefined, controller.signal),
        runId === null
          ? Promise.resolve<SimulationRun | null>(null)
          : client.getSimulationRun(runId, controller.signal),
      ] as const);

      if (
        !mountedRef.current ||
        controller.signal.aborted ||
        sequence !== refreshSequenceRef.current ||
        selectedExperimentRef.current?.id !== experimentId
      ) {
        return;
      }

      if (includeDetail && detailResult.status === "fulfilled" && detailResult.value !== null) {
        detailRef.current = detailResult.value;
        setExperimentDetail(detailResult.value);
      }
      if (summaryResult.status === "fulfilled") {
        summaryRef.current = summaryResult.value;
        setSummary(summaryResult.value);
      }
      if (decisionsResult.status === "fulfilled") {
        decisionsRef.current = decisionsResult.value.items;
        decisionsLoadedRef.current = true;
        setDecisions(decisionsResult.value.items);
      }
      if (runResult.status === "fulfilled" && runResult.value !== null) {
        selectedRunRef.current = runResult.value;
        setSelectedRun(runResult.value);
      }

      setStatus((current) => ({
        ...current,
        detail: includeDetail
          ? resultStatus(detailResult, hadDetail)
          : current.detail,
        summary: resultStatus(summaryResult, hadSummary),
        decisions: resultStatus(decisionsResult, hadDecisions),
        run: runId === null ? current.run : resultStatus(runResult, hadRun),
      }));
      setErrors((current) => ({
        ...current,
        detail: includeDetail ? resultError(detailResult) : current.detail,
        summary: resultError(summaryResult),
        decisions: resultError(decisionsResult),
        run: runId === null ? current.run : resultError(runResult),
      }));

      if (refreshControllerRef.current === controller) {
        refreshControllerRef.current = null;
      }
    },
    [client],
  );

  useEffect(() => {
    const commandControllers = commandControllersRef.current;
    mountedRef.current = true;
    void Promise.all([refreshHealth(true), loadExperiments(true)]);
    return () => {
      mountedRef.current = false;
      refreshSequenceRef.current += 1;
      healthControllerRef.current?.abort();
      experimentsControllerRef.current?.abort();
      refreshControllerRef.current?.abort();
      for (const controller of commandControllers) {
        controller.abort();
      }
      commandControllers.clear();
    };
  }, [loadExperiments, refreshHealth]);

  useEffect(() => {
    const experimentId = selectedExperiment?.id ?? null;
    refreshSequenceRef.current += 1;
    refreshControllerRef.current?.abort();
    detailRef.current = null;
    summaryRef.current = null;
    decisionsRef.current = [];
    decisionsLoadedRef.current = false;
    selectedRunRef.current = null;
    setExperimentDetail(null);
    setSummary(null);
    setDecisions([]);
    setSelectedRun(null);

    if (experimentId === null) {
      setStatus((current) => ({
        ...current,
        detail: "idle",
        summary: "idle",
        decisions: "idle",
        run: "idle",
      }));
      return;
    }

    setStatus((current) => ({
      ...current,
      detail: "loading",
      summary: "loading",
      decisions: "loading",
      run: "idle",
    }));
    setErrors((current) => ({
      ...current,
      detail: null,
      summary: null,
      decisions: null,
      run: null,
      command: null,
    }));
    void refreshSelection(experimentId, null, true);
  }, [refreshSelection, selectedExperiment?.id]);

  const selectedRunId = selectedRun?.run_id ?? null;
  const selectedRunIsActive = isActiveRunStatus(selectedRun?.status);

  useEffect(() => {
    const experimentId = selectedExperiment?.id;
    if (experimentId === undefined) {
      return;
    }
    const interval = selectedRunIsActive ? activePollIntervalMs : idlePollIntervalMs;
    let cancelled = false;
    let timer = 0;

    const poll = async (): Promise<void> => {
      await refreshSelection(experimentId, selectedRunId, false);
      if (!cancelled) {
        timer = window.setTimeout(() => {
          void poll();
        }, interval);
      }
    };

    timer = window.setTimeout(() => {
      void poll();
    }, interval);
    return () => {
      cancelled = true;
      window.clearTimeout(timer);
    };
  }, [refreshSelection, selectedExperiment?.id, selectedRunId, selectedRunIsActive]);

  const selectExperiment = useCallback(
    (experiment: Experiment): void => {
      if (selectedExperimentRef.current?.id === experiment.id) {
        return;
      }
      applySelectedExperiment(experiment);
    },
    [applySelectedExperiment],
  );

  const createDemo = useCallback(
    async (name: string, policyKind: PolicyKind, epsilon?: number): Promise<void> => {
      if (commandLocksRef.current.create) {
        return;
      }
      const trimmedName = name.trim();
      if (trimmedName === "" || trimmedName.length > 200) {
        setErrors((current) => ({ ...current, command: "Experiment name must contain 1 to 200 characters." }));
        return;
      }
      if (policyKind === "segmented_epsilon_greedy" && !isProbability(epsilon)) {
        setErrors((current) => ({ ...current, command: "Epsilon must be between 0 and 1." }));
        return;
      }

      const body: CreateDemoExperimentRequest = policyKind === "random"
        ? { name: trimmedName, policy_kind: "random" }
        : { name: trimmedName, policy_kind: "segmented_epsilon_greedy", epsilon: epsilon as number };
      const controller = new AbortController();
      commandControllersRef.current.add(controller);
      commandLocksRef.current.create = true;
      setCommandPending((current) => ({ ...current, create: true }));
      setErrors((current) => ({ ...current, command: null }));
      try {
        const created = await client.createDemoExperiment(body, controller.signal);
        if (!mountedRef.current || controller.signal.aborted) {
          return;
        }
        const nextExperiments = [created, ...experimentsRef.current.filter((item) => item.id !== created.id)];
        experimentsRef.current = nextExperiments;
        setExperiments(nextExperiments);
        setStatus((current) => ({ ...current, experiments: "ready" }));
        applySelectedExperiment(created);
      } catch (error: unknown) {
        if (mountedRef.current && !controller.signal.aborted && !isAbortError(error)) {
          setErrors((current) => ({ ...current, command: errorMessage(error) }));
        }
      } finally {
        commandControllersRef.current.delete(controller);
        commandLocksRef.current.create = false;
        if (mountedRef.current) {
          setCommandPending((current) => ({ ...current, create: false }));
        }
      }
    },
    [applySelectedExperiment, client],
  );

  const startRun = useCallback(async (): Promise<void> => {
    const experiment = selectedExperimentRef.current;
    if (commandLocksRef.current.start || experiment === null || isActiveRunStatus(selectedRunRef.current?.status)) {
      return;
    }
    if (!validSimulationForm(formValues)) {
      setErrors((current) => ({ ...current, command: "Enter a valid seed, rate, and maximum decision count." }));
      return;
    }

    const controller = new AbortController();
    commandControllersRef.current.add(controller);
    commandLocksRef.current.start = true;
    setCommandPending((current) => ({ ...current, start: true }));
    setErrors((current) => ({ ...current, command: null, run: null }));
    setStatus((current) => ({ ...current, run: selectedRunRef.current === null ? "loading" : current.run }));
    try {
      const run = await client.createSimulationRun(
        experiment.id,
        {
          seed: formValues.seed,
          requests_per_second: formValues.requestsPerSecond,
          max_decisions: formValues.maxDecisions,
        },
        controller.signal,
      );
      if (!mountedRef.current || controller.signal.aborted || selectedExperimentRef.current?.id !== experiment.id) {
        return;
      }
      selectedRunRef.current = run;
      setSelectedRun(run);
      setStatus((current) => ({ ...current, run: "ready" }));
      await refreshSelection(experiment.id, run.run_id, false);
    } catch (error: unknown) {
      if (mountedRef.current && !controller.signal.aborted && !isAbortError(error)) {
        setStatus((current) => ({ ...current, run: selectedRunRef.current === null ? "error" : "stale" }));
        setErrors((current) => ({ ...current, run: errorMessage(error), command: errorMessage(error) }));
      }
    } finally {
      commandControllersRef.current.delete(controller);
      commandLocksRef.current.start = false;
      if (mountedRef.current) {
        setCommandPending((current) => ({ ...current, start: false }));
      }
    }
  }, [client, formValues, refreshSelection]);

  const stopRun = useCallback(async (): Promise<void> => {
    const run = selectedRunRef.current;
    if (commandLocksRef.current.stop || run === null || !isActiveRunStatus(run.status)) {
      return;
    }
    const controller = new AbortController();
    commandControllersRef.current.add(controller);
    commandLocksRef.current.stop = true;
    setCommandPending((current) => ({ ...current, stop: true }));
    setErrors((current) => ({ ...current, command: null, run: null }));
    try {
      const stoppedRun = await client.stopSimulationRun(run.run_id, controller.signal);
      if (!mountedRef.current || controller.signal.aborted || selectedRunRef.current?.run_id !== run.run_id) {
        return;
      }
      selectedRunRef.current = stoppedRun;
      setSelectedRun(stoppedRun);
      setStatus((current) => ({ ...current, run: "ready" }));
      const experimentId = selectedExperimentRef.current?.id;
      if (experimentId !== undefined) {
        await refreshSelection(experimentId, stoppedRun.run_id, false);
      }
    } catch (error: unknown) {
      if (mountedRef.current && !controller.signal.aborted && !isAbortError(error)) {
        setStatus((current) => ({ ...current, run: "stale" }));
        setErrors((current) => ({ ...current, run: errorMessage(error), command: errorMessage(error) }));
      }
    } finally {
      commandControllersRef.current.delete(controller);
      commandLocksRef.current.stop = false;
      if (mountedRef.current) {
        setCommandPending((current) => ({ ...current, stop: false }));
      }
    }
  }, [client, refreshSelection]);

  const updateFormValue = useCallback((field: SimulationFormField, value: string | number): void => {
    const nextValue = value === "" ? "" : Number(value);
    setFormValues((current) => ({ ...current, [field]: Number.isNaN(nextValue) ? "" : nextValue }));
  }, []);

  const refresh = useCallback(async (): Promise<void> => {
    const experiment = selectedExperimentRef.current;
    const run = selectedRunRef.current;
    await Promise.all([
      refreshHealth(false),
      loadExperiments(false),
      experiment === null
        ? Promise.resolve()
        : refreshSelection(experiment.id, run?.run_id ?? null, false),
    ]);
  }, [loadExperiments, refreshHealth, refreshSelection]);

  const canStart = selectedExperiment !== null &&
    !isActiveRunStatus(selectedRun?.status) &&
    !commandPending.start &&
    !commandPending.stop &&
    validSimulationForm(formValues);
  const canStop = selectedRunIsActive && !commandPending.stop && !commandPending.start;

  return {
    health,
    experiments,
    selectedExperiment,
    experimentDetail,
    summary,
    decisions,
    selectedRun,
    formValues,
    status,
    errors,
    commandPending,
    isExperimentSwitchPending,
    canStart,
    canStop,
    selectExperiment,
    createDemo,
    startRun,
    stopRun,
    updateFormValue,
    refresh,
  };
}

function compareExperimentsNewestFirst(left: Experiment, right: Experiment): number {
  return Date.parse(right.created_at) - Date.parse(left.created_at);
}

function isActiveRunStatus(status: SimulationRunStatus | undefined): boolean {
  return status === "starting" || status === "running" || status === "stopping";
}

function validSimulationForm(values: SimulationFormValues): values is {
  seed: number;
  requestsPerSecond: number;
  maxDecisions: number;
} {
  return isSafeInteger(values.seed) &&
    isSafeInteger(values.requestsPerSecond) &&
    values.requestsPerSecond >= 1 &&
    values.requestsPerSecond <= 100 &&
    isSafeInteger(values.maxDecisions) &&
    values.maxDecisions >= 1 &&
    values.maxDecisions <= 100_000;
}

function isSafeInteger(value: number | ""): value is number {
  return typeof value === "number" && Number.isSafeInteger(value);
}

function isProbability(value: number | undefined): value is number {
  return typeof value === "number" && Number.isFinite(value) && value >= 0 && value <= 1;
}

function resultStatus<T>(result: PromiseSettledResult<T>, hadValue: boolean): PanelStatus {
  return result.status === "fulfilled" ? "ready" : hadValue ? "stale" : "error";
}

function resultError<T>(result: PromiseSettledResult<T>): string | null {
  return result.status === "rejected" ? errorMessage(result.reason) : null;
}

function errorMessage(error: unknown): string {
  if (error instanceof ApiProblemError) {
    return error.request_id === "" ? error.detail : `${error.detail} Request ID: ${error.request_id}`;
  }
  if (error instanceof Error && error.message !== "") {
    return error.message;
  }
  return "The request failed unexpectedly.";
}

function isAbortError(error: unknown): boolean {
  return error instanceof DOMException
    ? error.name === "AbortError"
    : error instanceof Error && error.name === "AbortError";
}