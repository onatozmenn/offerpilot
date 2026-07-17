import {
  CartesianGrid,
  Line,
  LineChart,
  ReferenceLine,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";

import type { LearningSeriesPoint, SimulationBenchmark } from "../types/api";

interface LearningChartProps {
  points: readonly LearningSeriesPoint[];
  randomBenchmark: SimulationBenchmark | null;
  oracleBenchmark: SimulationBenchmark | null;
  isLoading?: boolean;
  isError?: boolean;
  isStale?: boolean;
  error?: string | null;
}

export function LearningChart({
  points,
  randomBenchmark,
  oracleBenchmark,
  isLoading = false,
  isError = false,
  isStale = false,
  error = null,
}: LearningChartProps) {
  const currentPoint = points.at(-1) ?? null;
  const hasPoints = currentPoint !== null;
  const randomValue = randomBenchmark?.expected_average_reward ?? null;
  const oracleValue = oracleBenchmark?.expected_average_reward ?? null;

  return (
    <figure
      className="learning-chart"
      aria-labelledby="learning-chart-title"
      aria-busy={isLoading}
      data-stale={isStale || undefined}
    >
      <div className="section-heading">
        <div>
          <p className="section-heading__eyebrow">Learning trajectory</p>
          <h2 id="learning-chart-title">Cumulative observed reward</h2>
        </div>
        {isStale ? <span className="status-label status-label--stale">Stale data</span> : null}
      </div>

      {isError && error !== null ? <p className="inline-alert" role="alert">{error}</p> : null}

      <ul className="chart-legend" aria-label="Chart series">
        <LegendItem
          className="chart-legend__line--observed"
          label="Observed cumulative average"
          unavailableReason={null}
        />
        <LegendItem
          className="chart-legend__line--random"
          label="Random reference (simulation-only)"
          unavailableReason={randomValue === null ? (randomBenchmark?.reason ?? "not available yet") : null}
        />
        <LegendItem
          className="chart-legend__line--oracle"
          label="Oracle reference (simulation-only)"
          unavailableReason={oracleValue === null ? (oracleBenchmark?.reason ?? "not available yet") : null}
        />
      </ul>

      <div className="learning-chart__viewport" aria-hidden={hasPoints ? true : undefined}>
        {hasPoints ? (
          <ResponsiveContainer width="100%" height="100%">
            <LineChart data={[...points]} margin={{ top: 16, right: 20, bottom: 12, left: 4 }}>
              <CartesianGrid strokeDasharray="2 5" vertical={false} />
              <XAxis
                dataKey="timestamp"
                minTickGap={30}
                tickFormatter={formatAxisTimestamp}
                tickLine={false}
              />
              <YAxis
                domain={[0, 1]}
                width={46}
                tickCount={5}
                tickFormatter={formatPercentage}
                tickLine={false}
              />
              <Tooltip
                labelFormatter={(label) => formatFullTimestamp(String(label))}
                formatter={(value) => [formatReward(Number(value)), "Observed cumulative average"]}
              />
              {randomValue === null ? null : (
                <ReferenceLine
                  y={randomValue}
                  stroke="var(--color-benchmark-random)"
                  strokeDasharray="8 5"
                  strokeWidth={2}
                />
              )}
              {oracleValue === null ? null : (
                <ReferenceLine
                  y={oracleValue}
                  stroke="var(--color-benchmark-oracle)"
                  strokeDasharray="3 4 9 4"
                  strokeWidth={2}
                />
              )}
              <Line
                type="linear"
                dataKey="cumulative_average_reward"
                name="Observed cumulative average"
                stroke="var(--color-observed)"
                strokeWidth={3}
                dot={false}
                activeDot={{ r: 4 }}
                connectNulls={false}
                isAnimationActive={false}
              />
            </LineChart>
          </ResponsiveContainer>
        ) : (
          <div className="learning-chart__placeholder">
            {isLoading ? "Loading learning data..." : "No learning data yet. Start a simulation run."}
          </div>
        )}
      </div>

      <figcaption className="learning-chart__summary">
        {hasPoints ? (
          <>
            Observed cumulative average reward is <strong>{formatReward(currentPoint.cumulative_average_reward)}</strong>
            {" after "}<strong>{currentPoint.sample_count.toLocaleString("en-US")}</strong>{" outcomes. "}
            Random simulation reference: <BenchmarkText benchmark={randomBenchmark} />. Oracle simulation reference:{" "}
            <BenchmarkText benchmark={oracleBenchmark} />.
          </>
        ) : (
          "The chart will summarize cumulative observed reward after outcomes are recorded."
        )}
      </figcaption>
    </figure>
  );
}

interface LegendItemProps {
  className: string;
  label: string;
  unavailableReason: string | null;
}

function LegendItem({ className, label, unavailableReason }: LegendItemProps) {
  const unavailable = unavailableReason !== null;
  return (
    <li className={unavailable ? "chart-legend__item chart-legend__item--unavailable" : "chart-legend__item"}>
      <span className={`chart-legend__line ${className}`} aria-hidden="true" />
      <span>{label}{unavailable ? `: unavailable (${humanizeReason(unavailableReason)})` : ""}</span>
    </li>
  );
}

function BenchmarkText({ benchmark }: { benchmark: SimulationBenchmark | null }) {
  if (benchmark?.expected_average_reward === null || benchmark === null) {
    return <span title={benchmark?.reason ?? "Not available yet"}>unavailable</span>;
  }
  return <strong>{formatReward(benchmark.expected_average_reward)}</strong>;
}

function formatReward(value: number): string {
  return new Intl.NumberFormat("en-US", {
    minimumFractionDigits: 3,
    maximumFractionDigits: 3,
  }).format(value);
}

function formatPercentage(value: number): string {
  return new Intl.NumberFormat("en-US", { style: "percent", maximumFractionDigits: 0 }).format(value);
}

function formatAxisTimestamp(value: string): string {
  const date = new Date(value);
  if (Number.isNaN(date.valueOf())) {
    return value;
  }
  return new Intl.DateTimeFormat("en-US", { hour: "2-digit", minute: "2-digit" }).format(date);
}

function formatFullTimestamp(value: string): string {
  const date = new Date(value);
  if (Number.isNaN(date.valueOf())) {
    return value;
  }
  return new Intl.DateTimeFormat("en-US", {
    dateStyle: "medium",
    timeStyle: "medium",
  }).format(date);
}

function humanizeReason(value: string): string {
  return value.replaceAll("_", " ");
}