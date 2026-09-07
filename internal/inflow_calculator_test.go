package internal

import (
	"testing"
	"time"
)

func TestCalculateInflowsWithEmptySnapshotList(t *testing.T) {
	inflows := CalculateInflows(nil, 10*time.Minute)

	if len(inflows) != BucketArraySize {
		t.Errorf("expected %d buckets, got %d", BucketArraySize, len(inflows))
	}
	sum := 0.0
	for _, v := range inflows {
		sum += v
	}
	if sum != 0.0 {
		t.Errorf("expected sum 0, got %f", sum)
	}
}

func TestCalculateInflowsWithSingleBlockSnapshots(t *testing.T) {
	now := time.Now()

	buckets1 := make([]float64, BucketArraySize)
	buckets2 := make([]float64, BucketArraySize)
	for i := range buckets1 {
		buckets1[i] = 1000.0
		buckets2[i] = 2000.0
	}

	snapshots := []MempoolSnapshotBuckets{
		{Timestamp: now, BlockHeight: 100, Buckets: buckets1},
		{Timestamp: now.Add(5 * time.Minute), BlockHeight: 100, Buckets: buckets2},
	}

	inflows := CalculateInflows(snapshots, 10*time.Minute)

	if len(inflows) != BucketArraySize {
		t.Errorf("expected %d buckets, got %d", BucketArraySize, len(inflows))
	}
	if inflows[0] != 2000.0 {
		t.Errorf("expected inflows[0] = 2000, got %f", inflows[0])
	}
}

func TestCalculateInflowsWithConsistentInflowRate(t *testing.T) {
	now := time.Now()

	buckets1 := make([]float64, BucketArraySize)
	buckets2 := make([]float64, BucketArraySize)
	buckets3 := make([]float64, BucketArraySize)
	for i := range buckets1 {
		buckets1[i] = 1_000_000.0
		buckets2[i] = 2_000_000.0
		buckets3[i] = 3_000_000.0
	}

	snapshots := []MempoolSnapshotBuckets{
		{Timestamp: now, BlockHeight: 100, Buckets: buckets1},
		{Timestamp: now.Add(5 * time.Minute), BlockHeight: 100, Buckets: buckets2},
		{Timestamp: now.Add(10 * time.Minute), BlockHeight: 100, Buckets: buckets3},
	}

	inflows := CalculateInflows(snapshots, 10*time.Minute)

	if len(inflows) != BucketArraySize {
		t.Errorf("expected %d buckets, got %d", BucketArraySize, len(inflows))
	}
	if inflows[0] != 2_000_000.0 {
		t.Errorf("expected inflows[0] = 2000000, got %f", inflows[0])
	}
	if inflows[BucketArraySize-1] != 2_000_000.0 {
		t.Errorf("expected inflows[%d] = 2000000, got %f", BucketArraySize-1, inflows[BucketArraySize-1])
	}
}

func TestCalculateInflowsWithDifferentRatesPerBucket(t *testing.T) {
	now := time.Now()

	buckets1 := make([]float64, BucketArraySize)
	buckets2 := make([]float64, BucketArraySize)
	buckets1[0] = 1_000_000.0
	buckets1[1] = 2_000_000.0
	buckets1[2] = 3_000_000.0
	buckets2[0] = 2_000_000.0
	buckets2[1] = 4_000_000.0
	buckets2[2] = 6_000_000.0

	snapshots := []MempoolSnapshotBuckets{
		{Timestamp: now, BlockHeight: 100, Buckets: buckets1},
		{Timestamp: now.Add(5 * time.Minute), BlockHeight: 100, Buckets: buckets2},
	}

	inflows := CalculateInflows(snapshots, 10*time.Minute)

	if len(inflows) != BucketArraySize {
		t.Errorf("expected %d buckets, got %d", BucketArraySize, len(inflows))
	}
	if inflows[0] != 2_000_000.0 {
		t.Errorf("expected inflows[0] = 2000000, got %f", inflows[0])
	}
	if inflows[1] != 4_000_000.0 {
		t.Errorf("expected inflows[1] = 4000000, got %f", inflows[1])
	}
	if inflows[2] != 6_000_000.0 {
		t.Errorf("expected inflows[2] = 6000000, got %f", inflows[2])
	}
	if inflows[3] != 0.0 {
		t.Errorf("expected inflows[3] = 0, got %f", inflows[3])
	}
}

func TestCalculateInflowsConsidersOnlyFirstAndLastSnapshotPerBlockHeight(t *testing.T) {
	now := time.Now()

	buckets1 := make([]float64, BucketArraySize)
	buckets2 := make([]float64, BucketArraySize)
	buckets3 := make([]float64, BucketArraySize)
	for i := range buckets1 {
		buckets1[i] = 1000.0
		buckets2[i] = 500.0
		buckets3[i] = 2000.0
	}

	snapshots := []MempoolSnapshotBuckets{
		{Timestamp: now, BlockHeight: 100, Buckets: buckets1},
		{Timestamp: now.Add(100 * time.Second), BlockHeight: 100, Buckets: buckets2},
		{Timestamp: now.Add(5 * time.Minute), BlockHeight: 100, Buckets: buckets3},
	}

	inflows := CalculateInflows(snapshots, 10*time.Minute)

	if len(inflows) != BucketArraySize {
		t.Errorf("expected %d buckets, got %d", BucketArraySize, len(inflows))
	}
	if inflows[0] != 2000.0 {
		t.Errorf("expected inflows[0] = 2000, got %f", inflows[0])
	}
}

func TestCalculateInflowsHandlesMultipleBlockHeights(t *testing.T) {
	now := time.Now()

	b1 := make([]float64, BucketArraySize)
	b2 := make([]float64, BucketArraySize)
	b3 := make([]float64, BucketArraySize)
	b4 := make([]float64, BucketArraySize)
	b5 := make([]float64, BucketArraySize)
	b6 := make([]float64, BucketArraySize)
	for i := range b1 {
		b1[i] = 1000.0
		b2[i] = 500.0
		b3[i] = 2000.0
		b4[i] = 2000.0
		b5[i] = 1500.0
		b6[i] = 3000.0
	}

	snapshots := []MempoolSnapshotBuckets{
		{Timestamp: now, BlockHeight: 100, Buckets: b1},
		{Timestamp: now.Add(100 * time.Second), BlockHeight: 100, Buckets: b2},
		{Timestamp: now.Add(200 * time.Second), BlockHeight: 100, Buckets: b3},
		{Timestamp: now.Add(300 * time.Second), BlockHeight: 101, Buckets: b4},
		{Timestamp: now.Add(400 * time.Second), BlockHeight: 101, Buckets: b5},
		{Timestamp: now.Add(500 * time.Second), BlockHeight: 101, Buckets: b6},
	}

	inflows := CalculateInflows(snapshots, 10*time.Minute)

	if len(inflows) != BucketArraySize {
		t.Errorf("expected %d buckets, got %d", BucketArraySize, len(inflows))
	}
	if inflows[0] != 3000.0 {
		t.Errorf("expected inflows[0] = 3000, got %f", inflows[0])
	}
}

func TestCalculateInflowsObservationBoundaries(t *testing.T) {
	type observation struct {
		minutes int
		height  int
		weight  int64
	}
	tests := []struct {
		name      string
		samples   []observation
		timeframe time.Duration
		want      float64
	}{
		{
			name: "repeated height after reorganization",
			samples: []observation{
				{0, 100, 100}, {1, 100, 200},
				{2, 101, 10}, {3, 101, 110},
				{4, 100, 5000}, {5, 100, 5100},
			},
			timeframe: 10 * time.Minute,
			want:      1000,
		},
		{
			name: "no inflow across different heights",
			samples: []observation{
				{0, 100, 100}, {1, 102, 1000}, {2, 101, 2000},
			},
			timeframe: 10 * time.Minute,
			want:      0,
		},
		{
			name: "equal timestamps give no elapsed observation time",
			samples: []observation{
				{0, 100, 100}, {0, 100, 1000},
			},
			timeframe: 10 * time.Minute,
			want:      0,
		},
		{
			name: "zero duration group does not inflate measured group",
			samples: []observation{
				{0, 100, 100}, {0, 100, 1000},
				{1, 101, 100}, {2, 101, 200},
			},
			timeframe: 10 * time.Minute,
			want:      1000,
		},
		{
			name: "window start is inclusive and older observations excluded",
			samples: []observation{
				{0, 100, 10000}, {1, 100, 100}, {6, 100, 600},
			},
			timeframe: 5 * time.Minute,
			want:      1000,
		},
		{
			name: "negative growth contributes time but no inflow",
			samples: []observation{
				{0, 100, 1000}, {1, 100, 100},
				{2, 101, 100}, {3, 101, 300},
			},
			timeframe: 10 * time.Minute,
			want:      1000,
		},
		{
			name:      "zero timeframe",
			samples:   []observation{{0, 100, 100}, {1, 100, 200}},
			timeframe: 0,
			want:      0,
		},
		{
			name:      "negative timeframe",
			samples:   []observation{{0, 100, 100}, {1, 100, 200}},
			timeframe: -time.Minute,
			want:      0,
		},
	}
	base := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			snapshots := make([]MempoolSnapshotBuckets, len(tt.samples))
			for i, sample := range tt.samples {
				snapshots[i] = NewMempoolSnapshotBuckets(
					base.Add(time.Duration(sample.minutes)*time.Minute), sample.height,
					map[int]int64{BucketMax: sample.weight},
				)
			}
			inflows := CalculateInflows(snapshots, tt.timeframe)
			if inflows[0] != tt.want {
				t.Fatalf("inflow = %v, want %v", inflows[0], tt.want)
			}
			for i, value := range inflows[1:] {
				if value != 0 {
					t.Fatalf("unused bucket %d: got inflow %v, want 0", i+1, value)
				}
			}
		})
	}
}

func TestCalculateInflowsSubsecondIntervals(t *testing.T) {
	base := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	snapshots := []MempoolSnapshotBuckets{
		NewMempoolSnapshotBuckets(base, 100, map[int]int64{0: 100}),
		NewMempoolSnapshotBuckets(base.Add(500*time.Millisecond), 100, map[int]int64{0: 110}),
	}
	inflows := CalculateInflows(snapshots, time.Minute)
	if got := inflows[BucketMax]; got != 12000 {
		t.Fatalf("inflow = %v, want 12000", got)
	}
}

func TestCalculateInflowsUsesOptionalBlockHashes(t *testing.T) {
	base := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	for _, tt := range []struct {
		name   string
		hashes []string
		want   float64
	}{
		{"same-height replacement splits observations", []string{"a", "a", "b", "b"}, 500},
		{"empty hashes retain height-only behavior", []string{"", "", "", ""}, 1000},
		{"unchanged hash retains all observations", []string{"a", "a", "a", "a"}, 1000},
	} {
		t.Run(tt.name, func(t *testing.T) {
			minutes := []int{0, 1, 2, 5}
			weights := []int64{100, 200, 500, 600}
			snapshots := make([]MempoolSnapshotBuckets, len(minutes))
			for i, minute := range minutes {
				snapshots[i] = NewMempoolSnapshotBuckets(
					base.Add(time.Duration(minute)*time.Minute), 100,
					map[int]int64{0: weights[i]},
				)
				snapshots[i].BlockHash = tt.hashes[i]
			}
			if got := CalculateInflows(snapshots, 10*time.Minute)[BucketMax]; got != tt.want {
				t.Fatalf("inflow = %v, want %v", got, tt.want)
			}
		})
	}
}
