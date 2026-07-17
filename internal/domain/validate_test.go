package domain

import (
	"encoding/json"
	"math"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

var (
	testExperimentID = uuid.MustParse("00000000-0000-0000-0000-000000000010")
	testOfferA       = uuid.MustParse("00000000-0000-0000-0000-000000000001")
	testOfferB       = uuid.MustParse("00000000-0000-0000-0000-000000000002")
	testOfferC       = uuid.MustParse("00000000-0000-0000-0000-000000000003")
	testDecisionID   = uuid.MustParse("00000000-0000-0000-0000-000000000020")
	testEventID      = uuid.MustParse("00000000-0000-0000-0000-000000000030")
	testRunID        = uuid.MustParse("00000000-0000-0000-0000-000000000040")
	testNow          = time.Date(2026, 7, 17, 12, 0, 0, 0, time.UTC)
)

func TestValidateExperiment(t *testing.T) {
	for _, status := range []ExperimentStatus{
		ExperimentStatusDraft,
		ExperimentStatusRunning,
		ExperimentStatusPaused,
		ExperimentStatusCompleted,
	} {
		t.Run(string(status), func(t *testing.T) {
			experiment := validExperiment()
			experiment.Status = status
			if err := ValidateExperiment(experiment); err != nil {
				t.Fatalf("ValidateExperiment() error = %v", err)
			}
		})
	}

	for _, epsilon := range []float64{0, 0.15, 1} {
		t.Run("epsilon", func(t *testing.T) {
			experiment := validExperiment()
			experiment.Epsilon = &epsilon
			if err := ValidateExperiment(experiment); err != nil {
				t.Fatalf("ValidateExperiment() error = %v", err)
			}
		})
	}

	randomExperiment := validExperiment()
	randomExperiment.PolicyKind = PolicyKindRandom
	randomExperiment.Epsilon = nil
	if err := ValidateExperiment(randomExperiment); err != nil {
		t.Fatalf("ValidateExperiment(random) error = %v", err)
	}

	tests := []struct {
		name   string
		mutate func(*Experiment)
	}{
		{name: "nil id", mutate: func(value *Experiment) { value.ID = uuid.Nil }},
		{name: "empty slug", mutate: func(value *Experiment) { value.Slug = " " }},
		{name: "empty name", mutate: func(value *Experiment) { value.Name = "" }},
		{name: "unknown status", mutate: func(value *Experiment) { value.Status = "unknown" }},
		{name: "unknown policy", mutate: func(value *Experiment) { value.PolicyKind = "unknown" }},
		{name: "missing epsilon", mutate: func(value *Experiment) { value.Epsilon = nil }},
		{name: "negative epsilon", mutate: func(value *Experiment) { epsilon := -0.1; value.Epsilon = &epsilon }},
		{name: "epsilon above one", mutate: func(value *Experiment) { epsilon := 1.1; value.Epsilon = &epsilon }},
		{name: "epsilon NaN", mutate: func(value *Experiment) { epsilon := math.NaN(); value.Epsilon = &epsilon }},
		{name: "epsilon infinity", mutate: func(value *Experiment) { epsilon := math.Inf(1); value.Epsilon = &epsilon }},
		{name: "version zero", mutate: func(value *Experiment) { value.PolicyVersion = 0 }},
		{name: "zero created timestamp", mutate: func(value *Experiment) { value.CreatedAt = time.Time{} }},
		{name: "non UTC updated timestamp", mutate: func(value *Experiment) { value.UpdatedAt = value.UpdatedAt.In(time.FixedZone("offset", 3600)) }},
		{name: "updated before created", mutate: func(value *Experiment) { value.UpdatedAt = value.CreatedAt.Add(-time.Second) }},
		{name: "random epsilon present", mutate: func(value *Experiment) { value.PolicyKind = PolicyKindRandom }},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			experiment := validExperiment()
			test.mutate(&experiment)
			if err := ValidateExperiment(experiment); err == nil {
				t.Fatal("ValidateExperiment() error = nil")
			}
		})
	}
}

func TestValidateOfferAndOffers(t *testing.T) {
	categories := []OfferCategory{
		OfferCategoryTravel,
		OfferCategoryDining,
		OfferCategoryWellness,
		OfferCategoryHome,
		OfferCategoryTechnology,
		OfferCategoryEntertainment,
	}
	for _, category := range categories {
		offer := validOffer(testOfferA, "offer-a")
		offer.Category = category
		if err := ValidateOffer(offer); err != nil {
			t.Fatalf("ValidateOffer(%q) error = %v", category, err)
		}
	}

	offers := []Offer{validOffer(testOfferA, "offer-a"), validOffer(testOfferB, "offer-b")}
	if err := ValidateOffers(testExperimentID, offers); err != nil {
		t.Fatalf("ValidateOffers() error = %v", err)
	}

	tests := []struct {
		name   string
		mutate func([]Offer) []Offer
	}{
		{name: "empty", mutate: func([]Offer) []Offer { return nil }},
		{name: "nil id", mutate: func(values []Offer) []Offer { values[0].ID = uuid.Nil; return values }},
		{name: "wrong experiment", mutate: func(values []Offer) []Offer { values[0].ExperimentID = uuid.New(); return values }},
		{name: "empty slug", mutate: func(values []Offer) []Offer { values[0].Slug = ""; return values }},
		{name: "empty merchant", mutate: func(values []Offer) []Offer { values[0].MerchantName = ""; return values }},
		{name: "empty title", mutate: func(values []Offer) []Offer { values[0].Title = ""; return values }},
		{name: "invalid category", mutate: func(values []Offer) []Offer { values[0].Category = "unknown"; return values }},
		{name: "duplicate id", mutate: func(values []Offer) []Offer { values[1].ID = values[0].ID; return values }},
		{name: "duplicate slug", mutate: func(values []Offer) []Offer { values[1].Slug = "OFFER-A"; return values }},
		{name: "fewer than two active", mutate: func(values []Offer) []Offer { values[1].Active = false; return values }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			values := append([]Offer(nil), offers...)
			if err := ValidateOffers(testExperimentID, test.mutate(values)); err == nil {
				t.Fatal("ValidateOffers() error = nil")
			}
		})
	}
}

func TestSessionContextAndSegmentKey(t *testing.T) {
	deviceClasses := []DeviceClass{DeviceClassMobile, DeviceClassDesktop, DeviceClassTablet}
	dayparts := []Daypart{DaypartMorning, DaypartAfternoon, DaypartEvening, DaypartNight}
	categories := []OfferCategory{OfferCategoryTravel, OfferCategoryDining, OfferCategoryWellness, OfferCategoryHome, OfferCategoryTechnology, OfferCategoryEntertainment}
	visitorTypes := []VisitorType{VisitorTypeNew, VisitorTypeReturning}

	caseCount := 0
	for _, deviceClass := range deviceClasses {
		for _, daypart := range dayparts {
			for _, category := range categories {
				for _, visitorType := range visitorTypes {
					context := SessionContext{DeviceClass: deviceClass, Daypart: daypart, CategoryAffinity: category, VisitorType: visitorType}
					key, err := SegmentKey(context)
					if err != nil {
						t.Fatalf("SegmentKey(%#v) error = %v", context, err)
					}
					want := strings.Join([]string{string(deviceClass), string(daypart), string(category), string(visitorType)}, "|")
					if key != want {
						t.Fatalf("SegmentKey(%#v) = %q, want %q", context, key, want)
					}
					caseCount++
				}
			}
		}
	}
	if caseCount != 144 {
		t.Fatalf("validated %d contexts, want 144", caseCount)
	}

	invalid := []SessionContext{
		{DeviceClass: "watch", Daypart: DaypartMorning, CategoryAffinity: OfferCategoryTravel, VisitorType: VisitorTypeNew},
		{DeviceClass: DeviceClassMobile, Daypart: "dawn", CategoryAffinity: OfferCategoryTravel, VisitorType: VisitorTypeNew},
		{DeviceClass: DeviceClassMobile, Daypart: DaypartMorning, CategoryAffinity: "finance", VisitorType: VisitorTypeNew},
		{DeviceClass: DeviceClassMobile, Daypart: DaypartMorning, CategoryAffinity: OfferCategoryTravel, VisitorType: "guest"},
	}
	for _, context := range invalid {
		if _, err := SegmentKey(context); err == nil {
			t.Fatalf("SegmentKey(%#v) error = nil", context)
		}
	}
}

func TestRewardForOutcome(t *testing.T) {
	tests := []struct {
		kind OutcomeKind
		want float64
	}{
		{kind: OutcomeKindIgnored, want: 0},
		{kind: OutcomeKindClicked, want: 0.25},
		{kind: OutcomeKindConverted, want: 1},
	}
	for _, test := range tests {
		got, err := RewardForOutcome(test.kind)
		if err != nil {
			t.Fatalf("RewardForOutcome(%q) error = %v", test.kind, err)
		}
		if got != test.want {
			t.Fatalf("RewardForOutcome(%q) = %v, want %v", test.kind, got, test.want)
		}
	}
	if _, err := RewardForOutcome("unknown"); err == nil {
		t.Fatal("RewardForOutcome(unknown) error = nil")
	}
}

func TestCanonicalOfferIDs(t *testing.T) {
	input := []uuid.UUID{testOfferC, testOfferA, testOfferB}
	original := append([]uuid.UUID(nil), input...)
	got, err := CanonicalOfferIDs(input)
	if err != nil {
		t.Fatalf("CanonicalOfferIDs() error = %v", err)
	}
	want := []uuid.UUID{testOfferA, testOfferB, testOfferC}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("CanonicalOfferIDs() = %v, want %v", got, want)
	}
	if !reflect.DeepEqual(input, original) {
		t.Fatalf("CanonicalOfferIDs() mutated input: %v", input)
	}

	for _, values := range [][]uuid.UUID{nil, {uuid.Nil}, {testOfferA, testOfferA}} {
		if _, err := CanonicalOfferIDs(values); err == nil {
			t.Fatalf("CanonicalOfferIDs(%v) error = nil", values)
		}
	}
}

func TestValidateDistribution(t *testing.T) {
	eligible := []uuid.UUID{testOfferA, testOfferB, testOfferC}
	distribution := []ActionProbability{
		{OfferID: testOfferA, Probability: 0.2},
		{OfferID: testOfferB, Probability: 0.3},
		{OfferID: testOfferC, Probability: 0.5},
	}
	probability, err := ValidateDistribution(eligible, distribution, testOfferB)
	if err != nil {
		t.Fatalf("ValidateDistribution() error = %v", err)
	}
	if probability != 0.3 {
		t.Fatalf("ValidateDistribution() probability = %v, want 0.3", probability)
	}

	tests := []struct {
		name         string
		eligible     []uuid.UUID
		distribution []ActionProbability
		selected     uuid.UUID
	}{
		{name: "empty eligible", eligible: nil, distribution: nil, selected: testOfferA},
		{name: "nil eligible", eligible: []uuid.UUID{uuid.Nil}, distribution: []ActionProbability{{OfferID: uuid.Nil, Probability: 1}}, selected: testOfferA},
		{name: "duplicate eligible", eligible: []uuid.UUID{testOfferA, testOfferA}, distribution: distribution[:2], selected: testOfferA},
		{name: "selected nil", eligible: eligible, distribution: distribution, selected: uuid.Nil},
		{name: "selected missing", eligible: eligible, distribution: distribution, selected: uuid.New()},
		{name: "incomplete", eligible: eligible, distribution: distribution[:2], selected: testOfferA},
		{name: "wrong order", eligible: eligible, distribution: []ActionProbability{distribution[1], distribution[0], distribution[2]}, selected: testOfferA},
		{name: "duplicate action", eligible: eligible, distribution: []ActionProbability{distribution[0], distribution[0], distribution[2]}, selected: testOfferA},
		{name: "negative", eligible: eligible, distribution: probabilities(-0.1, 0.6, 0.5), selected: testOfferA},
		{name: "above one", eligible: eligible, distribution: probabilities(1.1, 0, 0), selected: testOfferA},
		{name: "NaN", eligible: eligible, distribution: probabilities(math.NaN(), 0.5, 0.5), selected: testOfferA},
		{name: "infinity", eligible: eligible, distribution: probabilities(math.Inf(1), 0, 0), selected: testOfferA},
		{name: "sum low", eligible: eligible, distribution: probabilities(0.2, 0.3, 0.49), selected: testOfferA},
		{name: "sum high", eligible: eligible, distribution: probabilities(0.2, 0.3, 0.51), selected: testOfferA},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := ValidateDistribution(test.eligible, test.distribution, test.selected); err == nil {
				t.Fatal("ValidateDistribution() error = nil")
			}
		})
	}
}

func TestValidateDistribution_ToleranceBoundary(t *testing.T) {
	eligible := []uuid.UUID{testOfferA, testOfferB}
	upperInside := math.Nextafter(1+ProbabilityTolerance, 1)
	upperOutside := math.Nextafter(1+ProbabilityTolerance, math.Inf(1))
	lowerInside := math.Nextafter(1-ProbabilityTolerance, 1)
	lowerOutside := math.Nextafter(1-ProbabilityTolerance, 0)

	for name, sum := range map[string]float64{"upper inside": upperInside, "lower inside": lowerInside} {
		t.Run(name, func(t *testing.T) {
			distribution := []ActionProbability{{OfferID: testOfferA, Probability: 0.5}, {OfferID: testOfferB, Probability: sum - 0.5}}
			if _, err := ValidateDistribution(eligible, distribution, testOfferA); err != nil {
				t.Fatalf("ValidateDistribution(sum=%0.18f) error = %v", sum, err)
			}
		})
	}
	for name, sum := range map[string]float64{"upper outside": upperOutside, "lower outside": lowerOutside} {
		t.Run(name, func(t *testing.T) {
			distribution := []ActionProbability{{OfferID: testOfferA, Probability: 0.5}, {OfferID: testOfferB, Probability: sum - 0.5}}
			if _, err := ValidateDistribution(eligible, distribution, testOfferA); err == nil {
				t.Fatalf("ValidateDistribution(sum=%0.18f) error = nil", sum)
			}
		})
	}
}

func TestValidateDistribution_PropertyLoop(t *testing.T) {
	for actionCount := 2; actionCount <= 100; actionCount++ {
		ids := make([]uuid.UUID, actionCount)
		for index := range ids {
			ids[index] = uuid.NewSHA1(uuid.Nil, []byte{byte(actionCount), byte(index)})
		}
		canonical, err := CanonicalOfferIDs(ids)
		if err != nil {
			t.Fatalf("actionCount=%d: CanonicalOfferIDs() error = %v", actionCount, err)
		}
		probability := 1 / float64(actionCount)
		distribution := make([]ActionProbability, actionCount)
		for index, offerID := range canonical {
			distribution[index] = ActionProbability{OfferID: offerID, Probability: probability}
		}
		if _, err := ValidateDistribution(canonical, distribution, canonical[actionCount/2]); err != nil {
			t.Fatalf("actionCount=%d: ValidateDistribution() error = %v", actionCount, err)
		}
	}
}

func TestValidatePropensity(t *testing.T) {
	if err := ValidatePropensity(0.25, 0.25); err != nil {
		t.Fatalf("ValidatePropensity() error = %v", err)
	}
	for _, values := range [][2]float64{{0, 0}, {-0.1, -0.1}, {1.1, 1.1}, {math.NaN(), 0.5}, {math.Inf(1), 1}, {0.25, 0.5}} {
		if err := ValidatePropensity(values[0], values[1]); err == nil {
			t.Fatalf("ValidatePropensity(%v, %v) error = nil", values[0], values[1])
		}
	}
}

func TestValidateDecision(t *testing.T) {
	decision := validDecision()
	if err := ValidateDecision(decision); err != nil {
		t.Fatalf("ValidateDecision() error = %v", err)
	}

	tests := []struct {
		name   string
		mutate func(*Decision)
	}{
		{name: "nil id", mutate: func(value *Decision) { value.ID = uuid.Nil }},
		{name: "segment mismatch", mutate: func(value *Decision) { value.SegmentKey = "wrong" }},
		{name: "uncanonical eligible", mutate: func(value *Decision) {
			value.EligibleOfferIDs[0], value.EligibleOfferIDs[1] = value.EligibleOfferIDs[1], value.EligibleOfferIDs[0]
		}},
		{name: "propensity mismatch", mutate: func(value *Decision) { value.Propensity = 0.4 }},
		{name: "unknown policy", mutate: func(value *Decision) { value.PolicyKind = "unknown" }},
		{name: "version zero", mutate: func(value *Decision) { value.PolicyVersion = 0 }},
		{name: "negative latency", mutate: func(value *Decision) { value.PolicyLatencyMicros = -1 }},
		{name: "nil simulation run", mutate: func(value *Decision) { id := uuid.Nil; value.SimulationRunID = &id }},
		{name: "empty request id", mutate: func(value *Decision) { value.RequestID = "" }},
		{name: "non UTC timestamp", mutate: func(value *Decision) { value.CreatedAt = value.CreatedAt.In(time.FixedZone("offset", 3600)) }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			value := validDecision()
			test.mutate(&value)
			if err := ValidateDecision(value); err == nil {
				t.Fatal("ValidateDecision() error = nil")
			}
		})
	}
}

func TestValidateOutcome(t *testing.T) {
	for _, kind := range []OutcomeKind{OutcomeKindIgnored, OutcomeKindClicked, OutcomeKindConverted} {
		reward, err := RewardForOutcome(kind)
		if err != nil {
			t.Fatalf("RewardForOutcome(%q) error = %v", kind, err)
		}
		outcome := validOutcome()
		outcome.Kind = kind
		outcome.Reward = reward
		if err := ValidateOutcome(outcome, testNow, 2*time.Minute); err != nil {
			t.Fatalf("ValidateOutcome(%q) error = %v", kind, err)
		}
	}

	atBoundary := validOutcome()
	atBoundary.OccurredAt = testNow.Add(2 * time.Minute)
	if err := ValidateOutcome(atBoundary, testNow, 2*time.Minute); err != nil {
		t.Fatalf("ValidateOutcome(at boundary) error = %v", err)
	}

	tests := []struct {
		name   string
		mutate func(*Outcome)
		skew   time.Duration
	}{
		{name: "nil event", mutate: func(value *Outcome) { value.EventID = uuid.Nil }, skew: 2 * time.Minute},
		{name: "unknown kind", mutate: func(value *Outcome) { value.Kind = "unknown" }, skew: 2 * time.Minute},
		{name: "reward mismatch", mutate: func(value *Outcome) { value.Reward = 0.5 }, skew: 2 * time.Minute},
		{name: "reward NaN", mutate: func(value *Outcome) { value.Reward = math.NaN() }, skew: 2 * time.Minute},
		{name: "zero occurred", mutate: func(value *Outcome) { value.OccurredAt = time.Time{} }, skew: 2 * time.Minute},
		{name: "future beyond skew", mutate: func(value *Outcome) { value.OccurredAt = testNow.Add(2*time.Minute + time.Nanosecond) }, skew: 2 * time.Minute},
		{name: "received beyond skew", mutate: func(value *Outcome) { value.ReceivedAt = testNow.Add(2*time.Minute + time.Nanosecond) }, skew: 2 * time.Minute},
		{name: "version zero", mutate: func(value *Outcome) { value.AppliedPolicyVersion = 0 }, skew: 2 * time.Minute},
		{name: "negative skew", mutate: func(*Outcome) {}, skew: -time.Second},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			value := validOutcome()
			test.mutate(&value)
			if err := ValidateOutcome(value, testNow, test.skew); err == nil {
				t.Fatal("ValidateOutcome() error = nil")
			}
		})
	}
}

func TestValidatePolicySnapshot(t *testing.T) {
	snapshot := PolicySnapshot{ExperimentID: testExperimentID, PolicyKind: PolicyKindRandom, PolicyVersion: 1, SchemaVersion: PolicySnapshotSchemaVersion, State: json.RawMessage(`{"version":1}`), CreatedAt: testNow}
	if err := ValidatePolicySnapshot(snapshot); err != nil {
		t.Fatalf("ValidatePolicySnapshot() error = %v", err)
	}

	tests := []struct {
		name   string
		mutate func(*PolicySnapshot)
	}{
		{name: "nil experiment", mutate: func(value *PolicySnapshot) { value.ExperimentID = uuid.Nil }},
		{name: "unknown policy", mutate: func(value *PolicySnapshot) { value.PolicyKind = "unknown" }},
		{name: "version zero", mutate: func(value *PolicySnapshot) { value.PolicyVersion = 0 }},
		{name: "unknown schema", mutate: func(value *PolicySnapshot) { value.SchemaVersion = 2 }},
		{name: "empty state", mutate: func(value *PolicySnapshot) { value.State = nil }},
		{name: "invalid state", mutate: func(value *PolicySnapshot) { value.State = json.RawMessage(`{`) }},
		{name: "zero timestamp", mutate: func(value *PolicySnapshot) { value.CreatedAt = time.Time{} }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			value := snapshot
			test.mutate(&value)
			if err := ValidatePolicySnapshot(value); err == nil {
				t.Fatal("ValidatePolicySnapshot() error = nil")
			}
		})
	}
}

func TestValidateSimulationRun(t *testing.T) {
	for _, status := range []SimulationRunStatus{SimulationRunStatusStarting, SimulationRunStatusRunning, SimulationRunStatusStopping, SimulationRunStatusCompleted, SimulationRunStatusFailed, SimulationRunStatusCancelled} {
		run := validSimulationRun()
		run.Status = status
		if err := ValidateSimulationRun(run); err != nil {
			t.Fatalf("ValidateSimulationRun(%q) error = %v", status, err)
		}
	}

	tests := []struct {
		name   string
		mutate func(*SimulationRun)
	}{
		{name: "nil id", mutate: func(value *SimulationRun) { value.ID = uuid.Nil }},
		{name: "rate zero", mutate: func(value *SimulationRun) { value.RequestsPerSecond = 0 }},
		{name: "rate excessive", mutate: func(value *SimulationRun) { value.RequestsPerSecond = 101 }},
		{name: "max decisions zero", mutate: func(value *SimulationRun) { value.MaxDecisions = 0 }},
		{name: "max decisions excessive", mutate: func(value *SimulationRun) { value.MaxDecisions = 100_001 }},
		{name: "unknown status", mutate: func(value *SimulationRun) { value.Status = "unknown" }},
		{name: "negative decisions", mutate: func(value *SimulationRun) { value.DecisionCount = -1 }},
		{name: "outcomes exceed decisions", mutate: func(value *SimulationRun) { value.OutcomeCount = value.DecisionCount + 1 }},
		{name: "negative observed sum", mutate: func(value *SimulationRun) { value.ObservedRewardSum = -1 }},
		{name: "NaN random sum", mutate: func(value *SimulationRun) { value.RandomExpectedRewardSum = math.NaN() }},
		{name: "infinite oracle sum", mutate: func(value *SimulationRun) { value.OracleExpectedRewardSum = math.Inf(1) }},
		{name: "zero started", mutate: func(value *SimulationRun) { value.StartedAt = time.Time{} }},
		{name: "updated before started", mutate: func(value *SimulationRun) { value.UpdatedAt = value.StartedAt.Add(-time.Second) }},
		{name: "stopped before started", mutate: func(value *SimulationRun) { stopped := value.StartedAt.Add(-time.Second); value.StoppedAt = &stopped }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			value := validSimulationRun()
			test.mutate(&value)
			if err := ValidateSimulationRun(value); err == nil {
				t.Fatal("ValidateSimulationRun() error = nil")
			}
		})
	}
}

func TestValidateProjections(t *testing.T) {
	point := LearningSeriesPoint{Timestamp: testNow, SampleCount: 10, CumulativeAverageReward: 0.4}
	if err := ValidateLearningSeriesPoint(point); err != nil {
		t.Fatalf("ValidateLearningSeriesPoint() error = %v", err)
	}
	for _, reward := range []float64{-1, math.NaN(), math.Inf(1), 1.1} {
		invalid := point
		invalid.CumulativeAverageReward = reward
		if err := ValidateLearningSeriesPoint(invalid); err == nil {
			t.Fatalf("ValidateLearningSeriesPoint(%v) error = nil", reward)
		}
	}

	reward := 0.5
	available := BenchmarkReference{Kind: BenchmarkKindRandom, ExpectedAverageReward: &reward, SampleCount: 10, SimulationOnly: true}
	if err := ValidateBenchmarkReference(available); err != nil {
		t.Fatalf("ValidateBenchmarkReference(available) error = %v", err)
	}
	unavailable := BenchmarkReference{Kind: BenchmarkKindOracle, Reason: "not_simulated", SimulationOnly: true}
	if err := ValidateBenchmarkReference(unavailable); err != nil {
		t.Fatalf("ValidateBenchmarkReference(unavailable) error = %v", err)
	}
	invalidReward := math.NaN()
	invalid := []BenchmarkReference{
		{Kind: "unknown", Reason: "not_simulated", SimulationOnly: true},
		{Kind: BenchmarkKindRandom, Reason: "not_simulated"},
		{Kind: BenchmarkKindRandom, SimulationOnly: true},
		{Kind: BenchmarkKindRandom, ExpectedAverageReward: &reward, SimulationOnly: true},
		{Kind: BenchmarkKindRandom, ExpectedAverageReward: &reward, SampleCount: 1, Reason: "unexpected", SimulationOnly: true},
		{Kind: BenchmarkKindRandom, ExpectedAverageReward: &invalidReward, SampleCount: 1, SimulationOnly: true},
	}
	for _, reference := range invalid {
		if err := ValidateBenchmarkReference(reference); err == nil {
			t.Fatalf("ValidateBenchmarkReference(%#v) error = nil", reference)
		}
	}
}

func TestPrivacySafeDomainShape(t *testing.T) {
	types := []reflect.Type{
		reflect.TypeOf(Experiment{}),
		reflect.TypeOf(Offer{}),
		reflect.TypeOf(SessionContext{}),
		reflect.TypeOf(ActionProbability{}),
		reflect.TypeOf(Decision{}),
		reflect.TypeOf(Outcome{}),
		reflect.TypeOf(PolicySnapshot{}),
		reflect.TypeOf(SimulationRun{}),
		reflect.TypeOf(LearningSeriesPoint{}),
		reflect.TypeOf(BenchmarkReference{}),
	}
	denied := []string{"name", "email", "phone", "address", "birth", "gender", "race", "income", "credit", "deviceid", "preciselocation", "latitude", "longitude", "userid", "customerid"}
	allowedBusinessNames := map[string]struct{}{
		"Experiment.Name":    {},
		"Offer.MerchantName": {},
	}

	for _, domainType := range types {
		for index := 0; index < domainType.NumField(); index++ {
			field := domainType.Field(index)
			qualified := domainType.Name() + "." + field.Name
			if _, allowed := allowedBusinessNames[qualified]; allowed {
				continue
			}
			normalized := strings.ToLower(field.Name)
			for _, fragment := range denied {
				if strings.Contains(normalized, fragment) {
					t.Fatalf("privacy-sensitive field %s contains denied concept %q", qualified, fragment)
				}
			}
		}
	}
}

func validExperiment() Experiment {
	epsilon := 0.15
	return Experiment{
		ID:            testExperimentID,
		Slug:          "evening-marketplace-demo",
		Name:          "Evening marketplace demo",
		Status:        ExperimentStatusRunning,
		PolicyKind:    PolicyKindSegmentedEpsilonGreedy,
		Epsilon:       &epsilon,
		PolicyVersion: 1,
		CreatedAt:     testNow,
		UpdatedAt:     testNow,
	}
}

func validOffer(id uuid.UUID, slug string) Offer {
	return Offer{
		ID:           id,
		ExperimentID: testExperimentID,
		Slug:         slug,
		MerchantName: "Fictional Merchant",
		Title:        "Synthetic offer",
		Description:  "For deterministic tests only.",
		Category:     OfferCategoryTravel,
		Active:       true,
	}
}

func validContext() SessionContext {
	return SessionContext{
		DeviceClass:      DeviceClassMobile,
		Daypart:          DaypartEvening,
		CategoryAffinity: OfferCategoryTravel,
		VisitorType:      VisitorTypeReturning,
	}
}

func validDecision() Decision {
	return Decision{
		ID:              testDecisionID,
		ExperimentID:    testExperimentID,
		SelectedOfferID: testOfferB,
		Context:         validContext(),
		SegmentKey:      "mobile|evening|travel|returning",
		EligibleOfferIDs: []uuid.UUID{
			testOfferA,
			testOfferB,
		},
		Distribution: []ActionProbability{
			{OfferID: testOfferA, Probability: 0.4},
			{OfferID: testOfferB, Probability: 0.6},
		},
		Propensity:          0.6,
		PolicyKind:          PolicyKindSegmentedEpsilonGreedy,
		PolicyVersion:       3,
		PolicyLatencyMicros: 120,
		RequestID:           "request-1",
		CreatedAt:           testNow,
	}
}

func validOutcome() Outcome {
	return Outcome{
		EventID:              testEventID,
		DecisionID:           testDecisionID,
		Kind:                 OutcomeKindClicked,
		Reward:               0.25,
		OccurredAt:           testNow.Add(-time.Second),
		ReceivedAt:           testNow,
		AppliedPolicyVersion: 4,
	}
}

func validSimulationRun() SimulationRun {
	stopped := testNow.Add(time.Minute)
	return SimulationRun{
		ID:                      testRunID,
		ExperimentID:            testExperimentID,
		Seed:                    20260717,
		RequestsPerSecond:       20,
		MaxDecisions:            1000,
		Status:                  SimulationRunStatusCompleted,
		DecisionCount:           100,
		OutcomeCount:            100,
		ErrorCount:              0,
		ObservedRewardSum:       25,
		RandomExpectedRewardSum: 20,
		OracleExpectedRewardSum: 35,
		StartedAt:               testNow,
		StoppedAt:               &stopped,
		UpdatedAt:               stopped,
	}
}

func probabilities(first, second, third float64) []ActionProbability {
	return []ActionProbability{
		{OfferID: testOfferA, Probability: first},
		{OfferID: testOfferB, Probability: second},
		{OfferID: testOfferC, Probability: third},
	}
}
