package augur

import (
	"errors"
	"math"
	"sort"
	"time"

	"github.com/lnliz/go-bitcoin-augur/internal"
)

var (
	DefaultBlockTargets  = []float64{3, 6, 9, 12, 18, 24, 36, 48, 72, 96, 144}
	DefaultProbabilities = []float64{0.05, 0.20, 0.50, 0.80, 0.95}
)

type FeeEstimator struct {
	probabilities           []float64
	blockTargets            []float64
	shortTermWindowDuration time.Duration
	longTermWindowDuration  time.Duration
	calculator              *internal.FeeEstimatesCalculator
}

type FeeEstimatorOption func(*FeeEstimator)

func WithProbabilities(p []float64) FeeEstimatorOption {
	return func(fe *FeeEstimator) {
		fe.probabilities = p
	}
}

func WithBlockTargets(bt []float64) FeeEstimatorOption {
	return func(fe *FeeEstimator) {
		fe.blockTargets = bt
	}
}

func WithShortTermWindow(d time.Duration) FeeEstimatorOption {
	return func(fe *FeeEstimator) {
		fe.shortTermWindowDuration = d
	}
}

func WithLongTermWindow(d time.Duration) FeeEstimatorOption {
	return func(fe *FeeEstimator) {
		fe.longTermWindowDuration = d
	}
}

func NewFeeEstimator(opts ...FeeEstimatorOption) (*FeeEstimator, error) {
	fe := &FeeEstimator{
		probabilities:           DefaultProbabilities,
		blockTargets:            DefaultBlockTargets,
		shortTermWindowDuration: 30 * time.Minute,
		longTermWindowDuration:  24 * time.Hour,
	}

	for _, opt := range opts {
		opt(fe)
	}

	if len(fe.probabilities) == 0 {
		return nil, errors.New("at least one probability level must be provided")
	}
	if len(fe.blockTargets) == 0 {
		return nil, errors.New("at least one block target must be provided")
	}
	for _, p := range fe.probabilities {
		if p < 0 || p > 1 {
			return nil, errors.New("all probabilities must be between 0.0 and 1.0")
		}
	}
	for _, bt := range fe.blockTargets {
		if bt <= 0 {
			return nil, errors.New("all block targets must be positive")
		}
	}

	fe.calculator = internal.NewFeeEstimatesCalculator(fe.probabilities, fe.blockTargets)
	return fe, nil
}

func (fe *FeeEstimator) CalculateEstimates(snapshots []MempoolSnapshot) (FeeEstimate, error) {
	return fe.CalculateEstimatesForBlocks(snapshots, nil)
}

func (fe *FeeEstimator) CalculateEstimatesForBlocks(snapshots []MempoolSnapshot, numOfBlocks *float64) (FeeEstimate, error) {
	if numOfBlocks != nil && *numOfBlocks < 3.0 {
		return FeeEstimate{}, errors.New("numOfBlocks must be at least 3 if specified")
	}

	if len(snapshots) == 0 {
		return FeeEstimate{Estimates: make(map[int]BlockTarget), Timestamp: time.Now()}, nil
	}

	ordered := make([]MempoolSnapshot, len(snapshots))
	copy(ordered, snapshots)
	sort.Slice(ordered, func(i, j int) bool {
		return ordered[i].Timestamp.Before(ordered[j].Timestamp)
	})

	simdSnapshots := make([]internal.MempoolSnapshotBuckets, len(ordered))
	for i, s := range ordered {
		simdSnapshots[i] = internal.NewMempoolSnapshotBuckets(s.Timestamp, s.BlockHeight, s.BucketedWeights)
	}

	latestMempoolWeights := simdSnapshots[len(simdSnapshots)-1].Buckets
	shortTermInflows := internal.CalculateInflows(simdSnapshots, fe.shortTermWindowDuration)
	longTermInflows := internal.CalculateInflows(simdSnapshots, fe.longTermWindowDuration)

	var calculator *internal.FeeEstimatesCalculator
	var targets []float64

	if numOfBlocks != nil {
		calculator = internal.NewFeeEstimatesCalculator(fe.probabilities, []float64{*numOfBlocks})
		targets = []float64{*numOfBlocks}
	} else {
		calculator = fe.calculator
		targets = fe.blockTargets
	}

	feeMatrix := calculator.GetFeeEstimates(latestMempoolWeights, shortTermInflows, longTermInflows)
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

type BlockTarget struct {
	Blocks        int
	Probabilities map[float64]float64
}

func (bt BlockTarget) GetFeeRate(probability float64) (float64, bool) {
	rate, ok := bt.Probabilities[probability]
	return rate, ok
}

type FeeEstimate struct {
	Estimates map[int]BlockTarget
	Timestamp time.Time
}

func (fe FeeEstimate) GetFeeRate(targetBlocks int, probability float64) (float64, bool) {
	target, ok := fe.Estimates[targetBlocks]
	if !ok {
		return 0, false
	}
	return target.GetFeeRate(probability)
}

func (fe FeeEstimate) GetEstimatesForTarget(targetBlocks int) (BlockTarget, bool) {
	target, ok := fe.Estimates[targetBlocks]
	return target, ok
}

func (fe FeeEstimate) GetNearestBlockTarget(targetBlocks int) (int, bool) {
	if len(fe.Estimates) == 0 {
		return 0, false
	}
	if _, ok := fe.Estimates[targetBlocks]; ok {
		return targetBlocks, true
	}

	nearest := 0
	minDiff := math.MaxInt
	for k := range fe.Estimates {
		diff := int(math.Abs(float64(k - targetBlocks)))
		if diff < minDiff {
			minDiff = diff
			nearest = k
		}
	}
	return nearest, true
}

func (fe FeeEstimate) GetAvailableBlockTargets() []int {
	targets := make([]int, 0, len(fe.Estimates))
	for k := range fe.Estimates {
		targets = append(targets, k)
	}
	sort.Ints(targets)
	return targets
}

func (fe FeeEstimate) GetAvailableConfidenceLevels() []float64 {
	seen := make(map[float64]bool)
	for _, bt := range fe.Estimates {
		for p := range bt.Probabilities {
			seen[p] = true
		}
	}
	levels := make([]float64, 0, len(seen))
	for p := range seen {
		levels = append(levels, p)
	}
	sort.Float64s(levels)
	return levels
}
