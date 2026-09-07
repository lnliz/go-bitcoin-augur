package internal

import "time"

type MempoolSnapshotBuckets struct {
	Timestamp   time.Time
	BlockHeight int
	BlockHash   string
	Buckets     []float64
}

// NewMempoolSnapshotBuckets converts validated, nonnegative sparse weights to
// the simulation's descending fee order. The caller may then set BlockHash.
func NewMempoolSnapshotBuckets(timestamp time.Time, blockHeight int, bucketedWeights map[int]int64) MempoolSnapshotBuckets {
	buckets := make([]float64, BucketArraySize)

	for bucket, weight := range bucketedWeights {
		switch {
		case bucket > BucketMax:
			// High-fee transactions still consume block space, even when their
			// fee rate exceeds the simulation ceiling.
			buckets[0] += float64(weight)
		case bucket >= BucketMin:
			buckets[BucketMax-bucket] += float64(weight)
		}
	}

	return MempoolSnapshotBuckets{
		Timestamp:   timestamp,
		BlockHeight: blockHeight,
		Buckets:     buckets,
	}
}
