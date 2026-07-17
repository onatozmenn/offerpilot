package evaluation

import (
	"math"
	"strings"
	"testing"
)

func TestEvaluate_OnPolicyEquality(t *testing.T) {
	records := make([]Record, 10)
	for index := range records {
		records[index] = Record{
			Reward:               0.5,
			BehaviorPropensity:   0.25,
			CandidateProbability: 0.25,
		}
	}

	result, err := Evaluate(records)
	if err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}
	assertEstimate(t, result, 0.5, 0.5)
	assertClose(t, "effective sample size", result.EffectiveSampleSize, 10, 1e-12)
	assertClose(t, "weight sum", result.WeightSum, 10, 1e-12)
	assertClose(t, "weight squared sum", result.WeightSquaredSum, 10, 1e-12)
	assertClose(t, "weighted reward sum", result.WeightedRewardSum, 5, 1e-12)
	assertClose(t, "min weight", result.MinWeight, 1, 1e-12)
	assertClose(t, "max weight", result.MaxWeight, 1, 1e-12)
	if result.SampleCount != 10 || result.Reason != "" {
		t.Fatalf("Evaluate() metadata = %#v", result)
	}
}

func TestEvaluate_NonUniformWeights(t *testing.T) {
	// Ten weight-2 rewarded records and ten weight-0.5 unrewarded records:
	// sum(w)=25, sum(w^2)=42.5, sum(w*r)=20, ESS=625/42.5.
	records := make([]Record, 0, 20)
	for range 10 {
		records = append(records, Record{Reward: 1, BehaviorPropensity: 0.5, CandidateProbability: 1})
	}
	for range 10 {
		records = append(records, Record{Reward: 0, BehaviorPropensity: 1, CandidateProbability: 0.5})
	}

	result, err := Evaluate(records)
	if err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}
	assertEstimate(t, result, 1, 0.8)
	assertClose(t, "effective sample size", result.EffectiveSampleSize, 625.0/42.5, 1e-12)
	assertClose(t, "weight sum", result.WeightSum, 25, 1e-12)
	assertClose(t, "weight squared sum", result.WeightSquaredSum, 42.5, 1e-12)
	assertClose(t, "weighted reward sum", result.WeightedRewardSum, 20, 1e-12)
	assertClose(t, "min weight", result.MinWeight, 0.5, 1e-12)
	assertClose(t, "max weight", result.MaxWeight, 2, 1e-12)
}

func TestEvaluate_InsufficientDataReasons(t *testing.T) {
	t.Run("no samples", func(t *testing.T) {
		result, err := Evaluate(nil)
		if err != nil {
			t.Fatalf("Evaluate() error = %v", err)
		}
		assertUnavailable(t, result, ReasonNoSamples)
		if result.SampleCount != 0 {
			t.Fatalf("SampleCount = %d, want 0", result.SampleCount)
		}
	})

	t.Run("zero candidate weight", func(t *testing.T) {
		records := make([]Record, 10)
		for index := range records {
			records[index] = Record{Reward: 1, BehaviorPropensity: 0.5, CandidateProbability: 0}
		}
		result, err := Evaluate(records)
		if err != nil {
			t.Fatalf("Evaluate() error = %v", err)
		}
		assertUnavailable(t, result, ReasonZeroCandidateWeight)
		if result.WeightSum != 0 || result.WeightSquaredSum != 0 || result.EffectiveSampleSize != 0 {
			t.Fatalf("zero-weight diagnostics = %#v", result)
		}
	})

	t.Run("effective sample immediately below threshold", func(t *testing.T) {
		// Nine weights of 1 and one weight of 2: ESS=121/13=9.307692...
		records := make([]Record, 10)
		for index := range records {
			records[index] = Record{Reward: 0.5, BehaviorPropensity: 1, CandidateProbability: 1}
		}
		records[9].BehaviorPropensity = 0.5

		result, err := Evaluate(records)
		if err != nil {
			t.Fatalf("Evaluate() error = %v", err)
		}
		assertUnavailable(t, result, ReasonLowEffectiveSampleSize)
		assertClose(t, "effective sample size", result.EffectiveSampleSize, 121.0/13.0, 1e-12)
		if result.EffectiveSampleSize >= MinimumEffectiveSampleSize {
			t.Fatalf("EffectiveSampleSize = %v, want below %v", result.EffectiveSampleSize, MinimumEffectiveSampleSize)
		}
	})
}

func TestEvaluate_InvalidRecords(t *testing.T) {
	valid := Record{Reward: 0.5, BehaviorPropensity: 0.5, CandidateProbability: 0.5}
	tests := []struct {
		name      string
		fieldName string
		mutate    func(*Record)
	}{
		{name: "negative reward", fieldName: "reward", mutate: func(value *Record) { value.Reward = -0.1 }},
		{name: "reward above one", fieldName: "reward", mutate: func(value *Record) { value.Reward = 1.1 }},
		{name: "reward NaN", fieldName: "reward", mutate: func(value *Record) { value.Reward = math.NaN() }},
		{name: "reward infinity", fieldName: "reward", mutate: func(value *Record) { value.Reward = math.Inf(1) }},
		{name: "zero behavior propensity", fieldName: "behavior propensity", mutate: func(value *Record) { value.BehaviorPropensity = 0 }},
		{name: "negative behavior propensity", fieldName: "behavior propensity", mutate: func(value *Record) { value.BehaviorPropensity = -0.1 }},
		{name: "behavior propensity above one", fieldName: "behavior propensity", mutate: func(value *Record) { value.BehaviorPropensity = 1.1 }},
		{name: "behavior propensity NaN", fieldName: "behavior propensity", mutate: func(value *Record) { value.BehaviorPropensity = math.NaN() }},
		{name: "behavior propensity infinity", fieldName: "behavior propensity", mutate: func(value *Record) { value.BehaviorPropensity = math.Inf(1) }},
		{name: "negative candidate probability", fieldName: "candidate probability", mutate: func(value *Record) { value.CandidateProbability = -0.1 }},
		{name: "candidate probability above one", fieldName: "candidate probability", mutate: func(value *Record) { value.CandidateProbability = 1.1 }},
		{name: "candidate probability NaN", fieldName: "candidate probability", mutate: func(value *Record) { value.CandidateProbability = math.NaN() }},
		{name: "candidate probability infinity", fieldName: "candidate probability", mutate: func(value *Record) { value.CandidateProbability = math.Inf(1) }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			record := valid
			test.mutate(&record)
			_, err := Evaluate([]Record{record})
			if err == nil {
				t.Fatal("Evaluate() error = nil")
			}
			if !strings.Contains(err.Error(), "record[0]") || !strings.Contains(err.Error(), test.fieldName) {
				t.Fatalf("Evaluate() error = %q, want index and %q", err, test.fieldName)
			}
		})
	}
}

func TestEvaluate_WeightOverflow(t *testing.T) {
	tests := []struct {
		name               string
		behaviorPropensity float64
		wantFragment       string
	}{
		{name: "weight division overflow", behaviorPropensity: math.SmallestNonzeroFloat64, wantFragment: "importance weight"},
		{name: "squared weight overflow", behaviorPropensity: 1e-200, wantFragment: "weighted value overflow"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := Evaluate([]Record{{Reward: 1, BehaviorPropensity: test.behaviorPropensity, CandidateProbability: 1}})
			if err == nil {
				t.Fatal("Evaluate() error = nil")
			}
			if !strings.Contains(err.Error(), test.wantFragment) {
				t.Fatalf("Evaluate() error = %q, want %q", err, test.wantFragment)
			}
		})
	}
}

func TestEvaluate_DeterministicCompensatedAccumulation(t *testing.T) {
	records := make([]Record, 1000)
	for index := range records {
		records[index] = Record{
			Reward:               float64(index%5) / 4,
			BehaviorPropensity:   0.25,
			CandidateProbability: 0.25,
		}
	}
	first, err := Evaluate(records)
	if err != nil {
		t.Fatalf("Evaluate(first) error = %v", err)
	}
	second, err := Evaluate(records)
	if err != nil {
		t.Fatalf("Evaluate(second) error = %v", err)
	}
	if !reflectResults(first, second) {
		t.Fatalf("repeated Evaluate() results differ:\n%#v\n%#v", first, second)
	}
	assertEstimate(t, first, 0.5, 0.5)
}

func assertEstimate(t *testing.T, result Result, wantIPS, wantSNIPS float64) {
	t.Helper()
	if result.IPS == nil || result.SNIPS == nil {
		t.Fatalf("estimates are unavailable: %#v", result)
	}
	assertClose(t, "IPS", *result.IPS, wantIPS, 1e-12)
	assertClose(t, "SNIPS", *result.SNIPS, wantSNIPS, 1e-12)
	if result.Reason != "" {
		t.Fatalf("Reason = %q, want empty", result.Reason)
	}
}

func assertUnavailable(t *testing.T, result Result, wantReason string) {
	t.Helper()
	if result.IPS != nil || result.SNIPS != nil {
		t.Fatalf("unavailable result contains estimates: %#v", result)
	}
	if result.Reason != wantReason {
		t.Fatalf("Reason = %q, want %q", result.Reason, wantReason)
	}
}

func assertClose(t *testing.T, field string, got, want, tolerance float64) {
	t.Helper()
	if math.Abs(got-want) > tolerance {
		t.Fatalf("%s = %.18f, want %.18f (tolerance %g)", field, got, want, tolerance)
	}
}

func reflectResults(left, right Result) bool {
	if left.SampleCount != right.SampleCount || left.EffectiveSampleSize != right.EffectiveSampleSize || left.WeightSum != right.WeightSum || left.WeightSquaredSum != right.WeightSquaredSum || left.WeightedRewardSum != right.WeightedRewardSum || left.MinWeight != right.MinWeight || left.MaxWeight != right.MaxWeight || left.Reason != right.Reason {
		return false
	}
	if (left.IPS == nil) != (right.IPS == nil) || (left.SNIPS == nil) != (right.SNIPS == nil) {
		return false
	}
	if left.IPS != nil && *left.IPS != *right.IPS {
		return false
	}
	return left.SNIPS == nil || *left.SNIPS == *right.SNIPS
}
