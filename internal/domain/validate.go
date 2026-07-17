package domain

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	ProbabilityTolerance           = 1e-9
	PolicySnapshotSchemaVersion    = 1
	MaxSimulationRequestsPerSecond = 100
	MaxSimulationDecisions         = 100_000
)

func ValidateExperiment(experiment Experiment) error {
	if err := ValidateUUID("experiment.id", experiment.ID); err != nil {
		return err
	}
	if strings.TrimSpace(experiment.Slug) == "" {
		return fmt.Errorf("experiment.slug must not be empty")
	}
	if strings.TrimSpace(experiment.Name) == "" {
		return fmt.Errorf("experiment.name must not be empty")
	}
	if !validExperimentStatus(experiment.Status) {
		return fmt.Errorf("experiment.status is invalid")
	}
	if !validPolicyKind(experiment.PolicyKind) {
		return fmt.Errorf("experiment.policy_kind is invalid")
	}
	if err := ValidateEpsilon(experiment.PolicyKind, experiment.Epsilon); err != nil {
		return err
	}
	if experiment.PolicyVersion < 1 {
		return fmt.Errorf("experiment.policy_version must be at least one")
	}
	if err := ValidateTimestamp("experiment.created_at", experiment.CreatedAt); err != nil {
		return err
	}
	if err := ValidateTimestamp("experiment.updated_at", experiment.UpdatedAt); err != nil {
		return err
	}
	if experiment.UpdatedAt.Before(experiment.CreatedAt) {
		return fmt.Errorf("experiment.updated_at must not precede created_at")
	}

	return nil
}

func ValidateOffer(offer Offer) error {
	if err := ValidateUUID("offer.id", offer.ID); err != nil {
		return err
	}
	if err := ValidateUUID("offer.experiment_id", offer.ExperimentID); err != nil {
		return err
	}
	if strings.TrimSpace(offer.Slug) == "" {
		return fmt.Errorf("offer.slug must not be empty")
	}
	if strings.TrimSpace(offer.MerchantName) == "" {
		return fmt.Errorf("offer.merchant_name must not be empty")
	}
	if strings.TrimSpace(offer.Title) == "" {
		return fmt.Errorf("offer.title must not be empty")
	}
	if !validOfferCategory(offer.Category) {
		return fmt.Errorf("offer.category is invalid")
	}

	return nil
}

func ValidateOffers(experimentID uuid.UUID, offers []Offer) error {
	if err := ValidateUUID("experiment_id", experimentID); err != nil {
		return err
	}
	if len(offers) == 0 {
		return fmt.Errorf("offers must not be empty")
	}

	ids := make(map[uuid.UUID]struct{}, len(offers))
	slugs := make(map[string]struct{}, len(offers))
	activeCount := 0
	for index, offer := range offers {
		if err := ValidateOffer(offer); err != nil {
			return fmt.Errorf("offers[%d]: %w", index, err)
		}
		if offer.ExperimentID != experimentID {
			return fmt.Errorf("offers[%d].experiment_id does not match experiment", index)
		}
		if _, exists := ids[offer.ID]; exists {
			return fmt.Errorf("offers contain duplicate id")
		}
		ids[offer.ID] = struct{}{}

		slug := strings.ToLower(offer.Slug)
		if _, exists := slugs[slug]; exists {
			return fmt.Errorf("offers contain duplicate slug")
		}
		slugs[slug] = struct{}{}
		if offer.Active {
			activeCount++
		}
	}

	if activeCount < 2 {
		return fmt.Errorf("offers must contain at least two active entries")
	}

	return nil
}

func ValidateSessionContext(context SessionContext) error {
	if !validDeviceClass(context.DeviceClass) {
		return fmt.Errorf("context.device_class is invalid")
	}
	if !validDaypart(context.Daypart) {
		return fmt.Errorf("context.daypart is invalid")
	}
	if !validOfferCategory(context.CategoryAffinity) {
		return fmt.Errorf("context.category_affinity is invalid")
	}
	if !validVisitorType(context.VisitorType) {
		return fmt.Errorf("context.visitor_type is invalid")
	}

	return nil
}

func SegmentKey(context SessionContext) (string, error) {
	if err := ValidateSessionContext(context); err != nil {
		return "", err
	}

	return strings.Join([]string{
		string(context.DeviceClass),
		string(context.Daypart),
		string(context.CategoryAffinity),
		string(context.VisitorType),
	}, "|"), nil
}

func RewardForOutcome(kind OutcomeKind) (float64, error) {
	switch kind {
	case OutcomeKindIgnored:
		return 0, nil
	case OutcomeKindClicked:
		return 0.25, nil
	case OutcomeKindConverted:
		return 1, nil
	default:
		return 0, fmt.Errorf("outcome kind is invalid")
	}
}

func CanonicalOfferIDs(offerIDs []uuid.UUID) ([]uuid.UUID, error) {
	if len(offerIDs) == 0 {
		return nil, fmt.Errorf("eligible offer ids must not be empty")
	}

	canonical := append([]uuid.UUID(nil), offerIDs...)
	for index, offerID := range canonical {
		if offerID == uuid.Nil {
			return nil, fmt.Errorf("eligible offer ids[%d] must not be nil", index)
		}
	}
	sort.Slice(canonical, func(left, right int) bool {
		return bytes.Compare(canonical[left][:], canonical[right][:]) < 0
	})
	for index := 1; index < len(canonical); index++ {
		if canonical[index] == canonical[index-1] {
			return nil, fmt.Errorf("eligible offer ids contain duplicates")
		}
	}

	return canonical, nil
}

func ValidateDistribution(eligibleOfferIDs []uuid.UUID, distribution []ActionProbability, selectedOfferID uuid.UUID) (float64, error) {
	if selectedOfferID == uuid.Nil {
		return 0, fmt.Errorf("selected offer id must not be nil")
	}
	canonical, err := CanonicalOfferIDs(eligibleOfferIDs)
	if err != nil {
		return 0, err
	}
	if len(distribution) != len(canonical) {
		return 0, fmt.Errorf("distribution must contain every eligible offer exactly once")
	}

	selectedProbability := 0.0
	selectedFound := false
	sum := 0.0
	for index, entry := range distribution {
		if entry.OfferID != canonical[index] {
			return 0, fmt.Errorf("distribution must use canonical eligible offer order")
		}
		if !finite(entry.Probability) || entry.Probability < 0 || entry.Probability > 1 {
			return 0, fmt.Errorf("distribution[%d].probability must be finite and between zero and one", index)
		}
		sum += entry.Probability
		if entry.OfferID == selectedOfferID {
			selectedProbability = entry.Probability
			selectedFound = true
		}
	}
	if !selectedFound {
		return 0, fmt.Errorf("selected offer id is not eligible")
	}
	if !finite(sum) || math.Abs(sum-1) > ProbabilityTolerance {
		return 0, fmt.Errorf("distribution probabilities must sum to one within tolerance")
	}

	return selectedProbability, nil
}

func ValidatePropensity(propensity, selectedProbability float64) error {
	if !finite(propensity) || propensity <= 0 || propensity > 1 {
		return fmt.Errorf("decision.propensity must be finite, positive, and at most one")
	}
	if !finite(selectedProbability) || propensity != selectedProbability {
		return fmt.Errorf("decision.propensity must equal the selected offer probability")
	}

	return nil
}

func ValidateDecision(decision Decision) error {
	if err := ValidateUUID("decision.id", decision.ID); err != nil {
		return err
	}
	if err := ValidateUUID("decision.experiment_id", decision.ExperimentID); err != nil {
		return err
	}
	if err := ValidateUUID("decision.selected_offer_id", decision.SelectedOfferID); err != nil {
		return err
	}
	if err := ValidateSessionContext(decision.Context); err != nil {
		return err
	}
	segmentKey, err := SegmentKey(decision.Context)
	if err != nil {
		return err
	}
	if decision.SegmentKey != segmentKey {
		return fmt.Errorf("decision.segment_key does not match context")
	}

	canonical, err := CanonicalOfferIDs(decision.EligibleOfferIDs)
	if err != nil {
		return err
	}
	if !equalUUIDs(decision.EligibleOfferIDs, canonical) {
		return fmt.Errorf("decision.eligible_offer_ids must use canonical order")
	}
	selectedProbability, err := ValidateDistribution(decision.EligibleOfferIDs, decision.Distribution, decision.SelectedOfferID)
	if err != nil {
		return err
	}
	if err := ValidatePropensity(decision.Propensity, selectedProbability); err != nil {
		return err
	}
	if !validPolicyKind(decision.PolicyKind) {
		return fmt.Errorf("decision.policy_kind is invalid")
	}
	if decision.PolicyVersion < 1 {
		return fmt.Errorf("decision.policy_version must be at least one")
	}
	if decision.PolicyLatencyMicros < 0 {
		return fmt.Errorf("decision.policy_latency_micros must not be negative")
	}
	if decision.SimulationRunID != nil && *decision.SimulationRunID == uuid.Nil {
		return fmt.Errorf("decision.simulation_run_id must not be nil UUID")
	}
	if strings.TrimSpace(decision.RequestID) == "" {
		return fmt.Errorf("decision.request_id must not be empty")
	}
	if err := ValidateTimestamp("decision.created_at", decision.CreatedAt); err != nil {
		return err
	}

	return nil
}

func ValidateOutcome(outcome Outcome, now time.Time, maxFutureSkew time.Duration) error {
	if err := ValidateUUID("outcome.event_id", outcome.EventID); err != nil {
		return err
	}
	if err := ValidateUUID("outcome.decision_id", outcome.DecisionID); err != nil {
		return err
	}
	reward, err := RewardForOutcome(outcome.Kind)
	if err != nil {
		return err
	}
	if !finite(outcome.Reward) || outcome.Reward != reward {
		return fmt.Errorf("outcome.reward does not match outcome kind")
	}
	if err := ValidateTimestamp("outcome.occurred_at", outcome.OccurredAt); err != nil {
		return err
	}
	if err := ValidateTimestamp("outcome.received_at", outcome.ReceivedAt); err != nil {
		return err
	}
	if err := ValidateTimestamp("now", now); err != nil {
		return err
	}
	if maxFutureSkew < 0 {
		return fmt.Errorf("max future skew must not be negative")
	}
	if outcome.OccurredAt.After(now.Add(maxFutureSkew)) {
		return fmt.Errorf("outcome.occurred_at exceeds allowed future skew")
	}
	if outcome.ReceivedAt.After(now.Add(maxFutureSkew)) {
		return fmt.Errorf("outcome.received_at exceeds allowed future skew")
	}
	if outcome.AppliedPolicyVersion < 1 {
		return fmt.Errorf("outcome.applied_policy_version must be at least one")
	}

	return nil
}

func ValidatePolicySnapshot(snapshot PolicySnapshot) error {
	if err := ValidateUUID("policy_snapshot.experiment_id", snapshot.ExperimentID); err != nil {
		return err
	}
	if !validPolicyKind(snapshot.PolicyKind) {
		return fmt.Errorf("policy_snapshot.policy_kind is invalid")
	}
	if snapshot.PolicyVersion < 1 {
		return fmt.Errorf("policy_snapshot.policy_version must be at least one")
	}
	if snapshot.SchemaVersion != PolicySnapshotSchemaVersion {
		return fmt.Errorf("policy_snapshot.schema_version is unsupported")
	}
	if len(snapshot.State) == 0 || !json.Valid(snapshot.State) {
		return fmt.Errorf("policy_snapshot.state must be valid JSON")
	}
	if err := ValidateTimestamp("policy_snapshot.created_at", snapshot.CreatedAt); err != nil {
		return err
	}

	return nil
}

func ValidateSimulationRun(run SimulationRun) error {
	if err := ValidateUUID("simulation_run.id", run.ID); err != nil {
		return err
	}
	if err := ValidateUUID("simulation_run.experiment_id", run.ExperimentID); err != nil {
		return err
	}
	if run.RequestsPerSecond < 1 || run.RequestsPerSecond > MaxSimulationRequestsPerSecond {
		return fmt.Errorf("simulation_run.requests_per_second is outside allowed bounds")
	}
	if run.MaxDecisions < 1 || run.MaxDecisions > MaxSimulationDecisions {
		return fmt.Errorf("simulation_run.max_decisions is outside allowed bounds")
	}
	if !validSimulationRunStatus(run.Status) {
		return fmt.Errorf("simulation_run.status is invalid")
	}
	if run.DecisionCount < 0 || run.OutcomeCount < 0 || run.ErrorCount < 0 {
		return fmt.Errorf("simulation_run counters must not be negative")
	}
	if run.OutcomeCount > run.DecisionCount {
		return fmt.Errorf("simulation_run.outcome_count must not exceed decision_count")
	}
	if err := ValidateRewardSum("simulation_run.observed_reward_sum", run.ObservedRewardSum); err != nil {
		return err
	}
	if err := ValidateRewardSum("simulation_run.random_expected_reward_sum", run.RandomExpectedRewardSum); err != nil {
		return err
	}
	if err := ValidateRewardSum("simulation_run.oracle_expected_reward_sum", run.OracleExpectedRewardSum); err != nil {
		return err
	}
	if err := ValidateTimestamp("simulation_run.started_at", run.StartedAt); err != nil {
		return err
	}
	if err := ValidateTimestamp("simulation_run.updated_at", run.UpdatedAt); err != nil {
		return err
	}
	if run.UpdatedAt.Before(run.StartedAt) {
		return fmt.Errorf("simulation_run.updated_at must not precede started_at")
	}
	if run.StoppedAt != nil {
		if err := ValidateTimestamp("simulation_run.stopped_at", *run.StoppedAt); err != nil {
			return err
		}
		if run.StoppedAt.Before(run.StartedAt) {
			return fmt.Errorf("simulation_run.stopped_at must not precede started_at")
		}
	}

	return nil
}

func ValidateLearningSeriesPoint(point LearningSeriesPoint) error {
	if err := ValidateTimestamp("learning_series.timestamp", point.Timestamp); err != nil {
		return err
	}
	if point.SampleCount < 1 {
		return fmt.Errorf("learning_series.sample_count must be positive")
	}
	if !finite(point.CumulativeAverageReward) || point.CumulativeAverageReward < 0 || point.CumulativeAverageReward > 1 {
		return fmt.Errorf("learning_series.cumulative_average_reward must be finite and between zero and one")
	}

	return nil
}

func ValidateBenchmarkReference(reference BenchmarkReference) error {
	if reference.Kind != BenchmarkKindRandom && reference.Kind != BenchmarkKindOracle {
		return fmt.Errorf("benchmark.kind is invalid")
	}
	if !reference.SimulationOnly {
		return fmt.Errorf("benchmark must be marked simulation-only")
	}
	if reference.SampleCount < 0 {
		return fmt.Errorf("benchmark.sample_count must not be negative")
	}
	if reference.ExpectedAverageReward == nil {
		if strings.TrimSpace(reference.Reason) == "" {
			return fmt.Errorf("benchmark.reason is required when value is unavailable")
		}
		return nil
	}
	if reference.SampleCount < 1 {
		return fmt.Errorf("benchmark.sample_count must be positive when value is available")
	}
	if strings.TrimSpace(reference.Reason) != "" {
		return fmt.Errorf("benchmark.reason must be empty when value is available")
	}
	if !finite(*reference.ExpectedAverageReward) || *reference.ExpectedAverageReward < 0 || *reference.ExpectedAverageReward > 1 {
		return fmt.Errorf("benchmark.expected_average_reward must be finite and between zero and one")
	}

	return nil
}

func ValidateEpsilon(kind PolicyKind, epsilon *float64) error {
	switch kind {
	case PolicyKindRandom:
		if epsilon != nil {
			return fmt.Errorf("experiment.epsilon must be absent for random policy")
		}
	case PolicyKindSegmentedEpsilonGreedy:
		if epsilon == nil || !finite(*epsilon) || *epsilon < 0 || *epsilon > 1 {
			return fmt.Errorf("experiment.epsilon must be finite and between zero and one")
		}
	default:
		return fmt.Errorf("experiment.policy_kind is invalid")
	}

	return nil
}

func ValidateRewardSum(field string, value float64) error {
	if !finite(value) || value < 0 {
		return fmt.Errorf("%s must be finite and non-negative", field)
	}

	return nil
}

func ValidateUUID(field string, value uuid.UUID) error {
	if value == uuid.Nil {
		return fmt.Errorf("%s must not be nil UUID", field)
	}

	return nil
}

func ValidateTimestamp(field string, value time.Time) error {
	if value.IsZero() {
		return fmt.Errorf("%s must not be zero", field)
	}
	_, offset := value.Zone()
	if offset != 0 {
		return fmt.Errorf("%s must be UTC", field)
	}

	return nil
}

func finite(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}

func equalUUIDs(left, right []uuid.UUID) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func validExperimentStatus(status ExperimentStatus) bool {
	switch status {
	case ExperimentStatusDraft, ExperimentStatusRunning, ExperimentStatusPaused, ExperimentStatusCompleted:
		return true
	default:
		return false
	}
}

func validPolicyKind(kind PolicyKind) bool {
	return kind == PolicyKindRandom || kind == PolicyKindSegmentedEpsilonGreedy
}

func validDeviceClass(deviceClass DeviceClass) bool {
	switch deviceClass {
	case DeviceClassMobile, DeviceClassDesktop, DeviceClassTablet:
		return true
	default:
		return false
	}
}

func validDaypart(daypart Daypart) bool {
	switch daypart {
	case DaypartMorning, DaypartAfternoon, DaypartEvening, DaypartNight:
		return true
	default:
		return false
	}
}

func validOfferCategory(category OfferCategory) bool {
	switch category {
	case OfferCategoryTravel, OfferCategoryDining, OfferCategoryWellness, OfferCategoryHome, OfferCategoryTechnology, OfferCategoryEntertainment:
		return true
	default:
		return false
	}
}

func validVisitorType(visitorType VisitorType) bool {
	return visitorType == VisitorTypeNew || visitorType == VisitorTypeReturning
}

func validSimulationRunStatus(status SimulationRunStatus) bool {
	switch status {
	case SimulationRunStatusStarting, SimulationRunStatusRunning, SimulationRunStatusStopping, SimulationRunStatusCompleted, SimulationRunStatusFailed, SimulationRunStatusCancelled:
		return true
	default:
		return false
	}
}
