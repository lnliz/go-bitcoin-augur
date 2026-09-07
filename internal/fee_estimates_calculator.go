package internal

import (
	"math"
	"slices"
)

const BlockSizeWeightUnits = 4_000_000

// FeeEstimatesCalculator combines short- and long-term inflow projections.
// Its caller validates probabilities and supplies increasing whole-number targets.
// Bucket arrays contain nonnegative weights ordered from highest fee to lowest.
type FeeEstimatesCalculator struct {
	probabilities []float64
	blockTargets  []float64
	// Cache quantiles through the largest configured target, indexed by horizon.
	// Larger per-call targets are calculated without mutating shared state.
	expectedBlocksMined [][]int
}

func NewFeeEstimatesCalculator(probabilities, blockTargets []float64) *FeeEstimatesCalculator {
	c := &FeeEstimatesCalculator{
		probabilities: slices.Clone(probabilities),
		blockTargets:  slices.Clone(blockTargets),
	}
	c.expectedBlocksMined = make([][]int, int(slices.Max(blockTargets))+1)
	for target := 1; target < len(c.expectedBlocksMined); target++ {
		c.expectedBlocksMined[target] = c.expectedBlocks(target)
	}
	return c
}

func (c *FeeEstimatesCalculator) GetFeeEstimates(mempoolSnapshot, shortInflows, longInflows []float64) [][]*float64 {
	return c.GetFeeEstimatesForTargets(mempoolSnapshot, shortInflows, longInflows, c.blockTargets)
}

// GetFeeEstimatesForTargets uses every whole-number horizon through the largest
// requested target to enforce monotonicity. The resulting fee for a target is
// independent of which other targets were requested or configured.
func (c *FeeEstimatesCalculator) GetFeeEstimatesForTargets(mempoolSnapshot, shortInflows, longInflows, targets []float64) [][]*float64 {
	initial := make([]float64, len(mempoolSnapshot))
	for i, weight := range mempoolSnapshot {
		initial[i] = weight + shortInflows[i]/2
	}
	previous := make([]float64, len(c.probabilities))
	for i := range previous {
		previous[i] = BucketMax + 1
	}
	result := make([][]*float64, len(targets))
	requested := 0
	for target := 1; target <= int(targets[len(targets)-1]); target++ {
		blocks := c.expectedBlocks(target)
		for j, probability := range c.probabilities {
			bucket := float64(BucketMin)
			if probability != 0 {
				short := c.runSimulation(initial, shortInflows, blocks[j], float64(target), BlockSizeWeightUnits)
				long := c.runSimulation(initial, longInflows, blocks[j], float64(target), BlockSizeWeightUnits)
				bucket = weightedEstimate(float64(short), float64(long), float64(target))
			}
			// A fee sufficient at a shorter horizon remains sufficient when waiting
			// longer. Always consider the same horizons, regardless of query shape.
			previous[j] = math.Min(previous[j], bucket)
		}
		if target == int(targets[requested]) {
			result[requested] = make([]*float64, len(c.probabilities))
			for j, bucket := range previous {
				if bucket <= BucketMax {
					fee := math.Exp(bucket / 100)
					result[requested][j] = &fee
				}
			}
			requested++
		}
	}
	return result
}

func (c *FeeEstimatesCalculator) runSimulation(
	initialWeights, addedWeights []float64,
	expectedBlocks int,
	meanBlocks, blockSize float64,
) int {
	if expectedBlocks <= 0 {
		return BucketMax + 1
	}

	// For any prefix of highest-fee buckets, initial weight A and constant
	// per-block inflow B give remaining weight Q(n) = max(0, A+n*(B-C)),
	// where C is block capacity. A new transaction needs positive spare
	// capacity: a prefix that exactly fills the blocks also requires bidding
	// above its last bucket. Find that prefix without simulating each block.
	capacity := float64(expectedBlocks) * blockSize
	weight := 0.0
	for i, initial := range initialWeights {
		weight += initial + addedWeights[i]*meanBlocks
		if weight >= capacity {
			return BucketMax - i + 1
		}
	}
	return BucketMin
}

// weightedEstimate blends only available projections. At and beyond one day,
// the long-term projection alone is used; the quadratic must not extrapolate.
func weightedEstimate(short, long, target float64) float64 {
	if target >= 144 {
		return long
	}
	if short > BucketMax || long > BucketMax {
		return BucketMax + 1
	}
	longWeight := 1 - math.Pow(1-target/144, 2)
	return short + (long-short)*longWeight
}

func (c *FeeEstimatesCalculator) expectedBlocks(target int) []int {
	if target < len(c.expectedBlocksMined) && c.expectedBlocksMined[target] != nil {
		return c.expectedBlocksMined[target]
	}
	blocks := make([]int, len(c.probabilities))
	for j, probability := range c.probabilities {
		if probability > 0 && probability < 1 {
			blocks[j] = poissonBlocks(float64(target), probability)
		}
	}
	return blocks
}
