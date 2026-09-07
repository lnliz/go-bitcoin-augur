package augur

import (
	"math"
	"testing"
	"time"
)

func TestMempoolTransactionFeeRate(t *testing.T) {
	tests := []struct {
		name     string
		weight   int64
		fee      int64
		expected float64
	}{
		{"standard tx", 1000, 250, 1.0},
		{"higher fee", 1000, 500, 2.0},
		{"large tx", 4000, 1000, 1.0},
		{"high fee rate", 400, 1000, 10.0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tx := MempoolTransaction{Weight: tc.weight, Fee: tc.fee}
			got := tx.FeeRate()
			if got != tc.expected {
				t.Errorf("FeeRate() = %f, want %f", got, tc.expected)
			}
		})
	}
}

func TestMempoolTransactionGetWeight(t *testing.T) {
	tx := MempoolTransaction{Weight: 5000, Fee: 100}
	if tx.GetWeight() != 5000 {
		t.Errorf("GetWeight() = %d, want 5000", tx.GetWeight())
	}
}

func TestNewMempoolSnapshotFromTransactions(t *testing.T) {
	now := time.Now()
	txs := []MempoolTransaction{
		{Weight: 1000, Fee: 250},
		{Weight: 1000, Fee: 500},
		{Weight: 1000, Fee: 250},
	}

	snap, err := NewMempoolSnapshotFromTransactions(txs, 800000, now)
	if err != nil {
		t.Fatal(err)
	}

	if snap.BlockHeight != 800000 {
		t.Errorf("BlockHeight = %d, want 800000", snap.BlockHeight)
	}
	if !snap.Timestamp.Equal(now) {
		t.Errorf("Timestamp mismatch")
	}
	if len(snap.BucketedWeights) == 0 {
		t.Error("BucketedWeights should not be empty")
	}

	var totalWeight int64
	for _, w := range snap.BucketedWeights {
		totalWeight += w
	}
	if totalWeight != 3000 {
		t.Errorf("total bucketed weight = %d, want 3000", totalWeight)
	}
}

func TestNewMempoolSnapshotFromTransactionsSameFeeRate(t *testing.T) {
	now := time.Now()
	txs := []MempoolTransaction{
		{Weight: 1000, Fee: 250},
		{Weight: 2000, Fee: 500},
	}

	snap, err := NewMempoolSnapshotFromTransactions(txs, 1, now)
	if err != nil {
		t.Fatal(err)
	}

	var totalWeight int64
	for _, w := range snap.BucketedWeights {
		totalWeight += w
	}
	if totalWeight != 3000 {
		t.Errorf("same-fee-rate txs should aggregate: got %d, want 3000", totalWeight)
	}
}

func TestNewMempoolSnapshotFromTransactionsEmpty(t *testing.T) {
	snap, err := NewMempoolSnapshotFromTransactions(nil, 1, time.Now())
	if err != nil {
		t.Fatal(err)
	}

	if snap.BlockHeight != 1 {
		t.Errorf("BlockHeight = %d, want 1", snap.BlockHeight)
	}
	if len(snap.BucketedWeights) != 0 {
		t.Errorf("empty tx list should produce empty buckets, got %d", len(snap.BucketedWeights))
	}
}

func TestNewEmptyMempoolSnapshot(t *testing.T) {
	now := time.Now()
	snap := NewEmptyMempoolSnapshot(42, now)

	if snap.BlockHeight != 42 {
		t.Errorf("BlockHeight = %d, want 42", snap.BlockHeight)
	}
	if !snap.Timestamp.Equal(now) {
		t.Errorf("Timestamp mismatch")
	}
	if snap.BucketedWeights == nil {
		t.Error("BucketedWeights should be initialized, not nil")
	}
	if len(snap.BucketedWeights) != 0 {
		t.Errorf("BucketedWeights should be empty, got %d entries", len(snap.BucketedWeights))
	}
}

func TestFeeRateFormula(t *testing.T) {
	tx := MempoolTransaction{Weight: 800, Fee: 400}
	expected := float64(400) * WUPerByte / float64(800)
	got := tx.FeeRate()
	if got != expected {
		t.Errorf("FeeRate() = %f, want %f", got, expected)
	}
}

func TestSnapshotConstructorRejectsInvalidInput(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for _, tx := range []MempoolTransaction{{Weight: 0, Fee: 1}, {Weight: -1, Fee: 1}, {Weight: 4, Fee: -1}} {
		if _, err := NewMempoolSnapshotFromTransactions([]MempoolTransaction{tx}, 1, now); err == nil {
			t.Errorf("accepted invalid transaction %+v", tx)
		}
	}
	for _, metadata := range []MempoolSnapshot{{BlockHeight: -1, Timestamp: now}, {BlockHeight: 1}} {
		if _, err := NewMempoolSnapshotFromTransactions(nil, metadata.BlockHeight, metadata.Timestamp); err == nil {
			t.Error("accepted invalid snapshot metadata")
		}
	}
	// Both entries map to the same positive-rate bucket, and their sum exceeds int64.
	tx := MempoolTransaction{Weight: math.MaxInt64, Fee: math.MaxInt64}
	if _, err := NewMempoolSnapshotFromTransactions([]MempoolTransaction{tx, tx}, 1, now); err == nil {
		t.Error("accepted bucket weight overflow")
	}
}

func TestZeroFeeAndFractionalVirtualSize(t *testing.T) {
	tx := MempoolTransaction{Weight: 401, Fee: 100}
	if got, want := tx.FeeRate(), 400.0/401; got != want {
		t.Errorf("continuous weight-based rate = %v, want %v", got, want)
	}
	snapshot, err := NewMempoolSnapshotFromTransactions([]MempoolTransaction{{Weight: 400, Fee: 0}}, 1, time.Now())
	if err != nil || len(snapshot.BucketedWeights) != 0 {
		t.Fatalf("zero fee snapshot: %+v, %v", snapshot, err)
	}
	if got := (MempoolTransaction{Weight: 0, Fee: 1}).FeeRate(); !math.IsNaN(got) {
		t.Fatalf("invalid fee rate = %v", got)
	}
}

func TestSnapshotRejectsOverflowWhenFoldingHighFeeBuckets(t *testing.T) {
	for _, weights := range []map[int]int64{
		{1000: math.MaxInt64, 1001: 1},
		{1001: math.MaxInt64, math.MaxInt: 1},
	} {
		snapshot := NewEmptyMempoolSnapshot(100, time.Now())
		snapshot.BucketedWeights = weights
		if err := snapshot.Validate(); err == nil {
			t.Fatalf("accepted overflowing high fee buckets: %v", weights)
		}
		if _, err := mustEstimator(t).CalculateEstimates([]MempoolSnapshot{snapshot}); err == nil {
			t.Fatalf("estimator accepted overflowing high fee buckets: %v", weights)
		}
	}
	snapshot := NewEmptyMempoolSnapshot(100, time.Now())
	snapshot.BucketedWeights = map[int]int64{1000: math.MaxInt64 - 1, 1001: 1, 999: math.MaxInt64}
	if err := snapshot.Validate(); err != nil {
		t.Fatalf("rejected separate buckets and highest bucket at int64 limit: %v", err)
	}
}
