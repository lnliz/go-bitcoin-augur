package internal

import (
	"math"
)

const (
	BucketMax       = 1000
	BucketMin       = -230
	BucketArraySize = BucketMax - BucketMin + 1
)

type FeeRateWeightPair interface {
	FeeRate() float64
	GetWeight() int64
}

func CreateFeeRateBuckets[T FeeRateWeightPair](pairs []T) map[int]int64 {
	buckets := make(map[int]int64)
	for _, pair := range pairs {
		bucket := calculateBucketIndex(pair.FeeRate())
		buckets[bucket] += pair.GetWeight()
	}
	return buckets
}

func calculateBucketIndex(feeRate float64) int {
	idx := int(math.Round(math.Log(feeRate) * 100))
	if idx > BucketMax {
		return BucketMax
	}
	return idx
}

const BlockSizeWeightUnits = 4_000_000

type FeeEstimatesCalculator struct {
	probabilities       []float64
	blockTargets        []float64
	expectedBlocksMined [][]float64
}

func NewFeeEstimatesCalculator(probabilities, blockTargets []float64) *FeeEstimatesCalculator {
	calc := &FeeEstimatesCalculator{
		probabilities: probabilities,
		blockTargets:  blockTargets,
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
		currentWeightsWithBuffer[i] = mempoolSnapshot[i] + shortIntervalInflows[i]/2.0
	}

	shortTermEstimates := c.runSimulations(currentWeightsWithBuffer, shortIntervalInflows, c.expectedBlocksMined)
	longTermEstimates := c.runSimulations(currentWeightsWithBuffer, longIntervalInflows, c.expectedBlocksMined)

	weightedEstimates := c.getWeightedEstimates(shortTermEstimates, longTermEstimates)
	feeRates := c.convertBucketsToFeeRates(weightedEstimates)
	monotoneFeeRates := c.enforceMonotonicity(feeRates)

	return c.prepareResultArray(monotoneFeeRates)
}

func (c *FeeEstimatesCalculator) runSimulations(
	initialWeights []float64,
	addedWeights []float64,
	expectedBlocksMined [][]float64,
) [][]float64 {
	result := make([][]float64, len(c.blockTargets))
	for i := range result {
		result[i] = make([]float64, len(c.probabilities))
	}

	for blockTargetIndex, blocks := range c.blockTargets {
		meanBlocks := int(blocks)

		for probIndex := range c.probabilities {
			expectedBlocks := int(expectedBlocksMined[blockTargetIndex][probIndex])

			simResult := c.runSimulation(initialWeights, addedWeights, expectedBlocks, meanBlocks, BlockSizeWeightUnits)
			result[blockTargetIndex][probIndex] = float64(simResult)
		}
	}

	return result
}

func (c *FeeEstimatesCalculator) runSimulation(
	initialWeights []float64,
	addedWeights []float64,
	expectedBlocks int,
	meanBlocks int,
	blockSize float64,
) int {
	if expectedBlocks <= 0 {
		return BucketMax + 1
	}

	expectedMiningTimeFactor := float64(meanBlocks) / float64(expectedBlocks)

	addedWeightsInOneBlock := make([]float64, len(addedWeights))
	for i := range addedWeights {
		addedWeightsInOneBlock[i] = addedWeights[i] * expectedMiningTimeFactor
	}

	currentWeights := make([]float64, len(initialWeights))
	copy(currentWeights, initialWeights)

	for block := 0; block < expectedBlocks; block++ {
		for i := range currentWeights {
			currentWeights[i] += addedWeightsInOneBlock[i]
		}
		currentWeights = c.mineBlock(currentWeights, blockSize)
	}

	return c.findBestIndex(currentWeights)
}

func (c *FeeEstimatesCalculator) mineBlock(currentWeights []float64, blockSize float64) []float64 {
	weightsRemaining := make([]float64, len(currentWeights))
	copy(weightsRemaining, currentWeights)

	weightUnitsRemaining := blockSize

	for i := range weightsRemaining {
		removedWeight := math.Min(weightsRemaining[i], weightUnitsRemaining)
		weightUnitsRemaining -= removedWeight
		weightsRemaining[i] -= removedWeight
	}

	return weightsRemaining
}

func (c *FeeEstimatesCalculator) findBestIndex(weightsRemaining []float64) int {
	index := -1
	for i, w := range weightsRemaining {
		if w != 0 {
			index = i - 1
			break
		}
	}

	if index == -1 {
		allZero := true
		for _, w := range weightsRemaining {
			if w != 0 {
				allZero = false
				break
			}
		}
		if allZero {
			return BucketMin
		}
		return BucketMax + 1
	}

	return BucketMax - index
}

func (c *FeeEstimatesCalculator) getWeightedEstimates(shortEstimates, longEstimates [][]float64) [][]float64 {
	weights := make([]float64, len(c.blockTargets))
	for i, target := range c.blockTargets {
		weights[i] = 1 - math.Pow(1-target/144.0, 2)
	}

	result := make([][]float64, len(shortEstimates))
	for i := range result {
		result[i] = make([]float64, len(shortEstimates[i]))
		for j := range result[i] {
			result[i][j] = shortEstimates[i][j]*(1.0-weights[i]) + longEstimates[i][j]*weights[i]
		}
	}

	return result
}

func (c *FeeEstimatesCalculator) convertBucketsToFeeRates(bucketEstimates [][]float64) [][]float64 {
	result := make([][]float64, len(bucketEstimates))
	for i := range result {
		result[i] = make([]float64, len(bucketEstimates[i]))
		for j := range result[i] {
			result[i][j] = math.Exp(bucketEstimates[i][j] / 100.0)
		}
	}
	return result
}

func (c *FeeEstimatesCalculator) prepareResultArray(feeRates [][]float64) [][]*float64 {
	maxAllowedFeeRate := math.Exp(float64(BucketMax) / 100.0)

	result := make([][]*float64, len(feeRates))
	for i := range result {
		result[i] = make([]*float64, len(feeRates[i]))
		for j := range result[i] {
			if feeRates[i][j] < maxAllowedFeeRate {
				val := feeRates[i][j]
				result[i][j] = &val
			}
		}
	}
	return result
}

func (c *FeeEstimatesCalculator) getExpectedBlocksMined() [][]float64 {
	blocks := make([][]float64, len(c.blockTargets))
	for i := range blocks {
		blocks[i] = make([]float64, len(c.probabilities))
	}

	for i, target := range c.blockTargets {
		maxTrials := int(target * 4)
		trialProbabilities := make([]float64, maxTrials)
		for x := 0; x < maxTrials; x++ {
			trialProbabilities[x] = 1.0 - poissonCDF(x-1, target)
		}

		for j, probability := range c.probabilities {
			numBlocks := -1
			for x := maxTrials - 1; x >= 0; x-- {
				if trialProbabilities[x] >= probability {
					numBlocks = x
					break
				}
			}
			if numBlocks != -1 {
				blocks[i][j] = float64(numBlocks)
			}
		}
	}

	return blocks
}

func (c *FeeEstimatesCalculator) enforceMonotonicity(feeRates [][]float64) [][]float64 {
	result := make([][]float64, len(feeRates))
	for i := range result {
		result[i] = make([]float64, len(feeRates[i]))
		copy(result[i], feeRates[i])
	}

	for j := 0; j < len(result[0]); j++ {
		prevRate := math.Inf(1)
		for i := 0; i < len(result); i++ {
			if result[i][j] > prevRate {
				result[i][j] = prevRate
			}
			prevRate = result[i][j]
		}
	}

	return result
}
