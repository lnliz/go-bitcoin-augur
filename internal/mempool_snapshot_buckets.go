package internal

import "time"

type MempoolSnapshotBuckets struct {
	Timestamp   time.Time
	BlockHeight int
	Buckets     []float64
}

func NewMempoolSnapshotBuckets(timestamp time.Time, blockHeight int, bucketedWeights map[int]int64) MempoolSnapshotBuckets {
	buckets := make([]float64, BucketArraySize)

	for bucket, weight := range bucketedWeights {
		if bucket >= BucketMin {
			idx := BucketMax - bucket
			if idx >= 0 && idx < BucketArraySize {
				buckets[idx] = float64(weight)
			}
		}
	}

	return MempoolSnapshotBuckets{
		Timestamp:   timestamp,
		BlockHeight: blockHeight,
		Buckets:     buckets,
	}
}
