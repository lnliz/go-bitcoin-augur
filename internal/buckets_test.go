package internal

import (
	"math"
	"testing"
)

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
	buckets, err := CreateFeeRateBuckets([]testTx{tx})
	if err != nil {
		t.Fatal(err)
	}

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
	buckets, err := CreateFeeRateBuckets([]testTx{tx1, tx2})
	if err != nil {
		t.Fatal(err)
	}

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
	buckets, err := CreateFeeRateBuckets([]testTx{tx1, tx2})
	if err != nil {
		t.Fatal(err)
	}

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
	buckets, err := CreateFeeRateBuckets(transactions)
	if err != nil {
		t.Fatal(err)
	}

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
	buckets, err := CreateFeeRateBuckets(transactions)
	if err != nil {
		t.Fatal(err)
	}

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
	buckets, err := CreateFeeRateBuckets(transactions)
	if err != nil {
		t.Fatal(err)
	}

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
	buckets, err := CreateFeeRateBuckets(transactions)
	if err != nil {
		t.Fatal(err)
	}

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

type feeRateWeight struct {
	feeRate float64
	weight  int64
}

func (p feeRateWeight) FeeRate() float64 { return p.feeRate }
func (p feeRateWeight) GetWeight() int64 { return p.weight }

func TestCreateFeeRateBucketsRejectsInvalidTransactions(t *testing.T) {
	tests := []struct {
		name string
		pair feeRateWeight
	}{
		{"zero weight", feeRateWeight{1, 0}},
		{"negative weight", feeRateWeight{1, -1}},
		{"negative fee rate", feeRateWeight{-1, 400}},
		{"NaN fee rate", feeRateWeight{math.NaN(), 400}},
		{"positive infinite fee rate", feeRateWeight{math.Inf(1), 400}},
		{"negative infinite fee rate", feeRateWeight{math.Inf(-1), 400}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buckets, err := CreateFeeRateBuckets([]feeRateWeight{tt.pair})
			if err == nil || buckets != nil {
				t.Fatalf("got buckets %v, error %v; want nil buckets and error", buckets, err)
			}
		})
	}
}

func TestCreateFeeRateBucketsOmitsZeroFee(t *testing.T) {
	buckets, err := CreateFeeRateBuckets([]testTx{{weight: 400, fee: 0}})
	if err != nil {
		t.Fatal(err)
	}
	if len(buckets) != 0 {
		t.Fatalf("zero fee produced buckets %v", buckets)
	}
}

func TestCreateFeeRateBucketsRejectsOverflow(t *testing.T) {
	pairs := []feeRateWeight{{1, math.MaxInt64}, {1, 1}}
	if buckets, err := CreateFeeRateBuckets(pairs); err == nil || buckets != nil {
		t.Fatalf("got buckets %v, error %v; want nil buckets and overflow error", buckets, err)
	}
	pairs[0].weight--
	buckets, err := CreateFeeRateBuckets(pairs)
	if err != nil {
		t.Fatal(err)
	}
	if buckets[0] != math.MaxInt64 {
		t.Fatalf("got bucket weight %d, want %d", buckets[0], int64(math.MaxInt64))
	}
}

func TestCreateFeeRateBucketsExtremeFiniteRates(t *testing.T) {
	buckets, err := CreateFeeRateBuckets([]feeRateWeight{
		{math.SmallestNonzeroFloat64, 100},
		{math.MaxFloat64, 200},
	})
	if err != nil {
		t.Fatal(err)
	}
	if buckets[BucketMax] != 200 {
		t.Fatalf("got highest bucket weight %d, want 200", buckets[BucketMax])
	}
	if len(buckets) != 2 {
		t.Fatalf("got %d buckets, want 2", len(buckets))
	}
	for bucket, weight := range buckets {
		if bucket != BucketMax && (bucket >= BucketMin || weight != 100) {
			t.Errorf("small positive fee: got bucket %d weight %d, want below minimum and weight 100", bucket, weight)
		}
	}
}

func TestCalculateBucketIndexMatchesUpstreamTieRounding(t *testing.T) {
	// These rates produce exact half-integer bucket coordinates in float64.
	// Kotlin rounds them to the nearest even integer.
	for _, tt := range []struct {
		feeRate float64
		want    int
	}{
		{math.Exp(0.125), 12},
		{math.Exp(1.125), 112},
		{math.Exp(4.125), 412},
	} {
		if got := calculateBucketIndex(tt.feeRate); got != tt.want {
			t.Errorf("bucket(%g) = %d, want %d", tt.feeRate, got, tt.want)
		}
	}
}

func TestCalculateBucketIndexSubnormalFeeRate(t *testing.T) {
	// SmallestNonzeroFloat64 is 2^-1074; round(100 * -1074 * ln(2)) = -74444.
	if got := calculateBucketIndex(math.SmallestNonzeroFloat64); got != -74444 {
		t.Fatalf("smallest positive fee rate bucket = %d, want -74444", got)
	}
}
