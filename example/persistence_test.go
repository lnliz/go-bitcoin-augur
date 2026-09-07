package main

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	augur "github.com/lnliz/go-bitcoin-augur"
)

func TestPersistenceSaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()
	persistence, err := NewMempoolPersistence(PersistenceConfig{DataDirectory: tmpDir})
	if err != nil {
		t.Fatal(err)
	}

	now := time.Now().Truncate(time.Second)
	snapshot := augur.MempoolSnapshot{
		BlockHeight: 800000,
		BlockHash:   "tip",
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

	dateDir := filepath.Join(tmpDir, now.UTC().Format("2006-01-02"))
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
	if loaded.BlockHash != snapshot.BlockHash {
		t.Errorf("lost block hash %q", loaded.BlockHash)
	}
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
	persistence, err := NewMempoolPersistence(PersistenceConfig{DataDirectory: tmpDir})
	if err != nil {
		t.Fatal(err)
	}

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
	persistence, err := NewMempoolPersistence(PersistenceConfig{DataDirectory: tmpDir})
	if err != nil {
		t.Fatal(err)
	}

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
	persistence, err := NewMempoolPersistence(PersistenceConfig{DataDirectory: tmpDir})
	if err != nil {
		t.Fatal(err)
	}

	now := time.Now()
	snapshots, err := persistence.GetSnapshots(now.Add(-1*time.Hour), now)
	if err != nil {
		t.Fatalf("failed to get snapshots: %v", err)
	}

	if len(snapshots) != 0 {
		t.Errorf("expected 0 snapshots, got %d", len(snapshots))
	}
}

func TestPersistenceRejectsCorruptSnapshots(t *testing.T) {
	now := time.Now().UTC()
	for _, body := range []string{`{"timestamp":"2025-01-01T00:00:00Z","bucketedWeights":{}}`, `{"blockHeight":null,"timestamp":"2025-01-01T00:00:00Z","bucketedWeights":{}}`, `{`, `{"blockHeight":1,"timestamp":"2025-01-01T00:00:00Z","bucketedWeights":{"invalid":100}}`, `{"blockHeight":1,"timestamp":"2025-01-01T00:00:00Z","bucketedWeights":{"1":-1}}`, `{"blockHeight":1,"bucketedWeights":{}}`} {
		t.Run(body, func(t *testing.T) {
			p, err := NewMempoolPersistence(PersistenceConfig{DataDirectory: t.TempDir()})
			if err != nil {
				t.Fatal(err)
			}
			dir := filepath.Join(p.dataDirectory, now.Format("2006-01-02"))
			if err := os.MkdirAll(dir, 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "bad.json"), []byte(body), 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := p.GetSnapshots(now, now); err == nil {
				t.Fatal("silently accepted corrupt snapshot")
			}
		})
	}
}

func TestPersistenceSubsecondSnapshots(t *testing.T) {
	p, err := NewMempoolPersistence(PersistenceConfig{DataDirectory: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Second)
	for _, timestamp := range []time.Time{now, now.Add(time.Nanosecond)} {
		if err := p.SaveSnapshot(augur.NewEmptyMempoolSnapshot(1, timestamp)); err != nil {
			t.Fatal(err)
		}
	}
	snapshots, err := p.GetSnapshots(now, now.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshots) != 2 {
		t.Fatalf("lost subsecond snapshot: %v", snapshots)
	}
}

func TestPersistenceReturnsDirectoryErrors(t *testing.T) {
	root := t.TempDir()
	now := time.Now().UTC()
	if err := os.WriteFile(filepath.Join(root, now.Format("2006-01-02")), nil, 0600); err != nil {
		t.Fatal(err)
	}
	p, err := NewMempoolPersistence(PersistenceConfig{DataDirectory: root})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.GetSnapshots(now, now); err == nil {
		t.Fatal("ignored invalid snapshot directory")
	}
	if _, err := p.GetSnapshots(now, now.Add(-time.Second)); err == nil {
		t.Fatal("accepted reversed time range")
	}
	if _, err := NewMempoolPersistence(PersistenceConfig{DataDirectory: filepath.Join(root, now.Format("2006-01-02"))}); err == nil {
		t.Fatal("accepted file as persistence directory")
	}
}

func TestPersistenceConcurrentReadersSeeCompleteSnapshots(t *testing.T) {
	p, err := NewMempoolPersistence(PersistenceConfig{DataDirectory: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	snapshot := augur.MempoolSnapshot{BlockHeight: 1, Timestamp: now, BucketedWeights: map[int]int64{1: 100}}
	if err := p.SaveSnapshot(snapshot); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	wg.Go(func() {
		for range 20 {
			if err := p.SaveSnapshot(snapshot); err != nil {
				t.Error(err)
				return
			}
		}
	})
	for range 4 {
		wg.Go(func() {
			for range 20 {
				loaded, err := p.GetSnapshots(now, now)
				if err != nil {
					t.Error(err)
					return
				}
				if len(loaded) != 1 || loaded[0].BucketedWeights[1] != 100 {
					t.Errorf("partial snapshot: %v", loaded)
					return
				}
			}
		})
	}
	wg.Wait()
}

func TestPersistenceAllowsExplicitGenesisHeight(t *testing.T) {
	p, err := NewMempoolPersistence(PersistenceConfig{DataDirectory: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	if err := p.SaveSnapshot(augur.NewEmptyMempoolSnapshot(0, now)); err != nil {
		t.Fatal(err)
	}
	loaded, err := p.GetSnapshots(now, now)
	if err != nil || len(loaded) != 1 || loaded[0].BlockHeight != 0 {
		t.Fatalf("loaded=%v err=%v", loaded, err)
	}
}
