package bandit

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/google/uuid"
	"github.com/onatozmenn/offerpilot/internal/domain"
)

type RandomPolicy struct {
	mu           sync.RWMutex
	experimentID uuid.UUID
	version      int64
	random       RandomSource
}

func NewRandomPolicy(experimentID uuid.UUID, initialVersion int64, random RandomSource) (*RandomPolicy, error) {
	if experimentID == uuid.Nil {
		return nil, fmt.Errorf("experiment id must not be nil")
	}
	if initialVersion < 1 {
		return nil, fmt.Errorf("initial version must be at least one")
	}
	if random == nil {
		return nil, fmt.Errorf("random source is required")
	}

	return &RandomPolicy{
		experimentID: experimentID,
		version:      initialVersion,
		random:       random,
	}, nil
}

func (policy *RandomPolicy) Kind() domain.PolicyKind {
	return domain.PolicyKindRandom
}

func (policy *RandomPolicy) Version() int64 {
	policy.mu.RLock()
	defer policy.mu.RUnlock()

	return policy.version
}

func (policy *RandomPolicy) Select(input SelectionInput) (Selection, error) {
	policy.mu.RLock()
	defer policy.mu.RUnlock()

	canonical, err := canonicalSelectionInput(input, policy.experimentID)
	if err != nil {
		return Selection{}, err
	}

	probability := 1 / float64(len(canonical))
	distribution := make([]domain.ActionProbability, len(canonical))
	for index, offerID := range canonical {
		distribution[index] = domain.ActionProbability{OfferID: offerID, Probability: probability}
	}

	selectedOfferID, err := sampleDistribution(policy.random, canonical, distribution)
	if err != nil {
		return Selection{}, err
	}

	return Selection{
		SelectedOfferID: selectedOfferID,
		Distribution:    distribution,
		PolicyKind:      domain.PolicyKindRandom,
		PolicyVersion:   policy.version,
	}, nil
}

func (policy *RandomPolicy) Update(update Update) error {
	policy.mu.Lock()
	defer policy.mu.Unlock()

	if _, err := validateUpdate(update, policy.experimentID, domain.PolicyKindRandom, policy.version); err != nil {
		return err
	}

	policy.version = update.AppliedPolicyVersion
	return nil
}

func (policy *RandomPolicy) Snapshot() (Snapshot, error) {
	policy.mu.RLock()
	defer policy.mu.RUnlock()

	state, err := json.Marshal(struct{}{})
	if err != nil {
		return Snapshot{}, fmt.Errorf("encode random snapshot: %w", err)
	}

	return Snapshot{
		SchemaVersion: SnapshotSchemaVersion,
		ExperimentID:  policy.experimentID,
		PolicyKind:    domain.PolicyKindRandom,
		PolicyVersion: policy.version,
		State:         state,
	}, nil
}

func (policy *RandomPolicy) Restore(snapshot Snapshot) error {
	if snapshot.SchemaVersion != SnapshotSchemaVersion {
		return fmt.Errorf("snapshot schema version is unsupported")
	}
	if snapshot.ExperimentID != policy.experimentID {
		return fmt.Errorf("snapshot experiment does not match policy")
	}
	if snapshot.PolicyKind != domain.PolicyKindRandom {
		return fmt.Errorf("snapshot policy kind does not match random policy")
	}
	if snapshot.PolicyVersion < 1 {
		return fmt.Errorf("snapshot policy version must be at least one")
	}
	if err := decodeSnapshotState(snapshot.State, &struct{}{}); err != nil {
		return err
	}

	policy.mu.Lock()
	defer policy.mu.Unlock()
	if snapshot.PolicyVersion < policy.version {
		return fmt.Errorf("snapshot policy version must not regress")
	}
	policy.version = snapshot.PolicyVersion

	return nil
}

var _ Policy = (*RandomPolicy)(nil)
