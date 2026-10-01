package augur

import (
	"math"
	"slices"
	"testing"
	"time"

	"github.com/lnliz/go-bitcoin-augur/internal"
)

type memorySample struct {
	minutes int
	height  int
	hash    string
	weights map[int]int64
}

// Expected fees from a46958a, before the memory optimization. Rows are targets
// 3, 6, 144, 1008; columns are probabilities 0.05, 0.5, 0.95.
var memoryEstimateVectors = []struct {
	name    string
	samples []memorySample
	want    [4][3]float64
}{
	{
		name: "windows and disappearing buckets",
		samples: []memorySample{
			{-1441, 100, "a", map[int]int64{900: 400_000_000}},
			{-1440, 100, "a", map[int]int64{500: 20_000_000, 200: 2_000_000, 0: 8_000_000}},
			{-60, 100, "a", map[int]int64{500: 25_000_000, 200: 1_000_000, 0: 12_000_000}},
			{-30, 101, "b", map[int]int64{400: 8_000_000, 100: 2_000_000, -230: 1_000_000}},
			{-15, 101, "b", map[int]int64{400: 9_000_000, 100: 3_000_000, -230: 2_000_000}},
			{0, 101, "b", map[int]int64{400: 10_000_000, 100: 1_000_000, -230: 3_000_000}},
		},
		want: [4][3]float64{
			{0.10025884372280375, 42.53104904662416, 55.14687056346381},
			{0.10025884372280375, 0.10025884372280375, 55.14687056346381},
			{0.10025884372280375, 0.10025884372280375, 0.10025884372280375},
			{0.10025884372280375, 0.10025884372280375, 0.10025884372280375},
		},
	},
	{
		name: "same height reorg and normalized hashes",
		samples: []memorySample{
			{-60, 100, "AAA", map[int]int64{700: 2_000_000, 100: 3_000_000}},
			{-50, 100, "aaa", map[int]int64{700: 3_000_000, 100: 5_000_000}},
			{-40, 100, "bbb", map[int]int64{600: 5_000_000}},
			{-30, 100, "bbb", map[int]int64{600: 7_000_000}},
			{-20, 101, "ccc", map[int]int64{500: 6_000_000, 0: 6_000_000}},
			{-10, 101, "ccc", map[int]int64{500: 8_000_000, 0: 12_000_000}},
			{0, 100, "aaa", map[int]int64{500: 9_000_000, 0: 10_000_000}},
		},
		want: [4][3]float64{
			{1.010050167084168, 149.9047361490467, 149.9047361490467},
			{1.010050167084168, 1.0959233196138054, 149.9047361490467},
			{0.10025884372280375, 1.010050167084168, 1.010050167084168},
			{0.10025884372280375, 1.010050167084168, 1.010050167084168},
		},
	},
	{
		name: "folded ceiling and ignored floor",
		samples: []memorySample{
			{-30, 100, "a", map[int]int64{1000: 2_000_000, 1001: 1_000_000, 1100: 1_000_000, -230: 4_000_000, -231: 9_000_000}},
			{-15, 100, "a", map[int]int64{1000: 3_000_000, 1001: 1_000_000, 1200: 1_000_000, 300: 2_000_000}},
			{0, 100, "a", map[int]int64{1001: 2_000_000, 1100: 2_000_000, 300: 4_000_000, 0: 1_000_000, -230: 2_000_000}},
		},
		want: [4][3]float64{
			{0.10025884372280375, 20.287399925240926, 0},
			{0.10025884372280375, 0.10025884372280375, 20.287399925240926},
			{0.10025884372280375, 0.10025884372280375, 0.10025884372280375},
			{0.10025884372280375, 0.10025884372280375, 0.10025884372280375},
		},
	},
}

func memoryHistory(samples []memorySample) []MempoolSnapshot {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	history := make([]MempoolSnapshot, len(samples))
	for i, sample := range samples {
		history[i] = MempoolSnapshot{
			Timestamp:       base.Add(time.Duration(sample.minutes) * time.Minute),
			BlockHeight:     sample.height,
			BlockHash:       sample.hash,
			BucketedWeights: sample.weights,
		}
	}
	return history
}

func matchesMemoryEstimate(got, want FeeEstimate) bool {
	if got.Timestamp != want.Timestamp || len(got.Estimates) != len(want.Estimates) || (got.Estimates == nil) != (want.Estimates == nil) {
		return false
	}
	for target, expected := range want.Estimates {
		actual, ok := got.Estimates[target]
		if !ok || actual.Blocks != expected.Blocks || len(actual.Probabilities) != len(expected.Probabilities) || (actual.Probabilities == nil) != (expected.Probabilities == nil) {
			return false
		}
		for probability, fee := range expected.Probabilities {
			rate, ok := actual.Probabilities[probability]
			if !ok {
				return false
			}
			// Positive fee values are ordered by their float64 bits. Allow at most
			// two ULP for architecture-specific math.Exp rounding in saved fixtures.
			a, b := math.Float64bits(rate), math.Float64bits(fee)
			if a < b {
				a, b = b, a
			}
			if a-b > 2 {
				return false
			}
		}
	}
	return true
}

func TestEstimatesMatchBeforeMemoryOptimization(t *testing.T) {
	targets := []float64{3, 6, 144, 1008}
	probabilities := []float64{0.05, 0.5, 0.95}
	estimator := mustEstimator(t, WithBlockTargets(targets), WithProbabilities(probabilities))
	for _, vector := range memoryEstimateVectors {
		t.Run(vector.name, func(t *testing.T) {
			history := memoryHistory(vector.samples)
			want := FeeEstimate{Timestamp: history[len(history)-1].Timestamp, Estimates: make(map[int]BlockTarget)}
			for i, target := range targets {
				fees := make(map[float64]float64)
				for j, probability := range probabilities {
					// Zero denotes an unavailable estimate; valid fees are positive.
					if fee := vector.want[i][j]; fee != 0 {
						fees[probability] = fee
					}
				}
				want.Estimates[int(target)] = BlockTarget{Blocks: int(target), Probabilities: fees}
			}
			slices.Reverse(history)
			if got := mustEstimate(t, estimator, history); !matchesMemoryEstimate(got, want) {
				t.Fatalf("batch estimates changed: got %+v, want %+v", got, want)
			}
			for _, target := range targets {
				got, err := estimator.CalculateEstimatesForBlocks(history, &target)
				single := FeeEstimate{Timestamp: want.Timestamp, Estimates: map[int]BlockTarget{int(target): want.Estimates[int(target)]}}
				if err != nil || !matchesMemoryEstimate(got, single) {
					t.Fatalf("target %g estimates changed: got %+v (%v), want %+v", target, got, err, single)
				}
			}
		})
	}
}

func BenchmarkCalculateEstimatesDay(b *testing.B) {
	weights := make(map[int]int64, 551)
	for bucket := internal.BucketMin; bucket < internal.BucketMin+551; bucket++ {
		weights[bucket] = 400_000
	}
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	history := make([]MempoolSnapshot, 2880)
	for i := range history {
		history[i] = MempoolSnapshot{Timestamp: base.Add(time.Duration(i) * 30 * time.Second), BlockHeight: 100 + i/20, BucketedWeights: weights}
	}
	estimator, err := NewFeeEstimator()
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	for b.Loop() {
		if _, err := estimator.CalculateEstimates(history); err != nil {
			b.Fatal(err)
		}
	}
}
