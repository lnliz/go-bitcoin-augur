package internal

import (
	"math"
	"testing"
)

func TestFillBucketWeightsDropsBucketsBelowMinimum(t *testing.T) {
	lowBucket := BucketMin - 1
	validBucket := BucketMin

	bucketedWeights := map[int]int64{
		lowBucket:   400,
		validBucket: 600,
	}

	var result [BucketArraySize]int64
	FillBucketWeights(&result, bucketedWeights)

	validIndex := BucketMax - validBucket
	if result[validIndex] != 600.0 {
		t.Errorf("expected bucket[%d] = 600, got %d", validIndex, result[validIndex])
	}

	var totalWeight int64
	for _, w := range result {
		totalWeight += w
	}
	if totalWeight != 600.0 {
		t.Errorf("expected total weight 600, got %d", totalWeight)
	}

}

func TestFillBucketWeightsIgnoresVeryLowFeeRates(t *testing.T) {
	veryLowFeeRate := 0.05
	veryLowBucket := int(math.Round(math.Log(veryLowFeeRate) * 100))

	if veryLowBucket >= BucketMin {
		t.Errorf("expected veryLowBucket < BUCKET_MIN")
	}

	validBucket := 0

	bucketedWeights := map[int]int64{
		veryLowBucket: 1000,
		validBucket:   500,
	}

	var result [BucketArraySize]int64
	FillBucketWeights(&result, bucketedWeights)

	var totalWeight int64
	for _, w := range result {
		totalWeight += w
	}
	if totalWeight != 500.0 {
		t.Errorf("expected total weight 500, got %d", totalWeight)
	}
}

func TestFillBucketWeightsPreservesAboveMaximumWeights(t *testing.T) {
	var result [BucketArraySize]int64
	FillBucketWeights(&result, map[int]int64{
		BucketMax:     100,
		BucketMax + 1: 200,
		math.MaxInt:   300,
		BucketMin:     400,
		math.MinInt:   500,
	})
	if got := result[0]; got != 600 {
		t.Errorf("highest bucket weight = %v, want 600", got)
	}
	if got := result[BucketArraySize-1]; got != 400 {
		t.Errorf("lowest bucket weight = %v, want 400", got)
	}
	var total int64
	for _, weight := range result {
		total += weight
	}
	if total != 1000 {
		t.Errorf("total weight = %v, want 1000", total)
	}
}

func TestFillBucketWeightsFoldsIntegerWeightsExactly(t *testing.T) {
	weights := map[int]int64{
		BucketMax:     1 << 53,
		BucketMax + 1: 1,
		math.MaxInt:   math.MaxInt64 - (1 << 53) - 1,
	}
	// Map traversal order must not affect folded totals near int64's limit.
	var result [BucketArraySize]int64
	for range 100 {
		FillBucketWeights(&result, weights)
		if got := result[0]; got != math.MaxInt64 {
			t.Fatalf("highest bucket weight = %d, want %d", got, int64(math.MaxInt64))
		}
	}
}

func TestFillBucketWeightsClearsReusedBuffer(t *testing.T) {
	var result [BucketArraySize]int64
	FillBucketWeights(&result, map[int]int64{BucketMax: 100, BucketMin: 200})
	FillBucketWeights(&result, map[int]int64{0: 300})
	if result[0] != 0 || result[BucketArraySize-1] != 0 || result[BucketMax] != 300 {
		t.Fatal("reused buffer retained weights from the previous snapshot")
	}
	FillBucketWeights(&result, nil)
	for _, weight := range result {
		if weight != 0 {
			t.Fatal("empty snapshot retained previous weights")
		}
	}
}
