package main

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"time"

	augur "github.com/lnliz/go-bitcoin-augur"
)

type MempoolPersistence struct {
	dataDirectory string
}

func NewMempoolPersistence(cfg PersistenceConfig) *MempoolPersistence {
	log.Printf("Initializing mempool persistence with data directory: %s", cfg.DataDirectory)
	if err := os.MkdirAll(cfg.DataDirectory, 0755); err != nil {
		log.Printf("Warning: failed to create data directory: %v", err)
	}
	return &MempoolPersistence{
		dataDirectory: cfg.DataDirectory,
	}
}

type snapshotJSON struct {
	BlockHeight     int              `json:"blockHeight"`
	Timestamp       time.Time        `json:"timestamp"`
	BucketedWeights map[string]int64 `json:"bucketedWeights"`
}

func (p *MempoolPersistence) SaveSnapshot(snapshot augur.MempoolSnapshot) error {
	utcTime := snapshot.Timestamp.UTC()
	dateStr := utcTime.Format("2006-01-02")
	dateDir := filepath.Join(p.dataDirectory, dateStr)

	if err := os.MkdirAll(dateDir, 0755); err != nil {
		return err
	}

	filename := filepath.Join(dateDir, formatSnapshotFilename(snapshot.BlockHeight, snapshot.Timestamp))
	log.Printf("Saving snapshot to %s", filename)

	sj := snapshotJSON{
		BlockHeight:     snapshot.BlockHeight,
		Timestamp:       snapshot.Timestamp,
		BucketedWeights: make(map[string]int64),
	}
	for k, v := range snapshot.BucketedWeights {
		sj.BucketedWeights[strconv.Itoa(k)] = v
	}

	data, err := json.Marshal(sj)
	if err != nil {
		return err
	}

	return os.WriteFile(filename, data, 0644)
}

func (p *MempoolPersistence) GetSnapshots(startTime, endTime time.Time) ([]augur.MempoolSnapshot, error) {
	log.Printf("Fetching snapshots from %s to %s", startTime.Format(time.RFC3339), endTime.Format(time.RFC3339))

	var snapshots []augur.MempoolSnapshot

	currentDate := startTime.UTC().Truncate(24 * time.Hour)
	endDate := endTime.UTC().Truncate(24 * time.Hour)

	for !currentDate.After(endDate) {
		dateStr := currentDate.Format("2006-01-02")
		dateDir := filepath.Join(p.dataDirectory, dateStr)

		if info, err := os.Stat(dateDir); err == nil && info.IsDir() {
			entries, err := os.ReadDir(dateDir)
			if err != nil {
				log.Printf("Error reading directory %s: %v", dateDir, err)
				currentDate = currentDate.Add(24 * time.Hour)
				continue
			}

			for _, entry := range entries {
				if filepath.Ext(entry.Name()) != ".json" {
					continue
				}

				filePath := filepath.Join(dateDir, entry.Name())
				data, err := os.ReadFile(filePath)
				if err != nil {
					log.Printf("Error reading snapshot file %s: %v", filePath, err)
					continue
				}

				var sj snapshotJSON
				if err := json.Unmarshal(data, &sj); err != nil {
					log.Printf("Error parsing snapshot file %s: %v", filePath, err)
					continue
				}

				if !sj.Timestamp.Before(startTime) && !sj.Timestamp.After(endTime) {
					snapshot := augur.MempoolSnapshot{
						BlockHeight:     sj.BlockHeight,
						Timestamp:       sj.Timestamp,
						BucketedWeights: make(map[int]int64),
					}
					for k, v := range sj.BucketedWeights {
						intKey, _ := strconv.Atoi(k)
						snapshot.BucketedWeights[intKey] = v
					}
					snapshots = append(snapshots, snapshot)
				}
			}
		}

		currentDate = currentDate.Add(24 * time.Hour)
	}

	sort.Slice(snapshots, func(i, j int) bool {
		return snapshots[i].Timestamp.Before(snapshots[j].Timestamp)
	})

	log.Printf("Found %d snapshots in date range", len(snapshots))
	return snapshots, nil
}

func formatSnapshotFilename(blockHeight int, timestamp time.Time) string {
	return strconv.Itoa(blockHeight) + "_" + strconv.FormatInt(timestamp.Unix(), 10) + ".json"
}
