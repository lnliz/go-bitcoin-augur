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
