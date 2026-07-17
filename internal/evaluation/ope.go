package evaluation

import (
	"fmt"
	"math"
)

const MinimumEffectiveSampleSize = 10.0

const (
	ReasonNoSamples              = "no_samples"
	ReasonZeroCandidateWeight    = "zero_candidate_weight"
	ReasonLowEffectiveSampleSize = "low_effective_sample_size"
)

type Record struct {
	Reward               float64
	BehaviorPropensity   float64
	CandidateProbability float64
}

type Result struct {
	IPS                 *float64
	SNIPS               *float64
	SampleCount         int
	EffectiveSampleSize float64
	WeightSum           float64
	WeightSquaredSum    float64
	WeightedRewardSum   float64
	MinWeight           float64
	MaxWeight           float64
	Reason              string
}

func Evaluate(records []Record) (Result, error) {
	result := Result{SampleCount: len(records)}
	if len(records) == 0 {
		result.Reason = ReasonNoSamples
		return result, nil
	}

	var weightSum compensatedSum
	var weightSquaredSum compensatedSum
	var weightedRewardSum compensatedSum
	for index, record := range records {
		if err := validateRecord(record); err != nil {
			return Result{}, fmt.Errorf("record[%d]: %w", index, err)
		}

		weight := record.CandidateProbability / record.BehaviorPropensity
		if !finite(weight) {
			return Result{}, fmt.Errorf("record[%d]: importance weight is not finite", index)
		}
		weightSquared := weight * weight
		weightedReward := weight * record.Reward
		if !finite(weightSquared) || !finite(weightedReward) {
			return Result{}, fmt.Errorf("record[%d]: weighted value overflow", index)
		}

		if index == 0 || weight < result.MinWeight {
			result.MinWeight = weight
		}
		if index == 0 || weight > result.MaxWeight {
			result.MaxWeight = weight
		}
		if err := weightSum.Add(weight); err != nil {
			return Result{}, fmt.Errorf("record[%d]: weight sum: %w", index, err)
		}
		if err := weightSquaredSum.Add(weightSquared); err != nil {
			return Result{}, fmt.Errorf("record[%d]: squared-weight sum: %w", index, err)
		}
		if err := weightedRewardSum.Add(weightedReward); err != nil {
			return Result{}, fmt.Errorf("record[%d]: weighted-reward sum: %w", index, err)
		}
	}

	result.WeightSum = weightSum.Value()
	result.WeightSquaredSum = weightSquaredSum.Value()
	result.WeightedRewardSum = weightedRewardSum.Value()
	if result.WeightSum == 0 {
		result.Reason = ReasonZeroCandidateWeight
		return result, nil
	}
	if result.WeightSquaredSum <= 0 {
		return Result{}, fmt.Errorf("squared-weight sum must be positive when total weight is positive")
	}

	weightSumSquared := result.WeightSum * result.WeightSum
	if !finite(weightSumSquared) {
		return Result{}, fmt.Errorf("effective sample size numerator overflow")
	}
	result.EffectiveSampleSize = weightSumSquared / result.WeightSquaredSum
	if !finite(result.EffectiveSampleSize) || result.EffectiveSampleSize < 0 || result.EffectiveSampleSize > float64(len(records))+1e-9 {
		return Result{}, fmt.Errorf("effective sample size is invalid")
	}
	if result.EffectiveSampleSize < MinimumEffectiveSampleSize {
		result.Reason = ReasonLowEffectiveSampleSize
		return result, nil
	}

	ips := result.WeightedRewardSum / float64(len(records))
	snips := result.WeightedRewardSum / result.WeightSum
	if !finite(ips) || !finite(snips) {
		return Result{}, fmt.Errorf("evaluation estimate is not finite")
	}
	result.IPS = &ips
	result.SNIPS = &snips

	return result, nil
}

func validateRecord(record Record) error {
	if !finite(record.Reward) || record.Reward < 0 || record.Reward > 1 {
		return fmt.Errorf("reward must be finite and between zero and one")
	}
	if !finite(record.BehaviorPropensity) || record.BehaviorPropensity <= 0 || record.BehaviorPropensity > 1 {
		return fmt.Errorf("behavior propensity must be finite, positive, and at most one")
	}
	if !finite(record.CandidateProbability) || record.CandidateProbability < 0 || record.CandidateProbability > 1 {
		return fmt.Errorf("candidate probability must be finite and between zero and one")
	}

	return nil
}

type compensatedSum struct {
	sum        float64
	correction float64
}

func (sum *compensatedSum) Add(value float64) error {
	updated := sum.sum + value
	if !finite(updated) {
		return fmt.Errorf("aggregate is not finite")
	}
	if math.Abs(sum.sum) >= math.Abs(value) {
		sum.correction += (sum.sum - updated) + value
	} else {
		sum.correction += (value - updated) + sum.sum
	}
	if !finite(sum.correction) {
		return fmt.Errorf("aggregate correction is not finite")
	}
	sum.sum = updated
	return nil
}

func (sum compensatedSum) Value() float64 {
	return sum.sum + sum.correction
}

func finite(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}
