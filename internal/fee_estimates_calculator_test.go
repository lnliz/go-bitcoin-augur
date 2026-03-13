package internal

import (
	"math"
	"testing"
	"time"
)

func newTestCalculator() *FeeEstimatesCalculator {
	return NewFeeEstimatesCalculator([]float64{0.5, 0.95}, []float64{3.0, 12.0, 144.0})
}

func TestMineBlockRemovesWeightsFromHighestFeeBucketsFirst(t *testing.T) {
	calc := newTestCalculator()
	weights := make([]float64, 5)
	for i := range weights {
		weights[i] = 1000.0
	}
	blockSize := 2500.0

	remaining := calc.mineBlock(weights, blockSize)

	if remaining[0] != 0.0 {
		t.Errorf("expected remaining[0] = 0, got %f", remaining[0])
	}
	if remaining[1] != 0.0 {
		t.Errorf("expected remaining[1] = 0, got %f", remaining[1])
	}
	if remaining[2] != 500.0 {
		t.Errorf("expected remaining[2] = 500, got %f", remaining[2])
	}
	if remaining[3] != 1000.0 {
		t.Errorf("expected remaining[3] = 1000, got %f", remaining[3])
	}
	if remaining[4] != 1000.0 {
		t.Errorf("expected remaining[4] = 1000, got %f", remaining[4])
	}
}

func TestFindBestIndexWhenAllWeightsMined(t *testing.T) {
	calc := newTestCalculator()
	weights := make([]float64, 5)

	result := calc.findBestIndex(weights)

	if result != BucketMin {
		t.Errorf("expected BUCKET_MIN (%d), got %d", BucketMin, result)
	}
}

func TestFindBestIndexWhenNoWeightsFullyMined(t *testing.T) {
	calc := newTestCalculator()
	weights := make([]float64, 5)
	for i := range weights {
		weights[i] = 1000.0
	}

	result := calc.findBestIndex(weights)

	expected := BucketMax + 1
	if result != expected {
		t.Errorf("expected %d (BucketMax+1, indicating no valid estimate), got %d", expected, result)
	}
}

func TestFindBestIndexWithPartiallyMinedWeights(t *testing.T) {
	calc := newTestCalculator()
	weights := []float64{0.0, 0.0, 500.0, 1000.0, 1000.0}

	result := calc.findBestIndex(weights)

	expected := BucketMax - 1
	if result != expected {
		t.Errorf("expected %d, got %d", expected, result)
	}
}

func TestRunSimulationWithSimpleCase(t *testing.T) {
	calc := newTestCalculator()
	initialWeights := make([]float64, 5)
	addedWeights := make([]float64, 5)
	for i := range initialWeights {
		initialWeights[i] = 1000.0
		addedWeights[i] = 100.0
	}

	result := calc.runSimulation(initialWeights, addedWeights, 2, 2, 2500.0)

	if result >= BucketMax {
		t.Errorf("expected result < BUCKET_MAX, got %d", result)
	}
}

func TestRunSimulationWithZeroExpectedBlocks(t *testing.T) {
	calc := newTestCalculator()
	initialWeights := make([]float64, 5)
	addedWeights := make([]float64, 5)
	for i := range initialWeights {
		initialWeights[i] = 1000.0
		addedWeights[i] = 100.0
	}

	result := calc.runSimulation(initialWeights, addedWeights, 0, 2, 2500.0)

	expected := BucketMax + 1
	if result != expected {
		t.Errorf("expected %d (BucketMax+1, indicating no valid estimate), got %d", expected, result)
	}
}

func TestMineBlockHandlesBlockSizeLargerThanTotalWeights(t *testing.T) {
	calc := newTestCalculator()
	weights := make([]float64, 5)
	for i := range weights {
		weights[i] = 1000.0
	}
	blockSize := 6000.0

	remaining := calc.mineBlock(weights, blockSize)

	for i := range remaining {
		if remaining[i] != 0.0 {
			t.Errorf("expected remaining[%d] = 0, got %f", i, remaining[i])
		}
	}
}

func TestMineBlockHandlesBlockSizeSmallerThanAnyWeight(t *testing.T) {
	calc := newTestCalculator()
	weights := make([]float64, 5)
	for i := range weights {
		weights[i] = 1000.0
	}
	blockSize := 500.0

	remaining := calc.mineBlock(weights, blockSize)

	if remaining[0] != 500.0 {
		t.Errorf("expected remaining[0] = 500, got %f", remaining[0])
	}
	for i := 1; i < 5; i++ {
		if remaining[i] != 1000.0 {
			t.Errorf("expected remaining[%d] = 1000, got %f", i, remaining[i])
		}
	}
}

func TestFindBestIndexWhenLastBucketIsMined(t *testing.T) {
	calc := newTestCalculator()
	weights := []float64{0.0, 1000.0, 1000.0, 1000.0, 1000.0}

	result := calc.findBestIndex(weights)

	if result != BucketMax {
		t.Errorf("expected BUCKET_MAX (%d), got %d", BucketMax, result)
	}
}

func TestRunSimulationWithLargeBlockSize(t *testing.T) {
	calc := newTestCalculator()
	initialWeights := make([]float64, 5)
	addedWeights := make([]float64, 5)
	for i := range initialWeights {
		initialWeights[i] = 1000.0
		addedWeights[i] = 100.0
	}

	result := calc.runSimulation(initialWeights, addedWeights, 2, 2, 6000.0)

	if result != BucketMin {
		t.Errorf("expected BUCKET_MIN (%d), got %d", BucketMin, result)
	}
}

func TestRunSimulationWithIntermediateMiningCase(t *testing.T) {
	calc := newTestCalculator()
	initialWeights := make([]float64, 5)
	addedWeights := make([]float64, 5)
	for i := range initialWeights {
		initialWeights[i] = 4.0
		addedWeights[i] = 4.0
	}

	result := calc.runSimulation(initialWeights, addedWeights, 2, 2, 12.0)

	expected := BucketMax - 1
	if result != expected {
		t.Errorf("expected %d, got %d", expected, result)
	}
}

func TestRunSimulationReturnsMinimumFeeBucketWhenAllBucketsMined(t *testing.T) {
	calc := newTestCalculator()
	initialWeights := make([]float64, 5)
	addedWeights := make([]float64, 5)
	for i := range initialWeights {
		initialWeights[i] = 4.0
		addedWeights[i] = 4.0
	}

	result := calc.runSimulation(initialWeights, addedWeights, 3, 3, 100.0)

	if result != BucketMin {
		t.Errorf("expected BUCKET_MIN (%d), got %d", BucketMin, result)
	}
}

func TestNearMinimumFeeBucketNeverEmitsSub01SatPerVB(t *testing.T) {
	calc := newTestCalculator()
	nearMinimumFeeRate := 0.0998
	bucketIndex := int(math.Round(math.Log(nearMinimumFeeRate) * 100))

	if bucketIndex != BucketMin {
		t.Errorf("expected bucket index %d, got %d", BucketMin, bucketIndex)
	}

	bucketedWeights := map[int]int64{bucketIndex: 4_000_000}
	snapshot := NewMempoolSnapshotBuckets(time.Now(), 800000, bucketedWeights)

	zeroInflows := make([]float64, BucketArraySize)

	estimates := calc.GetFeeEstimates(snapshot.Buckets, zeroInflows, zeroInflows)

	expectedFeeRate := math.Exp(float64(bucketIndex) / 100.0)

	for blockIdx, row := range estimates {
		for probIdx, fee := range row {
			if fee == nil {
				t.Errorf("estimate[%d][%d] should not be nil", blockIdx, probIdx)
				continue
			}
			if *fee < 0.1 {
				t.Errorf("estimate[%d][%d] should be >= 0.1 sat/vB, got %f", blockIdx, probIdx, *fee)
			}
			if math.Abs(*fee-expectedFeeRate) > 1e-12 {
				t.Errorf("estimate[%d][%d] should match expected fee rate %f, got %f", blockIdx, probIdx, expectedFeeRate, *fee)
			}
		}
	}
}

func TestRunSimulationReturnsInvalidIndexWhenNoBucketsFullyMined(t *testing.T) {
	calc := newTestCalculator()
	initialWeights := make([]float64, 5)
	addedWeights := make([]float64, 5)
	for i := range initialWeights {
		initialWeights[i] = 4.0
		addedWeights[i] = 4.0
	}

	result := calc.runSimulation(initialWeights, addedWeights, 3, 3, 1.0)

	expected := BucketMax + 1
	if result != expected {
		t.Errorf("expected %d (BucketMax+1, indicating no valid estimate), got %d", expected, result)
	}
}

func TestGetExpectedBlocksMinedReturnsValidBlocks(t *testing.T) {
	calc := newTestCalculator()
	result := calc.getExpectedBlocksMined()

	expected := [][]float64{
		{3.0, 1.0},
		{12.0, 7.0},
		{144.0, 125.0},
	}

	for i := range expected {
		for j := range expected[i] {
			if result[i][j] != expected[i][j] {
				t.Errorf("expected[%d][%d] = %f, got %f", i, j, expected[i][j], result[i][j])
			}
		}
	}
}

func TestGetWeightedEstimatesReturns144BlockEstimateEqualsLongEstimate(t *testing.T) {
	calc := newTestCalculator()

	shortEstimates := make([][]float64, 3)
	longEstimates := make([][]float64, 3)
	for i := range shortEstimates {
		shortEstimates[i] = []float64{1.0, 1.0}
		longEstimates[i] = []float64{100.0, 100.0}
	}

	result := calc.getWeightedEstimates(shortEstimates, longEstimates)

	expected := [][]float64{
		{5.082031250000005, 5.082031250000005},
		{16.81250000000001, 16.81250000000001},
		{100.0, 100.0},
	}

	for i := range expected {
		for j := range expected[i] {
			if math.Abs(result[i][j]-expected[i][j]) > 1e-10 {
				t.Errorf("result[%d][%d] = %f, expected %f", i, j, result[i][j], expected[i][j])
			}
		}
	}
}

func TestGetWeightedEstimatesReturnsSameWhenAllEstimatesAreEqual(t *testing.T) {
	calc := newTestCalculator()

	shortEstimates := make([][]float64, 3)
	longEstimates := make([][]float64, 3)
	for i := range shortEstimates {
		shortEstimates[i] = []float64{100.0, 100.0}
		longEstimates[i] = []float64{100.0, 100.0}
	}

	result := calc.getWeightedEstimates(shortEstimates, longEstimates)

	for i := range result {
		for j := range result[i] {
			if result[i][j] != 100.0 {
				t.Errorf("result[%d][%d] = %f, expected 100.0", i, j, result[i][j])
			}
		}
	}
}

func TestGetFeeEstimatesReturnsNilWhenNoBucketsFullyMined(t *testing.T) {
	calc := NewFeeEstimatesCalculator([]float64{0.5, 0.95}, []float64{3.0})

	hugeWeights := make([]float64, BucketArraySize)
	for i := range hugeWeights {
		hugeWeights[i] = 100_000_000_000.0
	}

	zeroInflows := make([]float64, BucketArraySize)

	estimates := calc.GetFeeEstimates(hugeWeights, zeroInflows, zeroInflows)

	for blockIdx, row := range estimates {
		for probIdx, fee := range row {
			if fee != nil {
				t.Errorf("estimate[%d][%d] should be nil when no buckets fully mined, got %f", blockIdx, probIdx, *fee)
			}
		}
	}
}

type testTx struct {
	weight int64
	fee    int64
}

func (t testTx) FeeRate() float64 {
	return float64(t.fee) * 4.0 / float64(t.weight)
}

func (t testTx) GetWeight() int64 {
	return t.weight
}

func TestCreateFeeRateBucketsSingleTransaction(t *testing.T) {
	tx := testTx{weight: 400, fee: 200}
	buckets := CreateFeeRateBuckets([]testTx{tx})

	expectedBucketIndex := int(math.Round(math.Log(2.0) * 100))

	if _, ok := buckets[expectedBucketIndex]; !ok {
		t.Errorf("expected bucket %d to exist", expectedBucketIndex)
	}
	if buckets[expectedBucketIndex] != 400 {
		t.Errorf("expected weight 400, got %d", buckets[expectedBucketIndex])
	}
}

func TestCreateFeeRateBucketsMultipleTransactionsSameBucket(t *testing.T) {
	tx1 := testTx{weight: 400, fee: 200}
	tx2 := testTx{weight: 800, fee: 400}
	buckets := CreateFeeRateBuckets([]testTx{tx1, tx2})

	expectedBucketIndex := int(math.Round(math.Log(2.0) * 100))

	if len(buckets) != 1 {
		t.Errorf("expected 1 bucket, got %d", len(buckets))
	}
	if buckets[expectedBucketIndex] != 1200 {
		t.Errorf("expected weight 1200, got %d", buckets[expectedBucketIndex])
	}
}

func TestCreateFeeRateBucketsTransactionsDifferentBuckets(t *testing.T) {
	tx1 := testTx{weight: 400, fee: 200}
	tx2 := testTx{weight: 400, fee: 400}
	buckets := CreateFeeRateBuckets([]testTx{tx1, tx2})

	if len(buckets) != 2 {
		t.Errorf("expected 2 buckets, got %d", len(buckets))
	}
	for _, weight := range buckets {
		if weight != 400 {
			t.Errorf("expected weight 400, got %d", weight)
		}
	}
}

func TestCreateFeeRateBucketsExponentialFeeRates(t *testing.T) {
	transactions := []testTx{
		{weight: 400, fee: 100},
		{weight: 400, fee: 272},
		{weight: 400, fee: 739},
		{weight: 400, fee: 2009},
	}
	buckets := CreateFeeRateBuckets(transactions)

	expectedBuckets := map[int]int64{
		0:   400,
		100: 400,
		200: 400,
		300: 400,
	}

	if len(buckets) != len(expectedBuckets) {
		t.Errorf("expected %d buckets, got %d", len(expectedBuckets), len(buckets))
	}
	for k, v := range expectedBuckets {
		if buckets[k] != v {
			t.Errorf("bucket %d: expected %d, got %d", k, v, buckets[k])
		}
	}
}

func TestCreateFeeRateBucketsDuplicateFeeRates(t *testing.T) {
	transactions := []testTx{
		{weight: 400, fee: 100},
		{weight: 400, fee: 100},
		{weight: 400, fee: 272},
		{weight: 400, fee: 272},
	}
	buckets := CreateFeeRateBuckets(transactions)

	expectedBuckets := map[int]int64{
		0:   800,
		100: 800,
	}

	if len(buckets) != len(expectedBuckets) {
		t.Errorf("expected %d buckets, got %d", len(expectedBuckets), len(buckets))
	}
	for k, v := range expectedBuckets {
		if buckets[k] != v {
			t.Errorf("bucket %d: expected %d, got %d", k, v, buckets[k])
		}
	}
}

func TestCreateFeeRateBucketsVeryHighFeeRates(t *testing.T) {
	transactions := []testTx{
		{weight: 400, fee: 1_000_000_000},
	}
	buckets := CreateFeeRateBuckets(transactions)

	if _, ok := buckets[BucketMax]; !ok {
		t.Error("expected bucket at BUCKET_MAX to exist")
	}
	if buckets[BucketMax] != 400 {
		t.Errorf("expected weight 400, got %d", buckets[BucketMax])
	}
}

func TestCreateFeeRateBucketsVeryLowFeeRates(t *testing.T) {
	transactions := []testTx{
		{weight: 400, fee: 10},
		{weight: 400, fee: 20},
		{weight: 400, fee: 100},
	}
	buckets := CreateFeeRateBuckets(transactions)

	bucket01 := int(math.Round(math.Log(0.1) * 100))
	bucket02 := int(math.Round(math.Log(0.2) * 100))
	bucket1 := 0

	if len(buckets) != 3 {
		t.Errorf("expected 3 buckets, got %d", len(buckets))
	}
	if _, ok := buckets[bucket01]; !ok {
		t.Errorf("expected bucket %d to exist", bucket01)
	}
	if _, ok := buckets[bucket02]; !ok {
		t.Errorf("expected bucket %d to exist", bucket02)
	}
	if _, ok := buckets[bucket1]; !ok {
		t.Errorf("expected bucket %d to exist", bucket1)
	}
	if buckets[bucket01] != 400 {
		t.Errorf("expected weight 400, got %d", buckets[bucket01])
	}
	if buckets[bucket02] != 400 {
		t.Errorf("expected weight 400, got %d", buckets[bucket02])
	}
	if buckets[bucket1] != 400 {
		t.Errorf("expected weight 400, got %d", buckets[bucket1])
	}
}
