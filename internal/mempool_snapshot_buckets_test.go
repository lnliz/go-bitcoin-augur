package internal

import (
	"math"
	"testing"
	"time"
)

func TestFromMempoolSnapshotDropsBucketsBelowMinimum(t *testing.T) {
	lowBucket := BucketMin - 1
	validBucket := BucketMin

	bucketedWeights := map[int]int64{
		lowBucket:   400,
		validBucket: 600,
	}

	result := NewMempoolSnapshotBuckets(time.Now(), 100, bucketedWeights)

	if len(result.Buckets) != BucketArraySize {
		t.Errorf("expected %d buckets, got %d", BucketArraySize, len(result.Buckets))
	}

	validIndex := BucketMax - validBucket
	if result.Buckets[validIndex] != 600.0 {
		t.Errorf("expected bucket[%d] = 600, got %d", validIndex, result.Buckets[validIndex])
	}

	var totalWeight int64
	for _, w := range result.Buckets {
		totalWeight += w
	}
	if totalWeight != 600.0 {
		t.Errorf("expected total weight 600, got %d", totalWeight)
	}

	if validIndex != len(result.Buckets)-1 {
		t.Errorf("expected validIndex to be last index")
	}
}

func TestFromMempoolSnapshotIgnoresVeryLowFeeRates(t *testing.T) {
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

	result := NewMempoolSnapshotBuckets(time.Now(), 100, bucketedWeights)

	var totalWeight int64
	for _, w := range result.Buckets {
		totalWeight += w
	}
	if totalWeight != 500.0 {
		t.Errorf("expected total weight 500, got %d", totalWeight)
	}
}

func TestFromMempoolSnapshotPreservesAboveMaximumWeights(t *testing.T) {
	result := NewMempoolSnapshotBuckets(time.Now(), 100, map[int]int64{
		BucketMax:     100,
		BucketMax + 1: 200,
		math.MaxInt:   300,
		BucketMin:     400,
		math.MinInt:   500,
	})
	if got := result.Buckets[0]; got != 600 {
		t.Errorf("highest bucket weight = %v, want 600", got)
	}
	if got := result.Buckets[BucketArraySize-1]; got != 400 {
		t.Errorf("lowest bucket weight = %v, want 400", got)
	}
	var total int64
	for _, weight := range result.Buckets {
		total += weight
	}
	if total != 1000 {
		t.Errorf("total weight = %v, want 1000", total)
	}
}

func TestFromMempoolSnapshotFoldsIntegerWeightsExactly(t *testing.T) {
	weights := map[int]int64{
		BucketMax:     1 << 53,
		BucketMax + 1: 1,
		math.MaxInt:   math.MaxInt64 - (1 << 53) - 1,
	}
	// Map traversal order must not affect folded totals near int64's limit.
	for range 100 {
		result := NewMempoolSnapshotBuckets(time.Now(), 100, weights)
		if got := result.Buckets[0]; got != math.MaxInt64 {
			t.Fatalf("highest bucket weight = %d, want %d", got, int64(math.MaxInt64))
		}
	}
}
