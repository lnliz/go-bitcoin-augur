package augur

import (
	"maps"
	"math"
	"reflect"
	"slices"
	"sync"
	"testing"
	"time"
)

func TestRejectInvalidConfiguration(t *testing.T) {
	for _, value := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		if _, err := NewFeeEstimator(WithProbabilities([]float64{value})); err == nil {
			t.Errorf("accepted probability %v", value)
		}
	}
	for _, value := range []float64{math.NaN(), math.Inf(1), math.Inf(-1), 0, -1, 0.5, 3.5, MaxBlockTarget + 1, math.MaxFloat64} {
		if _, err := NewFeeEstimator(WithBlockTargets([]float64{value})); err == nil {
			t.Errorf("accepted target %v", value)
		}
	}
	for _, options := range [][]FeeEstimatorOption{
		{nil}, {WithShortTermWindow(0)}, {WithLongTermWindow(0)},
		{WithShortTermWindow(-time.Minute)}, {WithLongTermWindow(time.Minute)},
	} {
		if _, err := NewFeeEstimator(options...); err == nil {
			t.Error("accepted invalid options")
		}
	}
	if _, err := NewFeeEstimator(WithBlockTargets([]float64{1, MaxBlockTarget})); err != nil {
		t.Fatal(err)
	}
}

func TestCustomTargetValidationBeforeEmptyHistory(t *testing.T) {
	estimator := mustEstimator(t)
	for _, target := range []float64{math.NaN(), math.Inf(1), math.Inf(-1), 0, 2, 3.5, MaxBlockTarget + 1} {
		if _, err := estimator.CalculateEstimatesForBlocks(nil, &target); err == nil {
			t.Errorf("accepted target %v with empty history", target)
		}
	}
}

func TestEstimatorCopiesAndNormalizesOptions(t *testing.T) {
	probabilities := []float64{0.95, 0.5, 0.5}
	targets := []float64{12, 3, 6, 3}
	estimator := mustEstimator(t, WithProbabilities(probabilities), WithBlockTargets(targets))
	if !slices.Equal(probabilities, []float64{0.95, 0.5, 0.5}) || !slices.Equal(targets, []float64{12, 3, 6, 3}) {
		t.Fatal("constructor mutated caller configuration")
	}
	probabilities[0] = math.NaN()
	targets[0] = math.Inf(1)
	snapshots := createSnapshotSequence(t, defaultSnapshotSequenceOptions())
	want := mustEstimate(t, mustEstimator(t, WithProbabilities([]float64{0.5, 0.95}), WithBlockTargets([]float64{3, 6, 12})), snapshots)
	got := mustEstimate(t, estimator, snapshots)
	if !reflect.DeepEqual(got, want) {
		t.Fatal("estimates depend on target ordering, duplicate configuration, or caller mutation")
	}
}

func TestValidateSnapshots(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	estimator := mustEstimator(t)
	for _, snapshot := range []MempoolSnapshot{
		{BlockHeight: -1, Timestamp: now},
		{BlockHeight: 1},
		{BlockHeight: 1, Timestamp: now, BucketedWeights: map[int]int64{0: -1}},
	} {
		if err := snapshot.Validate(); err == nil {
			t.Errorf("accepted invalid snapshot: %+v", snapshot)
		}
		if _, err := estimator.CalculateEstimates([]MempoolSnapshot{snapshot}); err == nil {
			t.Error("estimator accepted invalid snapshot")
		}
	}
	snapshot := NewEmptyMempoolSnapshot(1, now)
	if _, err := estimator.CalculateEstimates([]MempoolSnapshot{snapshot, snapshot}); err == nil {
		t.Error("accepted duplicate observation times")
	}
	for _, uninitialized := range []*FeeEstimator{nil, {}} {
		if _, err := uninitialized.CalculateEstimates([]MempoolSnapshot{snapshot}); err == nil {
			t.Error("accepted uninitialized estimator")
		}
	}
}

func TestEstimatesPreserveInputsAndIgnoreExpiredHistory(t *testing.T) {
	snapshots := createSnapshotSequence(t, defaultSnapshotSequenceOptions())
	estimator := mustEstimator(t)
	want := mustEstimate(t, estimator, snapshots)
	slices.Reverse(snapshots)
	before := slices.Clone(snapshots)
	for i := range before {
		before[i].BucketedWeights = maps.Clone(before[i].BucketedWeights)
	}
	got := mustEstimate(t, estimator, snapshots)
	if !reflect.DeepEqual(snapshots, before) || !reflect.DeepEqual(got, want) {
		t.Fatal("unordered input changed results or was modified")
	}
	old := NewEmptyMempoolSnapshot(1, snapshots[len(snapshots)-1].Timestamp.Add(-48*time.Hour))
	old.BucketedWeights[1000] = math.MaxInt64
	got = mustEstimate(t, estimator, append(snapshots, old))
	if !reflect.DeepEqual(got, want) {
		t.Fatal("expired snapshot changed estimates")
	}
}

func TestEstimatorConcurrentCalculations(t *testing.T) {
	estimator := mustEstimator(t)
	snapshots := createSnapshotSequence(t, defaultSnapshotSequenceOptions())
	want := mustEstimate(t, estimator, snapshots)
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			got, err := estimator.CalculateEstimates(snapshots)
			if err != nil || !reflect.DeepEqual(got, want) {
				t.Errorf("concurrent estimate changed: %v", err)
			}
		})
	}
	wg.Wait()
}

func TestNearestBlockTargetTiesAndIntegerLimits(t *testing.T) {
	for _, tc := range []struct {
		targets     []int
		query, want int
	}{
		{[]int{3, 9}, 6, 3},
		{[]int{3, 9}, math.MinInt, 3},
		{[]int{3, 9}, math.MaxInt, 9},
		{[]int{math.MinInt, math.MaxInt}, 0, math.MaxInt},
		{[]int{math.MaxInt}, math.MinInt, math.MaxInt},
	} {
		estimate := FeeEstimate{Estimates: make(map[int]BlockTarget)}
		for _, target := range tc.targets {
			estimate.Estimates[target] = BlockTarget{}
		}
		for range 50 {
			if got, ok := estimate.GetNearestBlockTarget(tc.query); !ok || got != tc.want {
				t.Fatalf("nearest(%d) = %d, %v; want %d", tc.query, got, ok, tc.want)
			}
		}
	}
}

func TestSnapshotTipHashSeparatesSameHeightReorg(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	history := make([]MempoolSnapshot, 4)
	weights := []int64{1_000_000, 2_000_000, 60_000_000, 61_000_000}
	for i := range history {
		history[i] = NewEmptyMempoolSnapshot(100, now.Add(time.Duration(i)*5*time.Minute))
		history[i].BucketedWeights[100] = weights[i]
		if i < 2 {
			history[i].BlockHash = "aaa"
		} else {
			history[i].BlockHash = "bbb"
		}
	}
	estimator := mustEstimator(t)
	got := mustEstimate(t, estimator, history)
	// A hash replacement at unchanged height must be equivalent to an observed
	// height change: neither interval is evidence of transaction inflow.
	history[2].BlockHeight++
	history[3].BlockHeight++
	want := mustEstimate(t, estimator, history)
	if !reflect.DeepEqual(got, want) {
		t.Fatal("same-height replacement tip contaminated inflow")
	}
	history[0].BlockHash = "AAA"
	if normalized := mustEstimate(t, estimator, history); !reflect.DeepEqual(normalized, want) {
		t.Fatal("case of a hex tip hash changed estimates")
	}
}
