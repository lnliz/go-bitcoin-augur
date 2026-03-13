package augur

import (
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

	snap := NewMempoolSnapshotFromTransactions(txs, 800000, now)

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

	snap := NewMempoolSnapshotFromTransactions(txs, 1, now)

	var totalWeight int64
	for _, w := range snap.BucketedWeights {
		totalWeight += w
	}
	if totalWeight != 3000 {
		t.Errorf("same-fee-rate txs should aggregate: got %d, want 3000", totalWeight)
	}
}

func TestNewMempoolSnapshotFromTransactionsEmpty(t *testing.T) {
	snap := NewMempoolSnapshotFromTransactions(nil, 1, time.Now())

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
