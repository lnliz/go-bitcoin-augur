package internal

import (
	"math"
	"slices"
)

const BlockSizeWeightUnits = 4_000_000

// FeeEstimatesCalculator combines short- and long-term inflow projections.
// Its caller validates probabilities and supplies increasing block targets.
// Bucket arrays contain nonnegative weights ordered from highest fee to lowest.
type FeeEstimatesCalculator struct {
	probabilities       []float64
	blockTargets        []float64
	expectedBlocksMined [][]int
}

func NewFeeEstimatesCalculator(probabilities, blockTargets []float64) *FeeEstimatesCalculator {
	calc := &FeeEstimatesCalculator{
		probabilities: slices.Clone(probabilities),
		blockTargets:  slices.Clone(blockTargets),
	}
	calc.expectedBlocksMined = calc.getExpectedBlocksMined()
	return calc
}

func (c *FeeEstimatesCalculator) GetFeeEstimates(
	mempoolSnapshot []float64,
	shortIntervalInflows []float64,
	longIntervalInflows []float64,
) [][]*float64 {
	currentWeightsWithBuffer := make([]float64, len(mempoolSnapshot))
	for i := range mempoolSnapshot {
		currentWeightsWithBuffer[i] = mempoolSnapshot[i] + shortIntervalInflows[i]/2
	}

	shortTermEstimates := c.runSimulations(currentWeightsWithBuffer, shortIntervalInflows)
	longTermEstimates := c.runSimulations(currentWeightsWithBuffer, longIntervalInflows)
	weightedEstimates := c.getWeightedEstimates(shortTermEstimates, longTermEstimates)

	// Correct target ordering in bucket space before converting to fee rates.
	// The extra bucket is an unavailable estimate, never an interpolated fee.
	for j := range c.probabilities {
		previous := float64(BucketMax + 1)
		for i := range weightedEstimates {
			weightedEstimates[i][j] = math.Min(previous, weightedEstimates[i][j])
			previous = weightedEstimates[i][j]
		}
	}

	result := make([][]*float64, len(weightedEstimates))
	for i, row := range weightedEstimates {
		result[i] = make([]*float64, len(row))
		for j, bucket := range row {
			if bucket <= BucketMax {
				fee := math.Exp(bucket / 100)
				result[i][j] = &fee
			}
		}
	}
	return result
}

func (c *FeeEstimatesCalculator) runSimulations(initialWeights, addedWeights []float64) [][]float64 {
	result := make([][]float64, len(c.blockTargets))
	for i, target := range c.blockTargets {
		result[i] = make([]float64, len(c.probabilities))
		for j, probability := range c.probabilities {
			if probability == 0 {
				// No confirmation confidence is requested, so the floor suffices.
				result[i][j] = BucketMin
				continue
			}
			result[i][j] = float64(c.runSimulation(initialWeights, addedWeights,
				c.expectedBlocksMined[i][j], target, BlockSizeWeightUnits))
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
	// where C is block capacity. Thus the first uncleared bucket can be found
	// from total weight and capacity, without allocating or simulating blocks.
	capacity := float64(expectedBlocks) * blockSize
	weight := 0.0
	for i, initial := range initialWeights {
		weight += initial + addedWeights[i]*meanBlocks
		if weight > capacity {
			return BucketMax - i + 1
		}
	}
	return BucketMin
}

func (c *FeeEstimatesCalculator) getWeightedEstimates(shortEstimates, longEstimates [][]float64) [][]float64 {
	result := make([][]float64, len(shortEstimates))
	for i := range result {
		// Beyond a day, use the long-term projection exclusively. Extending
		// the quadratic past 144 would eventually give it a negative weight.
		target := math.Min(c.blockTargets[i], 144)
		longWeight := 1 - math.Pow(1-target/144, 2)
		result[i] = make([]float64, len(shortEstimates[i]))
		for j, short := range shortEstimates[i] {
			long := longEstimates[i][j]
			switch {
			case longWeight == 1:
				result[i][j] = long
			case short > BucketMax || long > BucketMax:
				// A missing projection has no finite fee to average. Treat it
				// conservatively until a shorter target supplies a valid rate.
				result[i][j] = BucketMax + 1
			default:
				result[i][j] = short + (long-short)*longWeight
			}
		}
	}
	return result
}

func (c *FeeEstimatesCalculator) getExpectedBlocksMined() [][]int {
	blocks := make([][]int, len(c.blockTargets))
	for i, target := range c.blockTargets {
		blocks[i] = make([]int, len(c.probabilities))
		for j, probability := range c.probabilities {
			if probability > 0 && probability < 1 {
				blocks[i][j] = poissonBlocks(target, probability)
			}
		}
	}
	return blocks
}
