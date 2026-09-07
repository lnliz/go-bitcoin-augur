package internal

import "time"

// CalculateInflows estimates net positive bucket growth per ten minutes. The
// caller supplies validated snapshots in chronological order. Only observations
// inside timeframe are used; intervals crossing a block-height or hash change are
// excluded because mining changes the mempool independently of incoming demand.
func CalculateInflows(snapshots []MempoolSnapshotBuckets, timeframe time.Duration) []float64 {
	inflows := make([]float64, BucketArraySize)
	if len(snapshots) < 2 || timeframe <= 0 {
		return inflows
	}

	startTime := snapshots[len(snapshots)-1].Timestamp.Add(-timeframe)
	firstRelevant := 0
	for firstRelevant < len(snapshots) && snapshots[firstRelevant].Timestamp.Before(startTime) {
		firstRelevant++
	}
	snapshots = snapshots[firstRelevant:]

	var totalSeconds float64
	for start := 0; start < len(snapshots); {
		end := start + 1
		for end < len(snapshots) &&
			snapshots[end].BlockHeight == snapshots[start].BlockHeight &&
			snapshots[end].BlockHash == snapshots[start].BlockHash {
			end++
		}

		// Use contiguous runs, rather than grouping all snapshots by height: a
		// reorganization can return to an earlier height after blocks were mined.
		// When provided, tip hashes also distinguish replacements at one height.
		first, last := snapshots[start], snapshots[end-1]
		seconds := last.Timestamp.Sub(first.Timestamp).Seconds()
		if seconds > 0 {
			totalSeconds += seconds
			for i := range inflows {
				if delta := last.Buckets[i] - first.Buckets[i]; delta > 0 {
					inflows[i] += delta
				}
			}
		}
		start = end
	}

	if totalSeconds > 0 {
		normalizationFactor := (10 * time.Minute).Seconds() / totalSeconds
		for i := range inflows {
			inflows[i] *= normalizationFactor
		}
	}
	return inflows
}
