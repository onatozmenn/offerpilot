package bandit

import (
	"bytes"
	"encoding/json"
	"math"
	"reflect"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/onatozmenn/offerpilot/internal/domain"
)

var (
	epsilonExperimentID = uuid.MustParse("20000000-0000-0000-0000-000000000000")
	epsilonOfferA       = uuid.MustParse("00000000-0000-0000-0000-000000000011")
	epsilonOfferB       = uuid.MustParse("00000000-0000-0000-0000-000000000012")
	epsilonOfferC       = uuid.MustParse("00000000-0000-0000-0000-000000000013")
	epsilonOfferD       = uuid.MustParse("00000000-0000-0000-0000-000000000014")
)

func TestEpsilonGreedy_Constructor(t *testing.T) {
	if _, err := NewEpsilonGreedyPolicy(epsilonExperimentID, 0.15, DefaultPriorCount, DefaultPriorRewardSum, 1, NewLockedRandom(1)); err != nil {
		t.Fatalf("NewEpsilonGreedyPolicy(valid) error = %v", err)
	}

	tests := []struct {
		name           string
		experimentID   uuid.UUID
		epsilon        float64
		priorCount     float64
		priorRewardSum float64
		version        int64
		random         RandomSource
	}{
		{name: "nil experiment", epsilon: 0.1, priorCount: 2, priorRewardSum: 1, version: 1, random: NewLockedRandom(1)},
		{name: "negative epsilon", experimentID: epsilonExperimentID, epsilon: -0.1, priorCount: 2, priorRewardSum: 1, version: 1, random: NewLockedRandom(1)},
		{name: "epsilon above one", experimentID: epsilonExperimentID, epsilon: 1.1, priorCount: 2, priorRewardSum: 1, version: 1, random: NewLockedRandom(1)},
		{name: "epsilon NaN", experimentID: epsilonExperimentID, epsilon: math.NaN(), priorCount: 2, priorRewardSum: 1, version: 1, random: NewLockedRandom(1)},
		{name: "epsilon infinity", experimentID: epsilonExperimentID, epsilon: math.Inf(1), priorCount: 2, priorRewardSum: 1, version: 1, random: NewLockedRandom(1)},
		{name: "prior count zero", experimentID: epsilonExperimentID, epsilon: 0.1, priorCount: 0, priorRewardSum: 0, version: 1, random: NewLockedRandom(1)},
		{name: "prior count NaN", experimentID: epsilonExperimentID, epsilon: 0.1, priorCount: math.NaN(), priorRewardSum: 1, version: 1, random: NewLockedRandom(1)},
		{name: "negative prior reward", experimentID: epsilonExperimentID, epsilon: 0.1, priorCount: 2, priorRewardSum: -0.1, version: 1, random: NewLockedRandom(1)},
		{name: "prior reward above count", experimentID: epsilonExperimentID, epsilon: 0.1, priorCount: 2, priorRewardSum: 2.1, version: 1, random: NewLockedRandom(1)},
		{name: "version zero", experimentID: epsilonExperimentID, epsilon: 0.1, priorCount: 2, priorRewardSum: 1, random: NewLockedRandom(1)},
		{name: "nil random", experimentID: epsilonExperimentID, epsilon: 0.1, priorCount: 2, priorRewardSum: 1, version: 1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := NewEpsilonGreedyPolicy(test.experimentID, test.epsilon, test.priorCount, test.priorRewardSum, test.version, test.random); err == nil {
				t.Fatal("NewEpsilonGreedyPolicy() error = nil")
			}
		})
	}
}

func TestEpsilonGreedy_ColdStartAndEpsilonExtremes(t *testing.T) {
	for _, epsilon := range []float64{0, 0.3, 1} {
		t.Run(floatName(epsilon), func(t *testing.T) {
			policy := newTestEpsilonPolicy(t, epsilon, 1, &sequenceRandom{values: []float64{0}})
			selection, err := policy.Select(epsilonSelectionInput("segment-a", epsilonOfferC, epsilonOfferA, epsilonOfferB))
			if err != nil {
				t.Fatalf("Select() error = %v", err)
			}
			assertDistribution(t, selection.Distribution, []uuid.UUID{epsilonOfferA, epsilonOfferB, epsilonOfferC}, []float64{1.0 / 3, 1.0 / 3, 1.0 / 3})
			if selection.SelectedOfferID != epsilonOfferA || selection.PolicyKind != domain.PolicyKindSegmentedEpsilonGreedy || selection.PolicyVersion != 1 {
				t.Fatalf("Select() = %#v", selection)
			}

			state := snapshotState(t, policy)
			if len(state.Segments) != 1 || state.Segments[0].Key != "segment-a" || len(state.Segments[0].Offers) != 3 {
				t.Fatalf("cold-start snapshot = %#v", state)
			}
			for _, arm := range state.Segments[0].Offers {
				if arm.Count != DefaultPriorCount || arm.RewardSum != DefaultPriorRewardSum {
					t.Fatalf("cold-start arm = %#v", arm)
				}
			}
		})
	}
}

func TestEpsilonGreedy_FormulaBranches(t *testing.T) {
	t.Run("unique best", func(t *testing.T) {
		policy := newTestEpsilonPolicy(t, 0.3, 1, &sequenceRandom{values: []float64{0}})
		input := epsilonSelectionInput("segment", epsilonOfferA, epsilonOfferB, epsilonOfferC)
		if _, err := policy.Select(input); err != nil {
			t.Fatalf("Select(initial) error = %v", err)
		}
		if err := policy.Update(epsilonUpdate("segment", epsilonOfferA, input.OfferIDs, 1, 2, 1)); err != nil {
			t.Fatalf("Update() error = %v", err)
		}
		selection, err := policy.Select(input)
		if err != nil {
			t.Fatalf("Select(learned) error = %v", err)
		}
		assertDistribution(t, selection.Distribution, []uuid.UUID{epsilonOfferA, epsilonOfferB, epsilonOfferC}, []float64{0.8, 0.1, 0.1})
	})

	t.Run("partial tie", func(t *testing.T) {
		policy := newTestEpsilonPolicy(t, 0.3, 1, &sequenceRandom{values: []float64{0}})
		input := epsilonSelectionInput("segment", epsilonOfferA, epsilonOfferB, epsilonOfferC)
		if _, err := policy.Select(input); err != nil {
			t.Fatalf("Select(initial) error = %v", err)
		}
		if err := policy.Update(epsilonUpdate("segment", epsilonOfferC, input.OfferIDs, 1, 2, 0)); err != nil {
			t.Fatalf("Update() error = %v", err)
		}
		selection, err := policy.Select(input)
		if err != nil {
			t.Fatalf("Select(learned) error = %v", err)
		}
		assertDistribution(t, selection.Distribution, []uuid.UUID{epsilonOfferA, epsilonOfferB, epsilonOfferC}, []float64{0.45, 0.45, 0.1})
	})

	t.Run("fractional reward", func(t *testing.T) {
		policy := newTestEpsilonPolicy(t, 0.2, 1, &sequenceRandom{values: []float64{0}})
		input := epsilonSelectionInput("segment", epsilonOfferA, epsilonOfferB)
		if _, err := policy.Select(input); err != nil {
			t.Fatalf("Select(initial) error = %v", err)
		}
		if err := policy.Update(epsilonUpdate("segment", epsilonOfferA, input.OfferIDs, 1, 2, 0.25)); err != nil {
			t.Fatalf("Update() error = %v", err)
		}
		selection, err := policy.Select(input)
		if err != nil {
			t.Fatalf("Select(learned) error = %v", err)
		}
		assertDistribution(t, selection.Distribution, []uuid.UUID{epsilonOfferA, epsilonOfferB}, []float64{0.1, 0.9})

		state := snapshotState(t, policy)
		if state.Segments[0].Offers[0].Count != 3 || state.Segments[0].Offers[0].RewardSum != 1.25 {
			t.Fatalf("fractional state = %#v", state.Segments[0].Offers[0])
		}
	})
}

func TestEpsilonGreedy_EpsilonAfterLearning(t *testing.T) {
	tests := []struct {
		epsilon float64
		want    []float64
	}{
		{epsilon: 0, want: []float64{1, 0, 0}},
		{epsilon: 0.3, want: []float64{0.8, 0.1, 0.1}},
		{epsilon: 1, want: []float64{1.0 / 3, 1.0 / 3, 1.0 / 3}},
	}
	for _, test := range tests {
		t.Run(floatName(test.epsilon), func(t *testing.T) {
			policy := newTestEpsilonPolicy(t, test.epsilon, 1, &sequenceRandom{values: []float64{0}})
			input := epsilonSelectionInput("segment", epsilonOfferA, epsilonOfferB, epsilonOfferC)
			if _, err := policy.Select(input); err != nil {
				t.Fatalf("Select(initial) error = %v", err)
			}
			if err := policy.Update(epsilonUpdate("segment", epsilonOfferA, input.OfferIDs, 1, 2, 1)); err != nil {
				t.Fatalf("Update() error = %v", err)
			}
			selection, err := policy.Select(input)
			if err != nil {
				t.Fatalf("Select(learned) error = %v", err)
			}
			assertDistribution(t, selection.Distribution, []uuid.UUID{epsilonOfferA, epsilonOfferB, epsilonOfferC}, test.want)
		})
	}
}

func TestEpsilonGreedy_MultipleSegments(t *testing.T) {
	policy := newTestEpsilonPolicy(t, 0.2, 1, &sequenceRandom{values: []float64{0}})
	offers := []uuid.UUID{epsilonOfferA, epsilonOfferB}
	firstInput := epsilonSelectionInput("mobile|evening|travel|returning", offers...)
	secondInput := epsilonSelectionInput("desktop|morning|home|new", offers...)
	if _, err := policy.Select(firstInput); err != nil {
		t.Fatalf("Select(first segment) error = %v", err)
	}
	if _, err := policy.Select(secondInput); err != nil {
		t.Fatalf("Select(second segment) error = %v", err)
	}
	if err := policy.Update(epsilonUpdate(firstInput.SegmentKey, epsilonOfferA, offers, 1, 2, 1)); err != nil {
		t.Fatalf("Update(first segment) error = %v", err)
	}

	firstSelection, err := policy.Select(firstInput)
	if err != nil {
		t.Fatalf("Select(first learned) error = %v", err)
	}
	secondSelection, err := policy.Select(secondInput)
	if err != nil {
		t.Fatalf("Select(second unchanged) error = %v", err)
	}
	assertDistribution(t, firstSelection.Distribution, offers, []float64{0.9, 0.1})
	assertDistribution(t, secondSelection.Distribution, offers, []float64{0.5, 0.5})
}

func TestEpsilonGreedy_UpdateLifecycle(t *testing.T) {
	policy := newTestEpsilonPolicy(t, 0.1, 3, NewLockedRandom(1))
	input := epsilonSelectionInput("segment", epsilonOfferA, epsilonOfferB)
	if _, err := policy.Select(input); err != nil {
		t.Fatalf("Select() error = %v", err)
	}

	delayed := epsilonUpdate("segment", epsilonOfferA, input.OfferIDs, 1, 4, 0.25)
	if err := policy.Update(delayed); err != nil {
		t.Fatalf("Update(delayed) error = %v", err)
	}
	if policy.Version() != 4 {
		t.Fatalf("Version() = %d, want 4", policy.Version())
	}
	if err := policy.Update(delayed); err == nil {
		t.Fatal("duplicate Update() error = nil")
	}

	tests := []struct {
		name   string
		mutate func(*Update)
	}{
		{name: "wrong experiment", mutate: func(value *Update) { value.ExperimentID = uuid.New() }},
		{name: "wrong policy", mutate: func(value *Update) { value.PolicyKind = domain.PolicyKindRandom }},
		{name: "empty segment", mutate: func(value *Update) { value.SegmentKey = "" }},
		{name: "unknown segment", mutate: func(value *Update) { value.SegmentKey = "unknown" }},
		{name: "future selection version", mutate: func(value *Update) { value.SelectionPolicyVersion = 5 }},
		{name: "nonconsecutive version", mutate: func(value *Update) { value.AppliedPolicyVersion = 6 }},
		{name: "unknown action", mutate: func(value *Update) {
			value.SelectedOfferID = epsilonOfferC
			value.EligibleOfferIDs = append(value.EligibleOfferIDs, epsilonOfferC)
		}},
		{name: "negative reward", mutate: func(value *Update) { value.Reward = -0.1 }},
		{name: "reward above one", mutate: func(value *Update) { value.Reward = 1.1 }},
		{name: "reward NaN", mutate: func(value *Update) { value.Reward = math.NaN() }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			value := epsilonUpdate("segment", epsilonOfferA, input.OfferIDs, 4, 5, 0.5)
			test.mutate(&value)
			if err := policy.Update(value); err == nil {
				t.Fatal("Update() error = nil")
			}
			if policy.Version() != 4 {
				t.Fatalf("invalid update changed version to %d", policy.Version())
			}
		})
	}
}

func TestEpsilonGreedy_SelectionFailures(t *testing.T) {
	validInput := epsilonSelectionInput("segment", epsilonOfferA, epsilonOfferB)
	tests := []struct {
		name   string
		input  SelectionInput
		random RandomSource
	}{
		{name: "wrong experiment", input: SelectionInput{ExperimentID: uuid.New(), SegmentKey: "segment", OfferIDs: validInput.OfferIDs}, random: NewLockedRandom(1)},
		{name: "empty segment", input: SelectionInput{ExperimentID: epsilonExperimentID, OfferIDs: validInput.OfferIDs}, random: NewLockedRandom(1)},
		{name: "one offer", input: epsilonSelectionInput("segment", epsilonOfferA), random: NewLockedRandom(1)},
		{name: "duplicate offer", input: epsilonSelectionInput("segment", epsilonOfferA, epsilonOfferA), random: NewLockedRandom(1)},
		{name: "random error", input: validInput, random: &sequenceRandom{err: errTestRandom}},
		{name: "invalid draw", input: validInput, random: &sequenceRandom{values: []float64{1}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			policy := newTestEpsilonPolicy(t, 0.1, 1, test.random)
			if _, err := policy.Select(test.input); err == nil {
				t.Fatal("Select() error = nil")
			}
		})
	}
}

func TestEpsilonGreedy_SeededSequence(t *testing.T) {
	first := newTestEpsilonPolicy(t, 0.2, 1, NewLockedRandom(20260717))
	second := newTestEpsilonPolicy(t, 0.2, 1, NewLockedRandom(20260717))
	input := epsilonSelectionInput("segment", epsilonOfferD, epsilonOfferB, epsilonOfferA, epsilonOfferC)
	firstSequence := make([]uuid.UUID, 100)
	secondSequence := make([]uuid.UUID, 100)
	for index := range firstSequence {
		firstSelection, err := first.Select(input)
		if err != nil {
			t.Fatalf("first Select(%d) error = %v", index, err)
		}
		secondSelection, err := second.Select(input)
		if err != nil {
			t.Fatalf("second Select(%d) error = %v", index, err)
		}
		firstSequence[index] = firstSelection.SelectedOfferID
		secondSequence[index] = secondSelection.SelectedOfferID
	}
	if !reflect.DeepEqual(firstSequence, secondSequence) {
		t.Fatal("identically seeded epsilon-greedy policies diverged")
	}
}

func TestEpsilonGreedy_SnapshotRoundTripAndDeterminism(t *testing.T) {
	policy := newTestEpsilonPolicy(t, 0.2, 1, NewLockedRandom(1))
	secondInput := epsilonSelectionInput("segment-z", epsilonOfferD, epsilonOfferB)
	firstInput := epsilonSelectionInput("segment-a", epsilonOfferC, epsilonOfferA)
	if _, err := policy.Select(secondInput); err != nil {
		t.Fatalf("Select(second segment) error = %v", err)
	}
	if _, err := policy.Select(firstInput); err != nil {
		t.Fatalf("Select(first segment) error = %v", err)
	}
	if err := policy.Update(epsilonUpdate(firstInput.SegmentKey, epsilonOfferA, firstInput.OfferIDs, 1, 2, 0.25)); err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	firstSnapshot, err := policy.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot() error = %v", err)
	}
	secondSnapshot, err := policy.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot(second) error = %v", err)
	}
	if !bytes.Equal(firstSnapshot.State, secondSnapshot.State) {
		t.Fatalf("deterministic snapshots differ:\n%s\n%s", firstSnapshot.State, secondSnapshot.State)
	}
	if firstSnapshot.SchemaVersion != SnapshotSchemaVersion || firstSnapshot.PolicyVersion != 2 || firstSnapshot.PolicyKind != domain.PolicyKindSegmentedEpsilonGreedy {
		t.Fatalf("Snapshot() metadata = %#v", firstSnapshot)
	}

	state := decodeEpsilonState(t, firstSnapshot.State)
	if len(state.Segments) != 2 || state.Segments[0].Key != "segment-a" || state.Segments[1].Key != "segment-z" {
		t.Fatalf("snapshot segment ordering = %#v", state.Segments)
	}
	if state.Segments[0].Offers[0].OfferID != epsilonOfferA || state.Segments[0].Offers[1].OfferID != epsilonOfferC {
		t.Fatalf("snapshot offer ordering = %#v", state.Segments[0].Offers)
	}

	restored := newTestEpsilonPolicy(t, 0.2, 1, NewLockedRandom(1))
	if err := restored.Restore(firstSnapshot); err != nil {
		t.Fatalf("Restore() error = %v", err)
	}
	if restored.Version() != 2 {
		t.Fatalf("restored Version() = %d, want 2", restored.Version())
	}
	originalSelection, err := policy.Select(firstInput)
	if err != nil {
		t.Fatalf("original Select() error = %v", err)
	}
	restoredSelection, err := restored.Select(firstInput)
	if err != nil {
		t.Fatalf("restored Select() error = %v", err)
	}
	if originalSelection.PolicyKind != restoredSelection.PolicyKind || originalSelection.PolicyVersion != restoredSelection.PolicyVersion || !reflect.DeepEqual(originalSelection.Distribution, restoredSelection.Distribution) {
		t.Fatalf("restored policy state selection = %#v, want distribution/version from %#v", restoredSelection, originalSelection)
	}

	firstSnapshot.State[0] = '['
	thirdSnapshot, err := restored.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot(after caller mutation) error = %v", err)
	}
	if thirdSnapshot.State[0] != '{' {
		t.Fatal("snapshot state aliases caller-owned bytes")
	}
}

func TestEpsilonGreedy_SnapshotFailures(t *testing.T) {
	policy := newTestEpsilonPolicy(t, 0.2, 3, NewLockedRandom(1))
	for _, input := range []SelectionInput{
		epsilonSelectionInput("segment-a", epsilonOfferA, epsilonOfferB),
		epsilonSelectionInput("segment-b", epsilonOfferC, epsilonOfferD),
	} {
		if _, err := policy.Select(input); err != nil {
			t.Fatalf("Select(%q) error = %v", input.SegmentKey, err)
		}
	}
	validSnapshot, err := policy.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot() error = %v", err)
	}

	tests := []struct {
		name   string
		mutate func(*Snapshot)
	}{
		{name: "unknown outer schema", mutate: func(value *Snapshot) { value.SchemaVersion = 2 }},
		{name: "wrong experiment", mutate: func(value *Snapshot) { value.ExperimentID = uuid.New() }},
		{name: "wrong policy", mutate: func(value *Snapshot) { value.PolicyKind = domain.PolicyKindRandom }},
		{name: "version zero", mutate: func(value *Snapshot) { value.PolicyVersion = 0 }},
		{name: "version regression", mutate: func(value *Snapshot) { value.PolicyVersion = 2 }},
		{name: "empty state", mutate: func(value *Snapshot) { value.State = nil }},
		{name: "malformed state", mutate: func(value *Snapshot) { value.State = []byte(`{`) }},
		{name: "trailing state", mutate: func(value *Snapshot) { value.State = append(value.State, []byte(` {}`)...) }},
		{name: "unknown state field", mutate: func(value *Snapshot) {
			value.State = []byte(`{"epsilon":0.2,"prior_count":2,"prior_reward_sum":1,"segments":[],"unknown":true}`)
		}},
		{name: "epsilon mismatch", mutate: mutateEpsilonState(func(state *epsilonSnapshotState) { state.Epsilon = 0.3 })},
		{name: "prior count mismatch", mutate: mutateEpsilonState(func(state *epsilonSnapshotState) { state.PriorCount = 3 })},
		{name: "prior reward mismatch", mutate: mutateEpsilonState(func(state *epsilonSnapshotState) { state.PriorRewardSum = 0.5 })},
		{name: "empty segment key", mutate: mutateEpsilonState(func(state *epsilonSnapshotState) { state.Segments[0].Key = "" })},
		{name: "unsorted segments", mutate: mutateEpsilonState(func(state *epsilonSnapshotState) {
			state.Segments[0], state.Segments[1] = state.Segments[1], state.Segments[0]
		})},
		{name: "one offer", mutate: mutateEpsilonState(func(state *epsilonSnapshotState) { state.Segments[0].Offers = state.Segments[0].Offers[:1] })},
		{name: "nil offer", mutate: mutateEpsilonState(func(state *epsilonSnapshotState) { state.Segments[0].Offers[0].OfferID = uuid.Nil })},
		{name: "unsorted offers", mutate: mutateEpsilonState(func(state *epsilonSnapshotState) {
			state.Segments[0].Offers[0], state.Segments[0].Offers[1] = state.Segments[0].Offers[1], state.Segments[0].Offers[0]
		})},
		{name: "count below prior", mutate: mutateEpsilonState(func(state *epsilonSnapshotState) { state.Segments[0].Offers[0].Count = 1 })},
		{name: "fractional count", mutate: mutateEpsilonState(func(state *epsilonSnapshotState) { state.Segments[0].Offers[0].Count = 2.5 })},
		{name: "reward below prior", mutate: mutateEpsilonState(func(state *epsilonSnapshotState) { state.Segments[0].Offers[0].RewardSum = 0.5 })},
		{name: "reward above possible", mutate: mutateEpsilonState(func(state *epsilonSnapshotState) { state.Segments[0].Offers[0].RewardSum = 2 })},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			value := cloneSnapshot(validSnapshot)
			test.mutate(&value)
			before, err := policy.Snapshot()
			if err != nil {
				t.Fatalf("Snapshot(before invalid restore) error = %v", err)
			}
			if err := policy.Restore(value); err == nil {
				t.Fatal("Restore() error = nil")
			}
			after, err := policy.Snapshot()
			if err != nil {
				t.Fatalf("Snapshot(after invalid restore) error = %v", err)
			}
			if before.PolicyVersion != after.PolicyVersion || !bytes.Equal(before.State, after.State) {
				t.Fatal("invalid restore mutated policy state")
			}
		})
	}
}

func TestEpsilonGreedy_DistributionProperties(t *testing.T) {
	for _, epsilon := range []float64{0, 0.01, 0.15, 0.5, 1} {
		for actionCount := 2; actionCount <= 50; actionCount++ {
			policy := newTestEpsilonPolicy(t, epsilon, 1, &sequenceRandom{values: []float64{0}})
			offerIDs := make([]uuid.UUID, actionCount)
			for index := range offerIDs {
				offerIDs[index] = uuid.NewSHA1(uuid.Nil, []byte{byte(actionCount), byte(index), byte(epsilon * 100)})
			}
			selection, err := policy.Select(SelectionInput{ExperimentID: epsilonExperimentID, SegmentKey: "property", OfferIDs: offerIDs})
			if err != nil {
				t.Fatalf("epsilon=%v actions=%d: Select() error = %v", epsilon, actionCount, err)
			}
			canonical, err := domain.CanonicalOfferIDs(offerIDs)
			if err != nil {
				t.Fatalf("epsilon=%v actions=%d: CanonicalOfferIDs() error = %v", epsilon, actionCount, err)
			}
			if _, err := domain.ValidateDistribution(canonical, selection.Distribution, selection.SelectedOfferID); err != nil {
				t.Fatalf("epsilon=%v actions=%d: ValidateDistribution() error = %v", epsilon, actionCount, err)
			}
			for _, entry := range selection.Distribution {
				if math.IsNaN(entry.Probability) || math.IsInf(entry.Probability, 0) || entry.Probability < 0 || entry.Probability > 1 {
					t.Fatalf("epsilon=%v actions=%d: invalid probability %v", epsilon, actionCount, entry.Probability)
				}
			}
		}
	}

	eligible := []uuid.UUID{epsilonOfferA, epsilonOfferB}
	inside := math.Nextafter(1+domain.ProbabilityTolerance, 1)
	outside := math.Nextafter(1+domain.ProbabilityTolerance, math.Inf(1))
	insideDistribution := []domain.ActionProbability{{OfferID: epsilonOfferA, Probability: 0.5}, {OfferID: epsilonOfferB, Probability: inside - 0.5}}
	if _, err := domain.ValidateDistribution(eligible, insideDistribution, epsilonOfferA); err != nil {
		t.Fatalf("ValidateDistribution(inside tolerance) error = %v", err)
	}
	outsideDistribution := []domain.ActionProbability{{OfferID: epsilonOfferA, Probability: 0.5}, {OfferID: epsilonOfferB, Probability: outside - 0.5}}
	if _, err := domain.ValidateDistribution(eligible, outsideDistribution, epsilonOfferA); err == nil {
		t.Fatal("ValidateDistribution(outside tolerance) error = nil")
	}
}

func TestEpsilonGreedy_ConcurrentSelectAndUpdate(t *testing.T) {
	policy := newTestEpsilonPolicy(t, 0.15, 1, NewLockedRandom(20260717))
	input := epsilonSelectionInput("segment", epsilonOfferA, epsilonOfferB, epsilonOfferC, epsilonOfferD)
	if _, err := policy.Select(input); err != nil {
		t.Fatalf("Select(initial) error = %v", err)
	}

	const selectorCount = 8
	const selectionsPerSelector = 100
	const updateCount = 100
	errorsChannel := make(chan error, selectorCount*selectionsPerSelector+updateCount)
	var waitGroup sync.WaitGroup
	for range selectorCount {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			for range selectionsPerSelector {
				selection, err := policy.Select(input)
				if err != nil {
					errorsChannel <- err
					continue
				}
				canonical, err := domain.CanonicalOfferIDs(input.OfferIDs)
				if err != nil {
					errorsChannel <- err
					continue
				}
				if _, err := domain.ValidateDistribution(canonical, selection.Distribution, selection.SelectedOfferID); err != nil {
					errorsChannel <- err
				}
			}
		}()
	}
	waitGroup.Add(1)
	go func() {
		defer waitGroup.Done()
		for index := range updateCount {
			reward := float64(index%5) / 4
			if err := policy.Update(epsilonUpdate(input.SegmentKey, epsilonOfferA, input.OfferIDs, 1, int64(index+2), reward)); err != nil {
				errorsChannel <- err
				return
			}
		}
	}()
	waitGroup.Wait()
	close(errorsChannel)
	for err := range errorsChannel {
		t.Errorf("concurrent operation error = %v", err)
	}
	if policy.Version() != updateCount+1 {
		t.Fatalf("Version() = %d, want %d", policy.Version(), updateCount+1)
	}
}

func newTestEpsilonPolicy(t *testing.T, epsilon float64, version int64, random RandomSource) *EpsilonGreedyPolicy {
	t.Helper()
	policy, err := NewEpsilonGreedyPolicy(
		epsilonExperimentID,
		epsilon,
		DefaultPriorCount,
		DefaultPriorRewardSum,
		version,
		random,
	)
	if err != nil {
		t.Fatalf("NewEpsilonGreedyPolicy() error = %v", err)
	}
	return policy
}

func epsilonSelectionInput(segmentKey string, offerIDs ...uuid.UUID) SelectionInput {
	return SelectionInput{ExperimentID: epsilonExperimentID, SegmentKey: segmentKey, OfferIDs: offerIDs}
}

func epsilonUpdate(segmentKey string, selectedOfferID uuid.UUID, eligibleOfferIDs []uuid.UUID, selectionVersion, appliedVersion int64, reward float64) Update {
	return Update{
		ExperimentID:           epsilonExperimentID,
		SegmentKey:             segmentKey,
		SelectedOfferID:        selectedOfferID,
		EligibleOfferIDs:       append([]uuid.UUID(nil), eligibleOfferIDs...),
		SelectionPolicyVersion: selectionVersion,
		AppliedPolicyVersion:   appliedVersion,
		Reward:                 reward,
		PolicyKind:             domain.PolicyKindSegmentedEpsilonGreedy,
	}
}

func assertDistribution(t *testing.T, got []domain.ActionProbability, offerIDs []uuid.UUID, probabilities []float64) {
	t.Helper()
	if len(got) != len(offerIDs) || len(got) != len(probabilities) {
		t.Fatalf("distribution length = %d, offers = %d, probabilities = %d", len(got), len(offerIDs), len(probabilities))
	}
	for index, entry := range got {
		if entry.OfferID != offerIDs[index] {
			t.Fatalf("distribution[%d].OfferID = %s, want %s", index, entry.OfferID, offerIDs[index])
		}
		if math.Abs(entry.Probability-probabilities[index]) > 1e-15 {
			t.Fatalf("distribution[%d].Probability = %.18f, want %.18f", index, entry.Probability, probabilities[index])
		}
	}
}

func snapshotState(t *testing.T, policy *EpsilonGreedyPolicy) epsilonSnapshotState {
	t.Helper()
	snapshot, err := policy.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot() error = %v", err)
	}
	return decodeEpsilonState(t, snapshot.State)
}

func decodeEpsilonState(t *testing.T, raw json.RawMessage) epsilonSnapshotState {
	t.Helper()
	var state epsilonSnapshotState
	if err := json.Unmarshal(raw, &state); err != nil {
		t.Fatalf("decode snapshot state: %v", err)
	}
	return state
}

func mutateEpsilonState(mutate func(*epsilonSnapshotState)) func(*Snapshot) {
	return func(snapshot *Snapshot) {
		var state epsilonSnapshotState
		if err := json.Unmarshal(snapshot.State, &state); err != nil {
			panic(err)
		}
		mutate(&state)
		encoded, err := json.Marshal(state)
		if err != nil {
			panic(err)
		}
		snapshot.State = encoded
	}
}

func floatName(value float64) string {
	switch value {
	case 0:
		return "epsilon zero"
	case 1:
		return "epsilon one"
	default:
		return "epsilon middle"
	}
}

var errTestRandom = &testRandomError{}

type testRandomError struct{}

func (*testRandomError) Error() string { return "test random failure" }
