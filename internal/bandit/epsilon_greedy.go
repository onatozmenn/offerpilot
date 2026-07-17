package bandit

import (
	"encoding/json"
	"fmt"
	"math"
	"slices"
	"sort"
	"strings"
	"sync"

	"github.com/google/uuid"
	"github.com/onatozmenn/offerpilot/internal/domain"
)

const (
	DefaultPriorCount     = 2.0
	DefaultPriorRewardSum = 1.0
)

type EpsilonGreedyPolicy struct {
	mu             sync.RWMutex
	experimentID   uuid.UUID
	epsilon        float64
	priorCount     float64
	priorRewardSum float64
	version        int64
	random         RandomSource
	segments       map[string]map[uuid.UUID]armStatistic
}

type armStatistic struct {
	Count     float64
	RewardSum float64
}

type epsilonSnapshotState struct {
	Epsilon        float64           `json:"epsilon"`
	PriorCount     float64           `json:"prior_count"`
	PriorRewardSum float64           `json:"prior_reward_sum"`
	Segments       []snapshotSegment `json:"segments"`
}

type snapshotSegment struct {
	Key    string        `json:"key"`
	Offers []snapshotArm `json:"offers"`
}

type snapshotArm struct {
	OfferID   uuid.UUID `json:"offer_id"`
	Count     float64   `json:"count"`
	RewardSum float64   `json:"reward_sum"`
}

func NewEpsilonGreedyPolicy(
	experimentID uuid.UUID,
	epsilon float64,
	priorCount float64,
	priorRewardSum float64,
	initialVersion int64,
	random RandomSource,
) (*EpsilonGreedyPolicy, error) {
	if experimentID == uuid.Nil {
		return nil, fmt.Errorf("experiment id must not be nil")
	}
	if err := domain.ValidateEpsilon(domain.PolicyKindSegmentedEpsilonGreedy, &epsilon); err != nil {
		return nil, err
	}
	if !finiteBandit(priorCount) || priorCount <= 0 {
		return nil, fmt.Errorf("prior count must be finite and positive")
	}
	if !finiteBandit(priorRewardSum) || priorRewardSum < 0 || priorRewardSum > priorCount {
		return nil, fmt.Errorf("prior reward sum must be finite and between zero and prior count")
	}
	if initialVersion < 1 {
		return nil, fmt.Errorf("initial version must be at least one")
	}
	if random == nil {
		return nil, fmt.Errorf("random source is required")
	}

	return &EpsilonGreedyPolicy{
		experimentID:   experimentID,
		epsilon:        epsilon,
		priorCount:     priorCount,
		priorRewardSum: priorRewardSum,
		version:        initialVersion,
		random:         random,
		segments:       make(map[string]map[uuid.UUID]armStatistic),
	}, nil
}

func (policy *EpsilonGreedyPolicy) Kind() domain.PolicyKind {
	return domain.PolicyKindSegmentedEpsilonGreedy
}

func (policy *EpsilonGreedyPolicy) Version() int64 {
	policy.mu.RLock()
	defer policy.mu.RUnlock()

	return policy.version
}

func (policy *EpsilonGreedyPolicy) View() PolicyView {
	policy.mu.RLock()
	defer policy.mu.RUnlock()

	segmentKeys := make([]string, 0, len(policy.segments))
	for segmentKey := range policy.segments {
		segmentKeys = append(segmentKeys, segmentKey)
	}
	sort.Strings(segmentKeys)

	arms := make([]ArmView, 0)
	for _, segmentKey := range segmentKeys {
		statistics := policy.segments[segmentKey]
		offerIDs := make([]uuid.UUID, 0, len(statistics))
		for offerID := range statistics {
			offerIDs = append(offerIDs, offerID)
		}
		sort.Slice(offerIDs, func(left, right int) bool {
			return offerIDs[left].String() < offerIDs[right].String()
		})
		for _, offerID := range offerIDs {
			statistic := statistics[offerID]
			arms = append(arms, ArmView{
				SegmentKey: segmentKey,
				OfferID:    offerID,
				Count:      statistic.Count,
				RewardSum:  statistic.RewardSum,
				Mean:       statistic.RewardSum / statistic.Count,
			})
		}
	}

	epsilon := policy.epsilon
	return PolicyView{
		Kind:    domain.PolicyKindSegmentedEpsilonGreedy,
		Version: policy.version,
		Epsilon: &epsilon,
		Arms:    arms,
	}
}

func (policy *EpsilonGreedyPolicy) Select(input SelectionInput) (Selection, error) {
	policy.mu.Lock()
	defer policy.mu.Unlock()

	canonical, err := canonicalSelectionInput(input, policy.experimentID)
	if err != nil {
		return Selection{}, err
	}

	statistics, exists := policy.segments[input.SegmentKey]
	if !exists {
		statistics = make(map[uuid.UUID]armStatistic, len(canonical))
		policy.segments[input.SegmentKey] = statistics
	}
	for _, offerID := range canonical {
		if _, exists := statistics[offerID]; !exists {
			statistics[offerID] = armStatistic{Count: policy.priorCount, RewardSum: policy.priorRewardSum}
		}
	}

	bestMean := math.Inf(-1)
	bestCount := 0
	means := make([]float64, len(canonical))
	for index, offerID := range canonical {
		statistic := statistics[offerID]
		if err := validateArmStatistic(statistic, policy.priorCount, policy.priorRewardSum); err != nil {
			return Selection{}, fmt.Errorf("segment state for offer %s: %w", offerID, err)
		}
		mean := statistic.RewardSum / statistic.Count
		means[index] = mean
		switch {
		case mean > bestMean:
			bestMean = mean
			bestCount = 1
		case mean == bestMean:
			bestCount++
		}
	}

	explorationProbability := policy.epsilon / float64(len(canonical))
	exploitationProbability := (1 - policy.epsilon) / float64(bestCount)
	distribution := make([]domain.ActionProbability, len(canonical))
	for index, offerID := range canonical {
		probability := explorationProbability
		if means[index] == bestMean {
			probability += exploitationProbability
		}
		distribution[index] = domain.ActionProbability{OfferID: offerID, Probability: probability}
	}

	selectedOfferID, err := sampleDistribution(policy.random, canonical, distribution)
	if err != nil {
		return Selection{}, err
	}

	return Selection{
		SelectedOfferID: selectedOfferID,
		Distribution:    distribution,
		PolicyKind:      domain.PolicyKindSegmentedEpsilonGreedy,
		PolicyVersion:   policy.version,
	}, nil
}

func (policy *EpsilonGreedyPolicy) Update(update Update) error {
	policy.mu.Lock()
	defer policy.mu.Unlock()

	if _, err := validateUpdate(update, policy.experimentID, domain.PolicyKindSegmentedEpsilonGreedy, policy.version); err != nil {
		return err
	}
	statistics, exists := policy.segments[update.SegmentKey]
	if !exists {
		return fmt.Errorf("update segment is unknown")
	}
	statistic, exists := statistics[update.SelectedOfferID]
	if !exists {
		return fmt.Errorf("update selected action is unknown")
	}
	if err := validateArmStatistic(statistic, policy.priorCount, policy.priorRewardSum); err != nil {
		return fmt.Errorf("selected action state is invalid: %w", err)
	}

	nextStatistic := armStatistic{
		Count:     statistic.Count + 1,
		RewardSum: statistic.RewardSum + update.Reward,
	}
	if err := validateArmStatistic(nextStatistic, policy.priorCount, policy.priorRewardSum); err != nil {
		return fmt.Errorf("updated action state is invalid: %w", err)
	}

	statistics[update.SelectedOfferID] = nextStatistic
	policy.version = update.AppliedPolicyVersion
	return nil
}

func (policy *EpsilonGreedyPolicy) Snapshot() (Snapshot, error) {
	policy.mu.RLock()
	defer policy.mu.RUnlock()

	segmentKeys := make([]string, 0, len(policy.segments))
	for segmentKey := range policy.segments {
		segmentKeys = append(segmentKeys, segmentKey)
	}
	sort.Strings(segmentKeys)

	state := epsilonSnapshotState{
		Epsilon:        policy.epsilon,
		PriorCount:     policy.priorCount,
		PriorRewardSum: policy.priorRewardSum,
		Segments:       make([]snapshotSegment, 0, len(segmentKeys)),
	}
	for _, segmentKey := range segmentKeys {
		statistics := policy.segments[segmentKey]
		offerIDs := make([]uuid.UUID, 0, len(statistics))
		for offerID := range statistics {
			offerIDs = append(offerIDs, offerID)
		}
		canonical, err := domain.CanonicalOfferIDs(offerIDs)
		if err != nil {
			return Snapshot{}, fmt.Errorf("snapshot segment %q: %w", segmentKey, err)
		}

		segment := snapshotSegment{Key: segmentKey, Offers: make([]snapshotArm, len(canonical))}
		for index, offerID := range canonical {
			statistic := statistics[offerID]
			if err := validateArmStatistic(statistic, policy.priorCount, policy.priorRewardSum); err != nil {
				return Snapshot{}, fmt.Errorf("snapshot segment %q offer %s: %w", segmentKey, offerID, err)
			}
			segment.Offers[index] = snapshotArm{OfferID: offerID, Count: statistic.Count, RewardSum: statistic.RewardSum}
		}
		state.Segments = append(state.Segments, segment)
	}

	encoded, err := json.Marshal(state)
	if err != nil {
		return Snapshot{}, fmt.Errorf("encode epsilon-greedy snapshot: %w", err)
	}

	return Snapshot{
		SchemaVersion: SnapshotSchemaVersion,
		ExperimentID:  policy.experimentID,
		PolicyKind:    domain.PolicyKindSegmentedEpsilonGreedy,
		PolicyVersion: policy.version,
		State:         encoded,
	}, nil
}

func (policy *EpsilonGreedyPolicy) Restore(snapshot Snapshot) error {
	if snapshot.SchemaVersion != SnapshotSchemaVersion {
		return fmt.Errorf("snapshot schema version is unsupported")
	}
	if snapshot.ExperimentID != policy.experimentID {
		return fmt.Errorf("snapshot experiment does not match policy")
	}
	if snapshot.PolicyKind != domain.PolicyKindSegmentedEpsilonGreedy {
		return fmt.Errorf("snapshot policy kind does not match epsilon-greedy policy")
	}
	if snapshot.PolicyVersion < 1 {
		return fmt.Errorf("snapshot policy version must be at least one")
	}

	var state epsilonSnapshotState
	if err := decodeSnapshotState(snapshot.State, &state); err != nil {
		return err
	}
	if state.Epsilon != policy.epsilon || state.PriorCount != policy.priorCount || state.PriorRewardSum != policy.priorRewardSum {
		return fmt.Errorf("snapshot configuration does not match policy")
	}

	restoredSegments := make(map[string]map[uuid.UUID]armStatistic, len(state.Segments))
	previousSegmentKey := ""
	for segmentIndex, segment := range state.Segments {
		if strings.TrimSpace(segment.Key) == "" {
			return fmt.Errorf("snapshot segment[%d] key is empty", segmentIndex)
		}
		if segmentIndex > 0 && segment.Key <= previousSegmentKey {
			return fmt.Errorf("snapshot segments must be uniquely sorted")
		}
		if len(segment.Offers) < 2 {
			return fmt.Errorf("snapshot segment[%d] must contain at least two offers", segmentIndex)
		}

		statistics := make(map[uuid.UUID]armStatistic, len(segment.Offers))
		offerIDs := make([]uuid.UUID, len(segment.Offers))
		for offerIndex, arm := range segment.Offers {
			if arm.OfferID == uuid.Nil {
				return fmt.Errorf("snapshot segment[%d] offer[%d] id is nil", segmentIndex, offerIndex)
			}
			statistic := armStatistic{Count: arm.Count, RewardSum: arm.RewardSum}
			if err := validateArmStatistic(statistic, policy.priorCount, policy.priorRewardSum); err != nil {
				return fmt.Errorf("snapshot segment[%d] offer[%d]: %w", segmentIndex, offerIndex, err)
			}
			statistics[arm.OfferID] = statistic
			offerIDs[offerIndex] = arm.OfferID
		}
		canonical, err := domain.CanonicalOfferIDs(offerIDs)
		if err != nil {
			return fmt.Errorf("snapshot segment[%d]: %w", segmentIndex, err)
		}
		if !slices.Equal(offerIDs, canonical) {
			return fmt.Errorf("snapshot segment[%d] offers must use canonical order", segmentIndex)
		}

		restoredSegments[segment.Key] = statistics
		previousSegmentKey = segment.Key
	}

	policy.mu.Lock()
	defer policy.mu.Unlock()
	if snapshot.PolicyVersion < policy.version {
		return fmt.Errorf("snapshot policy version must not regress")
	}
	policy.version = snapshot.PolicyVersion
	policy.segments = restoredSegments

	return nil
}

func validateArmStatistic(statistic armStatistic, priorCount, priorRewardSum float64) error {
	if !finiteBandit(statistic.Count) || statistic.Count < priorCount {
		return fmt.Errorf("count must be finite and at least the prior count")
	}
	if !finiteBandit(statistic.RewardSum) || statistic.RewardSum < priorRewardSum || statistic.RewardSum > priorRewardSum+(statistic.Count-priorCount) {
		return fmt.Errorf("reward sum is inconsistent with count and priors")
	}
	if statistic.Count-priorCount != math.Trunc(statistic.Count-priorCount) {
		return fmt.Errorf("count must advance in whole observations")
	}

	return nil
}

func finiteBandit(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}

var _ Policy = (*EpsilonGreedyPolicy)(nil)
