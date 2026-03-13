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
