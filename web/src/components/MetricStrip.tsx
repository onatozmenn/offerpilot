interface MetricStripProps {
  sampleCount: number | null;
  averageReward: number | null;
  engagementRate: number | null;
  explorationRate: number | null;
  p95PolicyLatencyMicros: number | null;
  policyVersion: number | null;
  reasons: Readonly<Record<string, string>>;
  isLoading?: boolean;
  isStale?: boolean;
}

interface MetricDefinition {
  key: string;
  label: string;
  qualifier: string;
  value: number | null;
  format: (value: number) => string;
}

export function MetricStrip({
  sampleCount,
  averageReward,
  engagementRate,
  explorationRate,
  p95PolicyLatencyMicros,
  policyVersion,
  reasons,
  isLoading = false,
  isStale = false,
}: MetricStripProps) {
  const metrics: MetricDefinition[] = [
    {
      key: "decision_count",
      label: "Sample count",
      qualifier: "logged decisions",
      value: sampleCount,
      format: formatInteger,
    },
    {
      key: "average_reward",
      label: "Average reward",
      qualifier: "observed outcomes",
      value: averageReward,
      format: formatReward,
    },
    {
      key: "engagement_rate",
      label: "Engagement proxy",
      qualifier: "clicked or converted",
      value: engagementRate,
      format: formatPercentage,
    },
    {
      key: "exploration_rate",
      label: "Exploration",
      qualifier: "current policy",
      value: explorationRate,
      format: formatPercentage,
    },
    {
      key: "p95_policy_latency_micros",
      label: "P95 policy latency",
      qualifier: "selection time",
      value: p95PolicyLatencyMicros,
      format: formatLatency,
    },
    {
      key: "policy_version",
      label: "Policy version",
      qualifier: "current model",
      value: policyVersion,
      format: (value) => `v${formatInteger(value)}`,
    },
  ];

  return (
    <div className="metric-strip" aria-busy={isLoading} data-stale={isStale || undefined}>
      <div className="metric-strip__status" aria-live="polite">
        {isStale ? "Data is stale; refresh to update." : ""}
      </div>
      <dl className="metric-strip__list">
        {metrics.map((metric) => (
          <div className="metric-strip__item" key={metric.key}>
            <dt>{metric.label}</dt>
            <dd>
              {isLoading ? (
                <span className="metric-strip__loading" aria-label={`${metric.label} loading`}>
                  &mdash;
                </span>
              ) : metric.value === null ? (
                <UnavailableMetric label={metric.label} reason={reasons[metric.key]} />
              ) : (
                metric.format(metric.value)
              )}
              <span className="metric-strip__qualifier">{metric.qualifier}</span>
            </dd>
          </div>
        ))}
      </dl>
    </div>
  );
}

function UnavailableMetric({ label, reason }: { label: string; reason: string | undefined }) {
  const explanation = reason ?? "Not available yet.";
  return (
    <span
      className="metric-strip__unavailable"
      aria-label={`${label} unavailable: ${humanizeReason(explanation)}`}
      title={humanizeReason(explanation)}
    >
      &mdash;
    </span>
  );
}

function formatInteger(value: number): string {
  return new Intl.NumberFormat("en-US", { maximumFractionDigits: 0 }).format(value);
}

function formatReward(value: number): string {
  return new Intl.NumberFormat("en-US", {
    minimumFractionDigits: 3,
    maximumFractionDigits: 3,
  }).format(value);
}

function formatPercentage(value: number): string {
  return new Intl.NumberFormat("en-US", {
    style: "percent",
    minimumFractionDigits: 0,
    maximumFractionDigits: 1,
  }).format(value);
}

function formatLatency(value: number): string {
  if (value >= 1_000) {
    return `${new Intl.NumberFormat("en-US", { maximumFractionDigits: 2 }).format(value / 1_000)} ms`;
  }
  return `${formatInteger(value)} µs`;
}

function humanizeReason(reason: string): string {
  const normalized = reason.replaceAll("_", " ").trim();
  return normalized === "" ? "Not available yet." : `${normalized[0]?.toUpperCase() ?? ""}${normalized.slice(1)}.`;
}
