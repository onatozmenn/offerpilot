package bandit

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/rand"
	"strings"
	"sync"

	"github.com/google/uuid"
	"github.com/onatozmenn/offerpilot/internal/domain"
)

const SnapshotSchemaVersion = domain.PolicySnapshotSchemaVersion

type Policy interface {
	Kind() domain.PolicyKind
	Version() int64
	Select(SelectionInput) (Selection, error)
	Update(Update) error
	Snapshot() (Snapshot, error)
	Restore(Snapshot) error
}

type SelectionInput struct {
	ExperimentID uuid.UUID
	SegmentKey   string
	OfferIDs     []uuid.UUID
}

type Selection struct {
	SelectedOfferID uuid.UUID
	Distribution    []domain.ActionProbability
	PolicyKind      domain.PolicyKind
	PolicyVersion   int64
}

type Update struct {
	ExperimentID           uuid.UUID
	SegmentKey             string
	SelectedOfferID        uuid.UUID
	EligibleOfferIDs       []uuid.UUID
	SelectionPolicyVersion int64
	AppliedPolicyVersion   int64
	Reward                 float64
	PolicyKind             domain.PolicyKind
}

type Snapshot struct {
	SchemaVersion int
	ExperimentID  uuid.UUID
	PolicyKind    domain.PolicyKind
	PolicyVersion int64
	State         json.RawMessage
}

type RandomSource interface {
	Float64() (float64, error)
}

type LockedRandom struct {
	mu     sync.Mutex
	random *rand.Rand
}

func NewLockedRandom(seed int64) *LockedRandom {
	return &LockedRandom{random: rand.New(rand.NewSource(seed))}
}

func (source *LockedRandom) Float64() (float64, error) {
	if source == nil || source.random == nil {
		return 0, fmt.Errorf("random source is nil")
	}

	source.mu.Lock()
	defer source.mu.Unlock()

	return source.random.Float64(), nil
}

func canonicalSelectionInput(input SelectionInput, experimentID uuid.UUID) ([]uuid.UUID, error) {
	if input.ExperimentID == uuid.Nil || input.ExperimentID != experimentID {
		return nil, fmt.Errorf("selection experiment does not match policy")
	}
	if strings.TrimSpace(input.SegmentKey) == "" {
		return nil, fmt.Errorf("selection segment key must not be empty")
	}

	canonical, err := domain.CanonicalOfferIDs(input.OfferIDs)
	if err != nil {
		return nil, err
	}
	if len(canonical) < 2 {
		return nil, fmt.Errorf("selection requires at least two offers")
	}

	return canonical, nil
}

func validateUpdate(update Update, experimentID uuid.UUID, kind domain.PolicyKind, currentVersion int64) ([]uuid.UUID, error) {
	if update.ExperimentID == uuid.Nil || update.ExperimentID != experimentID {
		return nil, fmt.Errorf("update experiment does not match policy")
	}
	if update.PolicyKind != kind {
		return nil, fmt.Errorf("update policy kind does not match policy")
	}
	if strings.TrimSpace(update.SegmentKey) == "" {
		return nil, fmt.Errorf("update segment key must not be empty")
	}
	if update.SelectionPolicyVersion < 1 || update.SelectionPolicyVersion > currentVersion {
		return nil, fmt.Errorf("update selection policy version is invalid")
	}
	if update.AppliedPolicyVersion != currentVersion+1 {
		return nil, fmt.Errorf("update applied policy version must be consecutive")
	}
	if math.IsNaN(update.Reward) || math.IsInf(update.Reward, 0) || update.Reward < 0 || update.Reward > 1 {
		return nil, fmt.Errorf("update reward must be finite and between zero and one")
	}

	canonical, err := domain.CanonicalOfferIDs(update.EligibleOfferIDs)
	if err != nil {
		return nil, err
	}
	if len(canonical) < 2 {
		return nil, fmt.Errorf("update requires at least two eligible offers")
	}
	selectedFound := false
	for _, offerID := range canonical {
		if offerID == update.SelectedOfferID {
			selectedFound = true
			break
		}
	}
	if !selectedFound {
		return nil, fmt.Errorf("update selected offer is not eligible")
	}

	return canonical, nil
}

func sampleDistribution(source RandomSource, eligibleOfferIDs []uuid.UUID, distribution []domain.ActionProbability) (uuid.UUID, error) {
	if source == nil {
		return uuid.Nil, fmt.Errorf("random source is required")
	}
	if _, err := domain.ValidateDistribution(eligibleOfferIDs, distribution, eligibleOfferIDs[0]); err != nil {
		return uuid.Nil, fmt.Errorf("invalid distribution: %w", err)
	}

	draw, err := source.Float64()
	if err != nil {
		return uuid.Nil, fmt.Errorf("draw random value: %w", err)
	}
	if math.IsNaN(draw) || math.IsInf(draw, 0) || draw < 0 || draw >= 1 {
		return uuid.Nil, fmt.Errorf("random source returned a value outside [0,1)")
	}

	cumulative := 0.0
	for _, entry := range distribution {
		cumulative += entry.Probability
		if draw < cumulative {
			return entry.OfferID, nil
		}
	}

	return distribution[len(distribution)-1].OfferID, nil
}

func decodeSnapshotState(state json.RawMessage, destination any) error {
	if len(state) == 0 {
		return fmt.Errorf("snapshot state is empty")
	}

	decoder := json.NewDecoder(bytes.NewReader(state))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return fmt.Errorf("decode snapshot state: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return fmt.Errorf("snapshot state contains trailing data")
	}

	return nil
}

func cloneSnapshot(snapshot Snapshot) Snapshot {
	snapshot.State = append(json.RawMessage(nil), snapshot.State...)
	return snapshot
}
