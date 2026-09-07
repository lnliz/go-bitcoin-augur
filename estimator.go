package augur

import (
	"errors"
	"fmt"
	"math"
	"slices"
	"strings"
	"time"

	"github.com/lnliz/go-bitcoin-augur/internal"
)

// MaxBlockTarget is the largest supported horizon: one week at ten minutes per block.
const MaxBlockTarget = 1008

var (
	defaultBlockTargets  = [...]float64{3, 6, 9, 12, 18, 24, 36, 48, 72, 96, 144}
	defaultProbabilities = [...]float64{0.05, 0.20, 0.50, 0.80, 0.95}
	// DefaultBlockTargets lists the default horizons. Changing it does not configure estimators.
	DefaultBlockTargets = slices.Clone(defaultBlockTargets[:])
	// DefaultProbabilities lists the default confidence levels. Use WithProbabilities to customize them.
	DefaultProbabilities = slices.Clone(defaultProbabilities[:])
)

// FeeEstimator estimates fees from caller-supplied mempool history. Construct one
// with NewFeeEstimator. It performs no I/O and is safe for concurrent calculations
// provided callers do not mutate snapshots while they are being read.
type FeeEstimator struct {
	estimatorConfig
	calculator *internal.FeeEstimatesCalculator
}

type estimatorConfig struct {
	probabilities           []float64
	blockTargets            []float64
	shortTermWindowDuration time.Duration
	longTermWindowDuration  time.Duration
}

// FeeEstimatorOption configures a new estimator. Options apply only during
// construction, so they cannot invalidate an existing estimator's cached model.
type FeeEstimatorOption func(*estimatorConfig)

// WithProbabilities sets confidence levels in [0, 1].
func WithProbabilities(p []float64) FeeEstimatorOption {
	return func(fe *estimatorConfig) {
		fe.probabilities = slices.Clone(p)
	}
}

// WithBlockTargets sets whole-number horizons in [1, MaxBlockTarget].
func WithBlockTargets(bt []float64) FeeEstimatorOption {
	return func(fe *estimatorConfig) {
		fe.blockTargets = slices.Clone(bt)
	}
}

// WithShortTermWindow sets the recent inflow window (default 30 minutes).
func WithShortTermWindow(d time.Duration) FeeEstimatorOption {
	return func(fe *estimatorConfig) {
		fe.shortTermWindowDuration = d
	}
}

// WithLongTermWindow sets the historical inflow window (default 24 hours).
func WithLongTermWindow(d time.Duration) FeeEstimatorOption {
	return func(fe *estimatorConfig) {
		fe.longTermWindowDuration = d
	}
}

// NewFeeEstimator validates and copies its configuration. Targets must be whole
// numbers in [1, MaxBlockTarget]; probabilities must be finite and in [0, 1].
// A probability of zero makes no confirmation claim; one cannot yield an estimate.
// Repeated targets and probabilities are deduplicated and sorted.
func NewFeeEstimator(opts ...FeeEstimatorOption) (*FeeEstimator, error) {
	fe := &estimatorConfig{
		probabilities:           slices.Clone(defaultProbabilities[:]),
		blockTargets:            slices.Clone(defaultBlockTargets[:]),
		shortTermWindowDuration: 30 * time.Minute,
		longTermWindowDuration:  24 * time.Hour,
	}

	for _, opt := range opts {
		if opt == nil {
			return nil, errors.New("fee estimator option must not be nil")
		}
		opt(fe)
	}

	if len(fe.probabilities) == 0 {
		return nil, errors.New("at least one probability level must be provided")
	}
	if len(fe.blockTargets) == 0 {
		return nil, errors.New("at least one block target must be provided")
	}
	for _, p := range fe.probabilities {
		if math.IsNaN(p) || p < 0 || p > 1 {
			return nil, fmt.Errorf("probability must be finite and between 0 and 1: %v", p)
		}
	}
	for _, bt := range fe.blockTargets {
		if err := validateBlockTarget(bt, 1); err != nil {
			return nil, err
		}
	}

	if fe.shortTermWindowDuration <= 0 || fe.longTermWindowDuration < fe.shortTermWindowDuration {
		return nil, errors.New("windows must satisfy 0 < short-term window <= long-term window")
	}
	slices.Sort(fe.probabilities)
	fe.probabilities = slices.Compact(fe.probabilities)
	slices.Sort(fe.blockTargets)
	fe.blockTargets = slices.Compact(fe.blockTargets)

	return &FeeEstimator{
		estimatorConfig: *fe,
		calculator:      internal.NewFeeEstimatesCalculator(fe.probabilities, fe.blockTargets),
	}, nil
}

func validateBlockTarget(target float64, minimum float64) error {
	if math.IsNaN(target) || target < minimum || target > MaxBlockTarget || math.Trunc(target) != target {
		return fmt.Errorf("block target must be a whole number between %g and %d: %v", minimum, MaxBlockTarget, target)
	}
	return nil
}

// CalculateEstimates returns fees for all configured targets. Empty history has
// no estimates. One snapshot estimates the existing backlog with zero inflow;
// observations spanning a full long-term window are preferable.
func (fe *FeeEstimator) CalculateEstimates(snapshots []MempoolSnapshot) (FeeEstimate, error) {
	return fe.CalculateEstimatesForBlocks(snapshots, nil)
}

// CalculateEstimatesForBlocks calculates a whole-number target in [1, MaxBlockTarget].
// A target returns the same fees alone or as part of a configured target set.
// A nil target uses the configured targets. Snapshot timestamps must be distinct;
// inputs may be unordered and are never modified.
func (fe *FeeEstimator) CalculateEstimatesForBlocks(snapshots []MempoolSnapshot, numOfBlocks *float64) (FeeEstimate, error) {
	if fe == nil || fe.calculator == nil {
		return FeeEstimate{}, errors.New("fee estimator must be constructed with NewFeeEstimator")
	}
	if numOfBlocks != nil {
		if err := validateBlockTarget(*numOfBlocks, 1); err != nil {
			return FeeEstimate{}, err
		}
	}

	if len(snapshots) == 0 {
		return FeeEstimate{Estimates: make(map[int]BlockTarget), Timestamp: time.Now()}, nil
	}

	for i, snapshot := range snapshots {
		if err := snapshot.Validate(); err != nil {
			return FeeEstimate{}, fmt.Errorf("snapshot %d: %w", i, err)
		}
	}
	ordered := slices.Clone(snapshots)
	slices.SortFunc(ordered, func(a, b MempoolSnapshot) int { return a.Timestamp.Compare(b.Timestamp) })
	for i := 1; i < len(ordered); i++ {
		if ordered[i].Timestamp.Equal(ordered[i-1].Timestamp) {
			return FeeEstimate{}, fmt.Errorf("duplicate snapshot timestamp: %s", ordered[i].Timestamp.Format(time.RFC3339Nano))
		}
	}
	// Older observations cannot contribute to either window. Avoid expanding
	// their sparse bucket maps into dense simulation arrays.
	startTime := ordered[len(ordered)-1].Timestamp.Add(-fe.longTermWindowDuration)
	first := 0
	for ordered[first].Timestamp.Before(startTime) {
		first++
	}
	ordered = ordered[first:]

	bucketSnapshots := make([]internal.MempoolSnapshotBuckets, len(ordered))
	for i, s := range ordered {
		bucketSnapshots[i] = internal.NewMempoolSnapshotBuckets(s.Timestamp, s.BlockHeight, s.BucketedWeights)
		bucketSnapshots[i].BlockHash = strings.ToLower(s.BlockHash)
	}

	latest := bucketSnapshots[len(bucketSnapshots)-1].Buckets
	latestMempoolWeights := make([]float64, len(latest))
	for i, weight := range latest {
		latestMempoolWeights[i] = float64(weight)
	}
	shortTermInflows := internal.CalculateInflows(bucketSnapshots, fe.shortTermWindowDuration)
	longTermInflows := internal.CalculateInflows(bucketSnapshots, fe.longTermWindowDuration)

	targets := fe.blockTargets
	if numOfBlocks != nil {
		targets = []float64{*numOfBlocks}
	}
	feeMatrix := fe.calculator.GetFeeEstimatesForTargets(latestMempoolWeights, shortTermInflows, longTermInflows, targets)
	return fe.convertToFeeEstimate(feeMatrix, ordered[len(ordered)-1].Timestamp, targets), nil
}

func (fe *FeeEstimator) convertToFeeEstimate(feeMatrix [][]*float64, timestamp time.Time, targets []float64) FeeEstimate {
	estimates := make(map[int]BlockTarget)

	for blockIndex, meanBlocks := range targets {
		probs := make(map[float64]float64)
		for probIndex, prob := range fe.probabilities {
			if feeMatrix[blockIndex][probIndex] != nil {
				probs[prob] = *feeMatrix[blockIndex][probIndex]
			}
		}
		blockTarget := BlockTarget{
			Blocks:        int(meanBlocks),
			Probabilities: probs,
		}
		estimates[int(meanBlocks)] = blockTarget
	}

	return FeeEstimate{Estimates: estimates, Timestamp: timestamp}
}
