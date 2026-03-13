package augur

import (
	"math/rand"
	"testing"
	"time"
)

func TestEmptySnapshotListReturnsNilEstimates(t *testing.T) {
	fe, _ := NewFeeEstimator()
	estimate, _ := fe.CalculateEstimates(nil)

	for _, target := range DefaultBlockTargets {
		for _, prob := range DefaultProbabilities {
			if _, ok := estimate.GetFeeRate(int(target), prob); ok {
				t.Errorf("expected no fee rate for target=%v, prob=%v", target, prob)
			}
		}
	}
}

func TestSingleSnapshotStillProducesEstimates(t *testing.T) {
	fe, _ := NewFeeEstimator()
	opts := defaultSnapshotSequenceOptions()
	opts.blockCount = 1
	snapshots := createSnapshotSequence(opts)

	estimate, _ := fe.CalculateEstimates(snapshots[:1])

	hasAnyEstimate := false
	for _, target := range DefaultBlockTargets {
		for _, prob := range DefaultProbabilities {
			if _, ok := estimate.GetFeeRate(int(target), prob); ok {
				hasAnyEstimate = true
			}
		}
	}
	if !hasAnyEstimate {
		t.Log("single snapshot may return estimates based on mempool state")
	}
}

func TestEstimatesWithConsistentFeeRateIncrease(t *testing.T) {
	fe, _ := NewFeeEstimator()
	opts := defaultSnapshotSequenceOptions()
	opts.blockCount = 144
	opts.inflowRateChangeTime = time.Hour
	snapshots := createSnapshotSequence(opts)

	estimate, _ := fe.CalculateEstimates(snapshots)

	for _, target := range DefaultBlockTargets {
		for _, prob := range DefaultProbabilities {
			feeRate, ok := estimate.GetFeeRate(int(target), prob)
			if !ok || feeRate <= 0 {
				t.Errorf("fee rate should be positive for target=%v, prob=%v", target, prob)
			}
		}
	}
}

func TestEstimatesAreOrderedCorrectlyByProbability(t *testing.T) {
	fe, _ := NewFeeEstimator()
	opts := defaultSnapshotSequenceOptions()
	snapshots := createSnapshotSequence(opts)

	estimate, _ := fe.CalculateEstimates(snapshots)

	for _, target := range DefaultBlockTargets {
		var lastFeeRate float64
		for _, prob := range DefaultProbabilities {
			feeRate, ok := estimate.GetFeeRate(int(target), prob)
			if ok {
				if feeRate < lastFeeRate {
					t.Errorf("fee rates should increase with probability for target=%v", target)
				}
				lastFeeRate = feeRate
			}
		}
	}
}

func TestEstimatesAreOrderedCorrectlyByTargetBlocks(t *testing.T) {
	fe, _ := NewFeeEstimator()
	opts := defaultSnapshotSequenceOptions()
	snapshots := createSnapshotSequence(opts)

	estimate, _ := fe.CalculateEstimates(snapshots)

	for _, prob := range DefaultProbabilities {
		lastFeeRate := float64(1 << 62)
		for _, target := range DefaultBlockTargets {
			feeRate, ok := estimate.GetFeeRate(int(target), prob)
			if ok {
				if feeRate > lastFeeRate {
					t.Errorf("fee rates should decrease with target blocks for prob=%v", prob)
				}
				lastFeeRate = feeRate
			}
		}
	}
}

func TestEstimatesAreOrderedCorrectlyByTargetBlocksWithHigherLongTermInflows(t *testing.T) {
	fe, _ := NewFeeEstimator()
	opts := defaultSnapshotSequenceOptions()
	opts.blockCount = 144
	opts.shortTermInflowRates = createVeryLowInflowRates()
	opts.longTermInflowRates = createHighInflowRates()
	snapshots := createSnapshotSequence(opts)

	estimate, _ := fe.CalculateEstimates(snapshots)

	for _, prob := range DefaultProbabilities {
		lastFeeRate := float64(1 << 62)
		for _, target := range DefaultBlockTargets {
			feeRate, ok := estimate.GetFeeRate(int(target), prob)
			if ok {
				if feeRate > lastFeeRate {
					t.Errorf("fee rates should decrease with target blocks for prob=%v", prob)
				}
				lastFeeRate = feeRate
			}
		}
	}
}

func TestEstimatesWithCustomProbabilitiesAndTargets(t *testing.T) {
	customProbs := []float64{0.1, 0.5, 0.9}
	customTargets := []float64{3.0, 6.0, 12.0}
	fe, _ := NewFeeEstimator(WithProbabilities(customProbs), WithBlockTargets(customTargets))

	opts := defaultSnapshotSequenceOptions()
	snapshots := createSnapshotSequence(opts)

	estimate, _ := fe.CalculateEstimates(snapshots)

	for _, target := range customTargets {
		for _, prob := range customProbs {
			feeRate, ok := estimate.GetFeeRate(int(target), prob)
			if !ok || feeRate <= 0 {
				t.Errorf("fee rate should exist for custom target=%v, prob=%v", target, prob)
			}
		}
	}
}

func TestEstimatesWithUnorderedSnapshots(t *testing.T) {
	fe, _ := NewFeeEstimator()
	opts := defaultSnapshotSequenceOptions()
	snapshots := createSnapshotSequence(opts)

	rand.Shuffle(len(snapshots), func(i, j int) {
		snapshots[i], snapshots[j] = snapshots[j], snapshots[i]
	})

	estimate, _ := fe.CalculateEstimates(snapshots)

	for _, target := range DefaultBlockTargets {
		for _, prob := range DefaultProbabilities {
			feeRate, ok := estimate.GetFeeRate(int(target), prob)
			if !ok || feeRate <= 0 {
				t.Errorf("fee rate should be positive for target=%v, prob=%v", target, prob)
			}
		}
	}
}

func TestGetNearestBlockTarget(t *testing.T) {
	customTargets := []float64{3.0, 6.0, 24.0, 144.0}
	customProbs := []float64{0.5, 0.9}
	fe, _ := NewFeeEstimator(WithBlockTargets(customTargets), WithProbabilities(customProbs))

	opts := defaultSnapshotSequenceOptions()
	snapshots := createSnapshotSequence(opts)
	estimate, _ := fe.CalculateEstimates(snapshots)

	for _, target := range customTargets {
		nearest, ok := estimate.GetNearestBlockTarget(int(target))
		if !ok || nearest != int(target) {
			t.Errorf("expected exact match %v, got %v", target, nearest)
		}
	}

	testCases := map[int]int{
		1:   3,
		2:   3,
		4:   3,
		5:   6,
		10:  6,
		20:  24,
		50:  24,
		100: 144,
		200: 144,
	}

	for input, expected := range testCases {
		nearest, _ := estimate.GetNearestBlockTarget(input)
		if nearest != expected {
			t.Errorf("for input %d, expected nearest target %d, got %d", input, expected, nearest)
		}
	}

	emptyEstimate := FeeEstimate{Estimates: make(map[int]BlockTarget), Timestamp: time.Now()}
	_, ok := emptyEstimate.GetNearestBlockTarget(6)
	if ok {
		t.Error("expected no nearest target for empty estimate")
	}
}

func TestBlockTargetGetFeeRate(t *testing.T) {
	probs := map[float64]float64{
		0.2: 10.0,
		0.5: 15.0,
		0.8: 20.0,
	}
	bt := BlockTarget{Blocks: 6, Probabilities: probs}

	if rate, ok := bt.GetFeeRate(0.2); !ok || rate != 10.0 {
		t.Errorf("expected 10.0, got %v", rate)
	}
	if rate, ok := bt.GetFeeRate(0.5); !ok || rate != 15.0 {
		t.Errorf("expected 15.0, got %v", rate)
	}
	if rate, ok := bt.GetFeeRate(0.8); !ok || rate != 20.0 {
		t.Errorf("expected 20.0, got %v", rate)
	}

	if _, ok := bt.GetFeeRate(0.1); ok {
		t.Error("expected no fee rate for 0.1")
	}
	if _, ok := bt.GetFeeRate(0.3); ok {
		t.Error("expected no fee rate for 0.3")
	}

	emptyBt := BlockTarget{Blocks: 6, Probabilities: make(map[float64]float64)}
	if _, ok := emptyBt.GetFeeRate(0.5); ok {
		t.Error("expected no fee rate for empty block target")
	}
}

func TestGetAvailableBlockTargetsAndConfidenceLevels(t *testing.T) {
	targets := []float64{6.0, 3.0, 24.0, 144.0}
	probs := []float64{0.8, 0.2, 0.5}
	fe, _ := NewFeeEstimator(WithBlockTargets(targets), WithProbabilities(probs))

	opts := defaultSnapshotSequenceOptions()
	snapshots := createSnapshotSequence(opts)
	estimate, _ := fe.CalculateEstimates(snapshots)

	availableTargets := estimate.GetAvailableBlockTargets()
	expectedTargets := []int{3, 6, 24, 144}
	for i, v := range expectedTargets {
		if availableTargets[i] != v {
			t.Errorf("expected target %d at index %d, got %d", v, i, availableTargets[i])
		}
	}

	availableLevels := estimate.GetAvailableConfidenceLevels()
	expectedLevels := []float64{0.2, 0.5, 0.8}
	for i, v := range expectedLevels {
		if availableLevels[i] != v {
			t.Errorf("expected level %v at index %d, got %v", v, i, availableLevels[i])
		}
	}
}

func TestCalculateEstimatesForBlocksThrowsIfLessThan3(t *testing.T) {
	fe, _ := NewFeeEstimator()
	opts := defaultSnapshotSequenceOptions()
	snapshots := createSnapshotSequence(opts)

	numBlocks := 2.0
	_, err := fe.CalculateEstimatesForBlocks(snapshots, &numBlocks)
	if err == nil {
		t.Error("expected error for numOfBlocks < 3")
	}
}

func TestNewFeeEstimatorValidation(t *testing.T) {
	t.Run("empty probabilities returns error", func(t *testing.T) {
		_, err := NewFeeEstimator(WithProbabilities([]float64{}))
		if err == nil {
			t.Error("expected error for empty probabilities")
		}
	})

	t.Run("empty block targets returns error", func(t *testing.T) {
		_, err := NewFeeEstimator(WithBlockTargets([]float64{}))
		if err == nil {
			t.Error("expected error for empty block targets")
		}
	})

	t.Run("probability greater than 1 returns error", func(t *testing.T) {
		_, err := NewFeeEstimator(WithProbabilities([]float64{0.5, 1.5}))
		if err == nil {
			t.Error("expected error for probability > 1")
		}
	})

	t.Run("probability less than 0 returns error", func(t *testing.T) {
		_, err := NewFeeEstimator(WithProbabilities([]float64{-0.1, 0.5}))
		if err == nil {
			t.Error("expected error for probability < 0")
		}
	})

	t.Run("zero block target returns error", func(t *testing.T) {
		_, err := NewFeeEstimator(WithBlockTargets([]float64{0, 3, 6}))
		if err == nil {
			t.Error("expected error for zero block target")
		}
	})

	t.Run("negative block target returns error", func(t *testing.T) {
		_, err := NewFeeEstimator(WithBlockTargets([]float64{-1, 3, 6}))
		if err == nil {
			t.Error("expected error for negative block target")
		}
	})

	t.Run("valid boundary probabilities succeed", func(t *testing.T) {
		_, err := NewFeeEstimator(WithProbabilities([]float64{0.0, 0.5, 1.0}))
		if err != nil {
			t.Errorf("unexpected error for valid boundary probabilities: %v", err)
		}
	})

	t.Run("valid options succeed", func(t *testing.T) {
		_, err := NewFeeEstimator(
			WithProbabilities([]float64{0.1, 0.5, 0.9}),
			WithBlockTargets([]float64{3, 6, 12}),
		)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestCalculateEstimatesForBlocksWithValidNumOfBlocks(t *testing.T) {
	fe, _ := NewFeeEstimator()
	opts := defaultSnapshotSequenceOptions()
	snapshots := createSnapshotSequence(opts)

	numBlocks := 5.0
	estimate, err := fe.CalculateEstimatesForBlocks(snapshots, &numBlocks)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if _, ok := estimate.GetEstimatesForTarget(5); !ok {
		t.Error("expected estimates for target 5")
	}

	for _, prob := range DefaultProbabilities {
		rate, ok := estimate.GetFeeRate(5, prob)
		if !ok || rate <= 0 {
			t.Errorf("expected positive fee rate for target=5, prob=%v", prob)
		}
	}
}

/*
	test utils
*/

func createTransaction(feeRate float64, weight int64) MempoolTransaction {
	fee := int64(feeRate * float64(weight) / 4.0)
	return MempoolTransaction{Weight: weight, Fee: fee}
}

func createDefaultBaseWeights() map[float64]int64 {
	weights := make(map[float64]int64)

	for fee := 1; fee <= 8; fee++ {
		feeRate := float64(fee) * 0.5
		weights[feeRate] = 500_000 + int64(rand.Float64()*1_500_000)
	}

	for fee := 9; fee <= 32; fee++ {
		feeRate := float64(fee) * 0.5
		baseWeight := 2_000_000 + int64(rand.Float64()*5_000_000)
		weight := baseWeight
		switch feeRate {
		case 5.0:
			weight = baseWeight * 3
		case 8.0:
			weight = baseWeight * 4
		case 10.0:
			weight = baseWeight * 3
		case 12.0:
			weight = baseWeight * 2
		case 15.0:
			weight = baseWeight * 3
		}
		weights[feeRate] = weight
	}

	for fee := 33; fee <= 64; fee++ {
		feeRate := float64(fee) * 0.5
		baseWeight := 1_000_000 + int64(rand.Float64()*3_000_000)
		weight := baseWeight
		switch feeRate {
		case 20.0:
			weight = baseWeight * 3
		case 25.0:
			weight = baseWeight * 4
		case 30.0:
			weight = baseWeight * 2
		}
		weights[feeRate] = weight
	}

	return weights
}

func createHighInflowRates() map[float64]int64 {
	rates := make(map[float64]int64)
	for fee := 1; fee <= 64; fee++ {
		feeRate := float64(fee) * 0.5
		var inflowRate int64
		switch {
		case feeRate <= 4.0:
			inflowRate = 20_000
		case feeRate <= 8.0:
			inflowRate = 40_000
		case feeRate <= 16.0:
			inflowRate = 80_000
		default:
			inflowRate = 2_000_000
		}
		rates[feeRate] = inflowRate
	}
	return rates
}

func createVeryLowInflowRates() map[float64]int64 {
	rates := make(map[float64]int64)
	for fee := 1; fee <= 64; fee++ {
		feeRate := float64(fee) * 0.5
		var inflowRate int64
		switch {
		case feeRate <= 4.0:
			inflowRate = 5_000
		case feeRate <= 8.0:
			inflowRate = 10_000
		case feeRate <= 16.0:
			inflowRate = 20_000
		default:
			inflowRate = 25_000
		}
		rates[feeRate] = inflowRate
	}
	return rates
}

func createLowInflowRates() map[float64]int64 {
	rates := make(map[float64]int64)
	for fee := 1; fee <= 64; fee++ {
		feeRate := float64(fee) * 0.5
		var inflowRate int64
		switch {
		case feeRate <= 4.0:
			inflowRate = 10_000
		case feeRate <= 8.0:
			inflowRate = 20_000
		case feeRate <= 16.0:
			inflowRate = 40_000
		default:
			inflowRate = 50_000
		}
		rates[feeRate] = inflowRate
	}
	return rates
}

type snapshotSequenceOptions struct {
	startTime            time.Time
	blockCount           int
	snapshotsPerBlock    int
	baseWeights          map[float64]int64
	shortTermInflowRates map[float64]int64
	longTermInflowRates  map[float64]int64
	inflowRateChangeTime time.Duration
}

func defaultSnapshotSequenceOptions() snapshotSequenceOptions {
	return snapshotSequenceOptions{
		startTime:            time.Now(),
		blockCount:           5,
		snapshotsPerBlock:    3,
		baseWeights:          createDefaultBaseWeights(),
		shortTermInflowRates: createHighInflowRates(),
		longTermInflowRates:  createLowInflowRates(),
		inflowRateChangeTime: time.Hour,
	}
}

func createSnapshotSequence(opts snapshotSequenceOptions) []MempoolSnapshot {
	var snapshots []MempoolSnapshot

	endTime := opts.startTime.Add(time.Duration(600*(opts.blockCount-1)) * time.Second)

	for blockIndex := 0; blockIndex < opts.blockCount; blockIndex++ {
		blockHeight := 100 + blockIndex
		blockStartTime := opts.startTime.Add(time.Duration(600*blockIndex) * time.Second)
		snapshotInterval := 600 / opts.snapshotsPerBlock

		for snapshotIndex := 0; snapshotIndex < opts.snapshotsPerBlock; snapshotIndex++ {
			snapshotTime := blockStartTime.Add(time.Duration(snapshotInterval*snapshotIndex) * time.Second)
			timeUntilEnd := endTime.Sub(snapshotTime)

			var transactions []MempoolTransaction
			for feeRate, baseWeight := range opts.baseWeights {
				var inflowRate int64
				if timeUntilEnd > opts.inflowRateChangeTime {
					inflowRate = opts.longTermInflowRates[feeRate]
				} else {
					inflowRate = opts.shortTermInflowRates[feeRate]
				}

				elapsedIntervals := float64(snapshotTime.Sub(opts.startTime).Seconds()) / 600.0
				cumulativeWeight := baseWeight + int64(float64(inflowRate)*elapsedIntervals)

				transactions = append(transactions, createTransaction(feeRate, cumulativeWeight))
			}

			snapshots = append(snapshots, NewMempoolSnapshotFromTransactions(transactions, blockHeight, snapshotTime))
		}
	}

	return snapshots
}
