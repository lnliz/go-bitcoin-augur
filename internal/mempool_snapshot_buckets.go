package internal

import "time"

type MempoolSnapshotBuckets struct {
	Timestamp       time.Time
	BlockHeight     int
	BlockHash       string
	BucketedWeights map[int]int64
}

// NewMempoolSnapshotBuckets retains sparse weights without copying them.
// The caller validates the weights and must not mutate them during calculation.
func NewMempoolSnapshotBuckets(timestamp time.Time, blockHeight int, bucketedWeights map[int]int64) MempoolSnapshotBuckets {
	return MempoolSnapshotBuckets{
		Timestamp:       timestamp,
		BlockHeight:     blockHeight,
		BucketedWeights: bucketedWeights,
	}
}

// FillBucketWeights replaces dst with validated, nonnegative weights in
// descending fee order. Integer sums preserve precision when folding buckets
// above the ceiling; the caller validates that the folded sum fits in int64.
func FillBucketWeights(dst *[BucketArraySize]int64, bucketedWeights map[int]int64) {
	clear(dst[:])
	for bucket, weight := range bucketedWeights {
		switch {
		case bucket > BucketMax:
			// High-fee transactions still consume block space, even when their
			// fee rate exceeds the simulation ceiling.
			dst[0] += weight
		case bucket >= BucketMin:
			dst[BucketMax-bucket] += weight
		}
	}
}
