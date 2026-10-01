package main

import (
	"context"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	augur "github.com/lnliz/go-bitcoin-augur"
)

const maxEstimateAge = 2 * time.Minute

type mempoolSource interface {
	GetMempool(context.Context) (mempoolObservation, error)
}

type snapshotStore interface {
	SaveSnapshot(augur.MempoolSnapshot) error
	GetSnapshots(time.Time, time.Time) ([]augur.MempoolSnapshot, error)
}

// collectedEstimates is immutable after publication; HTTP and metrics readers
// share the estimate and the exact observations that produced it.
type collectedEstimates struct {
	estimate  augur.FeeEstimate
	snapshots []augur.MempoolSnapshot
}

type MempoolCollector struct {
	bitcoinClient      mempoolSource
	persistence        snapshotStore
	feeEstimator       *augur.FeeEstimator
	collectionInterval time.Duration
	latest             atomic.Pointer[collectedEstimates]
	history            []augur.MempoolSnapshot
	historyEnd         time.Time
	lifecycle          sync.Mutex
	cancel             context.CancelFunc
	done               chan struct{}
}

func NewMempoolCollector(bitcoinClient mempoolSource, persistence snapshotStore, feeEstimator *augur.FeeEstimator) *MempoolCollector {
	return &MempoolCollector{bitcoinClient: bitcoinClient, persistence: persistence, feeEstimator: feeEstimator, collectionInterval: 30 * time.Second}
}

func (c *MempoolCollector) Start() {
	c.lifecycle.Lock()
	defer c.lifecycle.Unlock()
	if c.cancel != nil {
		return
	}
	c.history = nil
	c.historyEnd = time.Time{}
	ctx, cancel := context.WithCancel(context.Background())
	c.cancel = cancel
	c.done = make(chan struct{})
	log.Printf("Starting mempool data collection every %s", c.collectionInterval)
	go func() {
		defer close(c.done)
		ticker := time.NewTicker(c.collectionInterval)
		defer ticker.Stop()
		for {
			if ctx.Err() != nil {
				return
			}
			if err := c.updateFeeEstimates(ctx); err != nil && ctx.Err() == nil {
				log.Printf("Error updating fee estimates: %v", err)
			}
			select {
			case <-ticker.C:
			case <-ctx.Done():
				return
			}
		}
	}()
}

func (c *MempoolCollector) Stop() {
	c.lifecycle.Lock()
	defer c.lifecycle.Unlock()
	if c.cancel == nil {
		return
	}
	c.cancel()
	<-c.done
	c.cancel = nil
}

func freshEstimate(estimate *augur.FeeEstimate, now time.Time) bool {
	return estimate != nil && !estimate.Timestamp.After(now) && now.Sub(estimate.Timestamp) <= maxEstimateAge
}

func (c *MempoolCollector) GetLatestFeeEstimate() *augur.FeeEstimate {
	state := c.latest.Load()
	if state == nil || !freshEstimate(&state.estimate, time.Now()) {
		return nil
	}
	return &state.estimate
}

func (c *MempoolCollector) GetFeeEstimateForTimestamp(unixTimestamp int64) (*augur.FeeEstimate, error) {
	return c.estimateAt(time.Unix(unixTimestamp, 0))
}

func (c *MempoolCollector) GetLatestFeeEstimateForBlockTarget(numOfBlocks float64) (*augur.FeeEstimate, error) {
	state := c.latest.Load()
	if state == nil || !freshEstimate(&state.estimate, time.Now()) {
		return nil, nil
	}
	estimate, err := c.feeEstimator.CalculateEstimatesForBlocks(state.snapshots, &numOfBlocks)
	if err != nil {
		return nil, err
	}
	return &estimate, nil
}

func (c *MempoolCollector) estimateAt(target time.Time) (*augur.FeeEstimate, error) {
	snapshots, err := c.persistence.GetSnapshots(target.Add(-24*time.Hour), target)
	if err != nil {
		return nil, err
	}
	if len(snapshots) == 0 {
		return nil, nil
	}
	estimate, err := c.feeEstimator.CalculateEstimates(snapshots)
	if err != nil {
		return nil, err
	}
	// A historical lookup must also be close to its requested time; otherwise
	// a day-old snapshot could be presented as a current estimate.
	if !freshEstimate(&estimate, target) {
		return nil, nil
	}
	return &estimate, nil
}

func (c *MempoolCollector) updateFeeEstimates(ctx context.Context) error {
	started := time.Now()
	observation, err := c.bitcoinClient.GetMempool(ctx)
	if err != nil {
		return err
	}
	// RPC transport and tip checks may take many seconds. Dating the observation
	// from the start keeps those delays from making old data appear fresh.
	snapshot, err := augur.NewMempoolSnapshotFromTransactions(observation.Transactions, observation.BlockHeight, started)
	if err != nil {
		return fmt.Errorf("create snapshot: %w", err)
	}
	snapshot.BlockHash = observation.BlockHash
	// Persisted timestamps have no monotonic clock component. Keep cached
	// observations comparable in the same way as observations read from disk.
	snapshot.Timestamp = snapshot.Timestamp.Round(0)
	if err := c.persistence.SaveSnapshot(snapshot); err != nil {
		return fmt.Errorf("save snapshot: %w", err)
	}
	start := snapshot.Timestamp.Add(-24 * time.Hour)
	var snapshots []augur.MempoolSnapshot
	if c.history == nil || !snapshot.Timestamp.After(c.historyEnd) || c.history[len(c.history)-1].Timestamp.Before(c.historyEnd) {
		// Reload through clock recovery, including its first forward poll, so
		// previously saved future observations reenter the window.
		c.history = nil
		snapshots, err = c.persistence.GetSnapshots(start, snapshot.Timestamp)
		if err != nil {
			return fmt.Errorf("read snapshots: %w", err)
		}
	} else {
		first := 0
		for first < len(c.history) && c.history[first].Timestamp.Before(start) {
			first++
		}
		retained := c.history[first:]
		// Published slices are immutable; only their read-only bucket maps
		// can be shared with the next window.
		snapshots = make([]augur.MempoolSnapshot, len(retained)+1)
		copy(snapshots, retained)
		snapshots[len(retained)] = snapshot
	}
	if len(snapshots) == 0 {
		return fmt.Errorf("saved snapshot was not returned by storage")
	}
	// Keep persisted observations even if estimation fails, so the next poll
	// uses the same history that a fresh disk read would return.
	c.history = snapshots
	if snapshot.Timestamp.After(c.historyEnd) {
		c.historyEnd = snapshot.Timestamp
	}
	estimate, err := c.feeEstimator.CalculateEstimates(snapshots)
	if err != nil {
		return fmt.Errorf("calculate fee estimates: %w", err)
	}
	c.latest.Store(&collectedEstimates{estimate: estimate, snapshots: snapshots})
	log.Printf("Updating fee estimates finished in %.2f seconds", time.Since(started).Seconds())
	return nil
}
