export type DeviceClass = "mobile" | "desktop" | "tablet";
export type Daypart = "morning" | "afternoon" | "evening" | "night";
export type OfferCategory =
  | "travel"
  | "dining"
  | "wellness"
  | "home"
  | "technology"
  | "entertainment";
export type VisitorType = "new" | "returning";
export type PolicyKind = "random" | "segmented_epsilon_greedy";
export type ExperimentStatus = "draft" | "running" | "paused" | "completed";
export type OutcomeKind = "ignored" | "clicked" | "converted";
export type SimulationRunStatus =
  | "starting"
  | "running"
  | "stopping"
  | "completed"
  | "failed"
  | "cancelled";
export type BenchmarkKind = "random" | "oracle";
export type OPEReason =
  | "no_samples"
  | "zero_candidate_weight"
  | "low_effective_sample_size";

export interface Health {
  status: "live" | "ready";
}

export interface SessionContext {
  device_class: DeviceClass;
  daypart: Daypart;
  category_affinity: OfferCategory;
  visitor_type: VisitorType;
}

export interface Offer {
  id: string;
  slug: string;
  merchant_name: string;
  title: string;
  description: string;
  category: OfferCategory;
  active: boolean;
}

export interface OfferSummary {
  id: string;
  slug: string;
  merchant_name: string;
  title: string;
  category: OfferCategory;
}

export interface ActionProbability {
  offer_id: string;
  probability: number;
}

export interface Experiment {
  id: string;
  slug: string;
  name: string;
  status: ExperimentStatus;
  policy_kind: PolicyKind;
  epsilon: number | null;
  policy_version: number;
  created_at: string;
  updated_at: string;
}

export interface ExperimentDetail extends Experiment {
  offers: Offer[];
}

export interface ExperimentPage {
  items: Experiment[];
  next_cursor: string | null;
}

export interface DecisionOutcome {
  event_id: string;
  outcome: OutcomeKind;
  reward: number;
  occurred_at: string;
  received_at: string;
  applied_policy_version: number;
}

export interface Decision {
  decision_id: string;
  experiment_id: string;
  context: SessionContext;
  selected_offer: OfferSummary;
  eligible_offer_ids: string[];
  propensity: number;
  distribution: ActionProbability[];
  policy_kind: PolicyKind;
  policy_version: number;
  policy_latency_micros: number;
  outcome: DecisionOutcome | null;
  created_at: string;
}

export interface DecisionPage {
  items: Decision[];
  next_cursor: string | null;
}

export interface Outcome {
  event_id: string;
  decision_id: string;
  outcome: OutcomeKind;
  reward: number;
  occurred_at: string;
  received_at: string;
  applied_policy_version: number;
}

export interface OfferPerformance {
  offer: Offer;
  selection_count: number;
  outcome_count: number;
  ignored_count: number;
  clicked_count: number;
  converted_count: number;
  reward_sum: number;
  empirical_mean: number | null;
  current_policy_mean: number | null;
  current_probability: number | null;
}

export interface LearningSeriesPoint {
  timestamp: string;
  sample_count: number;
  cumulative_average_reward: number;
}

export interface SimulationBenchmark {
  kind: BenchmarkKind;
  expected_average_reward: number | null;
  sample_count: number;
  reason: string | null;
  simulation_only: true;
}

export interface OPEEstimate {
  ips: number | null;
  snips: number | null;
  sample_count: number;
  effective_sample_size: number;
  weight_sum: number;
  min_weight: number;
  max_weight: number;
  reason: OPEReason | null;
}

export interface ExperimentSummary {
  experiment_id: string;
  policy_kind: PolicyKind;
  policy_version: number;
  exploration_rate: number | null;
  decision_count: number;
  outcome_count: number;
  reward_sum: number;
  average_reward: number | null;
  ignored_count: number;
  clicked_count: number;
  converted_count: number;
  p50_policy_latency_micros: number | null;
  p95_policy_latency_micros: number | null;
  offer_performance: OfferPerformance[];
  learning_series: LearningSeriesPoint[];
  random_benchmark: SimulationBenchmark;
  oracle_benchmark: SimulationBenchmark;
  ope: OPEEstimate;
  reasons: Record<string, string>;
  generated_at: string;
}

export interface SimulationRun {
  run_id: string;
  experiment_id: string;
  seed: number;
  requests_per_second: number;
  max_decisions: number;
  status: SimulationRunStatus;
  decision_count: number;
  outcome_count: number;
  error_count: number;
  observed_reward_sum: number;
  random_expected_reward_sum: number;
  oracle_expected_reward_sum: number;
  started_at: string;
  stopped_at: string | null;
  updated_at: string;
  error_code: string | null;
  error_detail: string | null;
}

export interface ApiProblem {
  type: string;
  title: string;
  status: number;
  code: string;
  detail: string;
  request_id: string;
}

export type CreateDemoExperimentRequest =
  | {
      name: string;
      policy_kind: "random";
    }
  | {
      name: string;
      policy_kind: "segmented_epsilon_greedy";
      epsilon: number;
    };

export interface CreateSimulationRunRequest {
  seed: number;
  requests_per_second: number;
  max_decisions: number;
}
