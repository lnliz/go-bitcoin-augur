package internal

import (
	"math"
	"testing"
)

func TestPoissonCDF(t *testing.T) {
	tests := []struct {
		k      int
		lambda float64
		want   float64
	}{
		{0, 1.0, 0.3678794411714423},
		{1, 1.0, 0.7357588823428847},
		{2, 1.0, 0.9196986029286058},
		{5, 3.0, 0.9160820579686966},
		{10, 5.0, 0.9863047314016171},
		{0, 0.5, 0.6065306597126334},
		{3, 2.0, 0.8571234604985472},
		{10, 10.0, 0.5830397501929856},
		{20, 10.0, 0.9984117393058061},
		{5, 12.0, 0.02034102941692837},
		{-1, 5.0, 0.0},
	}

	for _, tt := range tests {
		got := poissonCDF(tt.k, tt.lambda)
		if math.Abs(got-tt.want) > 1e-10 {
			t.Errorf("poissonCDF(%d, %f) = %f, want %f", tt.k, tt.lambda, got, tt.want)
		}
	}
}

func TestPoissonPMF(t *testing.T) {
	tests := []struct {
		k      int
		lambda float64
		want   float64
	}{
		{0, 1.0, 0.3678794411714423},
		{1, 1.0, 0.3678794411714423},
		{2, 1.0, 0.1839397205857211},
		{5, 3.0, 0.1008188134449936},
		{0, 5.0, 0.006737946999085467},
		{10, 10.0, 0.12511003572113336},
	}

	for _, tt := range tests {
		got := poissonPMF(tt.k, tt.lambda)
		if math.Abs(got-tt.want) > 1e-10 {
			t.Errorf("poissonPMF(%d, %f) = %f, want %f", tt.k, tt.lambda, got, tt.want)
		}
	}
}

func TestPoissonDegenerateAndInvalidMeans(t *testing.T) {
	if poissonCDF(0, 0) != 1 || poissonPMF(0, 0) != 1 || poissonPMF(1, 0) != 0 {
		t.Fatal("a zero-mean Poisson variable must equal zero with certainty")
	}
	for _, mean := range []float64{-1, math.NaN(), math.Inf(1), math.Inf(-1)} {
		if !math.IsNaN(poissonCDF(0, mean)) || !math.IsNaN(poissonPMF(0, mean)) {
			t.Errorf("invalid mean %g did not return NaN", mean)
		}
	}
}

func TestPoissonLargeMeanReference(t *testing.T) {
	// Reference values computed by summing exp(-lambda)*lambda^k/k! with
	// Python's decimal module at 420 decimal digits of precision.
	for _, tt := range []struct {
		k    int
		mean float64
		cdf  float64
		pmf  float64
	}{
		{143, 144, 0.488917849087363134, 0.0332259565155585238},
		{144, 144, 0.522143805602921657, 0.0332259565155585238},
		{154, 144, 0.810140094055207690, 0.0228819109323666156},
		{1007, 1008, 0.495811476722664652, 0.0125644619595143141},
		{1008, 1008, 0.508375938682178967, 0.0125644619595143141},
		{1018, 1008, 0.631333892471278679, 0.0118995113421566890},
	} {
		if got := poissonCDF(tt.k, tt.mean); math.Abs(got-tt.cdf) > 2e-12 {
			t.Errorf("CDF(%d, %g) = %.17g, want %.17g", tt.k, tt.mean, got, tt.cdf)
		}
		if got := poissonPMF(tt.k, tt.mean); math.Abs(got-tt.pmf)/tt.pmf > 2e-12 {
			t.Errorf("PMF(%d, %g) = %.17g, want %.17g", tt.k, tt.mean, got, tt.pmf)
		}
	}
}

func TestPoissonBlocksHighPrecisionReference(t *testing.T) {
	// Quantiles independently computed using the same 420-digit decimal
	// reference as above. Include tails too small for 1-CDF and quantiles
	// beyond the former four-times-the-mean search limit.
	// To reproduce: start mass[0] = Decimal(-mean).exp(), then generate
	// mass[n] = mass[n-1]*mean/n until n > mean and mass[n] < Decimal("1e-430").
	// Reverse-sum the masses to obtain P(X >= n); select the largest n with
	// tail[n] >= Decimal.from_float(probability), preserving binary64 inputs.
	probabilities := []float64{0.05, 0.5, 0.95, 1e-20, 1e-100,
		math.SmallestNonzeroFloat64, math.Nextafter(1, 0)}
	for _, tt := range []struct {
		mean float64
		want []int
	}{
		{1, []int{3, 1, 0, 20, 69, 177, 0}},
		{3, []int{6, 3, 1, 30, 92, 223, 0}},
		{12, []int{18, 12, 7, 56, 144, 319, 0}},
		{144, []int{164, 144, 125, 268, 467, 814, 57}},
		{1008, []int{1061, 1008, 956, 1316, 1755, 2457, 759}},
	} {
		for i, probability := range probabilities {
			if got := poissonBlocks(tt.mean, probability); got != tt.want[i] {
				t.Errorf("blocks(%g, %g) = %d, want %d", tt.mean, probability, got, tt.want[i])
			}
		}
	}
}

func BenchmarkPoissonBlocks(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		poissonBlocks(1008, 0.95)
	}
}
