package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"time"

	augur "github.com/lnliz/go-bitcoin-augur"
)

type MempoolPersistence struct{ dataDirectory string }

func NewMempoolPersistence(cfg PersistenceConfig) (*MempoolPersistence, error) {
	if err := os.MkdirAll(cfg.DataDirectory, 0755); err != nil {
		return nil, fmt.Errorf("create snapshot directory: %w", err)
	}
	return &MempoolPersistence{dataDirectory: cfg.DataDirectory}, nil
}

type snapshotJSON struct {
	BlockHash       string        `json:"blockHash,omitempty"`
	BlockHeight     *int          `json:"blockHeight"`
	Timestamp       time.Time     `json:"timestamp"`
	BucketedWeights map[int]int64 `json:"bucketedWeights"`
}

func (p *MempoolPersistence) SaveSnapshot(snapshot augur.MempoolSnapshot) error {
	if err := snapshot.Validate(); err != nil {
		return fmt.Errorf("invalid snapshot: %w", err)
	}
	dateDir := filepath.Join(p.dataDirectory, snapshot.Timestamp.UTC().Format("2006-01-02"))
	if err := os.MkdirAll(dateDir, 0755); err != nil {
		return err
	}
	data, err := json.Marshal(snapshotJSON{BlockHash: snapshot.BlockHash, BlockHeight: &snapshot.BlockHeight, Timestamp: snapshot.Timestamp, BucketedWeights: snapshot.BucketedWeights})
	if err != nil {
		return err
	}
	// Readers see either the complete old snapshot or the complete replacement.
	// Temporary files have no .json suffix and are ignored by GetSnapshots.
	file, err := os.CreateTemp(dateDir, ".snapshot-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if _, err := file.Write(data); err != nil {
		file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(file.Name(), filepath.Join(dateDir, formatSnapshotFilename(snapshot.BlockHeight, snapshot.Timestamp)))
}

func (p *MempoolPersistence) GetSnapshots(startTime, endTime time.Time) ([]augur.MempoolSnapshot, error) {
	if startTime.After(endTime) {
		return nil, fmt.Errorf("snapshot range start is after end")
	}
	var snapshots []augur.MempoolSnapshot
	currentDate := startTime.UTC().Truncate(24 * time.Hour)
	endDate := endTime.UTC().Truncate(24 * time.Hour)
	for !currentDate.After(endDate) {
		dateDir := filepath.Join(p.dataDirectory, currentDate.Format("2006-01-02"))
		entries, err := os.ReadDir(dateDir)
		if err != nil && !os.IsNotExist(err) {
			return nil, fmt.Errorf("read snapshots: %w", err)
		}
		for _, entry := range entries {
			if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
				continue
			}
			path := filepath.Join(dateDir, entry.Name())
			data, err := os.ReadFile(path)
			if err != nil {
				return nil, fmt.Errorf("read snapshot %s: %w", path, err)
			}
			var stored snapshotJSON
			if err := json.Unmarshal(data, &stored); err != nil {
				return nil, fmt.Errorf("decode snapshot %s: %w", path, err)
			}
			if stored.BlockHeight == nil {
				return nil, fmt.Errorf("invalid snapshot %s: missing blockHeight", path)
			}
			snapshot := augur.MempoolSnapshot{BlockHash: stored.BlockHash, BlockHeight: *stored.BlockHeight, Timestamp: stored.Timestamp, BucketedWeights: stored.BucketedWeights}
			if err := snapshot.Validate(); err != nil {
				return nil, fmt.Errorf("invalid snapshot %s: %w", path, err)
			}
			if !snapshot.Timestamp.Before(startTime) && !snapshot.Timestamp.After(endTime) {
				snapshots = append(snapshots, snapshot)
			}
		}
		currentDate = currentDate.AddDate(0, 0, 1)
	}
	sort.Slice(snapshots, func(i, j int) bool { return snapshots[i].Timestamp.Before(snapshots[j].Timestamp) })
	return snapshots, nil
}

func formatSnapshotFilename(blockHeight int, timestamp time.Time) string {
	return strconv.Itoa(blockHeight) + "_" + strconv.FormatInt(timestamp.Unix(), 10) + "_" + strconv.Itoa(timestamp.Nanosecond()) + ".json"
}
