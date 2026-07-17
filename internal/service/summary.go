package service

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/onatozmenn/offerpilot/internal/bandit"
	"github.com/onatozmenn/offerpilot/internal/domain"
	"github.com/onatozmenn/offerpilot/internal/evaluation"
)

const DefaultMaxLearningSeriesPoints = 120

type OfferPerformance struct {
	Offer              domain.Offer
	SelectionCount     int64
	OutcomeCount       int64
	IgnoredCount       int64
	ClickedCount       int64
	ConvertedCount     int64
	RewardSum          float64
	EmpiricalMean      *float64
	CurrentPolicyMean  *float64
	CurrentProbability *float64
}

type OPEEstimate struct {
	IPS                 *float64
	SNIPS               *float64
	SampleCount         int
	EffectiveSampleSize float64
	WeightSum           float64
	MinWeight           float64
	MaxWeight           float64
	Reason              string
}

type Summary struct {
	ExperimentID           uuid.UUID
	PolicyKind             domain.PolicyKind
	PolicyVersion          int64
	ExplorationRate        *float64
	DecisionCount          int64
	OutcomeCount           int64
	RewardSum              float64
	AverageReward          *float64
	IgnoredCount           int64
	ClickedCount           int64
	ConvertedCount         int64
	P50PolicyLatencyMicros *int64
	P95PolicyLatencyMicros *int64
	OfferPerformance       []OfferPerformance
	LearningSeries         []domain.LearningSeriesPoint
	RandomBenchmark        domain.BenchmarkReference
	OracleBenchmark        domain.BenchmarkReference
	OPE                    OPEEstimate
	Reasons                map[string]string
	GeneratedAt            time.Time
}

func (engine *Engine) Summary(
	ctx context.Context,
	experimentID uuid.UUID,
	maxLearningSeriesPoints int,
) (Summary, error) {
	if maxLearningSeriesPoints == 0 {
		maxLearningSeriesPoints = DefaultMaxLearningSeriesPoints
	}
	if maxLearningSeriesPoints < 1 || maxLearningSeriesPoints > DefaultMaxLearningSeriesPoints {
		return Summary{}, fmt.Errorf("max learning-series points must be between 1 and %d", DefaultMaxLearningSeriesPoints)
	}
	experiment, err := engine.store.GetExperiment(ctx, experimentID)
	if err != nil {
		return Summary{}, err
	}
	policyView, err := engine.PolicyView(experimentID)
	if err != nil {
		return Summary{}, err
	}
	aggregate, err := engine.store.GetSummaryAggregate(ctx, experimentID)
	if err != nil {
		return Summary{}, fmt.Errorf("load summary aggregate: %w", err)
	}
	series, err := engine.store.GetLearningSeries(ctx, experimentID, maxLearningSeriesPoints)
	if err != nil {
		return Summary{}, fmt.Errorf("load learning series: %w", err)
	}
	performanceRecords, err := engine.store.GetOfferPerformance(ctx, experimentID)
	if err != nil {
		return Summary{}, fmt.Errorf("load offer performance: %w", err)
	}
	activeOffers, err := engine.store.ListActiveOffers(ctx, experimentID)
	if err != nil {
		return Summary{}, fmt.Errorf("load active offers for policy projection: %w", err)
	}
	currentMeans, currentProbabilities, err := currentOfferProjection(policyView, activeOffers)
	if err != nil {
		return Summary{}, fmt.Errorf("project current policy: %w", err)
	}

	reasons := make(map[string]string)
	var averageReward *float64
	if aggregate.OutcomeCount == 0 {
		reasons["average_reward"] = "no_outcomes"
	} else {
		value := aggregate.RewardSum / float64(aggregate.OutcomeCount)
		averageReward = &value
	}
	offerPerformance := make([]OfferPerformance, len(performanceRecords))
	for index, record := range performanceRecords {
		entry := OfferPerformance{
			Offer:              record.Offer,
			SelectionCount:     record.SelectionCount,
			OutcomeCount:       record.OutcomeCount,
			IgnoredCount:       record.IgnoredCount,
			ClickedCount:       record.ClickedCount,
			ConvertedCount:     record.ConvertedCount,
			RewardSum:          record.RewardSum,
			CurrentPolicyMean:  cloneFloatPointer(currentMeans[record.Offer.ID]),
			CurrentProbability: cloneFloatPointer(currentProbabilities[record.Offer.ID]),
		}
		if record.OutcomeCount == 0 {
			reasons["offer:"+record.Offer.ID.String()+":empirical_mean"] = "no_outcomes"
		} else {
			value := record.RewardSum / float64(record.OutcomeCount)
			entry.EmpiricalMean = &value
		}
		offerPerformance[index] = entry
	}

	randomBenchmark, oracleBenchmark, err := engine.benchmarks(ctx, experimentID)
	if err != nil {
		return Summary{}, err
	}
	if randomBenchmark.ExpectedAverageReward == nil {
		reasons["random_benchmark"] = randomBenchmark.Reason
	}
	if oracleBenchmark.ExpectedAverageReward == nil {
		reasons["oracle_benchmark"] = oracleBenchmark.Reason
	}

	opeEstimate, err := engine.ope(ctx, experimentID, policyView)
	if err != nil {
		return Summary{}, err
	}
	if opeEstimate.Reason != "" {
		reasons["ope"] = opeEstimate.Reason
	}

	return Summary{
		ExperimentID:           experiment.ID,
		PolicyKind:             policyView.Kind,
		PolicyVersion:          policyView.Version,
		ExplorationRate:        cloneFloatPointer(policyView.Epsilon),
		DecisionCount:          aggregate.DecisionCount,
		OutcomeCount:           aggregate.OutcomeCount,
		RewardSum:              aggregate.RewardSum,
		AverageReward:          averageReward,
		IgnoredCount:           aggregate.IgnoredCount,
		ClickedCount:           aggregate.ClickedCount,
		ConvertedCount:         aggregate.ConvertedCount,
		P50PolicyLatencyMicros: cloneInt64Pointer(aggregate.P50PolicyLatencyMicros),
		P95PolicyLatencyMicros: cloneInt64Pointer(aggregate.P95PolicyLatencyMicros),
		OfferPerformance:       offerPerformance,
		LearningSeries:         append([]domain.LearningSeriesPoint(nil), series...),
		RandomBenchmark:        randomBenchmark,
		OracleBenchmark:        oracleBenchmark,
		OPE:                    opeEstimate,
		Reasons:                reasons,
		GeneratedAt:            engine.clock.Now().UTC(),
	}, nil
}

func (engine *Engine) benchmarks(
	ctx context.Context,
	experimentID uuid.UUID,
) (domain.BenchmarkReference, domain.BenchmarkReference, error) {
	randomReference := unavailableBenchmark(domain.BenchmarkKindRandom, "not_simulated")
	oracleReference := unavailableBenchmark(domain.BenchmarkKindOracle, "not_simulated")
	record, found, err := engine.store.GetLatestSimulationBenchmark(ctx, experimentID)
	if err != nil {
		return domain.BenchmarkReference{}, domain.BenchmarkReference{}, fmt.Errorf("load simulation benchmark: %w", err)
	}
	if !found {
		return randomReference, oracleReference, nil
	}
	if record.DecisionCount < 1 {
		return domain.BenchmarkReference{}, domain.BenchmarkReference{}, fmt.Errorf("simulation benchmark has no contributing decisions")
	}
	randomValue := record.RandomExpectedRewardSum / float64(record.DecisionCount)
	oracleValue := record.OracleExpectedRewardSum / float64(record.DecisionCount)
	randomReference = domain.BenchmarkReference{
		Kind:                  domain.BenchmarkKindRandom,
		ExpectedAverageReward: &randomValue,
		SampleCount:           record.DecisionCount,
		SimulationOnly:        true,
	}
	oracleReference = domain.BenchmarkReference{
		Kind:                  domain.BenchmarkKindOracle,
		ExpectedAverageReward: &oracleValue,
		SampleCount:           record.DecisionCount,
		SimulationOnly:        true,
	}
	if err := domain.ValidateBenchmarkReference(randomReference); err != nil {
		return domain.BenchmarkReference{}, domain.BenchmarkReference{}, fmt.Errorf("validate random benchmark: %w", err)
	}
	if err := domain.ValidateBenchmarkReference(oracleReference); err != nil {
		return domain.BenchmarkReference{}, domain.BenchmarkReference{}, fmt.Errorf("validate oracle benchmark: %w", err)
	}
	return randomReference, oracleReference, nil
}

func (engine *Engine) ope(
	ctx context.Context,
	experimentID uuid.UUID,
	policyView bandit.PolicyView,
) (OPEEstimate, error) {
	records, err := engine.store.ListDecisionOutcomes(ctx, experimentID)
	if err != nil {
		return OPEEstimate{}, fmt.Errorf("load OPE records: %w", err)
	}
	evaluationRecords := make([]evaluation.Record, len(records))
	for index, record := range records {
		candidateProbability, err := candidateProbability(policyView, record.Decision)
		if err != nil {
			return OPEEstimate{}, fmt.Errorf("candidate probability for decision %s: %w", record.Decision.ID, err)
		}
		evaluationRecords[index] = evaluation.Record{
			Reward:               record.Outcome.Reward,
			BehaviorPropensity:   record.Decision.Propensity,
			CandidateProbability: candidateProbability,
		}
	}
	result, err := evaluation.Evaluate(evaluationRecords)
	if err != nil {
		return OPEEstimate{}, fmt.Errorf("evaluate OPE records: %w", err)
	}
	return OPEEstimate{
		IPS:                 cloneFloatPointer(result.IPS),
		SNIPS:               cloneFloatPointer(result.SNIPS),
		SampleCount:         result.SampleCount,
		EffectiveSampleSize: result.EffectiveSampleSize,
		WeightSum:           result.WeightSum,
		MinWeight:           result.MinWeight,
		MaxWeight:           result.MaxWeight,
		Reason:              result.Reason,
	}, nil
}

func currentOfferProjection(
	view bandit.PolicyView,
	activeOffers []domain.Offer,
) (map[uuid.UUID]*float64, map[uuid.UUID]*float64, error) {
	offerIDs := make([]uuid.UUID, len(activeOffers))
	for index, offer := range activeOffers {
		offerIDs[index] = offer.ID
	}
	canonical, err := domain.CanonicalOfferIDs(offerIDs)
	if err != nil {
		return nil, nil, err
	}
	means := make(map[uuid.UUID]*float64, len(canonical))
	probabilities := make(map[uuid.UUID]*float64, len(canonical))
	if view.Kind == domain.PolicyKindRandom {
		probability := 1 / float64(len(canonical))
		for _, offerID := range canonical {
			value := probability
			probabilities[offerID] = &value
		}
		return means, probabilities, nil
	}
	if view.Kind != domain.PolicyKindSegmentedEpsilonGreedy || view.Epsilon == nil {
		return nil, nil, fmt.Errorf("unsupported policy view")
	}

	segmentArms := make(map[string]map[uuid.UUID]bandit.ArmView)
	weightedRewardSums := make(map[uuid.UUID]float64)
	weightedCounts := make(map[uuid.UUID]float64)
	for _, arm := range view.Arms {
		if _, exists := segmentArms[arm.SegmentKey]; !exists {
			segmentArms[arm.SegmentKey] = make(map[uuid.UUID]bandit.ArmView)
		}
		segmentArms[arm.SegmentKey][arm.OfferID] = arm
		weightedRewardSums[arm.OfferID] += arm.RewardSum
		weightedCounts[arm.OfferID] += arm.Count
	}
	for _, offerID := range canonical {
		if weightedCounts[offerID] > 0 {
			value := weightedRewardSums[offerID] / weightedCounts[offerID]
			means[offerID] = &value
		} else {
			value := 0.5
			means[offerID] = &value
		}
	}

	if len(segmentArms) == 0 {
		probability := 1 / float64(len(canonical))
		for _, offerID := range canonical {
			value := probability
			probabilities[offerID] = &value
		}
		return means, probabilities, nil
	}
	probabilitySums := make(map[uuid.UUID]float64, len(canonical))
	for _, arms := range segmentArms {
		segmentProbabilities := epsilonDistribution(*view.Epsilon, canonical, arms)
		for offerID, probability := range segmentProbabilities {
			probabilitySums[offerID] += probability
		}
	}
	for _, offerID := range canonical {
		value := probabilitySums[offerID] / float64(len(segmentArms))
		probabilities[offerID] = &value
	}
	return means, probabilities, nil
}

func candidateProbability(view bandit.PolicyView, decision domain.Decision) (float64, error) {
	if view.Kind == domain.PolicyKindRandom {
		if len(decision.EligibleOfferIDs) < 1 {
			return 0, fmt.Errorf("decision has no eligible offers")
		}
		return 1 / float64(len(decision.EligibleOfferIDs)), nil
	}
	if view.Kind != domain.PolicyKindSegmentedEpsilonGreedy || view.Epsilon == nil {
		return 0, fmt.Errorf("unsupported policy view")
	}
	arms := make(map[uuid.UUID]bandit.ArmView)
	for _, arm := range view.Arms {
		if arm.SegmentKey == decision.SegmentKey {
			arms[arm.OfferID] = arm
		}
	}
	distribution := epsilonDistribution(*view.Epsilon, decision.EligibleOfferIDs, arms)
	probability, exists := distribution[decision.SelectedOfferID]
	if !exists || math.IsNaN(probability) || math.IsInf(probability, 0) || probability < 0 || probability > 1 {
		return 0, fmt.Errorf("selected offer candidate probability is invalid")
	}
	return probability, nil
}

func epsilonDistribution(
	epsilon float64,
	offerIDs []uuid.UUID,
	arms map[uuid.UUID]bandit.ArmView,
) map[uuid.UUID]float64 {
	means := make(map[uuid.UUID]float64, len(offerIDs))
	bestMean := math.Inf(-1)
	bestCount := 0
	for _, offerID := range offerIDs {
		mean := 0.5
		if arm, exists := arms[offerID]; exists {
			mean = arm.Mean
		}
		means[offerID] = mean
		if mean > bestMean {
			bestMean = mean
			bestCount = 1
		} else if mean == bestMean {
			bestCount++
		}
	}
	probabilities := make(map[uuid.UUID]float64, len(offerIDs))
	for _, offerID := range offerIDs {
		probability := epsilon / float64(len(offerIDs))
		if means[offerID] == bestMean {
			probability += (1 - epsilon) / float64(bestCount)
		}
		probabilities[offerID] = probability
	}
	return probabilities
}

func unavailableBenchmark(kind domain.BenchmarkKind, reason string) domain.BenchmarkReference {
	return domain.BenchmarkReference{
		Kind:           kind,
		Reason:         reason,
		SimulationOnly: true,
	}
}

func cloneFloatPointer(value *float64) *float64 {
	if value == nil {
		return nil
	}
	clone := *value
	return &clone
}

func cloneInt64Pointer(value *int64) *int64 {
	if value == nil {
		return nil
	}
	clone := *value
	return &clone
}
