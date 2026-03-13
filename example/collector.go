package main

import (
	"log"
	"sync"
	"sync/atomic"
	"time"

	augur "github.com/lnliz/go-bitcoin-augur"
)

type MempoolCollector struct {
	bitcoinClient      *BitcoinRpcClient
	persistence        *MempoolPersistence
	feeEstimator       *augur.FeeEstimator
	collectionInterval time.Duration
	latestFeeEstimate  atomic.Pointer[augur.FeeEstimate]
	stopCh             chan struct{}
	wg                 sync.WaitGroup
}

func NewMempoolCollector(
	bitcoinClient *BitcoinRpcClient,
	persistence *MempoolPersistence,
	feeEstimator *augur.FeeEstimator,
) *MempoolCollector {
	return &MempoolCollector{
		bitcoinClient:      bitcoinClient,
		persistence:        persistence,
		feeEstimator:       feeEstimator,
		collectionInterval: 30 * time.Second,
	}
}

func (c *MempoolCollector) Start() {
	log.Printf("Starting mempool data collection every %s", c.collectionInterval)
	c.stopCh = make(chan struct{})

	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		ticker := time.NewTicker(c.collectionInterval)
		defer ticker.Stop()

		c.updateFeeEstimates()

		for {
			select {
			case <-ticker.C:
				c.updateFeeEstimates()
			case <-c.stopCh:
				return
			}
		}
	}()
}

func (c *MempoolCollector) Stop() {
	log.Println("Stopping mempool data collection")
	close(c.stopCh)
	c.wg.Wait()
}

func (c *MempoolCollector) GetLatestFeeEstimate() *augur.FeeEstimate {
	return c.latestFeeEstimate.Load()
}

func (c *MempoolCollector) GetFeeEstimateForTimestamp(unixTimestamp int64) (*augur.FeeEstimate, error) {
	targetTime := time.Unix(unixTimestamp, 0)
	startTime := targetTime.Add(-24 * time.Hour)

	log.Printf("Fetching snapshots from the last day for timestamp %d", unixTimestamp)
	snapshots, err := c.persistence.GetSnapshots(startTime, targetTime)
	if err != nil {
		return nil, err
	}
	log.Printf("Retrieved %d snapshots from the last day", len(snapshots))

	if len(snapshots) > 0 {
		log.Println("Calculating fee estimates")
		estimate, err := c.feeEstimator.CalculateEstimates(snapshots)
		if err != nil {
			return nil, err
		}
		return &estimate, nil
	}

	log.Println("No snapshots available for fee estimation")
	emptyEstimate := augur.FeeEstimate{
		Estimates: make(map[int]augur.BlockTarget),
		Timestamp: targetTime,
	}
	return &emptyEstimate, nil
}

func (c *MempoolCollector) GetLatestFeeEstimateForBlockTarget(numOfBlocks float64) (*augur.FeeEstimate, error) {
	now := time.Now()
	startTime := now.Add(-24 * time.Hour)

	log.Println("Fetching snapshots from the last day")
	snapshots, err := c.persistence.GetSnapshots(startTime, now)
	if err != nil {
		return nil, err
	}
	log.Printf("Retrieved %d snapshots from the last day", len(snapshots))

	if len(snapshots) > 0 {
		log.Println("Calculating fee estimates")
		estimate, err := c.feeEstimator.CalculateEstimatesForBlocks(snapshots, &numOfBlocks)
		if err != nil {
			return nil, err
		}
		return &estimate, nil
	}

	log.Println("No snapshots available for fee estimation")
	emptyEstimate := augur.FeeEstimate{
		Estimates: make(map[int]augur.BlockTarget),
		Timestamp: time.Unix(0, 0),
	}
	return &emptyEstimate, nil
}

func (c *MempoolCollector) updateFeeEstimates() {
	startTime := time.Now()
	log.Println("Collecting mempool data")

	blockHeight, transactions, err := c.bitcoinClient.GetHeightAndMempoolTransactions()
	if err != nil {
		log.Printf("Error fetching mempool data: %v", err)
		return
	}
	log.Printf("Got mempool data: %d transactions at height %d", len(transactions), blockHeight)

	snapshot := augur.NewMempoolSnapshotFromTransactions(transactions, blockHeight, time.Now())

	if err := c.persistence.SaveSnapshot(snapshot); err != nil {
		log.Printf("Error saving snapshot: %v", err)
	} else {
		log.Printf("Mempool snapshot saved: %d transactions at height %d", len(transactions), blockHeight)
	}

	now := time.Now()
	dayAgo := now.Add(-24 * time.Hour)

	log.Println("Fetching snapshots from the last day")
	snapshots, err := c.persistence.GetSnapshots(dayAgo, now)
	if err != nil {
		log.Printf("Error fetching snapshots: %v", err)
		return
	}
	log.Printf("Retrieved %d snapshots from the last day", len(snapshots))

	if len(snapshots) > 0 {
		log.Println("Calculating fee estimates")
		estimate, err := c.feeEstimator.CalculateEstimates(snapshots)
		if err != nil {
			log.Printf("Error calculating fee estimates: %v", err)
			return
		}
		c.latestFeeEstimate.Store(&estimate)
		log.Println("Fee estimates updated")
	} else {
		log.Println("No snapshots available for fee estimation")
	}

	elapsed := time.Since(startTime).Seconds()
	log.Printf("Updating fee estimates finished in %.2f seconds", elapsed)
}
