package service

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/google/uuid"
	"github.com/onatozmenn/offerpilot/internal/bandit"
	"github.com/onatozmenn/offerpilot/internal/domain"
)

type RecoveryResult struct {
	ExperimentsRecovered int
	OutcomesReplayed     int
	SnapshotsSaved       int
	InterruptedRuns      int64
}

func (engine *Engine) Recover(ctx context.Context) (RecoveryResult, error) {
	var result RecoveryResult
	reconciled, err := engine.store.ReconcileInterruptedRuns(ctx, engine.clock.Now().UTC())
	if err != nil {
		return RecoveryResult{}, fmt.Errorf("reconcile interrupted simulations: %w", err)
	}
	result.InterruptedRuns = reconciled

	experiments, err := engine.store.ListActiveExperiments(ctx)
	if err != nil {
		return RecoveryResult{}, fmt.Errorf("list active experiments for recovery: %w", err)
	}
	recoveredPolicies := make(map[uuid.UUID]bandit.Policy, len(experiments))
	for _, experiment := range experiments {
		policy, replayed, saved, err := engine.recoverExperiment(ctx, experiment)
		if err != nil {
			return RecoveryResult{}, fmt.Errorf("recover experiment %s: %w", experiment.ID, err)
		}
		recoveredPolicies[experiment.ID] = policy
		result.ExperimentsRecovered++
		result.OutcomesReplayed += replayed
		result.SnapshotsSaved += saved
	}

	engine.mu.Lock()
	engine.policies = recoveredPolicies
	engine.unhealthy = make(map[uuid.UUID]error)
	engine.updateLocks = make(map[uuid.UUID]*sync.Mutex, len(recoveredPolicies))
	for experimentID := range recoveredPolicies {
		engine.updateLocks[experimentID] = &sync.Mutex{}
	}
	engine.mu.Unlock()

	return result, nil
}

func (engine *Engine) recoverExperiment(
	ctx context.Context,
	experiment domain.Experiment,
) (bandit.Policy, int, int, error) {
	policy, err := engine.policyFactory.NewPolicy(experiment)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("create policy: %w", err)
	}
	if policy.Kind() != experiment.PolicyKind {
		return nil, 0, 0, fmt.Errorf("policy kind does not match experiment")
	}

	snapshot, err := engine.store.GetLatestPolicySnapshot(ctx, experiment.ID)
	snapshotMissing := errors.Is(err, ErrNotFound)
	if err != nil && !snapshotMissing {
		return nil, 0, 0, fmt.Errorf("load latest snapshot: %w", err)
	}
	snapshotsSaved := 0
	if snapshotMissing {
		if experiment.PolicyVersion != 1 || policy.Version() != 1 {
			return nil, 0, 0, fmt.Errorf("missing snapshot for non-new experiment")
		}
		initialSnapshot, err := policy.Snapshot()
		if err != nil {
			return nil, 0, 0, fmt.Errorf("snapshot new policy: %w", err)
		}
		domainInitialSnapshot, err := domainSnapshot(initialSnapshot, engine.clock.Now().UTC())
		if err != nil {
			return nil, 0, 0, err
		}
		if err := engine.store.SavePolicySnapshot(ctx, domainInitialSnapshot); err != nil {
			return nil, 0, 0, fmt.Errorf("save initial snapshot: %w", err)
		}
		snapshot = domainInitialSnapshot
		snapshotsSaved++
	} else {
		if snapshot.PolicyVersion > experiment.PolicyVersion {
			return nil, 0, 0, fmt.Errorf("snapshot version exceeds experiment version")
		}
		if err := policy.Restore(banditSnapshot(snapshot)); err != nil {
			return nil, 0, 0, fmt.Errorf("restore policy snapshot: %w", err)
		}
	}

	records, err := engine.store.ListDecisionOutcomesAfterVersion(ctx, experiment.ID, snapshot.PolicyVersion)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("load unapplied outcomes: %w", err)
	}
	expectedVersion := snapshot.PolicyVersion + 1
	for _, record := range records {
		if record.Outcome.AppliedPolicyVersion != expectedVersion {
			return nil, 0, 0, fmt.Errorf(
				"applied policy version gap: got %d, want %d",
				record.Outcome.AppliedPolicyVersion,
				expectedVersion,
			)
		}
		update := bandit.Update{
			ExperimentID:           experiment.ID,
			SegmentKey:             record.Decision.SegmentKey,
			SelectedOfferID:        record.Decision.SelectedOfferID,
			EligibleOfferIDs:       append([]uuid.UUID(nil), record.Decision.EligibleOfferIDs...),
			SelectionPolicyVersion: record.Decision.PolicyVersion,
			AppliedPolicyVersion:   record.Outcome.AppliedPolicyVersion,
			Reward:                 record.Outcome.Reward,
			PolicyKind:             record.Decision.PolicyKind,
		}
		if err := policy.Update(update); err != nil {
			return nil, 0, 0, fmt.Errorf("replay outcome version %d: %w", expectedVersion, err)
		}
		expectedVersion++
	}
	if policy.Version() != experiment.PolicyVersion {
		return nil, 0, 0, fmt.Errorf(
			"recovered policy version %d does not match experiment version %d",
			policy.Version(),
			experiment.PolicyVersion,
		)
	}
	if len(records) > 0 {
		checkpoint, err := policy.Snapshot()
		if err != nil {
			return nil, 0, 0, fmt.Errorf("snapshot recovered policy: %w", err)
		}
		domainCheckpoint, err := domainSnapshot(checkpoint, engine.clock.Now().UTC())
		if err != nil {
			return nil, 0, 0, err
		}
		if err := engine.store.SavePolicySnapshot(ctx, domainCheckpoint); err != nil {
			return nil, 0, 0, fmt.Errorf("save recovered checkpoint: %w", err)
		}
		snapshotsSaved++
	}

	return policy, len(records), snapshotsSaved, nil
}
