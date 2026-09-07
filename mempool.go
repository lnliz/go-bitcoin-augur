package augur

import (
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/lnliz/go-bitcoin-augur/internal"
)

// WUPerByte converts virtual bytes to weight units in the estimation model.
const WUPerByte = 4.0

// MempoolTransaction contains a positive weight in WU and a nonnegative fee in satoshis.
type MempoolTransaction struct {
	Weight int64
	Fee    int64
}

// FeeRate returns fee / (weight / 4), in satoshis per virtual byte. This is
// Augur's continuous weight-based model, without integer virtual-size rounding.
// Invalid transactions return NaN and are rejected by snapshot construction.
func (tx MempoolTransaction) FeeRate() float64 {
	if tx.Weight <= 0 || tx.Fee < 0 {
		return math.NaN()
	}
	return float64(tx.Fee) * WUPerByte / float64(tx.Weight)
}

func (tx MempoolTransaction) GetWeight() int64 {
	return tx.Weight
}

// MempoolSnapshot records a mempool at one observation time. Bucket keys are
// round-to-even(100 * ln(fee rate)); values are summed weight units. Maps remain caller-owned.
type MempoolSnapshot struct {
	BlockHeight int
	// BlockHash optionally identifies the observed tip. Supply it to separate
	// same-height reorgs in inflow history; an empty hash supports height-only data.
	BlockHash       string
	Timestamp       time.Time
	BucketedWeights map[int]int64
}

// NewMempoolSnapshotFromTransactions validates and aggregates transactions.
// Zero-fee transactions do not affect the modeled fee range and are omitted.
// Invalid metadata, invalid transactions, and bucket weight overflow return errors.
func NewMempoolSnapshotFromTransactions(transactions []MempoolTransaction, blockHeight int, timestamp time.Time) (MempoolSnapshot, error) {
	snapshot := NewEmptyMempoolSnapshot(blockHeight, timestamp)
	if err := snapshot.Validate(); err != nil {
		return MempoolSnapshot{}, err
	}
	bucketedWeights, err := internal.CreateFeeRateBuckets(transactions)
	if err != nil {
		return MempoolSnapshot{}, err
	}
	snapshot.BucketedWeights = bucketedWeights
	return snapshot, nil
}

// NewEmptyMempoolSnapshot initializes an empty map. Metadata is validated when
// the snapshot is passed to an estimator or its Validate method is called.
func NewEmptyMempoolSnapshot(blockHeight int, timestamp time.Time) MempoolSnapshot {
	return MempoolSnapshot{
		BlockHeight:     blockHeight,
		Timestamp:       timestamp,
		BucketedWeights: make(map[int]int64),
	}
}

// Validate checks snapshot metadata and nonnegative bucket weights. Nil buckets
// represent an empty mempool. Below-range buckets are ignored by estimation;
// above-range buckets are folded into the highest modeled bucket. Their combined
// weight, including the highest bucket itself, must fit in int64.
func (s MempoolSnapshot) Validate() error {
	if s.BlockHeight < 0 {
		return errors.New("block height must be nonnegative")
	}
	if s.Timestamp.IsZero() {
		return errors.New("snapshot timestamp must not be zero")
	}
	var highestBucketWeight int64
	for bucket, weight := range s.BucketedWeights {
		if weight < 0 {
			return fmt.Errorf("bucket %d has negative weight: %d", bucket, weight)
		}
		if bucket >= internal.BucketMax {
			if highestBucketWeight > math.MaxInt64-weight {
				return errors.New("combined weight in highest modeled bucket overflows int64")
			}
			highestBucketWeight += weight
		}
	}
	return nil
}
