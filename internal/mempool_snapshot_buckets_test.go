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
		t.Errorf("expected bucket[%d] = 600, got %f", validIndex, result.Buckets[validIndex])
	}

	totalWeight := 0.0
	for _, w := range result.Buckets {
		totalWeight += w
	}
	if totalWeight != 600.0 {
		t.Errorf("expected total weight 600, got %f", totalWeight)
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

	totalWeight := 0.0
	for _, w := range result.Buckets {
		totalWeight += w
	}
	if totalWeight != 500.0 {
		t.Errorf("expected total weight 500, got %f", totalWeight)
	}
}
