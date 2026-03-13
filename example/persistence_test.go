package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	augur "github.com/lnliz/go-bitcoin-augur"
)

func TestPersistenceSaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()
	persistence := NewMempoolPersistence(PersistenceConfig{DataDirectory: tmpDir})

	now := time.Now().Truncate(time.Second)
	snapshot := augur.MempoolSnapshot{
		BlockHeight: 800000,
		Timestamp:   now,
		BucketedWeights: map[int]int64{
			1:   1000,
			5:   5000,
			10:  10000,
			100: 50000,
		},
	}

	if err := persistence.SaveSnapshot(snapshot); err != nil {
		t.Fatalf("failed to save snapshot: %v", err)
	}

	dateDir := filepath.Join(tmpDir, now.Local().Format("2006-01-02"))
	entries, err := os.ReadDir(dateDir)
	if err != nil {
		t.Fatalf("failed to read date directory: %v", err)
	}
	if len(entries) != 1 {
		t.Errorf("expected 1 snapshot file, got %d", len(entries))
	}

	snapshots, err := persistence.GetSnapshots(now.Add(-1*time.Hour), now.Add(1*time.Hour))
	if err != nil {
		t.Fatalf("failed to get snapshots: %v", err)
	}

	if len(snapshots) != 1 {
		t.Fatalf("expected 1 snapshot, got %d", len(snapshots))
	}

	loaded := snapshots[0]
	if loaded.BlockHeight != snapshot.BlockHeight {
		t.Errorf("expected block height %d, got %d", snapshot.BlockHeight, loaded.BlockHeight)
	}
	if !loaded.Timestamp.Equal(snapshot.Timestamp) {
		t.Errorf("expected timestamp %v, got %v", snapshot.Timestamp, loaded.Timestamp)
	}
	if len(loaded.BucketedWeights) != len(snapshot.BucketedWeights) {
		t.Errorf("expected %d buckets, got %d", len(snapshot.BucketedWeights), len(loaded.BucketedWeights))
	}
	for k, v := range snapshot.BucketedWeights {
		if loaded.BucketedWeights[k] != v {
			t.Errorf("bucket %d: expected %d, got %d", k, v, loaded.BucketedWeights[k])
		}
	}
}

func TestPersistenceMultipleSnapshots(t *testing.T) {
	tmpDir := t.TempDir()
	persistence := NewMempoolPersistence(PersistenceConfig{DataDirectory: tmpDir})

	baseTime := time.Now().Truncate(time.Second)
	for i := 0; i < 5; i++ {
		snapshot := augur.MempoolSnapshot{
			BlockHeight:     800000 + i,
			Timestamp:       baseTime.Add(time.Duration(i) * time.Minute),
			BucketedWeights: map[int]int64{1: int64(1000 * (i + 1))},
		}
		if err := persistence.SaveSnapshot(snapshot); err != nil {
			t.Fatalf("failed to save snapshot %d: %v", i, err)
		}
	}

	snapshots, err := persistence.GetSnapshots(baseTime.Add(-1*time.Hour), baseTime.Add(1*time.Hour))
	if err != nil {
		t.Fatalf("failed to get snapshots: %v", err)
	}

	if len(snapshots) != 5 {
		t.Fatalf("expected 5 snapshots, got %d", len(snapshots))
	}

	for i := 0; i < len(snapshots)-1; i++ {
		if !snapshots[i].Timestamp.Before(snapshots[i+1].Timestamp) {
			t.Errorf("snapshots not sorted by timestamp")
		}
	}
}

func TestPersistenceTimeRangeFilter(t *testing.T) {
	tmpDir := t.TempDir()
	persistence := NewMempoolPersistence(PersistenceConfig{DataDirectory: tmpDir})

	baseTime := time.Now().Truncate(time.Second)

	for i := 0; i < 10; i++ {
		snapshot := augur.MempoolSnapshot{
			BlockHeight:     800000 + i,
			Timestamp:       baseTime.Add(time.Duration(i) * time.Minute),
			BucketedWeights: map[int]int64{1: 1000},
		}
		if err := persistence.SaveSnapshot(snapshot); err != nil {
			t.Fatalf("failed to save snapshot %d: %v", i, err)
		}
	}

	snapshots, err := persistence.GetSnapshots(baseTime.Add(2*time.Minute), baseTime.Add(5*time.Minute))
	if err != nil {
		t.Fatalf("failed to get snapshots: %v", err)
	}

	if len(snapshots) != 4 {
		t.Errorf("expected 4 snapshots in range, got %d", len(snapshots))
	}
}

func TestPersistenceEmptyDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	persistence := NewMempoolPersistence(PersistenceConfig{DataDirectory: tmpDir})

	now := time.Now()
	snapshots, err := persistence.GetSnapshots(now.Add(-1*time.Hour), now)
	if err != nil {
		t.Fatalf("failed to get snapshots: %v", err)
	}

	if len(snapshots) != 0 {
		t.Errorf("expected 0 snapshots, got %d", len(snapshots))
	}
}
