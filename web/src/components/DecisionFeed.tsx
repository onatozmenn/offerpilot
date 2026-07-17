import { useRef, useState } from "react";

import type { Decision, DecisionOutcome } from "../types/api";

interface DecisionFeedProps {
  items: readonly Decision[];
  isLoading?: boolean;
  isError?: boolean;
  isStale?: boolean;
  error?: string | null;
  onLoadMore?: () => Promise<void>;
}

const maximumRenderedDecisions = 200;

export function DecisionFeed({
  items,
  isLoading = false,
  isError = false,
  isStale = false,
  error = null,
  onLoadMore,
}: DecisionFeedProps) {
  const [isLoadingMore, setIsLoadingMore] = useState(false);
  const [loadMoreError, setLoadMoreError] = useState<string | null>(null);
  const loadMoreInFlightRef = useRef(false);
  const visibleItems = items.slice(0, maximumRenderedDecisions);
  const visibleError = error ?? loadMoreError;

  const loadMore = async (): Promise<void> => {
    if (onLoadMore === undefined || loadMoreInFlightRef.current) {
      return;
    }
    loadMoreInFlightRef.current = true;
    setIsLoadingMore(true);
    setLoadMoreError(null);
    try {
      await onLoadMore();
    } catch {
      setLoadMoreError("Could not load more decisions.");
    } finally {
      loadMoreInFlightRef.current = false;
      setIsLoadingMore(false);
    }
  };

  return (
    <section
      className="decision-feed"
      aria-labelledby="decision-feed-title"
      aria-busy={isLoading || isLoadingMore}
      data-stale={isStale || undefined}
    >
      <div className="section-heading">
        <div>
          <p className="section-heading__eyebrow">Audit stream</p>
          <h2 id="decision-feed-title">Recent decisions</h2>
        </div>
        <div className="section-heading__statuses">
          {items.length > maximumRenderedDecisions ? (
            <span className="status-label">Showing newest {maximumRenderedDecisions}</span>
          ) : null}
          {isStale ? <span className="status-label status-label--stale">Not refreshing</span> : null}
        </div>
      </div>

      {(isError || loadMoreError !== null) && visibleError !== null ? (
        <p className="inline-alert" role="alert">{visibleError}</p>
      ) : null}

      <div className="decision-feed__viewport" role="region" aria-labelledby="decision-feed-title" tabIndex={0}>
        <table className="data-table decision-feed__table">
          <caption className="sr-only">
            Recent policy decisions with privacy-safe context, selected fictional offer, propensity, version, and outcome.
          </caption>
          <thead>
            <tr>
              <th scope="col">Decision</th>
              <th scope="col">Context</th>
              <th scope="col">Selected offer</th>
              <th scope="col" className="numeric-cell">Propensity</th>
              <th scope="col" className="numeric-cell">Policy</th>
              <th scope="col">Outcome</th>
            </tr>
          </thead>
          <tbody>
            {visibleItems.map((decision) => (
              <tr key={decision.decision_id}>
                <th scope="row" className="decision-feed__decision">
                  <time dateTime={decision.created_at} title={decision.created_at}>
                    {formatTimestamp(decision.created_at)}
                  </time>
                  <code title={decision.decision_id}>{truncateIdentifier(decision.decision_id)}</code>
                </th>
                <td>
                  <div className="context-chips">
                    <ContextChip label="Device" value={decision.context.device_class} />
                    <ContextChip label="Daypart" value={decision.context.daypart} />
                    <ContextChip label="Affinity" value={decision.context.category_affinity} />
                    <ContextChip label="Visitor" value={decision.context.visitor_type} />
                  </div>
                </td>
                <td className="decision-feed__offer">
                  <span title={`${decision.selected_offer.merchant_name}: ${decision.selected_offer.title}`}>
                    {decision.selected_offer.merchant_name}
                  </span>
                  <small>{decision.selected_offer.title}</small>
                  <code title={decision.selected_offer.id}>{truncateIdentifier(decision.selected_offer.id)}</code>
                </td>
                <td className="numeric-cell">{formatPercentage(decision.propensity)}</td>
                <td className="numeric-cell"><span title={`Policy version ${decision.policy_version}`}>v{decision.policy_version}</span></td>
                <td><OutcomeValue outcome={decision.outcome} decisionVersion={decision.policy_version} /></td>
              </tr>
            ))}
            {visibleItems.length === 0 ? (
              <tr>
                <td className="data-table__empty" colSpan={6}>
                  {isLoading
                    ? "Loading recent decisions..."
                    : isError
                      ? "Recent decisions are unavailable."
                      : "No decisions have been recorded yet."}
                </td>
              </tr>
            ) : null}
          </tbody>
        </table>
      </div>

      {onLoadMore === undefined ? null : (
        <button
          className="button button--secondary decision-feed__load-more"
          type="button"
          disabled={isLoadingMore}
          onClick={() => void loadMore()}
        >
          {isLoadingMore ? "Loading..." : "Load older decisions"}
        </button>
      )}
    </section>
  );
}

function ContextChip({ label, value }: { label: string; value: string }) {
  return <span className="context-chip" aria-label={`${label}: ${humanize(value)}`}>{humanize(value)}</span>;
}

function OutcomeValue({ outcome, decisionVersion }: { outcome: DecisionOutcome | null; decisionVersion: number }) {
  if (outcome === null) {
    return <span className="outcome outcome--pending">Pending outcome</span>;
  }
  const versionDiffers = outcome.applied_policy_version !== decisionVersion;
  return (
    <span className={`outcome outcome--${outcome.outcome}`}>
      <span>{humanize(outcome.outcome)}</span>
      <small>Reward {formatReward(outcome.reward)}</small>
      {versionDiffers ? <small>Applied at v{outcome.applied_policy_version}</small> : null}
    </span>
  );
}

function truncateIdentifier(value: string): string {
  return value.length <= 8 ? value : `${value.slice(0, 8)}...`;
}

function formatTimestamp(value: string): string {
  const date = new Date(value);
  if (Number.isNaN(date.valueOf())) {
    return value;
  }
  return new Intl.DateTimeFormat("en-US", {
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
  }).format(date);
}

function formatPercentage(value: number): string {
  return new Intl.NumberFormat("en-US", { style: "percent", maximumFractionDigits: 1 }).format(value);
}

function formatReward(value: number): string {
  return new Intl.NumberFormat("en-US", { minimumFractionDigits: 2, maximumFractionDigits: 2 }).format(value);
}

function humanize(value: string): string {
  const normalized = value.replaceAll("_", " ");
  return `${normalized[0]?.toUpperCase() ?? ""}${normalized.slice(1)}`;
}