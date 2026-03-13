package augur

import (
	"time"

	"github.com/lnliz/go-bitcoin-augur/internal"
)

const WUPerByte = 4.0

type MempoolTransaction struct {
	Weight int64
	Fee    int64
}

func (tx MempoolTransaction) FeeRate() float64 {
	return float64(tx.Fee) * WUPerByte / float64(tx.Weight)
}

func (tx MempoolTransaction) GetWeight() int64 {
	return tx.Weight
}

type MempoolSnapshot struct {
	BlockHeight     int
	Timestamp       time.Time
	BucketedWeights map[int]int64
}

func NewMempoolSnapshotFromTransactions(transactions []MempoolTransaction, blockHeight int, timestamp time.Time) MempoolSnapshot {
	bucketedWeights := internal.CreateFeeRateBuckets(transactions)
	return MempoolSnapshot{
		BlockHeight:     blockHeight,
		Timestamp:       timestamp,
		BucketedWeights: bucketedWeights,
	}
}

func NewEmptyMempoolSnapshot(blockHeight int, timestamp time.Time) MempoolSnapshot {
	return MempoolSnapshot{
		BlockHeight:     blockHeight,
		Timestamp:       timestamp,
		BucketedWeights: make(map[int]int64),
	}
}
