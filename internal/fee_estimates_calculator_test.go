package internal

import (
	"math"
	"math/rand"
	"slices"
	"testing"
)

func TestRunSimulationCapacityBoundaries(t *testing.T) {
	calc := NewFeeEstimatesCalculator([]float64{0.5}, []float64{3})
	tests := []struct {
		name     string
		initial  []float64
		inflow   []float64
		blocks   int
		target   float64
		capacity float64
		want     int
	}{
		{"no blocks", []float64{0}, []float64{0}, 0, 3, 10, BucketMax + 1},
		{"empty", []float64{0, 0}, []float64{0, 0}, 3, 3, 10, BucketMin},
		{"exact capacity", []float64{2, 4}, []float64{2, 6}, 3, 3, 10, BucketMin},
		{"first bucket remains", []float64{31, 0}, []float64{0, 0}, 3, 3, 10, BucketMax + 1},
		{"second bucket remains", []float64{10, 21}, []float64{0, 0}, 3, 3, 10, BucketMax},
		{"third bucket remains", []float64{10, 20, 1}, []float64{0, 0, 0}, 3, 3, 10, BucketMax - 1},
		{"constant inflow", []float64{4, 4, 4, 4, 4}, []float64{4, 4, 4, 4, 4}, 2, 2, 12, BucketMax - 1},
		{"fractional duration", []float64{0, 0}, []float64{7, 0}, 1, 1.5, 10, BucketMax + 1},
		{"overflow is congestion", []float64{math.MaxFloat64}, []float64{math.MaxFloat64}, 3, 3, 10, BucketMax + 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			initial, inflow := slices.Clone(tt.initial), slices.Clone(tt.inflow)
			got := calc.runSimulation(initial, inflow, tt.blocks, tt.target, tt.capacity)
			if got != tt.want {
				t.Fatalf("got bucket %d, want %d", got, tt.want)
			}
			if !slices.Equal(initial, tt.initial) || !slices.Equal(inflow, tt.inflow) {
				t.Fatal("simulation mutated its inputs")
			}
		})
	}
}

func TestRunSimulationMatchesBlockByBlockReference(t *testing.T) {
	calc := NewFeeEstimatesCalculator([]float64{0.5}, []float64{3})
	rng := rand.New(rand.NewSource(1))
	for trial := 0; trial < 5000; trial++ {
		blocks := 1 + rng.Intn(50)
		target := float64(1 + rng.Intn(50))
		capacity := float64(1 + rng.Intn(100_000))
		initial := make([]float64, 1+rng.Intn(50))
		inflow := make([]float64, len(initial))
		for i := range initial {
			initial[i] = float64(rng.Intn(200_000))
			// Integer per-block arrivals make exact-capacity comparisons
			// independent of accumulated floating-point roundoff.
			inflow[i] = float64(rng.Intn(1000) * blocks)
		}
		got := calc.runSimulation(initial, inflow, blocks, target, capacity)
		want := simulateBlocksReference(initial, inflow, blocks, target, capacity)
		if got != want {
			t.Fatalf("trial %d: got %d, want %d (blocks %d, target %g, capacity %g)",
				trial, got, want, blocks, target, capacity)
		}
	}
}

// Deliberately simulate every block independently of the production formula.
func simulateBlocksReference(initial, inflow []float64, blocks int, target, capacity float64) int {
	weights := slices.Clone(initial)
	for block := 0; block < blocks; block++ {
		remainingCapacity := capacity
		for i := range weights {
			weights[i] += inflow[i] * target / float64(blocks)
			mined := math.Min(remainingCapacity, weights[i])
			weights[i] -= mined
			remainingCapacity -= mined
		}
	}
	for i, weight := range weights {
		if weight > 0 {
			return BucketMax - i + 1
		}
	}
	return BucketMin
}

func TestExpectedBlocksMined(t *testing.T) {
	calc := NewFeeEstimatesCalculator([]float64{0.5, 0.95}, []float64{3, 12, 144})
	want := [][]int{{3, 1}, {12, 7}, {144, 125}}
	for i, row := range calc.expectedBlocksMined {
		if !slices.Equal(row, want[i]) {
			t.Errorf("target %g: got %v, want %v", calc.blockTargets[i], row, want[i])
		}
	}
}

func TestWeightedEstimates(t *testing.T) {
	calc := NewFeeEstimatesCalculator([]float64{0.5}, []float64{3, 12, 144, 288, 1008})
	short := [][]float64{{1}, {1}, {1}, {1}, {1}}
	long := [][]float64{{100}, {100}, {100}, {100}, {100}}
	want := []float64{5.08203125, 16.8125, 100, 100, 100}
	got := calc.getWeightedEstimates(short, long)
	for i, row := range got {
		if math.Abs(row[0]-want[i]) > 1e-12 {
			t.Errorf("target %g: got %g, want %g", calc.blockTargets[i], row[0], want[i])
		}
	}
}

func TestWeightedEstimatesPreserveMissingProjections(t *testing.T) {
	calc := NewFeeEstimatesCalculator([]float64{0.5}, []float64{3, 12, 144, 288})
	short := [][]float64{{BucketMax + 1}, {BucketMin}, {BucketMax + 1}, {BucketMin}}
	long := [][]float64{{BucketMin}, {BucketMax + 1}, {BucketMin}, {BucketMax + 1}}
	want := []float64{BucketMax + 1, BucketMax + 1, BucketMin, BucketMax + 1}
	got := calc.getWeightedEstimates(short, long)
	for i, row := range got {
		if row[0] != want[i] {
			t.Errorf("target %g: got %g, want %g", calc.blockTargets[i], row[0], want[i])
		}
	}
}

func TestWeightedEstimatesPreserveIdenticalBoundaryBuckets(t *testing.T) {
	for target := 1; target <= 144; target++ {
		calc := &FeeEstimatesCalculator{blockTargets: []float64{float64(target)}}
		boundaries := [][]float64{{BucketMin, BucketMax}}
		got := calc.getWeightedEstimates(boundaries, boundaries)
		if !slices.Equal(got[0], boundaries[0]) {
			t.Fatalf("target %d: identical boundaries changed from %v to %v", target, boundaries[0], got[0])
		}
	}
}

func TestFeeEstimatesBoundaries(t *testing.T) {
	calc := NewFeeEstimatesCalculator([]float64{0, 0.5, 1}, []float64{3})
	zero := make([]float64, BucketArraySize)
	for _, tt := range []struct {
		name  string
		index int
		want  float64
	}{
		{"empty pool uses floor", -1, math.Exp(float64(BucketMin) / 100)},
		{"highest valid fee is included", 1, math.Exp(float64(BucketMax) / 100)},
		{"highest bucket congested", 0, math.Inf(1)},
	} {
		t.Run(tt.name, func(t *testing.T) {
			weights := make([]float64, BucketArraySize)
			if tt.index >= 0 {
				weights[tt.index] = 5 * BlockSizeWeightUnits
			}
			got := calc.GetFeeEstimates(weights, zero, zero)[0]
			if got[0] == nil || *got[0] != math.Exp(float64(BucketMin)/100) {
				t.Errorf("zero confidence must return floor, got %v", got[0])
			}
			if got[2] != nil {
				t.Errorf("certain confirmation is impossible, got %g", *got[2])
			}
			if math.IsInf(tt.want, 1) {
				if got[1] != nil {
					t.Errorf("congested ceiling must be unavailable, got %g", *got[1])
				}
			} else if got[1] == nil || *got[1] != tt.want {
				t.Errorf("got %v, want %g", got[1], tt.want)
			}
		})
	}
}

func TestFeeEstimatesDoNotBlendUnavailableProjectionIntoValidRate(t *testing.T) {
	calc := NewFeeEstimatesCalculator([]float64{0.5}, []float64{3})
	zero := make([]float64, BucketArraySize)
	long := make([]float64, BucketArraySize)
	long[0] = 5_000_000
	got := calc.GetFeeEstimates(zero, zero, long)
	if got[0][0] != nil {
		t.Fatalf("unavailable long-term projection became a valid fee: %g", *got[0][0])
	}
}

func TestFeeEstimatesMonotoneAndFinite(t *testing.T) {
	calc := NewFeeEstimatesCalculator([]float64{0, 0.05, 0.5, 0.95, 1}, []float64{1, 3, 12, 144, 288, 1008})
	rng := rand.New(rand.NewSource(2))
	for trial := 0; trial < 100; trial++ {
		weights := make([]float64, BucketArraySize)
		short := make([]float64, BucketArraySize)
		long := make([]float64, BucketArraySize)
		for i := range weights {
			weights[i] = rng.Float64() * 1_000_000
			short[i] = rng.Float64() * 10_000
			long[i] = rng.Float64() * 10_000
		}
		got := calc.GetFeeEstimates(weights, short, long)
		for i, row := range got {
			for j, fee := range row {
				value := feeOrInfinity(fee)
				if fee != nil && (math.IsNaN(value) || math.IsInf(value, 0) || value < 0.1) {
					t.Fatalf("invalid fee %g at [%d][%d]", value, i, j)
				}
				if i > 0 && value > feeOrInfinity(got[i-1][j]) {
					t.Fatalf("fee increases with target at [%d][%d]", i, j)
				}
				if j > 0 && value < feeOrInfinity(row[j-1]) {
					t.Fatalf("fee decreases with confidence at [%d][%d]", i, j)
				}
			}
		}
	}
}

func feeOrInfinity(fee *float64) float64 {
	if fee == nil {
		return math.Inf(1)
	}
	return *fee
}

func TestCalculatorCopiesConfiguration(t *testing.T) {
	probabilities, targets := []float64{0.5}, []float64{3}
	calc := NewFeeEstimatesCalculator(probabilities, targets)
	probabilities[0], targets[0] = 1, 1008
	if calc.probabilities[0] != 0.5 || calc.blockTargets[0] != 3 {
		t.Fatal("calculator configuration aliases caller slices")
	}
}

func BenchmarkFeeEstimates(b *testing.B) {
	calc := NewFeeEstimatesCalculator([]float64{0.05, 0.2, 0.5, 0.8, 0.95},
		[]float64{3, 6, 9, 12, 18, 24, 36, 48, 72, 96, 144})
	weights, inflows := make([]float64, BucketArraySize), make([]float64, BucketArraySize)
	for i := range weights {
		weights[i], inflows[i] = 100_000, 1_000
	}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		calc.GetFeeEstimates(weights, inflows, inflows)
	}
}
