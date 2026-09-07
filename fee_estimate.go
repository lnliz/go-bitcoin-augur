package augur

import (
	"sort"
	"time"
)

// BlockTarget holds fee rates in satoshis per virtual byte at each confidence level.
type BlockTarget struct {
	Blocks        int
	Probabilities map[float64]float64
}

func (bt BlockTarget) GetFeeRate(probability float64) (float64, bool) {
	rate, ok := bt.Probabilities[probability]
	return rate, ok
}

// FeeEstimate is a caller-owned result. Missing probabilities have no available estimate.
// Timestamp identifies the latest input snapshot, not the calculation time.
type FeeEstimate struct {
	Estimates map[int]BlockTarget
	Timestamp time.Time
}

func (fe FeeEstimate) GetFeeRate(targetBlocks int, probability float64) (float64, bool) {
	target, ok := fe.Estimates[targetBlocks]
	if !ok {
		return 0, false
	}
	return target.GetFeeRate(probability)
}

func (fe FeeEstimate) GetEstimatesForTarget(targetBlocks int) (BlockTarget, bool) {
	target, ok := fe.Estimates[targetBlocks]
	return target, ok
}

// GetNearestBlockTarget returns the closest configured target, choosing the smaller
// target on a tie.
func (fe FeeEstimate) GetNearestBlockTarget(targetBlocks int) (int, bool) {
	if len(fe.Estimates) == 0 {
		return 0, false
	}
	if _, ok := fe.Estimates[targetBlocks]; ok {
		return targetBlocks, true
	}

	nearest := 0
	minDiff := ^uint(0)
	found := false
	for k := range fe.Estimates {
		// Unsigned subtraction gives the exact distance even across int's
		// signed limits, without overflow or float64 precision loss.
		var diff uint
		if k >= targetBlocks {
			diff = uint(k) - uint(targetBlocks)
		} else {
			diff = uint(targetBlocks) - uint(k)
		}
		if !found || diff < minDiff || (diff == minDiff && k < nearest) {
			minDiff = diff
			nearest = k
			found = true
		}
	}
	return nearest, true
}

func (fe FeeEstimate) GetAvailableBlockTargets() []int {
	targets := make([]int, 0, len(fe.Estimates))
	for k := range fe.Estimates {
		targets = append(targets, k)
	}
	sort.Ints(targets)
	return targets
}

func (fe FeeEstimate) GetAvailableConfidenceLevels() []float64 {
	seen := make(map[float64]bool)
	for _, bt := range fe.Estimates {
		for p := range bt.Probabilities {
			seen[p] = true
		}
	}
	levels := make([]float64, 0, len(seen))
	for p := range seen {
		levels = append(levels, p)
	}
	sort.Float64s(levels)
	return levels
}
