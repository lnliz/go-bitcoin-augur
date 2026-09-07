package internal

import (
	"fmt"
	"math"
)

const (
	// Buckets are spaced logarithmically: bucket n represents exp(n/100) sat/vB.
	// The simulation stores these in descending order, highest fee first.
	BucketMax       = 1000
	BucketMin       = -230
	BucketArraySize = BucketMax - BucketMin + 1
)

type FeeRateWeightPair interface {
	FeeRate() float64
	GetWeight() int64
}

// CreateFeeRateBuckets retains positive fee rates below the simulation floor so
// snapshots preserve their input data. Zero-fee transactions cannot affect an
// estimate above that floor and are omitted.
func CreateFeeRateBuckets[T FeeRateWeightPair](pairs []T) (map[int]int64, error) {
	buckets := make(map[int]int64)
	for i, pair := range pairs {
		weight := pair.GetWeight()
		if weight <= 0 {
			return nil, fmt.Errorf("transaction %d: weight must be positive", i)
		}
		feeRate := pair.FeeRate()
		if math.IsNaN(feeRate) || math.IsInf(feeRate, 0) || feeRate < 0 {
			return nil, fmt.Errorf("transaction %d: fee rate must be finite and nonnegative", i)
		}
		if feeRate == 0 {
			continue
		}
		bucket := calculateBucketIndex(feeRate)
		if buckets[bucket] > math.MaxInt64-weight {
			return nil, fmt.Errorf("transaction %d: total weight overflows bucket %d", i, bucket)
		}
		buckets[bucket] += weight
	}
	return buckets, nil
}

// calculateBucketIndex requires a finite, positive fee rate.
func calculateBucketIndex(feeRate float64) int {
	logRate := math.Log(feeRate)
	if feeRate < 0x1p-1022 {
		// Normalize subnormal inputs before taking their logarithm.
		fraction, exponent := math.Frexp(feeRate)
		logRate = math.Log(fraction) + float64(exponent)*math.Ln2
	}
	// Kotlin's round uses ties-to-even, so keep snapshot buckets compatible.
	idx := int(math.RoundToEven(logRate * 100))
	if idx > BucketMax {
		return BucketMax
	}
	return idx
}
