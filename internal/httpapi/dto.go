package httpapi

import (
	"time"

	"github.com/onatozmenn/offerpilot/internal/domain"
	"github.com/onatozmenn/offerpilot/internal/service"
)

type createDemoExperimentRequest struct {
	Name       string            `json:"name"`
	PolicyKind domain.PolicyKind `json:"policy_kind"`
	Epsilon    *float64          `json:"epsilon,omitempty"`
}

type createDecisionRequest struct {
	ExperimentID string            `json:"experiment_id"`
	Context      sessionContextDTO `json:"context"`
}

type createOutcomeRequest struct {
	EventID    string             `json:"event_id"`
	DecisionID string             `json:"decision_id"`
	Outcome    domain.OutcomeKind `json:"outcome"`
	OccurredAt string             `json:"occurred_at"`
}

type createSimulationRunRequest struct {
	Seed              int64 `json:"seed"`
	RequestsPerSecond int   `json:"requests_per_second"`
	MaxDecisions      int   `json:"max_decisions"`
}

type sessionContextDTO struct {
	DeviceClass      domain.DeviceClass   `json:"device_class"`
	Daypart          domain.Daypart       `json:"daypart"`
	CategoryAffinity domain.OfferCategory `json:"category_affinity"`
	VisitorType      domain.VisitorType   `json:"visitor_type"`
}

func (dto sessionContextDTO) domain() domain.SessionContext {
	return domain.SessionContext{
		DeviceClass:      dto.DeviceClass,
		Daypart:          dto.Daypart,
		CategoryAffinity: dto.CategoryAffinity,
		VisitorType:      dto.VisitorType,
	}
}

type healthDTO struct {
	Status string `json:"status"`
}

type offerDTO struct {
	ID           string               `json:"id"`
	Slug         string               `json:"slug"`
	MerchantName string               `json:"merchant_name"`
	Title        string               `json:"title"`
	Description  string               `json:"description"`
	Category     domain.OfferCategory `json:"category"`
	Active       bool                 `json:"active"`
}

type selectedOfferDTO struct {
	ID           string               `json:"id"`
	Slug         string               `json:"slug"`
	MerchantName string               `json:"merchant_name"`
	Title        string               `json:"title"`
	Category     domain.OfferCategory `json:"category"`
}

type experimentDTO struct {
	ID            string                  `json:"id"`
	Slug          string                  `json:"slug"`
	Name          string                  `json:"name"`
	Status        domain.ExperimentStatus `json:"status"`
	PolicyKind    domain.PolicyKind       `json:"policy_kind"`
	Epsilon       *float64                `json:"epsilon"`
	PolicyVersion int64                   `json:"policy_version"`
	CreatedAt     string                  `json:"created_at"`
	UpdatedAt     string                  `json:"updated_at"`
}

type experimentDetailDTO struct {
	ID            string                  `json:"id"`
	Slug          string                  `json:"slug"`
	Name          string                  `json:"name"`
	Status        domain.ExperimentStatus `json:"status"`
	PolicyKind    domain.PolicyKind       `json:"policy_kind"`
	Epsilon       *float64                `json:"epsilon"`
	PolicyVersion int64                   `json:"policy_version"`
	CreatedAt     string                  `json:"created_at"`
	UpdatedAt     string                  `json:"updated_at"`
	Offers        []offerDTO              `json:"offers"`
}

type experimentPageDTO struct {
	Items      []experimentDTO `json:"items"`
	NextCursor *string         `json:"next_cursor"`
}

type actionProbabilityDTO struct {
	OfferID     string  `json:"offer_id"`
	Probability float64 `json:"probability"`
}

type decisionOutcomeDTO struct {
	EventID              string             `json:"event_id"`
	Outcome              domain.OutcomeKind `json:"outcome"`
	Reward               float64            `json:"reward"`
	OccurredAt           string             `json:"occurred_at"`
	ReceivedAt           string             `json:"received_at"`
	AppliedPolicyVersion int64              `json:"applied_policy_version"`
}

type decisionDTO struct {
	DecisionID          string                 `json:"decision_id"`
	ExperimentID        string                 `json:"experiment_id"`
	Context             sessionContextDTO      `json:"context"`
	SelectedOffer       selectedOfferDTO       `json:"selected_offer"`
	EligibleOfferIDs    []string               `json:"eligible_offer_ids"`
	Propensity          float64                `json:"propensity"`
	Distribution        []actionProbabilityDTO `json:"distribution"`
	PolicyKind          domain.PolicyKind      `json:"policy_kind"`
	PolicyVersion       int64                  `json:"policy_version"`
	PolicyLatencyMicros int64                  `json:"policy_latency_micros"`
	Outcome             *decisionOutcomeDTO    `json:"outcome"`
	CreatedAt           string                 `json:"created_at"`
}

type decisionPageDTO struct {
	Items      []decisionDTO `json:"items"`
	NextCursor *string       `json:"next_cursor"`
}

type outcomeDTO struct {
	EventID              string             `json:"event_id"`
	DecisionID           string             `json:"decision_id"`
	Outcome              domain.OutcomeKind `json:"outcome"`
	Reward               float64            `json:"reward"`
	OccurredAt           string             `json:"occurred_at"`
	ReceivedAt           string             `json:"received_at"`
	AppliedPolicyVersion int64              `json:"applied_policy_version"`
}

type offerPerformanceDTO struct {
	Offer              offerDTO `json:"offer"`
	SelectionCount     int64    `json:"selection_count"`
	OutcomeCount       int64    `json:"outcome_count"`
	IgnoredCount       int64    `json:"ignored_count"`
	ClickedCount       int64    `json:"clicked_count"`
	ConvertedCount     int64    `json:"converted_count"`
	RewardSum          float64  `json:"reward_sum"`
	EmpiricalMean      *float64 `json:"empirical_mean"`
	CurrentPolicyMean  *float64 `json:"current_policy_mean"`
	CurrentProbability *float64 `json:"current_probability"`
}

type learningSeriesPointDTO struct {
	Timestamp               string  `json:"timestamp"`
	SampleCount             int64   `json:"sample_count"`
	CumulativeAverageReward float64 `json:"cumulative_average_reward"`
}

type simulationBenchmarkDTO struct {
	Kind                  domain.BenchmarkKind `json:"kind"`
	ExpectedAverageReward *float64             `json:"expected_average_reward"`
	SampleCount           int64                `json:"sample_count"`
	Reason                *string              `json:"reason"`
	SimulationOnly        bool                 `json:"simulation_only"`
}

type opeEstimateDTO struct {
	IPS                 *float64 `json:"ips"`
	SNIPS               *float64 `json:"snips"`
	SampleCount         int      `json:"sample_count"`
	EffectiveSampleSize float64  `json:"effective_sample_size"`
	WeightSum           float64  `json:"weight_sum"`
	MinWeight           float64  `json:"min_weight"`
	MaxWeight           float64  `json:"max_weight"`
	Reason              *string  `json:"reason"`
}

type experimentSummaryDTO struct {
	ExperimentID           string                   `json:"experiment_id"`
	PolicyKind             domain.PolicyKind        `json:"policy_kind"`
	PolicyVersion          int64                    `json:"policy_version"`
	ExplorationRate        *float64                 `json:"exploration_rate"`
	DecisionCount          int64                    `json:"decision_count"`
	OutcomeCount           int64                    `json:"outcome_count"`
	RewardSum              float64                  `json:"reward_sum"`
	AverageReward          *float64                 `json:"average_reward"`
	IgnoredCount           int64                    `json:"ignored_count"`
	ClickedCount           int64                    `json:"clicked_count"`
	ConvertedCount         int64                    `json:"converted_count"`
	P50PolicyLatencyMicros *int64                   `json:"p50_policy_latency_micros"`
	P95PolicyLatencyMicros *int64                   `json:"p95_policy_latency_micros"`
	OfferPerformance       []offerPerformanceDTO    `json:"offer_performance"`
	LearningSeries         []learningSeriesPointDTO `json:"learning_series"`
	RandomBenchmark        simulationBenchmarkDTO   `json:"random_benchmark"`
	OracleBenchmark        simulationBenchmarkDTO   `json:"oracle_benchmark"`
	OPE                    opeEstimateDTO           `json:"ope"`
	Reasons                map[string]string        `json:"reasons"`
	GeneratedAt            string                   `json:"generated_at"`
}

type simulationRunDTO struct {
	RunID                   string                     `json:"run_id"`
	ExperimentID            string                     `json:"experiment_id"`
	Seed                    int64                      `json:"seed"`
	RequestsPerSecond       int                        `json:"requests_per_second"`
	MaxDecisions            int                        `json:"max_decisions"`
	Status                  domain.SimulationRunStatus `json:"status"`
	DecisionCount           int64                      `json:"decision_count"`
	OutcomeCount            int64                      `json:"outcome_count"`
	ErrorCount              int64                      `json:"error_count"`
	ObservedRewardSum       float64                    `json:"observed_reward_sum"`
	RandomExpectedRewardSum float64                    `json:"random_expected_reward_sum"`
	OracleExpectedRewardSum float64                    `json:"oracle_expected_reward_sum"`
	StartedAt               string                     `json:"started_at"`
	StoppedAt               *string                    `json:"stopped_at"`
	UpdatedAt               string                     `json:"updated_at"`
	ErrorCode               *string                    `json:"error_code"`
	ErrorDetail             *string                    `json:"error_detail"`
}

func newExperimentDTO(experiment domain.Experiment) experimentDTO {
	return experimentDTO{
		ID:            experiment.ID.String(),
		Slug:          experiment.Slug,
		Name:          experiment.Name,
		Status:        experiment.Status,
		PolicyKind:    experiment.PolicyKind,
		Epsilon:       cloneFloat(experiment.Epsilon),
		PolicyVersion: experiment.PolicyVersion,
		CreatedAt:     formatTimestamp(experiment.CreatedAt),
		UpdatedAt:     formatTimestamp(experiment.UpdatedAt),
	}
}

func newExperimentDetailDTO(experiment domain.Experiment, offers []domain.Offer) experimentDetailDTO {
	dto := experimentDetailDTO{
		ID:            experiment.ID.String(),
		Slug:          experiment.Slug,
		Name:          experiment.Name,
		Status:        experiment.Status,
		PolicyKind:    experiment.PolicyKind,
		Epsilon:       cloneFloat(experiment.Epsilon),
		PolicyVersion: experiment.PolicyVersion,
		CreatedAt:     formatTimestamp(experiment.CreatedAt),
		UpdatedAt:     formatTimestamp(experiment.UpdatedAt),
		Offers:        make([]offerDTO, len(offers)),
	}
	for index, offer := range offers {
		dto.Offers[index] = newOfferDTO(offer)
	}
	return dto
}

func newOfferDTO(offer domain.Offer) offerDTO {
	return offerDTO{
		ID:           offer.ID.String(),
		Slug:         offer.Slug,
		MerchantName: offer.MerchantName,
		Title:        offer.Title,
		Description:  offer.Description,
		Category:     offer.Category,
		Active:       offer.Active,
	}
}

func newSelectedOfferDTO(offer domain.Offer) selectedOfferDTO {
	return selectedOfferDTO{
		ID:           offer.ID.String(),
		Slug:         offer.Slug,
		MerchantName: offer.MerchantName,
		Title:        offer.Title,
		Category:     offer.Category,
	}
}

func newDecisionDTO(decision domain.Decision, selectedOffer domain.Offer, outcome *domain.Outcome) decisionDTO {
	dto := decisionDTO{
		DecisionID:          decision.ID.String(),
		ExperimentID:        decision.ExperimentID.String(),
		Context:             newSessionContextDTO(decision.Context),
		SelectedOffer:       newSelectedOfferDTO(selectedOffer),
		EligibleOfferIDs:    make([]string, len(decision.EligibleOfferIDs)),
		Propensity:          decision.Propensity,
		Distribution:        make([]actionProbabilityDTO, len(decision.Distribution)),
		PolicyKind:          decision.PolicyKind,
		PolicyVersion:       decision.PolicyVersion,
		PolicyLatencyMicros: decision.PolicyLatencyMicros,
		CreatedAt:           formatTimestamp(decision.CreatedAt),
	}
	for index, offerID := range decision.EligibleOfferIDs {
		dto.EligibleOfferIDs[index] = offerID.String()
	}
	for index, probability := range decision.Distribution {
		dto.Distribution[index] = actionProbabilityDTO{
			OfferID:     probability.OfferID.String(),
			Probability: probability.Probability,
		}
	}
	if outcome != nil {
		mapped := newDecisionOutcomeDTO(*outcome)
		dto.Outcome = &mapped
	}
	return dto
}

func newDecisionOutcomeDTO(outcome domain.Outcome) decisionOutcomeDTO {
	return decisionOutcomeDTO{
		EventID:              outcome.EventID.String(),
		Outcome:              outcome.Kind,
		Reward:               outcome.Reward,
		OccurredAt:           formatTimestamp(outcome.OccurredAt),
		ReceivedAt:           formatTimestamp(outcome.ReceivedAt),
		AppliedPolicyVersion: outcome.AppliedPolicyVersion,
	}
}

func newOutcomeDTO(outcome domain.Outcome) outcomeDTO {
	return outcomeDTO{
		EventID:              outcome.EventID.String(),
		DecisionID:           outcome.DecisionID.String(),
		Outcome:              outcome.Kind,
		Reward:               outcome.Reward,
		OccurredAt:           formatTimestamp(outcome.OccurredAt),
		ReceivedAt:           formatTimestamp(outcome.ReceivedAt),
		AppliedPolicyVersion: outcome.AppliedPolicyVersion,
	}
}

func newSummaryDTO(summary service.Summary) experimentSummaryDTO {
	dto := experimentSummaryDTO{
		ExperimentID:           summary.ExperimentID.String(),
		PolicyKind:             summary.PolicyKind,
		PolicyVersion:          summary.PolicyVersion,
		ExplorationRate:        cloneFloat(summary.ExplorationRate),
		DecisionCount:          summary.DecisionCount,
		OutcomeCount:           summary.OutcomeCount,
		RewardSum:              summary.RewardSum,
		AverageReward:          cloneFloat(summary.AverageReward),
		IgnoredCount:           summary.IgnoredCount,
		ClickedCount:           summary.ClickedCount,
		ConvertedCount:         summary.ConvertedCount,
		P50PolicyLatencyMicros: cloneInt64(summary.P50PolicyLatencyMicros),
		P95PolicyLatencyMicros: cloneInt64(summary.P95PolicyLatencyMicros),
		OfferPerformance:       make([]offerPerformanceDTO, len(summary.OfferPerformance)),
		LearningSeries:         make([]learningSeriesPointDTO, len(summary.LearningSeries)),
		RandomBenchmark:        newBenchmarkDTO(summary.RandomBenchmark),
		OracleBenchmark:        newBenchmarkDTO(summary.OracleBenchmark),
		OPE:                    newOPEEstimateDTO(summary.OPE),
		Reasons:                make(map[string]string, len(summary.Reasons)),
		GeneratedAt:            formatTimestamp(summary.GeneratedAt),
	}
	for index, performance := range summary.OfferPerformance {
		dto.OfferPerformance[index] = offerPerformanceDTO{
			Offer:              newOfferDTO(performance.Offer),
			SelectionCount:     performance.SelectionCount,
			OutcomeCount:       performance.OutcomeCount,
			IgnoredCount:       performance.IgnoredCount,
			ClickedCount:       performance.ClickedCount,
			ConvertedCount:     performance.ConvertedCount,
			RewardSum:          performance.RewardSum,
			EmpiricalMean:      cloneFloat(performance.EmpiricalMean),
			CurrentPolicyMean:  cloneFloat(performance.CurrentPolicyMean),
			CurrentProbability: cloneFloat(performance.CurrentProbability),
		}
	}
	for index, point := range summary.LearningSeries {
		dto.LearningSeries[index] = learningSeriesPointDTO{
			Timestamp:               formatTimestamp(point.Timestamp),
			SampleCount:             point.SampleCount,
			CumulativeAverageReward: point.CumulativeAverageReward,
		}
	}
	for key, reason := range summary.Reasons {
		dto.Reasons[key] = reason
	}
	return dto
}

func newBenchmarkDTO(reference domain.BenchmarkReference) simulationBenchmarkDTO {
	return simulationBenchmarkDTO{
		Kind:                  reference.Kind,
		ExpectedAverageReward: cloneFloat(reference.ExpectedAverageReward),
		SampleCount:           reference.SampleCount,
		Reason:                nullableString(reference.Reason),
		SimulationOnly:        reference.SimulationOnly,
	}
}

func newOPEEstimateDTO(estimate service.OPEEstimate) opeEstimateDTO {
	return opeEstimateDTO{
		IPS:                 cloneFloat(estimate.IPS),
		SNIPS:               cloneFloat(estimate.SNIPS),
		SampleCount:         estimate.SampleCount,
		EffectiveSampleSize: estimate.EffectiveSampleSize,
		WeightSum:           estimate.WeightSum,
		MinWeight:           estimate.MinWeight,
		MaxWeight:           estimate.MaxWeight,
		Reason:              nullableString(estimate.Reason),
	}
}

func newSimulationRunDTO(run domain.SimulationRun) simulationRunDTO {
	return simulationRunDTO{
		RunID:                   run.ID.String(),
		ExperimentID:            run.ExperimentID.String(),
		Seed:                    run.Seed,
		RequestsPerSecond:       run.RequestsPerSecond,
		MaxDecisions:            run.MaxDecisions,
		Status:                  run.Status,
		DecisionCount:           run.DecisionCount,
		OutcomeCount:            run.OutcomeCount,
		ErrorCount:              run.ErrorCount,
		ObservedRewardSum:       run.ObservedRewardSum,
		RandomExpectedRewardSum: run.RandomExpectedRewardSum,
		OracleExpectedRewardSum: run.OracleExpectedRewardSum,
		StartedAt:               formatTimestamp(run.StartedAt),
		StoppedAt:               formatOptionalTimestamp(run.StoppedAt),
		UpdatedAt:               formatTimestamp(run.UpdatedAt),
		ErrorCode:               cloneString(run.ErrorCode),
		ErrorDetail:             cloneString(run.ErrorDetail),
	}
}

func newSessionContextDTO(context domain.SessionContext) sessionContextDTO {
	return sessionContextDTO{
		DeviceClass:      context.DeviceClass,
		Daypart:          context.Daypart,
		CategoryAffinity: context.CategoryAffinity,
		VisitorType:      context.VisitorType,
	}
}

func formatTimestamp(value time.Time) string {
	return value.UTC().Format(time.RFC3339Nano)
}

func formatOptionalTimestamp(value *time.Time) *string {
	if value == nil {
		return nil
	}
	formatted := formatTimestamp(*value)
	return &formatted
}

func cloneFloat(value *float64) *float64 {
	if value == nil {
		return nil
	}
	clone := *value
	return &clone
}

func cloneInt64(value *int64) *int64 {
	if value == nil {
		return nil
	}
	clone := *value
	return &clone
}

func cloneString(value *string) *string {
	if value == nil {
		return nil
	}
	clone := *value
	return &clone
}

func nullableString(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
