package internal

import "time"

type MempoolSnapshotBuckets struct {
	Timestamp   time.Time
	BlockHeight int
	BlockHash   string
	Buckets     []int64
}

// NewMempoolSnapshotBuckets converts validated, nonnegative sparse weights to
// descending fee order. Retaining integer weights makes inflow differences
// exact even above float64's integer precision. The caller validates that
// weights folded into the highest bucket sum within int64 and may set BlockHash.
func NewMempoolSnapshotBuckets(timestamp time.Time, blockHeight int, bucketedWeights map[int]int64) MempoolSnapshotBuckets {
	buckets := make([]int64, BucketArraySize)

	for bucket, weight := range bucketedWeights {
		switch {
		case bucket > BucketMax:
			// High-fee transactions still consume block space, even when their
			// fee rate exceeds the simulation ceiling.
			buckets[0] += weight
		case bucket >= BucketMin:
			buckets[BucketMax-bucket] += weight
		}
	}

	return MempoolSnapshotBuckets{
		Timestamp:   timestamp,
		BlockHeight: blockHeight,
		Buckets:     buckets,
	}
}
