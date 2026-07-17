package bandit

import (
	"errors"
	"math"
	"reflect"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/onatozmenn/offerpilot/internal/domain"
)

var (
	randomExperimentID = uuid.MustParse("10000000-0000-0000-0000-000000000000")
	randomOfferA       = uuid.MustParse("00000000-0000-0000-0000-000000000001")
	randomOfferB       = uuid.MustParse("00000000-0000-0000-0000-000000000002")
	randomOfferC       = uuid.MustParse("00000000-0000-0000-0000-000000000003")
	randomOfferD       = uuid.MustParse("00000000-0000-0000-0000-000000000004")
)

func TestRandomPolicy_UniformDistribution(t *testing.T) {
	tests := []struct {
		name     string
		offerIDs []uuid.UUID
	}{
		{name: "two actions", offerIDs: []uuid.UUID{randomOfferB, randomOfferA}},
		{name: "four actions", offerIDs: []uuid.UUID{randomOfferD, randomOfferB, randomOfferC, randomOfferA}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			policy := newTestRandomPolicy(t, 1, &sequenceRandom{values: []float64{0}})
			selection, err := policy.Select(SelectionInput{
				ExperimentID: randomExperimentID,
				SegmentKey:   "mobile|evening|travel|returning",
				OfferIDs:     test.offerIDs,
			})
			if err != nil {
				t.Fatalf("Select() error = %v", err)
			}

			canonical, err := domain.CanonicalOfferIDs(test.offerIDs)
			if err != nil {
				t.Fatalf("CanonicalOfferIDs() error = %v", err)
			}
			if selection.PolicyKind != domain.PolicyKindRandom || selection.PolicyVersion != 1 {
				t.Fatalf("Select() metadata = %#v", selection)
			}
			if selection.SelectedOfferID != canonical[0] {
				t.Fatalf("Select() selected = %s, want %s", selection.SelectedOfferID, canonical[0])
			}
			if len(selection.Distribution) != len(canonical) {
				t.Fatalf("Select() distribution length = %d, want %d", len(selection.Distribution), len(canonical))
			}
			wantProbability := 1 / float64(len(canonical))
			for index, entry := range selection.Distribution {
				if entry.OfferID != canonical[index] || entry.Probability != wantProbability {
					t.Fatalf("distribution[%d] = %#v, want offer %s probability %v", index, entry, canonical[index], wantProbability)
				}
			}
			if _, err := domain.ValidateDistribution(canonical, selection.Distribution, selection.SelectedOfferID); err != nil {
				t.Fatalf("ValidateDistribution() error = %v", err)
			}
		})
	}
}

func TestRandomPolicy_SeededSequence(t *testing.T) {
	first := newTestRandomPolicy(t, 1, NewLockedRandom(20260717))
	second := newTestRandomPolicy(t, 1, NewLockedRandom(20260717))
	input := SelectionInput{
		ExperimentID: randomExperimentID,
		SegmentKey:   "desktop|morning|home|new",
		OfferIDs:     []uuid.UUID{randomOfferD, randomOfferA, randomOfferC, randomOfferB},
	}

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
		t.Fatalf("identically seeded policies diverged")
	}
}

func TestRandomPolicy_InvalidConstructionAndSelection(t *testing.T) {
	if _, err := NewRandomPolicy(uuid.Nil, 1, NewLockedRandom(1)); err == nil {
		t.Fatal("NewRandomPolicy(nil experiment) error = nil")
	}
	if _, err := NewRandomPolicy(randomExperimentID, 0, NewLockedRandom(1)); err == nil {
		t.Fatal("NewRandomPolicy(version zero) error = nil")
	}
	if _, err := NewRandomPolicy(randomExperimentID, 1, nil); err == nil {
		t.Fatal("NewRandomPolicy(nil random) error = nil")
	}

	valid := SelectionInput{ExperimentID: randomExperimentID, SegmentKey: "segment", OfferIDs: []uuid.UUID{randomOfferA, randomOfferB}}
	tests := []struct {
		name   string
		input  SelectionInput
		random RandomSource
	}{
		{name: "wrong experiment", input: SelectionInput{ExperimentID: uuid.New(), SegmentKey: "segment", OfferIDs: valid.OfferIDs}, random: &sequenceRandom{values: []float64{0}}},
		{name: "empty segment", input: SelectionInput{ExperimentID: randomExperimentID, OfferIDs: valid.OfferIDs}, random: &sequenceRandom{values: []float64{0}}},
		{name: "one action", input: SelectionInput{ExperimentID: randomExperimentID, SegmentKey: "segment", OfferIDs: []uuid.UUID{randomOfferA}}, random: &sequenceRandom{values: []float64{0}}},
		{name: "duplicate actions", input: SelectionInput{ExperimentID: randomExperimentID, SegmentKey: "segment", OfferIDs: []uuid.UUID{randomOfferA, randomOfferA}}, random: &sequenceRandom{values: []float64{0}}},
		{name: "nil action", input: SelectionInput{ExperimentID: randomExperimentID, SegmentKey: "segment", OfferIDs: []uuid.UUID{randomOfferA, uuid.Nil}}, random: &sequenceRandom{values: []float64{0}}},
		{name: "random error", input: valid, random: &sequenceRandom{err: errors.New("draw failed")}},
		{name: "negative draw", input: valid, random: &sequenceRandom{values: []float64{-0.1}}},
		{name: "draw one", input: valid, random: &sequenceRandom{values: []float64{1}}},
		{name: "draw NaN", input: valid, random: &sequenceRandom{values: []float64{math.NaN()}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			policy := newTestRandomPolicy(t, 1, test.random)
			if _, err := policy.Select(test.input); err == nil {
				t.Fatal("Select() error = nil")
			}
		})
	}
}

func TestRandomPolicy_UpdateLifecycle(t *testing.T) {
	policy := newTestRandomPolicy(t, 3, NewLockedRandom(1))
	input := SelectionInput{ExperimentID: randomExperimentID, SegmentKey: "segment", OfferIDs: []uuid.UUID{randomOfferA, randomOfferB}}
	before, err := policy.Select(input)
	if err != nil {
		t.Fatalf("Select(before update) error = %v", err)
	}

	update := validRandomUpdate()
	update.SelectionPolicyVersion = 1
	update.AppliedPolicyVersion = 4
	if err := policy.Update(update); err != nil {
		t.Fatalf("Update(delayed decision) error = %v", err)
	}
	if policy.Version() != 4 {
		t.Fatalf("Version() = %d, want 4", policy.Version())
	}

	after, err := policy.Select(input)
	if err != nil {
		t.Fatalf("Select(after update) error = %v", err)
	}
	if !reflect.DeepEqual(before.Distribution, after.Distribution) {
		t.Fatalf("random distribution changed after update")
	}
	if after.PolicyVersion != 4 {
		t.Fatalf("Select() version = %d, want 4", after.PolicyVersion)
	}

	if err := policy.Update(update); err == nil {
		t.Fatal("duplicate Update() error = nil")
	}

	tests := []struct {
		name   string
		mutate func(*Update)
	}{
		{name: "wrong experiment", mutate: func(value *Update) { value.ExperimentID = uuid.New() }},
		{name: "wrong policy", mutate: func(value *Update) { value.PolicyKind = domain.PolicyKindSegmentedEpsilonGreedy }},
		{name: "empty segment", mutate: func(value *Update) { value.SegmentKey = "" }},
		{name: "future selection version", mutate: func(value *Update) { value.SelectionPolicyVersion = 5 }},
		{name: "nonconsecutive applied version", mutate: func(value *Update) { value.AppliedPolicyVersion = 6 }},
		{name: "negative reward", mutate: func(value *Update) { value.Reward = -0.1 }},
		{name: "reward above one", mutate: func(value *Update) { value.Reward = 1.1 }},
		{name: "reward NaN", mutate: func(value *Update) { value.Reward = math.NaN() }},
		{name: "unknown selected action", mutate: func(value *Update) { value.SelectedOfferID = randomOfferC }},
		{name: "one eligible action", mutate: func(value *Update) { value.EligibleOfferIDs = []uuid.UUID{randomOfferA} }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			value := validRandomUpdate()
			value.SelectionPolicyVersion = 4
			value.AppliedPolicyVersion = 5
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

func TestRandomPolicy_SnapshotRoundTrip(t *testing.T) {
	policy := newTestRandomPolicy(t, 1, NewLockedRandom(1))
	update := validRandomUpdate()
	if err := policy.Update(update); err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	snapshot, err := policy.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot() error = %v", err)
	}
	if snapshot.SchemaVersion != SnapshotSchemaVersion || snapshot.ExperimentID != randomExperimentID || snapshot.PolicyKind != domain.PolicyKindRandom || snapshot.PolicyVersion != 2 || string(snapshot.State) != "{}" {
		t.Fatalf("Snapshot() = %#v", snapshot)
	}

	restored := newTestRandomPolicy(t, 1, NewLockedRandom(1))
	if err := restored.Restore(snapshot); err != nil {
		t.Fatalf("Restore() error = %v", err)
	}
	if restored.Version() != 2 {
		t.Fatalf("restored Version() = %d, want 2", restored.Version())
	}

	snapshot.State[0] = '['
	secondSnapshot, err := restored.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot(after caller mutation) error = %v", err)
	}
	if string(secondSnapshot.State) != "{}" {
		t.Fatalf("snapshot state aliased caller data: %q", secondSnapshot.State)
	}
}

func TestRandomPolicy_SnapshotFailures(t *testing.T) {
	policy := newTestRandomPolicy(t, 2, NewLockedRandom(1))
	validSnapshot := Snapshot{SchemaVersion: SnapshotSchemaVersion, ExperimentID: randomExperimentID, PolicyKind: domain.PolicyKindRandom, PolicyVersion: 2, State: []byte(`{}`)}
	tests := []struct {
		name   string
		mutate func(*Snapshot)
	}{
		{name: "unknown schema", mutate: func(value *Snapshot) { value.SchemaVersion = 2 }},
		{name: "wrong experiment", mutate: func(value *Snapshot) { value.ExperimentID = uuid.New() }},
		{name: "wrong policy", mutate: func(value *Snapshot) { value.PolicyKind = domain.PolicyKindSegmentedEpsilonGreedy }},
		{name: "version zero", mutate: func(value *Snapshot) { value.PolicyVersion = 0 }},
		{name: "version regression", mutate: func(value *Snapshot) { value.PolicyVersion = 1 }},
		{name: "empty state", mutate: func(value *Snapshot) { value.State = nil }},
		{name: "unknown state field", mutate: func(value *Snapshot) { value.State = []byte(`{"unexpected":true}`) }},
		{name: "trailing state", mutate: func(value *Snapshot) { value.State = []byte(`{} {}`) }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			value := cloneSnapshot(validSnapshot)
			test.mutate(&value)
			if err := policy.Restore(value); err == nil {
				t.Fatal("Restore() error = nil")
			}
			if policy.Version() != 2 {
				t.Fatalf("invalid restore changed version to %d", policy.Version())
			}
		})
	}
}

func TestRandomPolicy_DistributionTolerance(t *testing.T) {
	eligible := []uuid.UUID{randomOfferA, randomOfferB}
	inside := math.Nextafter(1+domain.ProbabilityTolerance, 1)
	outside := math.Nextafter(1+domain.ProbabilityTolerance, math.Inf(1))
	insideDistribution := []domain.ActionProbability{{OfferID: randomOfferA, Probability: 0.5}, {OfferID: randomOfferB, Probability: inside - 0.5}}
	if _, err := domain.ValidateDistribution(eligible, insideDistribution, randomOfferA); err != nil {
		t.Fatalf("ValidateDistribution(inside tolerance) error = %v", err)
	}
	outsideDistribution := []domain.ActionProbability{{OfferID: randomOfferA, Probability: 0.5}, {OfferID: randomOfferB, Probability: outside - 0.5}}
	if _, err := domain.ValidateDistribution(eligible, outsideDistribution, randomOfferA); err == nil {
		t.Fatal("ValidateDistribution(outside tolerance) error = nil")
	}
}

func TestRandomPolicy_ConcurrentSelections(t *testing.T) {
	policy := newTestRandomPolicy(t, 1, NewLockedRandom(20260717))
	input := SelectionInput{ExperimentID: randomExperimentID, SegmentKey: "segment", OfferIDs: []uuid.UUID{randomOfferA, randomOfferB, randomOfferC, randomOfferD}}

	const goroutines = 16
	const selectionsPerGoroutine = 100
	errorsChannel := make(chan error, goroutines*selectionsPerGoroutine)
	var waitGroup sync.WaitGroup
	for range goroutines {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			for range selectionsPerGoroutine {
				selection, err := policy.Select(input)
				if err != nil {
					errorsChannel <- err
					continue
				}
				if _, err := domain.ValidateDistribution(input.OfferIDs, selection.Distribution, selection.SelectedOfferID); err != nil {
					errorsChannel <- err
				}
			}
		}()
	}
	waitGroup.Wait()
	close(errorsChannel)
	for err := range errorsChannel {
		t.Errorf("concurrent Select() error = %v", err)
	}
}

func newTestRandomPolicy(t *testing.T, version int64, random RandomSource) *RandomPolicy {
	t.Helper()
	policy, err := NewRandomPolicy(randomExperimentID, version, random)
	if err != nil {
		t.Fatalf("NewRandomPolicy() error = %v", err)
	}
	return policy
}

func validRandomUpdate() Update {
	return Update{
		ExperimentID:           randomExperimentID,
		SegmentKey:             "mobile|evening|travel|returning",
		SelectedOfferID:        randomOfferA,
		EligibleOfferIDs:       []uuid.UUID{randomOfferA, randomOfferB},
		SelectionPolicyVersion: 1,
		AppliedPolicyVersion:   2,
		Reward:                 0.25,
		PolicyKind:             domain.PolicyKindRandom,
	}
}

type sequenceRandom struct {
	mu     sync.Mutex
	values []float64
	index  int
	err    error
}

func (random *sequenceRandom) Float64() (float64, error) {
	random.mu.Lock()
	defer random.mu.Unlock()

	if random.err != nil {
		return 0, random.err
	}
	if len(random.values) == 0 {
		return 0, errors.New("no random values")
	}
	value := random.values[random.index%len(random.values)]
	random.index++
	return value, nil
}
