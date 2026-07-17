import type { OfferPerformance } from "../types/api";

interface OfferPerformanceTableProps {
  rows: readonly OfferPerformance[];
  reasons: Readonly<Record<string, string>>;
  isLoading?: boolean;
  isError?: boolean;
  isStale?: boolean;
  error?: string | null;
}

export function OfferPerformanceTable({
  rows,
  reasons,
  isLoading = false,
  isError = false,
  isStale = false,
  error = null,
}: OfferPerformanceTableProps) {
  return (
    <section
      className="offer-performance"
      aria-labelledby="offer-performance-title"
      aria-busy={isLoading}
      data-stale={isStale || undefined}
    >
      <div className="section-heading">
        <div>
          <p className="section-heading__eyebrow">Offer comparison</p>
          <h2 id="offer-performance-title">Performance by offer</h2>
        </div>
        {isStale ? <span className="status-label status-label--stale">Stale data</span> : null}
      </div>

      {isError && error !== null ? <p className="inline-alert" role="alert">{error}</p> : null}

      <div
        className="data-table-region data-table-region--horizontal"
        role="region"
        aria-labelledby="offer-performance-title"
        tabIndex={0}
      >
        <table className="data-table offer-performance__table">
          <caption className="sr-only">
            Server-projected offer selections, terminal outcomes, empirical rewards, and current policy probabilities.
          </caption>
          <thead>
            <tr>
              <th scope="col">Merchant and offer</th>
              <th scope="col">Category</th>
              <th scope="col" className="numeric-cell">Selections</th>
              <th scope="col" className="numeric-cell">Outcomes</th>
              <th scope="col" className="numeric-cell">Ignored</th>
              <th scope="col" className="numeric-cell">Clicked</th>
              <th scope="col" className="numeric-cell">Converted</th>
              <th scope="col" className="numeric-cell">Empirical mean</th>
              <th scope="col">Current probability</th>
            </tr>
          </thead>
          <tbody>
            {rows.map((row) => (
              <tr key={row.offer.id}>
                <th scope="row" className="offer-performance__identity">
                  <span>{row.offer.merchant_name}</span>
                  <small>{row.offer.title}</small>
                </th>
                <td><span className="category-label">{humanize(row.offer.category)}</span></td>
                <td className="numeric-cell">{formatInteger(row.selection_count)}</td>
                <td className="numeric-cell">{formatInteger(row.outcome_count)}</td>
                <td className="numeric-cell">{formatInteger(row.ignored_count)}</td>
                <td className="numeric-cell">{formatInteger(row.clicked_count)}</td>
                <td className="numeric-cell">{formatInteger(row.converted_count)}</td>
                <td className="numeric-cell">
                  {row.empirical_mean === null ? (
                    <UnavailableValue reason={reasons[`offer:${row.offer.id}:empirical_mean`]} />
                  ) : (
                    formatPercentage(row.empirical_mean)
                  )}
                </td>
                <td>
                  {row.current_probability === null ? (
                    <UnavailableValue reason={reasons[`offer:${row.offer.id}:current_probability`]} />
                  ) : (
                    <ProbabilityValue value={row.current_probability} />
                  )}
                </td>
              </tr>
            ))}
            {rows.length === 0 ? (
              <tr>
                <td className="data-table__empty" colSpan={9}>
                  {isLoading
                    ? "Loading offer performance..."
                    : isError
                      ? "Offer performance is unavailable."
                      : "No offer performance has been recorded yet."}
                </td>
              </tr>
            ) : null}
          </tbody>
        </table>
      </div>
      <p className="data-table__scroll-hint">Scroll horizontally to inspect every metric.</p>
    </section>
  );
}

function ProbabilityValue({ value }: { value: number }) {
  const width = `${Math.max(0, Math.min(100, value * 100))}%`;
  return (
    <span className="probability-value">
      <span className="probability-value__text">{formatPercentage(value)}</span>
      <span className="probability-value__track" aria-hidden="true">
        <span className="probability-value__fill" style={{ width }} />
      </span>
    </span>
  );
}

function UnavailableValue({ reason }: { reason: string | undefined }) {
  const title = humanize(reason ?? "not available yet");
  return <span className="unavailable-value" aria-label={`Unavailable: ${title}`} title={title}>&mdash;</span>;
}

function formatInteger(value: number): string {
  return value.toLocaleString("en-US", { maximumFractionDigits: 0 });
}

function formatPercentage(value: number): string {
  return new Intl.NumberFormat("en-US", { style: "percent", maximumFractionDigits: 1 }).format(value);
}

function humanize(value: string): string {
  const normalized = value.replaceAll("_", " ");
  return `${normalized[0]?.toUpperCase() ?? ""}${normalized.slice(1)}`;
}
