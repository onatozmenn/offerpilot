import { Activity, CircleAlert, Plus, RefreshCw, X } from "lucide-react";
import { lazy, Suspense, useState } from "react";
import type { FormEvent } from "react";

import { DecisionFeed } from "./components/DecisionFeed";
import { MetricStrip } from "./components/MetricStrip";
import { OfferPerformanceTable } from "./components/OfferPerformanceTable";
import { SimulationControls } from "./components/SimulationControls";
import { useDashboard } from "./hooks/useDashboard";
import type { PanelStatus } from "./hooks/useDashboard";
import type { PolicyKind } from "./types/api";

const LearningChart = lazy(async () => {
  const module = await import("./components/LearningChart");
  return { default: module.LearningChart };
});

export default function App() {
  const dashboard = useDashboard();
  const [createOpen, setCreateOpen] = useState(false);
  const [experimentName, setExperimentName] = useState("Marketplace learning demo");
  const [policyKind, setPolicyKind] = useState<PolicyKind>("segmented_epsilon_greedy");
  const [epsilon, setEpsilon] = useState("0.15");
  const [isRefreshPending, setIsRefreshPending] = useState(false);

  const noExperiments = dashboard.status.experiments === "ready" && dashboard.experiments.length === 0;
  const showCreate = createOpen || noExperiments;
  const fatalExperimentsError = dashboard.status.experiments === "error" && dashboard.experiments.length === 0;
  const parsedEpsilon = Number(epsilon);
  const epsilonValid = epsilon.trim() !== "" && Number.isFinite(parsedEpsilon) && parsedEpsilon >= 0 && parsedEpsilon <= 1;
  const createValid = experimentName.trim().length >= 1 &&
    experimentName.trim().length <= 200 &&
    (policyKind === "random" || epsilonValid);

  const handleCreate = async (event: FormEvent<HTMLFormElement>): Promise<void> => {
    event.preventDefault();
    if (!createValid || dashboard.commandPending.create) {
      return;
    }
    const created = await dashboard.createDemo(
      experimentName,
      policyKind,
      policyKind === "segmented_epsilon_greedy" ? parsedEpsilon : undefined,
    );
    if (created) {
      setCreateOpen(false);
    }
  };

  const refresh = async (): Promise<void> => {
    if (isRefreshPending) {
      return;
    }
    setIsRefreshPending(true);
    try {
      await dashboard.refresh();
    } finally {
      setIsRefreshPending(false);
    }
  };

  return (
    <div className="app-shell" id="top">
      <header className="topbar">
        <div className="topbar__inner">
          <a className="brand-lockup" href="#top" aria-label="OfferPilot dashboard home">
            <h1>OfferPilot</h1>
          </a>

          <div className="topbar__operations">
            <HealthStatus status={dashboard.status.health} ready={dashboard.health?.status === "ready"} />
            <label className="experiment-select">
              <span>Experiment</span>
              <select
                value={dashboard.selectedExperiment?.id ?? ""}
                disabled={dashboard.status.experiments === "loading" || dashboard.experiments.length === 0}
                onChange={(event) => {
                  const selected = dashboard.experiments.find((item) => item.id === event.currentTarget.value);
                  if (selected !== undefined) {
                    dashboard.selectExperiment(selected);
                  }
                }}
              >
                {dashboard.experiments.length === 0 ? <option value="">No experiments</option> : null}
                {dashboard.experiments.map((experiment) => (
                  <option value={experiment.id} key={experiment.id}>
                    {experiment.name} ({experiment.status})
                  </option>
                ))}
              </select>
            </label>
            <button
              className="icon-button"
              type="button"
              aria-label="Refresh dashboard"
              title="Refresh dashboard"
              disabled={isRefreshPending}
              onClick={() => void refresh()}
            >
              <RefreshCw size={18} aria-hidden="true" className={isRefreshPending ? "spin" : undefined} />
            </button>
            <button className="button button--secondary" type="button" onClick={() => setCreateOpen(true)}>
              <Plus size={17} aria-hidden="true" />
              New experiment
            </button>
          </div>
        </div>
      </header>

      {showCreate ? (
        <section className="create-band" aria-labelledby="create-experiment-title">
          <div className="band-content">
            <div className="section-heading section-heading--compact">
              <div>
                <p className="section-heading__eyebrow">Fictional marketplace catalog</p>
                <h2 id="create-experiment-title">Create demo experiment</h2>
              </div>
              {!noExperiments ? (
                <button
                  className="icon-button"
                  type="button"
                  aria-label="Close experiment form"
                  title="Close"
                  onClick={() => setCreateOpen(false)}
                >
                  <X size={18} aria-hidden="true" />
                </button>
              ) : null}
            </div>
            <form className="create-form" onSubmit={(event) => void handleCreate(event)}>
              <label>
                <span>Name</span>
                <input
                  type="text"
                  required
                  minLength={1}
                  maxLength={200}
                  value={experimentName}
                  onChange={(event) => setExperimentName(event.currentTarget.value)}
                />
              </label>
              <label>
                <span>Policy</span>
                <select value={policyKind} onChange={(event) => setPolicyKind(event.currentTarget.value as PolicyKind)}>
                  <option value="segmented_epsilon_greedy">Segmented epsilon-greedy</option>
                  <option value="random">Random baseline</option>
                </select>
              </label>
              {policyKind === "segmented_epsilon_greedy" ? (
                <label>
                  <span>Epsilon</span>
                  <input
                    type="number"
                    min={0}
                    max={1}
                    step={0.01}
                    required
                    value={epsilon}
                    aria-invalid={!epsilonValid}
                    onChange={(event) => setEpsilon(event.currentTarget.value)}
                  />
                </label>
              ) : null}
              <button className="button button--primary" type="submit" disabled={!createValid || dashboard.commandPending.create}>
                <Plus size={17} aria-hidden="true" />
                {dashboard.commandPending.create ? "Creating..." : "Create experiment"}
              </button>
            </form>
            {dashboard.errors.command === null ? null : (
              <p className="inline-alert create-form__error" role="alert">{dashboard.errors.command}</p>
            )}
          </div>
        </section>
      ) : null}

      {fatalExperimentsError ? (
        <main className="fatal-state">
          <CircleAlert size={28} aria-hidden="true" />
          <h2>Dashboard unavailable</h2>
          <p>{dashboard.errors.experiments ?? "The experiment list could not be loaded."}</p>
          <button className="button button--primary" type="button" onClick={() => void refresh()}>
            <RefreshCw size={17} aria-hidden="true" />
            Retry
          </button>
        </main>
      ) : dashboard.selectedExperiment === null ? (
        <main className="empty-workspace" aria-busy={dashboard.status.experiments === "loading"}>
          <Activity size={30} aria-hidden="true" />
          <h2>{dashboard.status.experiments === "loading" ? "Loading experiments..." : "Create the first experiment"}</h2>
          <p>
            {dashboard.status.experiments === "loading"
              ? "Preparing the operational workspace."
              : "A demo experiment adds a fictional offer catalog and an online-learning policy."}
          </p>
        </main>
      ) : (
        <main className="dashboard">
          <section className="band band--controls" aria-labelledby="simulation-controls-title">
            <div className="band-content">
              <div className="experiment-context">
                <div>
                  <p className="section-heading__eyebrow">Active experiment</p>
                  <h2 id="simulation-controls-title">{dashboard.selectedExperiment.name}</h2>
                </div>
                <div className="experiment-context__meta">
                  <span>{humanize(dashboard.selectedExperiment.policy_kind)}</span>
                  <span>Policy v{dashboard.selectedExperiment.policy_version}</span>
                  {dashboard.experimentDetail === null ? null : <span>{dashboard.experimentDetail.offers.length} offers</span>}
                </div>
              </div>
              <SimulationControls
                seed={dashboard.formValues.seed}
                requestsPerSecond={dashboard.formValues.requestsPerSecond}
                maxDecisions={dashboard.formValues.maxDecisions}
                run={dashboard.selectedRun}
                isStartPending={dashboard.commandPending.start}
                isStopPending={dashboard.commandPending.stop}
                error={dashboard.errors.run ?? dashboard.errors.command}
                onChange={(field, value) => dashboard.updateFormValue(field, value)}
                onStart={dashboard.startRun}
                onStop={dashboard.stopRun}
              />
            </div>
          </section>

          <section className="band band--metrics" aria-labelledby="headline-metrics-title">
            <div className="band-content">
              <div className="section-heading section-heading--compact">
                <div>
                  <p className="section-heading__eyebrow">Current projection</p>
                  <h2 id="headline-metrics-title">Headline metrics</h2>
                </div>
              </div>
              <MetricStrip
                sampleCount={dashboard.summary?.decision_count ?? null}
                averageReward={dashboard.summary?.average_reward ?? null}
                engagementRate={dashboard.summary?.engagement_rate ?? null}
                explorationRate={dashboard.summary?.exploration_rate ?? null}
                p95PolicyLatencyMicros={dashboard.summary?.p95_policy_latency_micros ?? null}
                policyVersion={dashboard.summary?.policy_version ?? null}
                reasons={dashboard.summary?.reasons ?? {}}
                isLoading={dashboard.status.summary === "loading" && dashboard.summary === null}
                isStale={dashboard.status.summary === "stale"}
              />
              {dashboard.status.summary === "error" && dashboard.errors.summary !== null ? (
                <p className="inline-alert" role="alert">{dashboard.errors.summary}</p>
              ) : null}
            </div>
          </section>

          <section className="band band--learning">
            <div className="band-content">
              <Suspense fallback={<LearningChartFallback />}>
                <LearningChart
                  points={dashboard.summary?.learning_series ?? []}
                  randomBenchmark={dashboard.summary?.random_benchmark ?? null}
                  oracleBenchmark={dashboard.summary?.oracle_benchmark ?? null}
                  isLoading={dashboard.status.summary === "loading" && dashboard.summary === null}
                  isError={dashboard.status.summary === "error" || dashboard.status.summary === "stale"}
                  isStale={dashboard.status.summary === "stale"}
                  error={dashboard.errors.summary}
                />
              </Suspense>
            </div>
          </section>

          <section className="band band--offers">
            <div className="band-content">
              <OfferPerformanceTable
                rows={dashboard.summary?.offer_performance ?? []}
                reasons={dashboard.summary?.reasons ?? {}}
                isLoading={dashboard.status.summary === "loading" && dashboard.summary === null}
                isError={dashboard.status.summary === "error" || dashboard.status.summary === "stale"}
                isStale={dashboard.status.summary === "stale"}
                error={dashboard.errors.summary}
              />
            </div>
          </section>

          <section className="band band--feed">
            <div className="band-content">
              <DecisionFeed
                items={dashboard.decisions}
                isLoading={dashboard.status.decisions === "loading" && dashboard.decisions.length === 0}
                isError={dashboard.status.decisions === "error" || dashboard.status.decisions === "stale"}
                isStale={dashboard.status.decisions === "stale"}
                error={dashboard.errors.decisions}
              />
            </div>
          </section>
        </main>
      )}
    </div>
  );
}

function HealthStatus({ status, ready }: { status: PanelStatus; ready: boolean }) {
  const label = status === "loading"
    ? "API health loading"
    : ready
      ? "API ready"
      : status === "stale"
        ? "API health stale"
        : "API not ready";
  return <span className="sr-only" role="status" aria-label={label} aria-live="polite">{label}</span>;
}

function LearningChartFallback() {
  return (
    <figure className="learning-chart" aria-labelledby="learning-chart-title" aria-busy="true">
      <div className="section-heading">
        <div>
          <p className="section-heading__eyebrow">Learning trajectory</p>
          <h2 id="learning-chart-title">Cumulative observed reward</h2>
        </div>
      </div>
      <div className="learning-chart__viewport">
        <div className="learning-chart__placeholder" role="status">Loading visualization...</div>
      </div>
    </figure>
  );
}

function humanize(value: string): string {
  const normalized = value.replaceAll("_", " ");
  return `${normalized[0]?.toUpperCase() ?? ""}${normalized.slice(1)}`;
}