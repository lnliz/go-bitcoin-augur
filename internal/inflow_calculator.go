package internal

import (
	"sort"
	"time"
)

func CalculateInflows(snapshots []MempoolSnapshotBuckets, timeframe time.Duration) []float64 {
	if len(snapshots) == 0 {
		return make([]float64, BucketArraySize)
	}

	ordered := make([]MempoolSnapshotBuckets, len(snapshots))
	copy(ordered, snapshots)
	sort.Slice(ordered, func(i, j int) bool {
		return ordered[i].Timestamp.Before(ordered[j].Timestamp)
	})

	endTime := ordered[len(ordered)-1].Timestamp
	startTime := endTime.Add(-timeframe)

	var relevant []MempoolSnapshotBuckets
	for _, s := range ordered {
		if !s.Timestamp.Before(startTime) && !s.Timestamp.After(endTime) {
			relevant = append(relevant, s)
		}
	}

	inflows := make([]float64, BucketArraySize)

	byBlock := make(map[int][]MempoolSnapshotBuckets)
	for _, s := range relevant {
		byBlock[s.BlockHeight] = append(byBlock[s.BlockHeight], s)
	}

	var totalTimeSpan time.Duration
	for _, blockSnapshots := range byBlock {
		if len(blockSnapshots) == 0 {
			continue
		}

		sort.Slice(blockSnapshots, func(i, j int) bool {
			return blockSnapshots[i].Timestamp.Before(blockSnapshots[j].Timestamp)
		})

		first := blockSnapshots[0]
		last := blockSnapshots[len(blockSnapshots)-1]

		totalTimeSpan += last.Timestamp.Sub(first.Timestamp)

		for i := 0; i < BucketArraySize; i++ {
			delta := last.Buckets[i] - first.Buckets[i]
			if delta > 0 {
				inflows[i] += delta
			}
		}
	}

	if totalTimeSpan > 0 {
		tenMinutes := 10 * time.Minute
		normalizationFactor := float64(tenMinutes) / float64(totalTimeSpan)
		for i := range inflows {
			inflows[i] *= normalizationFactor
		}
	}

	return inflows
}
